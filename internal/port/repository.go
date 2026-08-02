package port

import (
	"context"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
)

// TagsRepository manages tag lifecycle.
type TagsRepository interface {
	Create(ctx context.Context, t *domain.Tags) error
	FindByID(ctx context.Context, id string) (*domain.Tags, error)
	FindByIDs(ctx context.Context, ids []string) ([]*domain.Tags, error)
	FindByName(ctx context.Context, name string) (*domain.Tags, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Tags, error)
	Update(ctx context.Context, t *domain.Tags) error
	Delete(ctx context.Context, id string) error
}

// UpdateMonitoringStateRequest carries the columns mutated by the monitoring
// worker after a check cycle. Pointer semantics: nil preserves the existing
// column value; a non-nil pointer writes it. For nullable timestamp columns
// the outer pointer is double-indirected so callers can distinguish
// "preserve" (outer nil) from "set to NULL" (outer non-nil, inner nil).
type UpdateMonitoringStateRequest struct {
	Status               *domain.ResourceStatus
	FailureCount         *int
	LastChecked          **time.Time
	LastStatusTransition **time.Time
	FlapStartedAt        **time.Time
}

// UpdateMetadataRequest carries the SSL/domain expiry fields populated by the
// metadata-enrichment path. Same nil-vs-non-nil semantics as
// UpdateMonitoringStateRequest; nullable timestamps use **time.Time.
type UpdateMetadataRequest struct {
	SSLExpirationDate    **time.Time
	SSLIssuer            *string
	DomainExpirationDate **time.Time
	DomainRegistrar      *string
}

// ResourceRepository manages monitored resources.
type ResourceRepository interface {
	Create(ctx context.Context, r *domain.Resource) (*domain.Resource, error)
	FindByID(ctx context.Context, id string) (*domain.Resource, error)
	FindByHeartbeatSlug(ctx context.Context, slug string) (*domain.Resource, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Resource, error)
	Update(ctx context.Context, r *domain.Resource) error
	Delete(ctx context.Context, id string) error
	FindActive(ctx context.Context, limit, offset int) ([]*domain.Resource, error)
	FindByTag(ctx context.Context, tagName string, limit, offset int) ([]*domain.Resource, error)
	FindByComponentID(ctx context.Context, componentID string) ([]*domain.Resource, error)
	CountByComponentID(ctx context.Context, componentID string) (int64, error)
	FindMissedHeartbeats(ctx context.Context, now time.Time, limit int) ([]*domain.Resource, error)
	UpdateLastPingAt(ctx context.Context, id string, at time.Time) error
	UpdateStatus(ctx context.Context, id string, status domain.ResourceStatus) error
	UpdateMonitoringState(ctx context.Context, id string, req UpdateMonitoringStateRequest) error
	UpdateMetadata(ctx context.Context, id string, req UpdateMetadataRequest) error
	FindScheduledResources(ctx context.Context) ([]*domain.Resource, error)
	ListResourcesByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error)
	// Host link — additive. SetResourceHostID links a monitor to a
	// host; ClearResourceHostIDByHost unlinks every monitor of a host (host delete).
	SetResourceHostID(ctx context.Context, resourceID string, hostID *string) error
	ClearResourceHostIDByHost(ctx context.Context, hostID string) error
}

// ---------------------------------------------------------------------------
// Agent device monitoring
// ---------------------------------------------------------------------------

// HostRepository persists monitored hosts and their denormalized latest snapshot.
type HostRepository interface {
	Create(ctx context.Context, h *domain.Host) error
	FindByID(ctx context.Context, id string) (*domain.Host, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Host, error)
	Count(ctx context.Context) (int64, error)
	Delete(ctx context.Context, id string) error
	UpdateSnapshot(ctx context.Context, h *domain.Host) error
}

// HostCredentialRepository persists per-host bearer credentials (hash only).
type HostCredentialRepository interface {
	Create(ctx context.Context, c *domain.HostCredential) error
	FindActiveByHash(ctx context.Context, hash string) (*domain.HostCredential, error)
	ListByHost(ctx context.Context, hostID string) ([]*domain.HostCredential, error)
	DeactivateByID(ctx context.Context, id string) error
	DeactivateAllForHost(ctx context.Context, hostID string) error
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
	DeleteByHost(ctx context.Context, hostID string) error
}

