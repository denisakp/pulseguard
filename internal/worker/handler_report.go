package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// TypeReportCheck is the Asynq task type for the monthly report catch-up scan.
const TypeReportCheck = "report:check"

// reportGenerator is the slice of *service.ReportService the worker depends on.
type reportGenerator interface {
	GenerateAndDeliver(ctx context.Context, period string) error
}

// ReportTaskHandler runs the monthly report catch-up scan. On each
// tick it generates the previous completed calendar month if it has no report
// row yet (idempotency + enablement live in the service). A missed run (instance
// down on the 1st) self-heals on the next tick/startup.
type ReportTaskHandler struct {
	reports reportGenerator
}

func NewReportTaskHandler(reports reportGenerator) *ReportTaskHandler {
	return &ReportTaskHandler{reports: reports}
}

// previousMonthPeriod returns the YYYY-MM of the calendar month before now (UTC).
func previousMonthPeriod(now time.Time) string {
	firstOfThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return firstOfThis.AddDate(0, 0, -1).Format("2006-01")
}

// ProcessTask generates+delivers the previous month's report. It never returns
// an error that would abort the scheduled job (delivery failures are recorded
// by the service; unexpected errors are logged and swallowed).
func (h *ReportTaskHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	period := previousMonthPeriod(time.Now().UTC())
	if err := h.reports.GenerateAndDeliver(ctx, period); err != nil {
		slog.Error("report:check failed", "period", period, "error", err)
	}
	return nil
}
