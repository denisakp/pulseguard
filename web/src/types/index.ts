/**
 * Resource metadata containing SSL and domain information
 */
export interface ResourceMetadata {
  ssl_expiration_date?: string
  ssl_issuer?: string
  domain_expiration_date?: string
  domain_registrar?: string
  ssl_days_remaining?: number | null
  domain_days_remaining?: number | null
}

/**
 * Resource represents a monitor target (HTTP, TCP, etc.)
 */
export interface Resource {
  id: string
  name: string
  type: 'http' | 'tcp' | 'dns' | 'icmp' | 'heartbeat' | 'keyword' | 'protocol'
  target: string
  interval: number // in seconds
  timeout: number // in seconds
  status: 'up' | 'down' | 'error' | 'unknown' | 'paused' | 'pending' | 'flapping' | 'waiting'
  is_active: boolean
  failure_count: number
  confirmation_checks: number
  confirmation_interval: number
  expiry_alert_thresholds?: string | null
  expiry_status?: 'ok' | 'warning' | 'critical' | 'expired'
  last_checked?: string
  created_at: string
  updated_at: string
  component_id?: string // Optional component assignment
  tags?: Tag[]
  incidents?: Incident[]
  incident_count_30d?: number
  uptime_7d?: number // 0..1 ratio over last 7 days
  uptime_30d?: number // 0..1 ratio over last 30 days
  response_time?: number // avg response time in ms over the same window
  uptime?: number // Overall uptime percentage
  hourly_uptime?: HourlyUptimeStat[] // Hourly uptime data for sparklines
  response_times?: ResponseTime[] // Response time history
  metadata?: ResourceMetadata // SSL and domain metadata
  metadata_pending?: boolean // true when backend enrichment is in progress
  flap_detection_enabled?: boolean
  flap_threshold?: number
  flap_window_seconds?: number
  flap_max_duration_minutes?: number
  reminder_interval_minutes?: number
  last_status_transition?: string | null
  flap_started_at?: string | null
  // Heartbeat-specific fields (only present when type === 'heartbeat')
  heartbeat_slug?: string // UUID v4 — present only in detail response
  heartbeat_interval?: number // seconds (60–86400)
  heartbeat_grace?: number // seconds (60–3600)
  last_ping_at?: string | null // ISO 8601 or null
  waiting?: boolean // true when last_ping_at is null (never pinged)
  // Keyword-specific fields (only present when type === 'keyword')
  keyword?: string // literal string to search for (max 500 chars)
  keyword_mode?: 'contains' | 'not_contains'
  // Protocol-specific fields (only present when type === 'protocol')
  protocol_type?: 'redis' | 'mongodb' | 'ftp' | 'ssh' | 'mysql' | 'postgres' | 'rabbitmq' | 'kafka'
  protocol_port?: number // 1–65535; absent = use protocol default
}

/**
 * Hourly uptime statistics
 */
export interface HourlyUptimeStat {
  hour: string
  uptime_percent: number
  successful_count: number
  total_count: number
}

/**
 * Response time data point
 */
export interface ResponseTime {
  timestamp: string
  response_time: number // in milliseconds
}

/**
 * Stats summary for a given time range
 */
export interface StatsSummary {
  overall_uptime: number // Uptime percentage
  incidents: number // Number of incidents
  without_incidents_duration: string // Duration without incidents (e.g., "5h 30m")
  affected_monitors: number // Number of monitors affected
}

export interface CreateResource {
  name: string
  type: 'http' | 'tcp' | 'dns' | 'icmp' | 'heartbeat' | 'keyword' | 'protocol'
  target?: string
  interval?: number
  timeout?: number
  confirmation_checks?: number
  confirmation_interval?: number
  tags: string[]
  component_id?: string // Optional component assignment
  expiry_alert_thresholds?: string // Comma-separated days, e.g. "30,14,7,1"
  flap_detection_enabled?: boolean
  flap_threshold?: number
  flap_window_seconds?: number
  flap_max_duration_minutes?: number
  reminder_interval_minutes?: number
  // Heartbeat-specific
  heartbeat_interval?: number // seconds (60–86400)
  heartbeat_grace?: number // seconds (60–3600)
  // Keyword-specific
  keyword?: string
  keyword_mode?: 'contains' | 'not_contains'
  // Protocol-specific
  protocol_type?: 'redis' | 'mongodb' | 'ftp' | 'ssh' | 'mysql' | 'postgres' | 'rabbitmq' | 'kafka'
  protocol_port?: number // 1–65535; absent = use protocol default
}