// HostMetricsRepository persists and prunes host metric samples.
type HostMetricsRepository interface {
	Insert(ctx context.Context, s *domain.HostMetricSample) error
	ListInRange(ctx context.Context, hostID string, from, to time.Time) ([]*domain.HostMetricSample, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteByHost(ctx context.Context, hostID string) error
	Decimate(ctx context.Context, cutoff time.Time) (int64, error)
}

// ComponentRepository manages logical component groups.
type ComponentRepository interface {
	Create(ctx context.Context, c *domain.Component) (*domain.Component, error)
	FindByID(ctx context.Context, id string) (*domain.Component, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Component, error)
	Update(ctx context.Context, c *domain.Component) error
	Delete(ctx context.Context, id string) error
	UpdateLastNotificationStatus(ctx context.Context, id string, status domain.ComponentStatus) error
}

// IncidentRepository manages incidents (unresolved vs resolved).
type IncidentRepository interface {
	Create(ctx context.Context, i *domain.Incident) (*domain.Incident, error)
	FindByID(ctx context.Context, id string) (*domain.Incident, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Incident, error)
	Update(ctx context.Context, i *domain.Incident) error
	Delete(ctx context.Context, id string) error
	FindUnresolved(ctx context.Context, limit, offset int) ([]*domain.Incident, error)
	FindByResource(ctx context.Context, resourceID string, limit, offset int) ([]*domain.Incident, error)
	GetIncidentStats(ctx context.Context, hours int) (int, int, error)
	FindActiveByResourceID(ctx context.Context, resourceID string) (*domain.Incident, error)
	HasActiveIncident(ctx context.Context) (bool, error)
	FindLastResolved(ctx context.Context) (*domain.Incident, error)
	CountByResourceID(ctx context.Context, resourceID string) (int64, error)
	ListIncidentsByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error)
}

// IncidentEventStepRepository manages lifecycle steps.
type IncidentEventStepRepository interface {
	Create(ctx context.Context, s *domain.IncidentEventStep) (*domain.IncidentEventStep, error)
	FindByID(ctx context.Context, id string) (*domain.IncidentEventStep, error)
	FindLastByIncidentAndStep(ctx context.Context, incidentID string, step domain.IncidentEventStepType) (*domain.IncidentEventStep, error)
	List(ctx context.Context, limit, offset int) ([]*domain.IncidentEventStep, error)
	Update(ctx context.Context, s *domain.IncidentEventStep) error
	Delete(ctx context.Context, id string) error
}

// NotificationRepository handles notification events.
type NotificationRepository interface {
	Create(ctx context.Context, n *domain.NotificationEvent) error
	FindByID(ctx context.Context, id string) (*domain.NotificationEvent, error)
	List(ctx context.Context, limit, offset int) ([]*domain.NotificationEvent, error)
	Update(ctx context.Context, n *domain.NotificationEvent) error
	Delete(ctx context.Context, id string) error
	FindPending(ctx context.Context, limit, offset int) ([]*domain.NotificationEvent, error)
	ClaimPending(ctx context.Context, id, claimOwner string, claimedAt time.Time) (bool, error)
	MarkAsSent(ctx context.Context, id string, processedAt time.Time) error
	MarkAsFailed(ctx context.Context, id, lastError string, processedAt time.Time) error
	MarkAsExpired(ctx context.Context, id, lastError string, processedAt time.Time) error
}

// MonitoringActivityRepository manages monitoring activity records.
type MonitoringActivityRepository interface {
	Create(ctx context.Context, activity *domain.MonitoringActivity) error
	List(ctx context.Context, limit, offset int) ([]*domain.MonitoringActivity, error)
	FindByResourceID(ctx context.Context, resourceID string, limit, offset int) ([]*domain.MonitoringActivity, error)
	CountTransitionsInWindow(ctx context.Context, resourceID string, windowStart time.Time) (int, error)
	GetUptimeStats(ctx context.Context, resourceID string) ([]domain.UptimeStat, error)
	GetRecentResponseTimes(ctx context.Context, resourceID string, limit int) ([]domain.ResponseTimePoint, error)
	GetGlobalUptimeStats(ctx context.Context, hours int) (float64, error)
	GetUptimeByWindow(ctx context.Context, resourceID string, hours int) (*float64, error)
	GetAvgResponseTimeByWindow(ctx context.Context, resourceID string, hours int) (*int, error)
}

// ResourceScheduler defines the interface for scheduling monitoring tasks
// at the service layer (schedule/unschedule with domain.Resource).
// Named ResourceScheduler to avoid collision with the full Scheduler interface.
type ResourceScheduler interface {
	Schedule(ctx context.Context, r *domain.Resource) error
	Unschedule(ctx context.Context, resourceID string) error
}

