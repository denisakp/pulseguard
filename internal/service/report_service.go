package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/denisakp/ogoune/pkg/notifier"
)

// Report service domain errors.
var (
	ErrReportNotFound   = errors.New("report: not found")
	ErrReportValidation = errors.New("report: validation failed")
)

// ReportService owns the monthly-report configuration, generation, and delivery.
type ReportService struct {
	settings  port.ReportSettingsRepository
	history   port.ReportHistoryRepository
	resources port.ResourceRepository
	uptime    port.UptimeDailyAggRepository
	incidents port.IncidentRepository
	channels  port.NotificationChannelRepository

	// deliver sends the report email. Defaults to smtpDeliver (SMTP notification
	// channel transport); overridable in tests to exercise delivered/failed paths.
	deliver func(ctx context.Context, recipient, period string, uptimePct float64, incidentCount int, downtime int64, breakdown []domain.ReportBreakdownLine) error
}

func NewReportService(
	settings port.ReportSettingsRepository,
	history port.ReportHistoryRepository,
	resources port.ResourceRepository,
	uptime port.UptimeDailyAggRepository,
	incidents port.IncidentRepository,
	channels port.NotificationChannelRepository,
) *ReportService {
	s := &ReportService{
		settings:  settings,
		history:   history,
		resources: resources,
		uptime:    uptime,
		incidents: incidents,
		channels:  channels,
	}
	s.deliver = s.smtpDeliver
	return s
}

// GetSettings returns the config, synthesizing a safe default when unsaved.
func (s *ReportService) GetSettings(ctx context.Context) (*domain.ReportSettings, error) {
	got, err := s.settings.Get(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &domain.ReportSettings{
				Enabled:        false,
				RecipientEmail: "",
				Schedule:       domain.ReportScheduleMonthly1st,
				Scope:          domain.ReportScopeAllResources,
			}, nil
		}
		return nil, err
	}
	return got, nil
}

// SaveSettings validates and upserts the config. Enabled requires a recipient.
func (s *ReportService) SaveSettings(ctx context.Context, in *domain.ReportSettings) (*domain.ReportSettings, error) {
	in.RecipientEmail = strings.TrimSpace(in.RecipientEmail)
	if in.Schedule == "" {
		in.Schedule = domain.ReportScheduleMonthly1st
	}
	if in.Scope == "" {
		in.Scope = domain.ReportScopeAllResources
	}
	if in.Enabled {
		if in.RecipientEmail == "" || !strings.Contains(in.RecipientEmail, "@") {
			return nil, fmt.Errorf("%w: recipientEmail is required and must be a valid email when enabled", ErrReportValidation)
		}
	}
	if in.Schedule != domain.ReportScheduleMonthly1st {
		return nil, fmt.Errorf("%w: unsupported schedule", ErrReportValidation)
	}
	if in.Scope != domain.ReportScopeAllResources {
		return nil, fmt.Errorf("%w: unsupported scope", ErrReportValidation)
	}
	return s.settings.Upsert(ctx, in)
}

// ListHistory returns recorded reports newest-first, limit clamped [1,50], default 6.
func (s *ReportService) ListHistory(ctx context.Context, limit int) ([]*domain.ReportHistory, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 50 {
		limit = 50
	}
	return s.history.ListRecent(ctx, limit)
}

// GeneratePreview computes a summary for the current in-progress month WITHOUT persisting.
func (s *ReportService) GeneratePreview(ctx context.Context) (*domain.ReportHistory, error) {
	now := time.Now().UTC()
	period := now.Format("2006-01")
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	uptimePct, incidentCount, downtime, breakdown, err := s.aggregate(ctx, from, now)
	if err != nil {
		return nil, err
	}
	return &domain.ReportHistory{
		Period:          period,
		SentAt:          now,
		Status:          domain.ReportStatusPending,
		UptimePct:       uptimePct,
		IncidentCount:   incidentCount,
		DowntimeSeconds: downtime,
		RecipientEmail:  settings.RecipientEmail,
		Breakdown:       breakdown,
	}, nil
}

