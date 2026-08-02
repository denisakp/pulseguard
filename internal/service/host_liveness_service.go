package service

import (
	"context"
	"encoding/json"
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

const hostLivenessPageSize = 500

// hostLister is the subset of the host repository the liveness scan needs.
type hostLister interface {
	List(ctx context.Context, limit, offset int) ([]*domain.Host, error)
}

// feedEmitter is the subset of the notification feed used to raise alerts.
type feedEmitter interface {
	Emit(ctx context.Context, n domain.EmittedNotification) error
}

// HostLivenessService periodically scans agent-backed hosts and raises a feed
// notification (plus optional SMTP) when one goes offline past the freshness
// threshold, and a recovery notification when it returns. At most one offline
// alert + one recovery per offline episode, tracked durably in host_alert_state
// so it is flap-safe and restart-safe (spec 083, US1).
type HostLivenessService struct {
	hosts            hostLister
	state            port.HostAlertStateRepository
	feed             feedEmitter
	channels         port.NotificationChannelRepository
	freshness        time.Duration
	externalDelivery bool
	now              func() time.Time
	wg               sync.WaitGroup
}

func NewHostLivenessService(
	hosts hostLister,
	state port.HostAlertStateRepository,
	feed feedEmitter,
	channels port.NotificationChannelRepository,
	freshness time.Duration,
	externalDelivery bool,
) *HostLivenessService {
	if freshness <= 0 {
		freshness = 45 * time.Second
	}
	return &HostLivenessService{
		hosts:            hosts,
		state:            state,
		feed:             feed,
		channels:         channels,
		freshness:        freshness,
		externalDelivery: externalDelivery,
		now:              func() time.Time { return time.Now().UTC() },
	}
}

// SetNow overrides the clock (tests).
func (s *HostLivenessService) SetNow(fn func() time.Time) { s.now = fn }

// Detect runs one scan cycle. Fire-and-forget: per-host failures are logged and
// skipped so the whole fleet is always evaluated; it never returns an error that
// would abort the recurring loop.
func (s *HostLivenessService) Detect(ctx context.Context) error {
	now := s.now()
	var offlineAlerts, recoveries int

	for offset := 0; ; offset += hostLivenessPageSize {
		hosts, err := s.hosts.List(ctx, hostLivenessPageSize, offset)
		if err != nil {
			slog.Error("host liveness: list hosts failed", "error", err, "offset", offset)
			return nil // swallow — try again next cycle
		}
		for _, h := range hosts {
			if h.LastSeenAt == nil {
				continue // never reported — not agent-backed yet
			}
			if s.reconcile(ctx, h, h.IsOnline(now, s.freshness), now) {
				if h.IsOnline(now, s.freshness) {
					recoveries++
				} else {
					offlineAlerts++
				}
			}
		}
		if len(hosts) < hostLivenessPageSize {
			break
		}
	}

	if offlineAlerts > 0 || recoveries > 0 {
		slog.Info("host liveness scan complete", "offline_alerts", offlineAlerts, "recoveries", recoveries)
	}
	return nil
}

// reconcile applies the state machine for one host. Returns true when it raised
// an alert or recovery this cycle.
func (s *HostLivenessService) reconcile(ctx context.Context, h *domain.Host, online bool, now time.Time) bool {
	stored, err := s.state.Get(ctx, h.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		slog.Error("host liveness: get alert state failed", "host_id", h.ID, "error", err)
		return false
	}
	if errors.Is(err, repository.ErrNotFound) {
		stored = nil
	}

	switch {
	case online:
		// Recovery only if we had alerted an offline episode.
		if stored != nil && stored.State == domain.HostAlertStateOffline {
			alerted := stored.Alerted
			s.upsert(ctx, h.ID, domain.HostAlertStateOnline, nil, false, now)
			if alerted {
				s.emitRecovery(ctx, h)
				return true
			}
		}
		return false
	default: // offline
		switch {
		case stored == nil || stored.State == domain.HostAlertStateOnline:
			// First offline observation — record, do not alert (anti-flap / +1 cycle).
			t := now
			s.upsert(ctx, h.ID, domain.HostAlertStateOffline, &t, false, now)
			return false
		case !stored.Alerted:
			// Confirmed offline across a full cycle → alert once.
			s.upsert(ctx, h.ID, domain.HostAlertStateOffline, stored.OfflineSince, true, now)
			s.emitOffline(ctx, h, stored.OfflineSince)
			return true
		default:
			return false // already alerted
		}
	}
}

func (s *HostLivenessService) upsert(ctx context.Context, hostID, state string, offlineSince *time.Time, alerted bool, now time.Time) {
	if err := s.state.Upsert(ctx, &domain.HostAlertState{
		HostID:       hostID,
		State:        state,
		OfflineSince: offlineSince,
		Alerted:      alerted,
		UpdatedAt:    now,
	}); err != nil {
		slog.Error("host liveness: upsert alert state failed", "host_id", hostID, "error", err)
	}
}

func (s *HostLivenessService) emitOffline(ctx context.Context, h *domain.Host, since *time.Time) {
	title := fmt.Sprintf("Host %s went offline", h.Name)
	body := fmt.Sprintf("%s stopped reporting metrics and is now offline.", h.Name)
	s.emit(ctx, h, domain.NotificationSeverityError, title, body, since)
}

func (s *HostLivenessService) emitRecovery(ctx context.Context, h *domain.Host) {
	title := fmt.Sprintf("Host %s is back online", h.Name)
	body := fmt.Sprintf("%s resumed reporting metrics.", h.Name)
	s.emit(ctx, h, domain.NotificationSeveritySuccess, title, body, nil)
}

// emit records the feed notification (always) and, when external delivery is
// enabled, sends to the oldest SMTP channel. Both are best-effort.
func (s *HostLivenessService) emit(ctx context.Context, h *domain.Host, severity, title, body string, since *time.Time) {
	deepLink := "/hosts/" + h.ID
	payload := map[string]any{"host_id": h.ID}
	if since != nil {
		payload["offline_since"] = since.UTC().Format(time.RFC3339)
	}
	raw, _ := json.Marshal(payload)

	if err := s.feed.Emit(ctx, domain.EmittedNotification{
		Category:   domain.NotificationCategoryHost,
		Severity:   severity,
		Title:      title,
		DeepLink:   &deepLink,
		Payload:    raw,
		OccurredAt: s.now(),
	}); err != nil {
		slog.Error("host liveness: feed emit failed", "host_id", h.ID, "error", err)
	}

	if !s.externalDelivery {
		return
	}
	err := deliverOperatorSMTP(ctx, s.channels, notifier.OperatorNotification{Title: title, Body: body})
	if err != nil && !errors.Is(err, ErrNoSMTPChannel) {
		slog.Error("host liveness: smtp delivery failed", "host_id", h.ID, "error", err)
	}
}

// Start launches the scan as a recurring goroutine on the given interval. The
// first cycle runs after one interval. Blocks until ctx is cancelled.
func (s *HostLivenessService) Start(ctx context.Context, interval time.Duration) error {
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
				slog.Info("host liveness scanner stopped")
				return
			case <-ticker.C:
				if err := s.Detect(ctx); err != nil {
					slog.Error("host liveness scan cycle error", "error", err)
				}
			}
		}
	}()
	slog.Info("host liveness scanner started", "interval", interval)
	return nil
}

// Wait blocks until the background goroutine has exited (tests).
func (s *HostLivenessService) Wait() { s.wg.Wait() }
