package v1

// CreateComponentRequest is the request body for POST /api/v1/components.
// ResourceIDs is required (at least one) — a component groups resources.
// @name CreateComponentRequest
type CreateComponentRequest struct {
	Name                  string   `json:"name"`
	Description           *string  `json:"description,omitempty"`
	ResourceIDs           []string `json:"resource_ids"`
	GroupingWindowSeconds *int     `json:"grouping_window_seconds,omitempty"`
}

// UpdateComponentRequest is the request body for PUT/PATCH /api/v1/components/:id.
// All fields are optional (partial update).
// @name UpdateComponentRequest
type UpdateComponentRequest struct {
	Name                  *string `json:"name,omitempty"`
	Description           *string `json:"description,omitempty"`
	GroupingWindowSeconds *int    `json:"grouping_window_seconds,omitempty"`
}

// BulkResourceIDsRequest is the request body for the component bulk assign/remove endpoints.
// @name BulkResourceIDsRequest
type BulkResourceIDsRequest struct {
	ResourceIDs []string `json:"resource_ids"`
}