export type UpdateResource = Partial<CreateResource>

/**
 * Tag represents a label for organizing resources
 */
export interface Tag {
  id: string
  name: string
  color?: string
  description?: string
  created_at: string
  updated_at: string
}

export interface CreateTag {
  name: string
  color?: string
  description?: string
}

/**
 * Event types for notifications
 */
export type EventType = 'down' | 'up' | 'expiry'

/**
 * MonitoringActivity represents a health check result
 */
export interface MonitoringActivity {
  id: string
  resource_id: string
  message: string
  success: boolean
  response_time: number // in milliseconds
  response_data?: string
  created_at: string
  updated_at: string
}

/**
 * Incident event step types
 */
export type IncidentEventStepType =
  | 'detected'
  | 'resolved'
  | 'alert_sent'
  | 'resource_down_alert'
  | 'resource_up_alert'

/**
 * Incident event step represents a step in the lifecycle of an incident
 */
export interface IncidentEventStep {
  id: string
  incident_id: string
  step: IncidentEventStepType
  message?: string
  created_at: string
  updated_at: string
}

/**
 * Incident diagnostics containing rich technical context about failures
 */
export interface IncidentDiagnostics {
  id: string
  incident_id: string
  // Request details
  request_method: string
  request_url: string
  request_headers?: Record<string, string>
  request_timeout: number
  // Response details
  http_status_code: number
  response_headers?: Record<string, string>
  response_body?: string
  response_size: number
  // Error context
  failure_type: string
  error_message: string
  error_summary: string
  // Timing breakdown
  total_duration: number // milliseconds
  dns_duration: number
  tls_duration: number
  first_byte_duration: number
  // Flags
  body_truncated: boolean
  body_encoded: boolean
  // ICMP network diagnostics (optional, present only when enrichment ran)
  icmp_available?: boolean | null
  icmp_reachable?: boolean | null
  icmp_rtt_ms?: number | null
  root_cause_hint?: string | null
  // Keyword monitor diagnostics (present only when type === 'keyword')
  keyword?: string | null
  keyword_mode?: string | null
  keyword_found?: boolean | null
}

/**
 * Incident represents a detected downtime event
 */
export interface Incident {
  id: string
  resource_id: string
  resource?: Resource
  reason: string
  cause: string
  started_at: string
  resolved_at?: string | null
  details?: string
  diagnostics?: IncidentDiagnostics
  event_steps?: IncidentEventStep[]
  created_at: string
  updated_at: string
}

/**
 * ICMP capability availability state returned by GET /system/capabilities
 */
export interface ICMPAvailabilityState {
  enabled: boolean
  capability_available: boolean
  reason: string
}

/**
 * System capabilities response
 */
export interface SystemCapabilities {
  icmp: ICMPAvailabilityState
}

export interface IncidentsQueryParams {
  unresolved?: boolean
  status?: 'open' | 'resolved'
  monitor_id?: string
  page?: number
  per_page?: number
}

/**
 * API Response wrapper for paginated results
 */
export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

/**
 * API Error response
 */
export interface ApiError {
  message: string
  code?: string
}

export interface ExpirationStatus {
  text: string
  color: string
  type: 'success' | 'warning' | 'danger'
}

/**
 * Status page types based on /status endpoint
 */

/**
 * Status page settings
 */
export interface StatusPageSettings {
  name: string
  homepage_url?: string
  umami_website_id?: string
  umami_script_url?: string
  enable_details_page: boolean
  show_uptime_percentage: boolean
}

/**
 * Daily status for a single day in the 90-day window
 */
export type DailyStatus = 'up' | 'degraded' | 'down' | 'no_data'

/**
 * Current status of a resource (simplified for status page)
 */
export type ResourceCurrentStatus = 'up' | 'down' | 'degraded'

/**
 * Resource status information for status page
 */
export interface ResourceStatusInfo {
  id: string
  name: string
  current_status: ResourceCurrentStatus
  uptime_percentage_last_90_days: number
  daily_status_last_90_days: DailyStatus[]
}

/**
 * Global status for all systems
 */
export type GlobalStatus = 'all_systems_operational' | 'some_systems_down'

/**\n * Component status info for status page
 */
export interface ComponentStatusInfo {
  id: string
  name: string
  status: 'up' | 'degraded' | 'down'
  resources: ResourceStatusInfo[]
}

/**
 * Complete status page data response
 */
export interface StatusPageData {
  global_status: GlobalStatus
  generated_at: string
  resources: ResourceStatusInfo[]
  components?: ComponentStatusInfo[]
  settings?: StatusPageSettings
}

