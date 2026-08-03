package v1

// SearchResult is a single unified command-palette hit (spec 084).
// @name SearchResult
type SearchResult struct {
	// ID is a stable, category-prefixed id: "resource:<id>" | "incident:<id>" | "page:<label>".
	ID string `json:"id"`
	// Category is one of: resource | incident | page.
	Category string `json:"category"`
	// Label is the primary human-readable text (monitor name, incident cause, page name).
	Label string `json:"label"`
	// Meta is short context (e.g. a monitor's target), omitted when empty.
	Meta string `json:"meta,omitempty"`
	// DeepLink is the in-app path that opens the item (e.g. /resources/<id>).
	DeepLink string `json:"deep_link"`
}

// SearchResponse is the payload of GET /api/v1/search.
// @name SearchResponse
type SearchResponse struct {
	Results         []SearchResult `json:"results"`
	Total           int            `json:"total"`
	QueryDurationMs int64          `json:"query_duration_ms"`
}
