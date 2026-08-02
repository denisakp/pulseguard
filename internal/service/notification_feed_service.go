package service

import (
	"context"
	"errors"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
)

// ErrNotificationNotFound is returned when a notification id does not exist.
var ErrNotificationNotFound = errors.New("notification not found")

// NotificationFeedService is the in-app notification feed.
// Distinct from NotificationService (outbound channel dispatch).
type NotificationFeedService struct {
	repo port.NotificationFeedRepository
}

func NewNotificationFeedService(repo port.NotificationFeedRepository) *NotificationFeedService {
	return &NotificationFeedService{repo: repo}
}

// ListForUser returns the page of notifications visible to userID plus the total count.
func (s *NotificationFeedService) ListForUser(ctx context.Context, userID string, category *string, limit, offset int) ([]*domain.FeedNotification, int64, error) {
	items, err := s.repo.ListForUser(ctx, userID, category, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountForUser(ctx, userID, category)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// MarkRead marks a single notification read; ErrNotificationNotFound if absent.
func (s *NotificationFeedService) MarkRead(ctx context.Context, id string) error {
	n, err := s.repo.MarkRead(ctx, id, time.Now())
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

// MarkAllRead marks all visible unread notifications read up to `before` (default now).
func (s *NotificationFeedService) MarkAllRead(ctx context.Context, userID string, before time.Time) (int64, error) {
	if before.IsZero() {
		before = time.Now()
	}
	return s.repo.MarkAllRead(ctx, userID, before, time.Now())
}

// Emit persists a notification from a producer. Errors are returned to the caller,
// but producers MUST treat emission as fire-and-forget (log, never propagate).
func (s *NotificationFeedService) Emit(ctx context.Context, n domain.EmittedNotification) error {
	occurred := n.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now()
	}
	_, err := s.repo.Create(ctx, &domain.FeedNotification{
		UserID:      n.UserID,
		Category:    n.Category,
		Severity:    n.Severity,
		Title:       n.Title,
		Description: n.Description,
		DeepLink:    n.DeepLink,
		Payload:     n.Payload,
		OccurredAt:  occurred,
	})
	return err
}

// DeleteOlderThan prunes notifications older than cutoff (retention job).
func (s *NotificationFeedService) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.repo.DeleteOlderThan(ctx, cutoff)
}
