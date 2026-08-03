package v1

// IncidentUpdateResponse is the v1 API representation of an incident timeline update.
// @name IncidentUpdateResponse
type IncidentUpdateResponse struct {
	ID         string `json:"id"`
	IncidentID string `json:"incident_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	PostedBy   string `json:"posted_by,omitempty"`
	PostedAt   string `json:"posted_at"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// IncidentUpdateRequest is the request body for creating/updating an incident update.
// @name IncidentUpdateRequest
type IncidentUpdateRequest struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
