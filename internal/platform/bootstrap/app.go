// Package bootstrap contains the application composition root.
// It organizes initialization by concern without introducing new abstractions.
package bootstrap

import (
	"log/slog"
	"net/http"

	"github.com/denisakp/ogoune/internal/config"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/ee/license"
	"github.com/denisakp/ogoune/internal/metrics"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/scheduler"
	"github.com/denisakp/ogoune/internal/service"
	svcintegrations "github.com/denisakp/ogoune/internal/service/integrations"
	"github.com/denisakp/ogoune/internal/worker"
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
)

const AppVersion = "1.0.0-beta"

// App holds all initialized application components.
// Each Init* function populates a subset of fields.
type App struct {
	// Config phase
	Cfg *config.Config

	// Database phase
	ResourceRepo              port.ResourceRepository
	IncidentRepo              port.IncidentRepository
	IncidentEventStepRepo     port.IncidentEventStepRepository
	IncidentDiagnosticsRepo   port.IncidentDiagnosticsRepository
	NotificationRepo          port.NotificationRepository
	MaintenanceRepo           port.MaintenanceRepository
	NotificationChannelRepo   port.NotificationChannelRepository
	MonitoringActivityRepo    port.MonitoringActivityRepository
	TagsRepo                  port.TagsRepository
	StatusPageSettingsRepo    port.StatusPageSettingsRepository
	ComponentRepo             port.ComponentRepository
	UserRepo                  port.UserRepository
	APIKeyRepo                port.APIKeyRepository
	ResourceCredentialRepo    port.ResourceCredentialRepository
	ExpiryNotificationLogRepo port.ExpiryNotificationLogRepository
	SessionRepo               port.SessionRepository
	TwoFactorResetTokenRepo   port.TwoFactorResetTokenRepository
	EscalationRepo            port.EscalationRepository
	UptimeDailyAggRepo        port.UptimeDailyAggRepository
	IncidentUpdateRepo        port.IncidentUpdateRepository
	NotificationFeedRepo      port.NotificationFeedRepository
	DashboardRepo             port.DashboardRepository
	ReportSettingsRepo        port.ReportSettingsRepository
	ReportHistoryRepo         port.ReportHistoryRepository
	AnnouncementRepo          port.AnnouncementRepository
	// Spec 079 — Agent device monitoring
	HostRepo           port.HostRepository
	HostCredentialRepo port.HostCredentialRepository
	HostMetricsRepo    port.HostMetricsRepository
	// Spec 083 — agent-down alert state + unread-notification escalation state
	HostAlertStateRepo              port.HostAlertStateRepository
	NotificationEscalationStateRepo port.NotificationEscalationStateRepository

	// Metrics phase
	MetricsRecorder       domain.MetricsRecorder
	MetricsRegistry       *prometheus.Registry
	PublicStatusCacheMetr *metrics.PublicStatusMetrics

	// Scheduler phase
	SchedulerCfg          *scheduler.Config
	RuntimeScheduler      scheduler.Scheduler
	SchedulerAdapter      port.ResourceScheduler
	ConfirmationScheduler port.ConfirmationRescheduler
	AsynqClient           *asynq.Client
	AsynqInspector        *asynq.Inspector
	AsynqScheduler        *asynq.Scheduler
	RedisOpt              asynq.RedisClientOpt
	MaintenanceScheduler  port.MaintenanceScheduler

	// Worker phase
	Processor           *worker.Processor
	DetectorIncidentSvc port.MonitoringIncidentProcessor

	// Services phase
	ResourceService         *service.ResourceService
	ComponentService        *service.ComponentService
	AuthService             *service.AuthService
	JWTManager              *service.JWTManager
	APIKeyService           *service.APIKeyService
	SessionService          *service.SessionService
	TwoFactorService        *service.TwoFactorService
	EscalationService       *service.EscalationService
	NotificationFeedService *service.NotificationFeedService
	DashboardService        *service.DashboardService
	ReportService           *service.ReportService
	AnnouncementService     *service.AnnouncementService
	IntegrationsService     *svcintegrations.IntegrationsService
	// Spec 079 — Agent device monitoring
	HostService           *service.HostService
	HostCredentialService *service.HostCredentialService
	HostMetricsService    *service.HostMetricsService

	// Spec 060 — Public status page
	PublicStatusService   *service.PublicStatusService
	UptimeAggregator      *service.UptimeAggregator
	IncidentUpdateService *service.IncidentUpdateService

	// Router phase
	RootRouter *chi.Mux
	Server     *http.Server
}

func LogStartupEdition() {
	if license.IsEnterprise() {
		slog.Info("Ogoune Enterprise Edition")
	} else {
		slog.Info("Ogoune Community Edition")
	}
}
