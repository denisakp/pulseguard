package worker

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
)

// TypeHostMetricsRetention is the Asynq task type for the daily host-metrics
// retention job (spec 079): decimate old samples, then purge beyond the window.
const TypeHostMetricsRetention = "host:metrics:retention"

// hostMetricsRetentionRunner is the slice of HostMetricsService the job needs.
type hostMetricsRetentionRunner interface {
	RunRetention(ctx context.Context) error
}

// HostMetricsRetentionHandler runs the daily host-metrics retention pass in both
// runtime modes (TimingWheel + Asynq).
type HostMetricsRetentionHandler struct {
	svc hostMetricsRetentionRunner
}

func NewHostMetricsRetentionHandler(svc hostMetricsRetentionRunner) *HostMetricsRetentionHandler {
	return &HostMetricsRetentionHandler{svc: svc}
}

// ProcessTask decimates then purges host metric samples per the configured
// windows. Errors are returned so the scheduler can log/retry.
func (h *HostMetricsRetentionHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	if err := h.svc.RunRetention(ctx); err != nil {
		slog.Error("host:metrics:retention failed", "error", err)
		return err
	}
	slog.Info("host:metrics:retention completed")
	return nil
}