// NotificationChannelRepository manages notification channels.
// ResourceCredentialRepository manages optional auth credentials for protocol-aware resources.
type ResourceCredentialRepository interface {
	Get(ctx context.Context, resourceID string) (*domain.ResourceCredential, error)
	Upsert(ctx context.Context, cred *domain.ResourceCredential) error
	Delete(ctx context.Context, resourceID string) error
	Exists(ctx context.Context, resourceID string) (bool, error)
}

type NotificationChannelRepository interface {
	Create(ctx context.Context, channel *domain.NotificationChannel) error
	FindByID(ctx context.Context, id string) (*domain.NotificationChannel, error)
	List(ctx context.Context, limit, offset int) ([]*domain.NotificationChannel, error)
	Update(ctx context.Context, channel *domain.NotificationChannel) error
	Delete(ctx context.Context, id string) error
	FindByType(ctx context.Context, channelType domain.NotificationChannelType) ([]*domain.NotificationChannel, error)
	FindDefaultChannels(ctx context.Context) ([]*domain.NotificationChannel, error)
	FindByResourceID(ctx context.Context, resourceID string) ([]*domain.NotificationChannel, error)
	FindByComponentID(ctx context.Context, componentID string) ([]*domain.NotificationChannel, error)
	// Spec 060 follow-up — per-channel dispatch counters.
	MarkSent(ctx context.Context, channelID string, at time.Time) error
	MarkFailure(ctx context.Context, channelID string, at time.Time) error
}

// MaintenanceRepository manages maintenance windows.
type MaintenanceRepository interface {
	Create(ctx context.Context, m *domain.Maintenance) (*domain.Maintenance, error)
	FindByID(ctx context.Context, id string) (*domain.Maintenance, error)
	List(ctx context.Context, status string, limit, offset int) ([]*domain.Maintenance, error)
	Update(ctx context.Context, m *domain.Maintenance) error
	Delete(ctx context.Context, id string) error
	FindActiveForResource(ctx context.Context, resourceID string, now time.Time) ([]*domain.Maintenance, error)
}

// StatusPageSettingsRepository manages status page configuration.
type StatusPageSettingsRepository interface {
	Get(ctx context.Context) (*domain.StatusPageSettings, error)
	Upsert(ctx context.Context, settings *domain.StatusPageSettings) error
}

// UserRepository manages user accounts and authentication.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, userID string, hashedPassword string) error
	UpdateLastLogin(ctx context.Context, userID string) error
	UpdateTwoFactorSecret(ctx context.Context, userID string, secret string, enabled bool) error
}

// APIKeyRepository manages API key persistence and lookup.
type APIKeyRepository interface {
	Create(ctx context.Context, key *domain.APIKey) error
	FindByID(ctx context.Context, id, userID string) (*domain.APIKey, error)
	FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.APIKey, error)
	UpdateLastUsed(ctx context.Context, id string, at time.Time, ip string) error
	Revoke(ctx context.Context, id, userID string) error
	CountByUserID(ctx context.Context, userID string) (int64, error)
}

// IncidentDiagnosticsRepository manages detailed diagnostic information for incidents.
type IncidentDiagnosticsRepository interface {
	Create(ctx context.Context, d *domain.IncidentDiagnostics) (*domain.IncidentDiagnostics, error)
	FindByIncidentID(ctx context.Context, incidentID string) (*domain.IncidentDiagnostics, error)
	Update(ctx context.Context, d *domain.IncidentDiagnostics) error
	Delete(ctx context.Context, id string) error
}

// ExpiryNotificationLogRepository manages deduplication records for expiry alerts.
type ExpiryNotificationLogRepository interface {
	CountByKey(ctx context.Context, resourceID, expiryType string, threshold int) (int64, error)
	Create(ctx context.Context, log *domain.ExpiryNotificationLog) error
	DeleteByResourceIDAndType(ctx context.Context, resourceID, expiryType string) error
	DeleteOlderThan(ctx context.Context, cutoff time.Time) error
}

// DashboardRepository persists custom dashboards. Config-only.
type DashboardRepository interface {
	Create(ctx context.Context, d *domain.Dashboard) (*domain.Dashboard, error)
	FindByID(ctx context.Context, id string) (*domain.Dashboard, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Dashboard, error)
	Update(ctx context.Context, d *domain.Dashboard) error
	UpdateWidgets(ctx context.Context, id string, widgets []domain.WidgetInstance, at time.Time) error
	Delete(ctx context.Context, id string) error
}

// AnnouncementRepository persists operator announcement banners (option 2).
type AnnouncementRepository interface {
	Create(ctx context.Context, a *domain.Announcement) (*domain.Announcement, error)
	ListActive(ctx context.Context) ([]*domain.Announcement, error)
	Delete(ctx context.Context, id string) error
}

