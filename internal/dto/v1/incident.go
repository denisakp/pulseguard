package v1

import "github.com/denisakp/ogoune/internal/domain"

// IncidentResponse is the v1 API representation of an incident.
//
// It is a superset of the legacy root shape: it keeps the v1-native `monitor_id`
// + derived `status`, and also carries the rich fields the frontend renders
// (`resource_id`, the embedded `resource`, `details`, `event_steps`,
// `diagnostics`, `updated_at`) so the migration needs no frontend type change.
// The heavy fields (`resource`, `event_steps`, `diagnostics`) are populated on
// the detail endpoint; the list endpoint hydrates `resource` only.
// @name IncidentResponse
type IncidentResponse struct {
	ID          string                      `json:"id"`
	MonitorID   string                      `json:"monitor_id"`
	ResourceID  string                      `json:"resource_id"`
	Resource    domain.Resource             `json:"resource"`
	Cause       string                      `json:"cause"`
	Status      string                      `json:"status"` // "open" or "resolved"
	Details     string                      `json:"details"`
	EventSteps  []domain.IncidentEventStep  `json:"event_steps"`
	Diagnostics *domain.IncidentDiagnostics `json:"diagnostics"`
	StartedAt   string                      `json:"started_at"`
	ResolvedAt  *string                     `json:"resolved_at"`
	CreatedAt   string                      `json:"created_at"`
	UpdatedAt   string                      `json:"updated_at"`
}

// IncidentListFilters holds validated query parameters for listing incidents.
type IncidentListFilters struct {
	MonitorID string // optional ULID filter
	Status    string // "open", "resolved", or "" (all)
}
