package v1

// TagResponse is the v1 API representation of a tag.
// @name TagResponse
type TagResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// CreateTagRequest is the request body for POST /api/v1/tags.
// @name CreateTagRequest
type CreateTagRequest struct {
	Name        string  `json:"name"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

// UpdateTagRequest is the request body for PUT /api/v1/tags/:id.
// All fields are optional (PATCH semantics).
// @name UpdateTagRequest
type UpdateTagRequest struct {
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}
