package store

import "github.com/denisakp/ogoune/internal/port"

// Compile-time guarantees that every sqlc-backed wrapper satisfies its port
// interface. The legacy GORM compile-time checks were removed in spec 052.
var (
	_ port.TagsRepository                  = (*TagsRepositorySQLC)(nil)
	_ port.ResourceRepository              = (*ResourceRepositorySQLC)(nil)
	_ port.ComponentRepository             = (*ComponentRepositorySQLC)(nil)
	_ port.IncidentRepository              = (*IncidentRepositorySQLC)(nil)
	_ port.IncidentEventStepRepository     = (*IncidentEventStepRepositorySQLC)(nil)
	_ port.NotificationRepository          = (*NotificationRepositorySQLC)(nil)
	_ port.MonitoringActivityRepository    = (*MonitoringActivityRepositorySQLC)(nil)
	_ port.NotificationChannelRepository   = (*NotificationChannelRepositorySQLC)(nil)
	_ port.MaintenanceRepository           = (*MaintenanceRepositorySQLC)(nil)
	_ port.StatusPageSettingsRepository    = (*StatusPageSettingsRepositorySQLC)(nil)
	_ port.UserRepository                  = (*UserRepositorySQLC)(nil)
	_ port.APIKeyRepository                = (*APIKeyRepositorySQLC)(nil)
	_ port.IncidentDiagnosticsRepository   = (*IncidentDiagnosticsRepositorySQLC)(nil)
	_ port.ExpiryNotificationLogRepository = (*ExpiryNotificationLogRepositorySQLC)(nil)
	_ port.ResourceCredentialRepository    = (*ResourceCredentialRepositorySQLC)(nil)

	// Spec 059 — Settings slice
	_ port.SessionRepository             = (*SessionRepositorySQLC)(nil)
	_ port.TwoFactorResetTokenRepository = (*TwoFactorResetTokenRepositorySQLC)(nil)
	_ port.EscalationRepository          = (*EscalationRepositorySQLC)(nil)

	// Spec 060 — Public Status Pages
	_ port.UptimeDailyAggRepository = (*UptimeDailyAggRepositorySQLC)(nil)
	_ port.IncidentUpdateRepository = (*IncidentUpdateRepositorySQLC)(nil)

	// Spec 072 — In-app notification feed
	_ port.NotificationFeedRepository = (*NotificationFeedRepositorySQLC)(nil)

	// Spec 075 — Custom dashboards
	_ port.DashboardRepository = (*DashboardRepositorySQLC)(nil)

	// Spec 076 — Monthly reports
	_ port.ReportSettingsRepository = (*ReportSettingsRepositorySQLC)(nil)
	_ port.ReportHistoryRepository  = (*ReportHistoryRepositorySQLC)(nil)

	// Announcements (operator banners)
	_ port.AnnouncementRepository = (*AnnouncementRepositorySQLC)(nil)

	// Spec 079 — Agent device monitoring
	_ port.HostRepository           = (*HostRepositorySQLC)(nil)
	_ port.HostAlertStateRepository = (*HostAlertStateRepositorySQLC)(nil)

	_ port.NotificationEscalationStateRepository = (*NotificationEscalationStateRepositorySQLC)(nil)
	_ port.HostCredentialRepository = (*HostCredentialRepositorySQLC)(nil)
	_ port.HostMetricsRepository    = (*HostMetricRepositorySQLC)(nil)
)
