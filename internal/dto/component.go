package dto

import "github.com/denisakp/ogoune/internal/domain"

// CreateComponentPayload represents the input to create a component.
// ResourceIDs is required and must contain at least one resource.
type CreateComponentPayload struct {
	GroupingWindowSeconds *int     `json:"grouping_window_seconds,omitempty"`
	Name                  string   `json:"name"`
	Description           *string  `json:"description,omitempty"`
	ResourceIDs           []string `json:"resource_ids"` // Required: minimum 1 resource
}

// UpdateComponentPayload represents partial updates to a component.
type UpdateComponentPayload struct {
	Name                  *string `json:"name,omitempty"`
	Description           *string `json:"description,omitempty"`
	GroupingWindowSeconds *int    `json:"grouping_window_seconds,omitempty"`
}

// BulkAssignPayload assigns multiple resources to a component.
type BulkAssignPayload struct {
	ResourceIDs []string `json:"resource_ids"`
}

// BulkRemovePayload removes multiple resources from their components.
type BulkRemovePayload struct {
	ResourceIDs []string `json:"resource_ids"`
}

// ComponentResourceSnapshot is a lightweight view of a resource inside a component.
type ComponentResourceSnapshot struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Status domain.ResourceStatus `json:"status"`
}

// ComponentResponse returns component metadata and derived status.
type ComponentResponse struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	Description           *string                     `json:"description,omitempty"`
	Status                domain.ComponentStatus      `json:"status"`
	ImpactedResources     []ComponentResourceSnapshot `json:"impacted_resources"`
	Resources             []ComponentResourceSnapshot `json:"resources"`
	GroupingWindowSeconds int                         `json:"grouping_window_seconds"`
	CreatedAt             string                      `json:"created_at"`
	UpdatedAt             string                      `json:"updated_at"`
}
