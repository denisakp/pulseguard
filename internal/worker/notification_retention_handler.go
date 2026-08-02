package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// TypeNotificationRetention is the Asynq task type for the daily notification
// feed retention job.
const TypeNotificationRetention = "notification:retention"

// notificationPruner is the narrow slice of NotificationFeedRepository the job needs.
type notificationPruner interface {
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// NotificationRetentionHandler deletes feed notifications older than the
// retention window. Runs once per day in both runtime modes.
type NotificationRetentionHandler struct {
	repo          notificationPruner
	retentionDays int
}

// NewNotificationRetentionHandler creates the handler. retentionDays <= 0 is
// clamped to 90 to avoid pruning the entire feed.
func NewNotificationRetentionHandler(repo notificationPruner, retentionDays int) *NotificationRetentionHandler {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return &NotificationRetentionHandler{repo: repo, retentionDays: retentionDays}
}

// ProcessTask deletes notifications with occurred_at older than now-retentionDays.
func (h *NotificationRetentionHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	cutoff := time.Now().AddDate(0, 0, -h.retentionDays)
	deleted, err := h.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		slog.Error("notification retention: delete failed", "error", err, "cutoff", cutoff)
		return err
	}
	slog.Info("notification retention: pruned", "deleted", deleted, "cutoff", cutoff, "retention_days", h.retentionDays)
	return nil
}