/**
 * Public monitor detail types based on /status/:id endpoint
 */

/**
 * Event type for recent events
 */
export type MonitorEventType = 'up' | 'down'

/**
 * Recent event in monitor timeline
 */
export interface MonitorRecentEvent {
  type: MonitorEventType
  timestamp: string
  duration: string | null
  reason: string
  details: string | null
}

/**
 * Uptime summary for different time periods
 */
export interface MonitorUptimeSummary {
  last_24_hours: number
  last_7_days: number
  last_30_days: number
  last_90_days: number
}

/**
 * Response time summary for 7 days
 */
export interface MonitorResponseTimeSummary {
  avg_ms: number
  min_ms: number
  max_ms: number
}

/**
 * Public monitor detail data
 */
export interface PublicMonitorDetail {
  id: string
  name: string
  current_status: ResourceCurrentStatus
  last_updated: string
  uptime_history_90_days: DailyStatus[]
  uptime_summary: MonitorUptimeSummary
  response_time_summary_7_days: MonitorResponseTimeSummary
  recent_events: MonitorRecentEvent[]
  maintenance?: MaintenanceBanner
}

// =====================================================================
// Spec 060 — Public Status Page DTOs
// =====================================================================

export type PublicVerdictStatus = 'operational' | 'partial_degradation' | 'major_outage'
export type PublicVerdictColor = 'green' | 'yellow' | 'orange' | 'red'

export interface PublicVerdict {
  status: PublicVerdictStatus
  label: string
  color: PublicVerdictColor
}

export type PublicAggregatedState = 'up' | 'degraded' | 'down' | 'maintenance' | 'unknown'

export interface PublicUptimeRibbonDay {
  day: string
  // null = no data for that day. The UI surfaces these as "unknown" cells,
  // and the 90-day average is computed over known days only.
  ratio: number | null
}

export interface PublicResourceSummary {
  id: string
  name: string
  host: string
  current_state: PublicAggregatedState
  uptime_90d_ratio: number
  uptime_ribbon: PublicUptimeRibbonDay[]
}

export interface PublicComponentSummary {
  id: string
  name: string
  aggregated_state: PublicAggregatedState
  resources: PublicResourceSummary[]
}

export type PublicIncidentSeverity = 'minor' | 'major' | 'critical'

export interface PublicIncidentSummary {
  id: string
  title: string
  started_at: string
  resolved_at: string | null
  severity: PublicIncidentSeverity
  component_id?: string
  resource_id?: string
}

export interface PublicBranding {
  name: string
  homepage_url?: string
  logo_url_light?: string
  logo_url_dark?: string
  favicon_url?: string
  primary_color?: string
}

export interface PublicUptimeWindow {
  earliest_day?: string
  latest_day: string
}

export interface PublicStatusSummary {
  generated_at: string
  branding: PublicBranding
  uptime_window: PublicUptimeWindow
  verdict: PublicVerdict
  components: PublicComponentSummary[]
  standalone_resources: PublicResourceSummary[]
  current_month_incidents: PublicIncidentSummary[]
}

export interface PublicIncidentMonth {
  year_month: string
  count: number
  incidents: PublicIncidentSummary[]
}

export interface PublicStatusIncidentsArchive {
  generated_at: string
  total: number
  months: PublicIncidentMonth[]
}

export interface PublicUptimeDay {
  day: string
  uptime_ratio: number
  samples: number
  incidents: number
  downtime_seconds: number
  related_incidents: PublicIncidentSummary[]
}

export interface PublicStatusUptimeRange {
  generated_at: string
  days: PublicUptimeDay[]
}

export type PublicIncidentUpdateStatus = 'investigating' | 'identified' | 'monitoring' | 'resolved'

export interface PublicIncidentUpdate {
  id: string
  status: PublicIncidentUpdateStatus
  message: string
  posted_at: string
}

export interface PublicIncidentDetail {
  id: string
  title: string
  severity: PublicIncidentSeverity
  started_at: string
  resolved_at: string | null
  component_id?: string
  resource_id?: string
  updates: PublicIncidentUpdate[]
}

export interface PublicResourceWindow {
  uptime_ratio: number
  incidents: number
}

export interface PublicStatusResourceWindows {
  id: string
  name: string
  windows: {
    '24h': PublicResourceWindow
    '7d': PublicResourceWindow
    '30d': PublicResourceWindow
    '90d': PublicResourceWindow
  }
  recent_incidents: PublicIncidentSummary[]
}