// GenerateAndDeliver generates and emails the report for a period (YYYY-MM),
// idempotent per period. Called by the scheduled worker. Never returns an error
// that should abort the caller on a delivery failure (logs + records failed).
func (s *ReportService) GenerateAndDeliver(ctx context.Context, period string) error {
	if _, err := s.history.FindByPeriod(ctx, period); err == nil {
		return nil // already generated — idempotent
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}

	from, to, err := monthBounds(period)
	if err != nil {
		return err
	}

	uptimePct, incidentCount, downtime, breakdown, err := s.aggregate(ctx, from, to)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	status := domain.ReportStatusDelivered
	if sendErr := s.deliver(ctx, settings.RecipientEmail, period, uptimePct, incidentCount, downtime, breakdown); sendErr != nil {
		slog.Error("report delivery failed", "period", period, "error", sendErr)
		status = domain.ReportStatusFailed
	}

	_, createErr := s.history.Create(ctx, &domain.ReportHistory{
		Period:          period,
		SentAt:          now,
		Status:          status,
		UptimePct:       uptimePct,
		IncidentCount:   incidentCount,
		DowntimeSeconds: downtime,
		RecipientEmail:  settings.RecipientEmail,
		Breakdown:       breakdown,
	})
	if createErr != nil {
		if errors.Is(createErr, repository.ErrDuplicate) {
			slog.Info("report already generated concurrently", "period", period)
			return nil
		}
		return createErr
	}

	if status == domain.ReportStatusDelivered {
		settings.LastSentAt = &now
		if _, err := s.settings.Upsert(ctx, settings); err != nil {
			slog.Error("failed to advance report last_sent_at", "period", period, "error", err)
		}
	}
	return nil
}

// aggregate gathers resources + daily aggregates + period incidents and folds
// them into the report totals (reuses pre-aggregated data only).
func (s *ReportService) aggregate(ctx context.Context, from, to time.Time) (uptimePct float64, incidentCount int, downtimeSec int64, breakdown []domain.ReportBreakdownLine, err error) {
	resources, err := s.resources.List(ctx, 10000, 0)
	if err != nil {
		return 0, 0, 0, nil, fmt.Errorf("list resources: %w", err)
	}
	ids := make([]string, 0, len(resources))
	for _, r := range resources {
		ids = append(ids, r.ID)
	}

	var aggs []*domain.UptimeDailyAgg
	if len(ids) > 0 {
		aggs, err = s.uptime.FindRange(ctx, ids, from, to)
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf("uptime range: %w", err)
		}
	}

	// Fetch the period's incidents once; bucket by resource for the breakdown.
	incs, total, err := s.incidents.ListIncidentsByFilter(ctx, dynquery.IncidentFilter{From: &from, To: &to}, 1, 10000)
	if err != nil {
		return 0, 0, 0, nil, fmt.Errorf("list incidents: %w", err)
	}
	byResource := make(map[string]int, len(incs))
	for _, i := range incs {
		byResource[i.ResourceID]++
	}

	uptimePct, downtimeSec, breakdown = aggregatePeriod(resources, aggs, byResource)
	return uptimePct, total, downtimeSec, breakdown, nil
}

// smtpDeliver resolves an SMTP notification channel and sends the report to recipient.
func (s *ReportService) smtpDeliver(ctx context.Context, recipient, period string, uptimePct float64, incidentCount int, downtime int64, breakdown []domain.ReportBreakdownLine) error {
	_, cfg, err := resolveOldestSMTP(ctx, s.channels)
	if err != nil {
		return err
	}
	n := notifier.NewSMTPNotifier(recipient, cfg.Sender, cfg.Host, string(cfg.Port), smtpUser(cfg), cfg.Password)

	lines := make([]notifier.ReportBreakdownLine, 0, len(breakdown))
	for _, b := range breakdown {
		lines = append(lines, notifier.ReportBreakdownLine{Name: b.Name, UptimePct: b.UptimePct, Incidents: b.Incidents})
	}
	return n.Send(ctx, notifier.NotificationPayload{Report: &notifier.ReportNotification{
		Period:          period,
		Recipient:       recipient,
		UptimePct:       uptimePct,
		IncidentCount:   incidentCount,
		DowntimeSeconds: downtime,
		Breakdown:       lines,
	}})
}

// monthBounds parses a YYYY-MM period into [monthStart, nextMonthStart) UTC.
func monthBounds(period string) (time.Time, time.Time, error) {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: invalid period %q", ErrReportValidation, period)
	}
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	return from, to, nil
}
