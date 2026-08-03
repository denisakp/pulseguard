package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
)

// ComponentV1RepositoryInterface is the read surface over the component repository
// used by the read-only status-page handler (components back status pages).
type ComponentV1RepositoryInterface interface {
	List(ctx context.Context, limit, offset int) ([]*domain.Component, error)
	FindByID(ctx context.Context, id string) (*domain.Component, error)
}

// StatusPageV1Handler handles read-only status page list via the component repository.
type StatusPageV1Handler struct {
	repo ComponentV1RepositoryInterface
}

// NewStatusPageV1Handler creates a new StatusPageV1Handler.
func NewStatusPageV1Handler(repo ComponentV1RepositoryInterface) *StatusPageV1Handler {
	return &StatusPageV1Handler{repo: repo}
}

// mapStatusPageResponse maps a domain.Component to a v1 StatusPageResponse.
// The overall_status is derived from LastNotificationStatus.
func mapStatusPageResponse(c *domain.Component) dtoV1.StatusPageResponse {
	return dtoV1.StatusPageResponse{
		ID:            c.ID,
		Name:          c.Name,
		Description:   c.Description,
		OverallStatus: string(c.LastNotificationStatus),
		CreatedAt:     c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// List handles GET /api/v1/status-pages
//
// @Summary     List status pages (component-based, read-only)
// @Tags        status-pages
// @Security    BearerAuth
// @Produce     json
// @Param       page     query int false "Page number (default 1)"
// @Param       per_page query int false "Items per page (1-100, default 20)"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} dtoV1.ErrorResponse
// @Router      /status-pages [get]
func (h *StatusPageV1Handler) List(w http.ResponseWriter, r *http.Request) {
	params, errs := parsePagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}

	offset := (params.Page - 1) * params.PerPage
	items, err := h.repo.List(r.Context(), params.PerPage, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list status pages")
		return
	}

	all, err := h.repo.List(r.Context(), 10000, 0)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to count status pages")
		return
	}

	data := make([]dtoV1.StatusPageResponse, 0, len(items))
	for _, c := range items {
		data = append(data, mapStatusPageResponse(c))
	}

	respondPaginated(w, data, dtoV1.MetaResponse{
		Page:    params.Page,
		PerPage: params.PerPage,
		Total:   len(all),
	})
}
