package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"time"

	dbruntime "github.com/denisakp/ogoune/internal/database"
	"github.com/denisakp/ogoune/internal/repository/store"
	"github.com/denisakp/ogoune/internal/service"
	svcintegrations "github.com/denisakp/ogoune/internal/service/integrations"
)

// InitDatabase opens the database runtime and wires all sqlc-backed
// repositories onto the App struct. Post-decom, the legacy
// `SQLC_*` env flags are gone; sqlc is the sole impl. Unknown `SQLC_*` vars
// in the operator's environment are silently ignored (FR-004).
func InitDatabase(app *App) {
	cfg := app.Cfg

	if err := dbruntime.Init(context.Background(), dbruntime.Config{
		Driver:      dbruntime.Driver(cfg.DBDriver),
		DatabaseURL: cfg.DatabaseUrl,
		SQLitePath:  cfg.SQLitePath,
		LogLevel:    cfg.DBLogLevel,
	}); err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := dbruntime.Ping(ctx); err != nil {
		slog.Error("database health check failed", "error", err)
		os.Exit(1)
	}

	slog.Info("database connection established")

	rt, err := dbruntime.ActiveRuntime()
	if err != nil {
		slog.Error("failed to get database runtime", "error", err)
		os.Exit(1)
	}

	// Wire all repositories — sqlc is the only impl.
	app.ResourceRepo = store.NewResourceRepositorySQLC(rt)
	app.IncidentRepo = store.NewIncidentRepositorySQLC(rt)
	app.IncidentEventStepRepo = store.NewIncidentEventStepRepositorySQLC(rt)
	app.IncidentDiagnosticsRepo = store.NewIncidentDiagnosticsRepositorySQLC(rt)
	app.NotificationRepo = store.NewNotificationRepositorySQLC(rt)
	app.MaintenanceRepo = store.NewMaintenanceRepositorySQLC(rt)
	app.NotificationChannelRepo = store.NewNotificationChannelRepositorySQLC(rt)
	app.MonitoringActivityRepo = store.NewMonitoringActivityRepositorySQLC(rt)
	app.TagsRepo = store.NewTagsRepositorySQLC(rt)
	app.StatusPageSettingsRepo = store.NewStatusPageSettingsRepositorySQLC(rt)
	app.ComponentRepo = store.NewComponentRepositorySQLC(rt)
	app.UserRepo = store.NewUserRepositorySQLC(rt)
	app.APIKeyRepo = store.NewAPIKeyRepositorySQLC(rt)
	app.ResourceCredentialRepo = store.NewResourceCredentialRepositorySQLC(rt)
	app.ExpiryNotificationLogRepo = store.NewExpiryNotificationLogRepositorySQLC(rt)
	app.SessionRepo = store.NewSessionRepositorySQLC(rt)
	app.TwoFactorResetTokenRepo = store.NewTwoFactorResetTokenRepositorySQLC(rt)
	app.EscalationRepo = store.NewEscalationRepositorySQLC(rt)
	app.UptimeDailyAggRepo = store.NewUptimeDailyAggRepositorySQLC(rt)
	app.IncidentUpdateRepo = store.NewIncidentUpdateRepositorySQLC(rt)
	app.NotificationFeedRepo = store.NewNotificationFeedRepositorySQLC(rt)
	// Built here (not InitServices) because InitWorker runs first and wires this
	// as the incident notification emitter.
	app.NotificationFeedService = service.NewNotificationFeedService(app.NotificationFeedRepo)

	app.DashboardRepo = store.NewDashboardRepositorySQLC(rt)
	app.DashboardService = service.NewDashboardService(app.DashboardRepo)

	// Reports: built here (not InitServices) because InitWorker runs
	// first and registers the scheduled report generator.
	app.ReportSettingsRepo = store.NewReportSettingsRepositorySQLC(rt)
	app.ReportHistoryRepo = store.NewReportHistoryRepositorySQLC(rt)
	app.ReportService = service.NewReportService(
		app.ReportSettingsRepo,
		app.ReportHistoryRepo,
		app.ResourceRepo,
		app.UptimeDailyAggRepo,
		app.IncidentRepo,
		app.NotificationChannelRepo,
	)

	// Announcement banners (option 2) — instance-wide operator messages.
	app.AnnouncementRepo = store.NewAnnouncementRepositorySQLC(rt)
	app.AnnouncementService = service.NewAnnouncementService(app.AnnouncementRepo)

	// Agent device monitoring. Services built here (not InitServices)
	// because InitWorker runs first and registers the metrics-retention job.
	app.HostRepo = store.NewHostRepositorySQLC(rt)
	app.HostCredentialRepo = store.NewHostCredentialRepositorySQLC(rt)
	app.HostMetricsRepo = store.NewHostMetricRepositorySQLC(rt)
	app.HostCredentialService = service.NewHostCredentialService(app.HostCredentialRepo)
	app.HostService = service.NewHostService(app.HostRepo, app.HostCredentialService, app.HostMetricsRepo, app.ResourceRepo, cfg.HostFreshnessThreshold)
	app.HostMetricsService = service.NewHostMetricsService(app.HostMetricsRepo, app.HostRepo, cfg.HostMetricsRawWindow, time.Duration(cfg.HostMetricsRetentionDays)*24*time.Hour)

	// Config-derived observability integrations — read-only.
	app.IntegrationsService = svcintegrations.NewIntegrationsService(app.ResourceRepo, app.ComponentRepo)

	// Seed-time services that the worker layer depends on must be built
	// before InitWorker runs (InitServices is too late). The full
	// PublicStatusService stays in InitServices since it has no worker dep.
	app.IncidentUpdateService = service.NewIncidentUpdateService(app.IncidentUpdateRepo)
}