/**
 * Notification Channel Types
 */
export type NotificationChannelType = 'smtp' | 'slack' | 'sms' | 'webhook'

/**
 * SMTP Configuration for notification channel
 */
export interface SMTPConfig {
  host: string
  port: number
  username: string
  password: string
  sender: string
  recipients: string[]
  cc?: string[]
  bcc?: string[]
  subject?: string
}

/**
 * Slack Configuration for notification channel
 */
export interface SlackConfig {
  webhook_url: string
  channel?: string
  username?: string
}

/**
 * SMS Configuration for notification channel
 */
export interface SMSConfig {
  provider: string // e.g., twilio, nexmo
  account_sid?: string
  auth_token?: string
  from_number: string
  to_numbers: string[]
}

/**
 * Generic notification configuration
 */
export type NotificationConfig = SMTPConfig | SlackConfig | SMSConfig

/**
 * Notification Channel
 */
export interface NotificationChannel {
  id: string
  name: string
  type: NotificationChannelType
  config: NotificationConfig
  enabled_by_default: boolean
  last_sent_at?: string | null
  last_failure_at?: string | null
  failures_24h?: number
  created_at: string
  updated_at: string
}

export interface CreateNotificationChannel {
  name: string
  type: NotificationChannelType
  config: NotificationConfig
  enabled_by_default: boolean
}

export type UpdateNotificationChannel = Partial<CreateNotificationChannel>

export interface TestNotificationChannelConfig {
  type: NotificationChannelType
  config: NotificationConfig
}

// Maintenance windows
export type MaintenanceStrategy = 'one_time' | 'cron'
export type MaintenanceStatus = 'scheduled' | 'active' | 'finished' | 'cancelled'

export interface Maintenance {
  id: string
  title: string
  description?: string | null
  strategy: MaintenanceStrategy
  status: MaintenanceStatus
  start_at?: string | null
  end_at?: string | null
  cron_expr?: string | null
  window_minutes?: number | null
  timezone?: string | null
  effective_from?: string | null
  effective_until?: string | null
  resources?: Resource[]
  created_at?: string
  updated_at?: string
}

export interface CreateMaintenance {
  title: string
  description?: string | null
  strategy: MaintenanceStrategy
  start_at?: string
  end_at?: string
  cron_expr?: string
  window_minutes?: number
  timezone?: string
  effective_from?: string
  effective_until?: string
  resource_ids: string[]
}

export type UpdateMaintenance = Partial<CreateMaintenance>

// Public maintenance banner attached to monitor detail
export interface MaintenanceBanner {
  status: MaintenanceStatus // expected: 'active' | 'scheduled'
  title: string
  start_at?: string | null
  end_at?: string | null
  timezone?: string | null
}

// Status Page Settings Management
export type StatusPageLogoSlot = 'light' | 'dark' | 'favicon'

export type StatusPageThemeKey =
  | '--status-bg'
  | '--status-text'
  | '--status-up'
  | '--status-degraded'
  | '--status-down'
  | '--status-radius'

export type StatusPageThemeOverrides = Partial<Record<StatusPageThemeKey, string>>

export interface StatusPageSettingsRequest {
  name: string
  homepage_url: string
  custom_domain: string
  umami_website_id: string
  umami_script_url: string
  enable_details_page: boolean
  show_uptime_percentage: boolean
  hide_paused_monitors: boolean
  show_incident_history: boolean
  logo_url_light?: string
  logo_url_dark?: string
  favicon_url?: string
  primary_color?: string
  theme_overrides?: StatusPageThemeOverrides
}

export interface StatusPageDNSRecord {
  type: 'CNAME' | 'TXT'
  host: string
  value: string
  status: 'pending' | 'verified' | 'failed'
  last_error?: string | null
}

export interface StatusPageSettingsResponse {
  id: string
  name: string
  homepage_url: string
  custom_domain: string
  umami_website_id: string
  umami_script_url: string
  enable_details_page: boolean
  show_uptime_percentage: boolean
  hide_paused_monitors: boolean
  show_incident_history: boolean
  custom_domain_status: 'pending' | 'verified' | 'failed'
  custom_domain_ssl_status: 'none' | 'provisioning' | 'active'
  custom_domain_dns_records: StatusPageDNSRecord[]
  logo_url_light: string
  logo_url_dark: string
  favicon_url: string
  primary_color: string
  theme_overrides: StatusPageThemeOverrides
  created_at: string
  updated_at: string
}

/**
 * User profile
 */
