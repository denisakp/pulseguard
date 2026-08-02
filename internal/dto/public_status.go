// Package dto — public status page DTOs.
// These shapes are wire-stable and consumed by the public Vue bundle plus any
// third-party integration. They MUST stay in sync with
// `specs/060-prd-008-status-pages/contracts/public-status-api.md`.
package dto

import "time"

type PublicVerdictStatus string

const (
	VerdictOperational        PublicVerdictStatus = "operational"
	VerdictPartialDegradation PublicVerdictStatus = "partial_degradation"
	VerdictMajorOutage        PublicVerdictStatus = "major_outage"
)

type PublicAggregatedState string

const (
	PublicStateUp          PublicAggregatedState = "up"
	PublicStateDegraded    PublicAggregatedState = "degraded"
	PublicStateDown        PublicAggregatedState = "down"
	PublicStateMaintenance PublicAggregatedState = "maintenance"
	PublicStateUnknown     PublicAggregatedState = "unknown"
)

type PublicIncidentSeverity string

const (
	PublicSeverityMinor    PublicIncidentSeverity = "minor"
	PublicSeverityMajor    PublicIncidentSeverity = "major"
	PublicSeverityCritical PublicIncidentSeverity = "critical"
)

type PublicVerdict struct {
	Status PublicVerdictStatus `json:"status"`
	Label  string              `json:"label"`
	Color  string              `json:"color"`
}

type PublicRibbonEntry struct {
	Day   string   `json:"day"`
	Ratio *float64 `json:"ratio"`
}

type PublicResource struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Host           string                `json:"host"`
	CurrentState   PublicAggregatedState `json:"current_state"`
	Uptime90dRatio float64               `json:"uptime_90d_ratio"`
	UptimeRibbon   []PublicRibbonEntry   `json:"uptime_ribbon"`
}

type PublicComponent struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	AggregatedState PublicAggregatedState `json:"aggregated_state"`
	Resources       []PublicResource      `json:"resources"`
}

type PublicIncidentSummary struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	StartedAt   time.Time              `json:"started_at"`
	ResolvedAt  *time.Time             `json:"resolved_at"`
	Severity    PublicIncidentSeverity `json:"severity"`
	ComponentID string                 `json:"component_id,omitempty"`
	ResourceID  string                 `json:"resource_id,omitempty"`
}

type PublicBranding struct {
	Name           string `json:"name"`
	HomepageURL    string `json:"homepage_url,omitempty"`
	LogoURLLight   string `json:"logo_url_light,omitempty"`
	LogoURLDark    string `json:"logo_url_dark,omitempty"`
	FaviconURL     string `json:"favicon_url,omitempty"`
	PrimaryColor   string `json:"primary_color,omitempty"`
	UmamiWebsiteID string `json:"umami_website_id,omitempty"`
	UmamiScriptURL string `json:"umami_script_url,omitempty"`
}

type PublicUptimeWindow struct {
	// EarliestDay is the first day for which any uptime data exists, in
	// YYYY-MM-DD UTC. Empty when no data yet.
	EarliestDay string `json:"earliest_day,omitempty"`
	// LatestDay is the most recent day we can report on (today UTC).
	LatestDay string `json:"latest_day"`
}

type PublicStatus struct {
	GeneratedAt           time.Time               `json:"generated_at"`
	Branding              PublicBranding          `json:"branding"`
	Verdict               PublicVerdict           `json:"verdict"`
	UptimeWindow          PublicUptimeWindow      `json:"uptime_window"`
	Components            []PublicComponent       `json:"components"`
	StandaloneResources   []PublicResource        `json:"standalone_resources"`
	CurrentMonthIncidents []PublicIncidentSummary `json:"current_month_incidents"`
}

type PublicIncidentMonth struct {
	YearMonth string                  `json:"year_month"`
	Count     int                     `json:"count"`
	Incidents []PublicIncidentSummary `json:"incidents"`
}

type PublicIncidentsResponse struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Total       int                   `json:"total"`
	Months      []PublicIncidentMonth `json:"months"`
}

type PublicUptimeDay struct {
	Day              string                  `json:"day"`
	UptimeRatio      float64                 `json:"uptime_ratio"`
	Samples          int                     `json:"samples"`
	Incidents        int                     `json:"incidents"`
	DowntimeSeconds  int                     `json:"downtime_seconds"`
	RelatedIncidents []PublicIncidentSummary `json:"related_incidents"`
}

type PublicUptimeResponse struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Days        []PublicUptimeDay `json:"days"`
}

type PublicWindowStats struct {
	UptimeRatio float64 `json:"uptime_ratio"`
	Incidents   int     `json:"incidents"`
}

// ---------- US7: Incident detail page (Claude-style timeline) ----------

type PublicIncidentUpdateStatus string

const (
	PublicUpdateInvestigating PublicIncidentUpdateStatus = "investigating"
	PublicUpdateIdentified    PublicIncidentUpdateStatus = "identified"
	PublicUpdateMonitoring    PublicIncidentUpdateStatus = "monitoring"
	PublicUpdateResolved      PublicIncidentUpdateStatus = "resolved"
)

type PublicIncidentUpdate struct {
	ID       string                     `json:"id"`
	Status   PublicIncidentUpdateStatus `json:"status"`
	Message  string                     `json:"message"`
	PostedAt time.Time                  `json:"posted_at"`
}

type PublicIncidentDetail struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Severity    PublicIncidentSeverity `json:"severity"`
	StartedAt   time.Time              `json:"started_at"`
	ResolvedAt  *time.Time             `json:"resolved_at"`
	ComponentID string                 `json:"component_id,omitempty"`
	ResourceID  string                 `json:"resource_id,omitempty"`
	Updates     []PublicIncidentUpdate `json:"updates"`
}

type PublicResourceWindowsResponse struct {
	ID              string                       `json:"id"`
	Name            string                       `json:"name"`
	Windows         map[string]PublicWindowStats `json:"windows"`
	Daily30d        []PublicRibbonEntry          `json:"daily_30d"`
	RecentIncidents []PublicIncidentSummary      `json:"recent_incidents"`
}
