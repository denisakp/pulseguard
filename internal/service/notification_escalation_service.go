package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/pkg/notifier"
)

// digestListLimit bounds how many items are itemized in the digest body; the
// count reflects the true total.
const digestListLimit = 20

// escalationFeed is the feed-repository subset the escalation scan needs.
type escalationFeed interface {
	ListUnreadForEscalation(ctx context.Context, categories []string, cutoff time.Time, limit int) ([]*domain.FeedNotification, error)
	CountUnreadForEscalation(ctx context.Context, categories []string, cutoff time.Time) (int, error)
}

// NotificationEscalationService periodically emails a grouped digest to the
// oldest SMTP channel when actionable feed notifications stay unread past a
// configured age, deduplicated by a durable high-water mark so it is not re-sent
// while the unread set is unchanged (spec 083, US2).
type NotificationEscalationService struct {
	feed       escalationFeed
	state      port.NotificationEscalationStateRepository
	categories []string
	unreadAge  time.Duration
	deliver    func(ctx context.Context, op notifier.OperatorNotification) error
	now        func() time.Time
	wg         sync.WaitGroup
}

func NewNotificationEscalationService(
	feed escalationFeed,
	state port.NotificationEscalationStateRepository,
	channels port.NotificationChannelRepository,
	unreadAge time.Duration,
) *NotificationEscalationService {
	if unreadAge <= 0 {
		unreadAge = 30 * time.Minute
	}
	return &NotificationEscalationService{
		feed:       feed,
		state:      state,
		categories: domain.EscalationCategories,
		unreadAge:  unreadAge,
		deliver: func(ctx context.Context, op notifier.OperatorNotification) error {
			return deliverOperatorSMTP(ctx, channels, op)
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

// SetNow overrides the clock (tests).
func (s *NotificationEscalationService) SetNow(fn func() time.Time) { s.now = fn }

// SetDeliver overrides the digest delivery function (tests).
func (s *NotificationEscalationService) SetDeliver(fn func(ctx context.Context, op notifier.OperatorNotification) error) {
	s.deliver = fn
}

// Detect runs one escalation cycle. Fail-safe: any error is logged and swallowed;
// it never aborts the recurring loop or affects monitoring.
func (s *NotificationEscalationService) Detect(ctx context.Context) error {
	now := s.now()
	cutoff := now.Add(-s.unreadAge)

	items, err := s.feed.ListUnreadForEscalation(ctx, s.categories, cutoff, digestListLimit)
	if err != nil {
		slog.Error("escalation: list unread failed", "error", err)
		return nil
	}
	if len(items) == 0 {
		return nil // nothing qualifies
	}
	// items are newest-first; the newest occurred_at is the dedup high-water mark.
	maxOccurred := items[0].OccurredAt

	stored, err := s.state.Get(ctx)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		slog.Error("escalation: get state failed", "error", err)
		return nil
	}
	if stored != nil && stored.WatermarkOccurredAt != nil && !maxOccurred.After(*stored.WatermarkOccurredAt) {
		return nil // already digested this state — nothing newer since
	}

	total, err := s.feed.CountUnreadForEscalation(ctx, s.categories, cutoff)
	if err != nil {
		slog.Error("escalation: count unread failed", "error", err)
		total = len(items)
	}

	op := buildDigest(items, total)
	if err := s.deliver(ctx, op); err != nil {
		if errors.Is(err, ErrNoSMTPChannel) {
			slog.Info("escalation: no SMTP channel configured — digest skipped", "unread", total)
		} else {
			slog.Error("escalation: digest delivery failed", "error", err)
		}
		return nil // do not advance the watermark — retry next cycle
	}

	digestAt := now
	if err := s.state.Upsert(ctx, &domain.NotificationEscalationState{
		WatermarkOccurredAt: &maxOccurred,
		LastDigestAt:        &digestAt,
	}); err != nil {
		slog.Error("escalation: upsert state failed", "error", err)
	}
	slog.Info("escalation digest sent", "unread", total)
	return nil
}

func buildDigest(items []*domain.FeedNotification, total int) notifier.OperatorNotification {
	plural := "s"
	if total == 1 {
		plural = ""
	}
	list := make([]string, 0, len(items)+1)
	for _, n := range items {
		list = append(list, n.Title)
	}
	if total > len(items) {
		list = append(list, fmt.Sprintf("…and %d more", total-len(items)))
	}
	return notifier.OperatorNotification{
		Title: fmt.Sprintf("%d unread notification%s need attention", total, plural),
		Body:  "You have Ogoune notifications that have been unread for a while. Sign in to review them.",
		Items: list,
	}
}

// Start launches the escalation scan as a recurring goroutine. The first cycle
// runs after one interval. Blocks until ctx is cancelled.
func (s *NotificationEscalationService) Start(ctx context.Context, interval time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("notification escalation scanner stopped")
				return
			case <-ticker.C:
				if err := s.Detect(ctx); err != nil {
					slog.Error("escalation cycle error", "error", err)
				}
			}
		}
	}()
	slog.Info("notification escalation scanner started", "interval", interval, "unread_age", s.unreadAge)
	return nil
}

// Wait blocks until the background goroutine has exited (tests).
func (s *NotificationEscalationService) Wait() { s.wg.Wait() }
