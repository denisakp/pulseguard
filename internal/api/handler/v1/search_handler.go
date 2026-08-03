package v1

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
)

const (
	searchMinQueryLen  = 2
	searchDefaultLimit = 30
	searchMaxLimit     = 100
)

// SearchV1ServiceInterface is the search service surface used by the handler.
type SearchV1ServiceInterface interface {
	Search(ctx context.Context, q string, opts service.SearchOpts) (dtoV1.SearchResponse, error)
}

// SearchHandler serves the command-palette search endpoint (spec 084).
type SearchHandler struct {
	service SearchV1ServiceInterface
}

func NewSearchHandler(svc SearchV1ServiceInterface) *SearchHandler {
	return &SearchHandler{service: svc}
}

// List handles GET /api/v1/search
//
// @Summary     Search monitors, incidents, and pages
// @Description Command-palette search across monitors (name/target), incidents (cause), and app pages. Read-only.
// @Tags        search
// @Security    BearerAuth
// @Produce     json
// @Param       q          query string true  "Search query (min 2 characters, matched literally)"
// @Param       limit      query int    false "Max results (default 30, max 100)"
// @Param       categories query string false "Comma-separated filter: resource,incident,page (default all)"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.SearchResponse]
// @Failure     401 {object} dtoV1.ErrorResponse
// @Failure     422 {object} dtoV1.ErrorResponse
// @Router      /search [get]
func (h *SearchHandler) List(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < searchMinQueryLen {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED",
			"query must be at least 2 characters",
			dtoV1.FieldError{Field: "q", Message: "must be at least 2 characters"})
		return
	}

	limit := searchDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	var cats []string
	if c := strings.TrimSpace(r.URL.Query().Get("categories")); c != "" {
		for _, part := range strings.Split(c, ",") {
			if p := strings.TrimSpace(part); p != "" {
				cats = append(cats, p)
			}
		}
	}

	resp, err := h.service.Search(r.Context(), q, service.SearchOpts{Limit: limit, Categories: cats})
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
		return
	}
	respond(w, http.StatusOK, resp)
}