// ReportSettingsRepository persists the single instance-wide monthly-report
// configuration. Get returns repository.ErrNotFound when unsaved.
type ReportSettingsRepository interface {
	Get(ctx context.Context) (*domain.ReportSettings, error)
	Upsert(ctx context.Context, s *domain.ReportSettings) (*domain.ReportSettings, error)
}

// ReportHistoryRepository persists generated monthly reports.
type ReportHistoryRepository interface {
	Create(ctx context.Context, r *domain.ReportHistory) (*domain.ReportHistory, error)
	ListRecent(ctx context.Context, limit int) ([]*domain.ReportHistory, error)
	FindByPeriod(ctx context.Context, period string) (*domain.ReportHistory, error)
}

// NotificationFeedRepository persists in-app notification-feed items.
// Distinct from NotificationRepository (outbound dispatch events).
type NotificationFeedRepository interface {
	Create(ctx context.Context, n *domain.FeedNotification) (*domain.FeedNotification, error)
	ListForUser(ctx context.Context, userID string, category *string, limit, offset int) ([]*domain.FeedNotification, error)
	CountForUser(ctx context.Context, userID string, category *string) (int64, error)
	MarkRead(ctx context.Context, id string, at time.Time) (int64, error)
	MarkAllRead(ctx context.Context, userID string, before, at time.Time) (int64, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// SessionRepository — spec 059 FR-008/009/009a.
type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	FindByID(ctx context.Context, id string) (*domain.Session, error)
	ListActiveByUser(ctx context.Context, userID string) ([]*domain.Session, error)
	UpdateLastActive(ctx context.Context, id string, at time.Time) error
	Revoke(ctx context.Context, id string, at time.Time) error
	RevokeAllExcept(ctx context.Context, userID, currentSessionID string, at time.Time) (int64, error)
}

// TwoFactorResetTokenRepository — spec 059 FR-012a magic-link recovery.
type TwoFactorResetTokenRepository interface {
	Create(ctx context.Context, t *domain.TwoFactorResetToken) error
	ConsumeByHash(ctx context.Context, tokenHash string, at time.Time) (*domain.TwoFactorResetToken, error)
	CountRecentByUser(ctx context.Context, userID string, since time.Time) (int64, error)
	DeleteExpired(ctx context.Context, cutoff time.Time) error
}

// EscalationRepository — spec 059 FR-023..FR-026a.
type EscalationRepository interface {
	Create(ctx context.Context, p *domain.EscalationPolicy) error
	FindByID(ctx context.Context, id string) (*domain.EscalationPolicy, error)
	List(ctx context.Context) ([]*domain.EscalationPolicy, error)
	Update(ctx context.Context, p *domain.EscalationPolicy) error
	Delete(ctx context.Context, id string) error
	Reorder(ctx context.Context, order []string) error
	NextPriority(ctx context.Context) (int, error)
}

// Custom-domain DNS state has been folded into StatusPageSettings — see
// migration 0018. The standalone CustomDomainRepository has been removed.

// UptimeDailyAggRepository — spec 060 FR-004 / FR-008 / FR-026.
// Persists per-resource, per-UTC-day uptime aggregates produced by the cron.
type UptimeDailyAggRepository interface {
	// Upsert inserts or updates the row for (ResourceID, Day).
	Upsert(ctx context.Context, agg *domain.UptimeDailyAgg) error
	// FindRange returns aggregates for the given resources between [from, to] (inclusive).
	FindRange(ctx context.Context, resourceIDs []string, from, to time.Time) ([]*domain.UptimeDailyAgg, error)
	// FindForResource returns the daily aggregates of one resource over [from, to].
	FindForResource(ctx context.Context, resourceID string, from, to time.Time) ([]*domain.UptimeDailyAgg, error)
	// FindEarliestDay returns the smallest Day across all resources.
	// Returns the zero time when the table is empty.
	FindEarliestDay(ctx context.Context) (time.Time, error)
}

// IncidentUpdateRepository — spec 060 / US7. Persists per-incident lifecycle
// status updates (investigating / identified / monitoring / resolved).
type IncidentUpdateRepository interface {
	Create(ctx context.Context, u *domain.IncidentUpdate) (*domain.IncidentUpdate, error)
	FindByID(ctx context.Context, id string) (*domain.IncidentUpdate, error)
	ListByIncident(ctx context.Context, incidentID string) ([]*domain.IncidentUpdate, error)
	Update(ctx context.Context, u *domain.IncidentUpdate) error
	Delete(ctx context.Context, id string) error
}