export interface User {
  email: string
  name: string
  user_id: string
  force_password_change: boolean
  two_factor_enabled: boolean
}

/**
 * Component represents a logical grouping of resources
 * Component status is derived from member resource statuses
 */
export interface Component {
  id: string
  name: string
  description?: string
  status: 'up' | 'degraded' | 'down'
  impacted_resources: ComponentResourceSnapshot[]
  resources: ComponentResourceSnapshot[]
  created_at: string
  updated_at: string
  grouping_window_seconds?: number
}

/**
 * Snapshot of a resource within a component context
 */
export interface ComponentResourceSnapshot {
  id: string
  name: string
  status: ResourceCurrentStatus
}

export interface CreateComponent {
  name: string
  description?: string
  resource_ids: string[] // Required: at least one resource
  grouping_window_seconds?: number
}

export type UpdateComponent = Partial<Omit<CreateComponent, 'resource_ids'>>

// Extend UpdateComponent to also allow updating grouping_window_seconds
export interface UpdateComponentPayload extends UpdateComponent {
  grouping_window_seconds?: number
}

/**
 * Bulk operation payloads for component management
 */
export interface BulkAssignPayload {
  resource_ids: string[]
}

export interface BulkRemovePayload {
  resource_ids: string[]
}

export interface LiveStats {
  uptime_2h: number | null
  uptime_24h: number | null
  uptime_7d: number | null
  uptime_30d: number | null
  avg_response_time_24h: number | null
  last_response_time: number | null
}

export interface LiveActiveIncident {
  id: string
  started_at: string
  cause: string
}

export interface LiveSnapshot {
  resource: Resource
  stats: LiveStats
  active_incident: LiveActiveIncident | null
  recent_activities: MonitoringActivity[]
  fetched_at: string
}

// ─── Feature 028: Resource credentials (auth variants) ──────────────────────

/**
 * Protocol types that accept optional authentication credentials.
 */
export type CredentialProtocolType = 'redis' | 'mysql' | 'postgres'

/**
 * Request payload for POST /resources/{id}/credentials and .../credentials/test.
 * Password is plaintext on the wire; it is encrypted server-side at rest and
 * never returned by any subsequent read.
 */
export interface CredentialCreatePayload {
  username?: string
  password: string
  options?: Record<string, unknown>
}

/**
 * Response of GET / POST /resources/{id}/credentials.
 * `password` is always the mask string `••••••••` — the plaintext value is never
 * returned by any endpoint.
 */
export interface CredentialResponse {
  resource_id: string
  has_credentials: boolean
  username?: string
  password: string
  created_at: string
  updated_at: string
}

/**
 * Response of POST /resources/{id}/credentials/test.
 */
export interface TestConnectionResponse {
  status: 'ok' | 'failed'
  cause?: string
  latency_ms: number
}

// ─── Spec 069: Cross-cutting UI ─────────────────────────────────────────────

export type {
  NotificationCategory,
  NotificationSeverity,
  NotificationFeedItem,
  NotificationReadState,
} from './notifications'
export type { SearchResultCategory, SearchResult } from './searchPalette'
export type {
  KeyboardShortcutSection,
  KeyboardShortcutKind,
  KeyboardShortcut,
} from './keyboard'

// ─── Spec 070: Reports + Dashboards ─────────────────────────────────────────

export type {
  MonthlyReport,
  ReportStatus,
  ReportResourceBreakdown,
  ReportHistoryEntry,
} from './reports'
export type {
  WidgetTypeId,
  WidgetArchetype,
  WidgetDefinition,
  WidgetInstance,
  ResourceType,
  DashboardScopeMode,
  DashboardScope,
  DashboardTimeRange,
  DashboardRefreshInterval,
  DashboardVisibility,
  Dashboard,
  DashboardHealthStatus,
  DashboardHealth,
} from './dashboards'

// ─── Spec 071: Toolbox + Metrics ────────────────────────────────────────────

export type {
  DnsRecordType,
  DnsResolver,
  DnsLookupRequest,
  DnsRecord,
  DnsLookupResponse,
  PortPreset,
  PortStatus,
  PortScanRequest,
  PortResult,
  PortScanResponse,
  SslCertificate,
  SslVulnCheck,
  SslCheckRequest,
  SslCheckResponse,
  WhoisRequest,
  WhoisResponse,
  DnsHistoryEntry,
} from './toolbox'

export type {
  DiskUsage,
  Host,
  HostMetricSample,
  HostCredentialResult,
  RegisterHostResult,
  HostMetricRange,
  MonitorSummary,
} from './host'
