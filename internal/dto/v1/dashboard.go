package v1

// Dashboards DTOs. camelCase, mirrors the frozen frontend
// `Dashboard` / `WidgetInstance` / `DashboardScope` shapes.

// DashboardScopePayload is the selection payload for a scope mode.
// @name DashboardScopePayload
type DashboardScopePayload struct {
	TagIDs       []string `json:"tagIds,omitempty"`
	ComponentIDs []string `json:"componentIds,omitempty"`
	Types        []string `json:"types,omitempty"`
	ResourceIDs  []string `json:"resourceIds,omitempty"`
}

// DashboardScope selects which resources a dashboard covers.
// @name DashboardScope
type DashboardScope struct {
	Mode    string                `json:"mode"`
	Payload DashboardScopePayload `json:"payload"`
}

// WidgetInstance is one placed widget within a dashboard.
// @name WidgetInstance
type WidgetInstance struct {
	ID           string         `json:"id"`
	WidgetTypeID string         `json:"widgetTypeId"`
	Position     int            `json:"position"`
	Title        *string        `json:"title,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
}

// DashboardResponse is a persisted dashboard.
// @name DashboardResponse
type DashboardResponse struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Scope            DashboardScope   `json:"scope"`
	Widgets          []WidgetInstance `json:"widgets"`
	DefaultTimeRange string           `json:"defaultTimeRange"`
	RefreshInterval  string           `json:"refreshInterval"`
	Visibility       string           `json:"visibility"`
	OwnerID          string           `json:"ownerId"`
	OwnerName        string           `json:"ownerName"`
	CreatedAt        string           `json:"createdAt"`
	UpdatedAt        string           `json:"updatedAt"`
}

// CreateDashboardRequest is the body of POST /api/v1/dashboards.
// @name CreateDashboardRequest
type CreateDashboardRequest struct {
	Name             string           `json:"name"`
	Scope            DashboardScope   `json:"scope"`
	Widgets          []WidgetInstance `json:"widgets"`
	DefaultTimeRange string           `json:"defaultTimeRange"`
	RefreshInterval  string           `json:"refreshInterval"`
	Visibility       string           `json:"visibility"`
}

// UpdateDashboardRequest is the body of PATCH /api/v1/dashboards/{id}.
// Every field is optional — only provided fields are applied (partial patch).
// @name UpdateDashboardRequest
type UpdateDashboardRequest struct {
	Name             *string          `json:"name,omitempty"`
	Scope            *DashboardScope  `json:"scope,omitempty"`
	Widgets          []WidgetInstance `json:"widgets,omitempty"`
	DefaultTimeRange *string          `json:"defaultTimeRange,omitempty"`
	RefreshInterval  *string          `json:"refreshInterval,omitempty"`
	Visibility       *string          `json:"visibility,omitempty"`
}

// SaveLayoutRequest is the body of PUT /api/v1/dashboards/{id}/layout.
// @name SaveLayoutRequest
type SaveLayoutRequest struct {
	Widgets []WidgetInstance `json:"widgets"`
}
