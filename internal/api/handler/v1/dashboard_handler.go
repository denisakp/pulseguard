package v1

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

// DashboardV1ServiceInterface is the slice of *DashboardService used by the handler.
type DashboardV1ServiceInterface interface {
	List(ctx context.Context, limit, offset int) ([]*domain.Dashboard, error)
	Get(ctx context.Context, id string) (*domain.Dashboard, error)
	Create(ctx context.Context, userID string, d *domain.Dashboard) (*domain.Dashboard, error)
	Update(ctx context.Context, userID, id string, patch service.DashboardUpdate) (*domain.Dashboard, error)
	SaveLayout(ctx context.Context, userID, id string, widgets []domain.WidgetInstance) (*domain.Dashboard, error)
	Delete(ctx context.Context, userID, id string) error
}

// DashboardHandler exposes /api/v1/dashboards.
type DashboardHandler struct {
	service DashboardV1ServiceInterface
}

func NewDashboardHandler(svc DashboardV1ServiceInterface) *DashboardHandler {
	return &DashboardHandler{service: svc}
}

// List handles GET /api/v1/dashboards.
//
// @Summary  List all dashboards (instance-wide, newest-updated first)
// @Tags     dashboards
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} map[string]interface{}
// @Failure  401 {object} dtoV1.ErrorResponse
// @Router   /dashboards [get]
func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context(), 200, 0)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list dashboards")
		return
	}
	data := make([]dtoV1.DashboardResponse, len(items))
	for i, d := range items {
		data[i] = mapDashboard(d)
	}
	respond(w, http.StatusOK, data)
}

// Get handles GET /api/v1/dashboards/{id}.
//
// @Summary  Get a dashboard
// @Tags     dashboards
// @Security BearerAuth
// @Produce  json
// @Param    id path string true "Dashboard ID"
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.DashboardResponse]
// @Failure  404 {object} dtoV1.ErrorResponse
// @Router   /dashboards/{id} [get]
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.service.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.mapDashboardError(w, r, err)
		return
	}
	respond(w, http.StatusOK, mapDashboard(d))
}

// Create handles POST /api/v1/dashboards.
//
// @Summary  Create a dashboard (owner = caller)
// @Tags     dashboards
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body dtoV1.CreateDashboardRequest true "Dashboard"
// @Success  201 {object} dtoV1.SingleResponse[dtoV1.DashboardResponse]
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  422 {object} dtoV1.ErrorResponse
// @Router   /dashboards [post]
func (h *DashboardHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.CreateDashboardRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	d := &domain.Dashboard{
		Name:             req.Name,
		Scope:            scopeFromDTO(req.Scope),
		Widgets:          widgetsFromDTO(req.Widgets),
		DefaultTimeRange: req.DefaultTimeRange,
		RefreshInterval:  req.RefreshInterval,
		Visibility:       req.Visibility,
	}
	created, err := h.service.Create(r.Context(), userIDFromContext(r), d)
	if err != nil {
		h.mapDashboardError(w, r, err)
		return
	}
	respond(w, http.StatusCreated, mapDashboard(created))
}

// Update handles PATCH /api/v1/dashboards/{id} (partial patch, owner-only).
//
// @Summary  Update a dashboard (partial patch; owner only)
// @Tags     dashboards
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string                        true "Dashboard ID"
// @Param    body body dtoV1.UpdateDashboardRequest   true "Partial patch"
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.DashboardResponse]
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  404 {object} dtoV1.ErrorResponse
// @Failure  422 {object} dtoV1.ErrorResponse
// @Router   /dashboards/{id} [patch]
func (h *DashboardHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.UpdateDashboardRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	patch := service.DashboardUpdate{
		Name:             req.Name,
		DefaultTimeRange: req.DefaultTimeRange,
		RefreshInterval:  req.RefreshInterval,
		Visibility:       req.Visibility,
	}
	if req.Scope != nil {
		s := scopeFromDTO(*req.Scope)
		patch.Scope = &s
	}
	if req.Widgets != nil {
		patch.Widgets = widgetsFromDTO(req.Widgets)
		patch.WidgetsSet = true
	}
	updated, err := h.service.Update(r.Context(), userIDFromContext(r), chi.URLParam(r, "id"), patch)
	if err != nil {
		h.mapDashboardError(w, r, err)
		return
	}
	respond(w, http.StatusOK, mapDashboard(updated))
}

// SaveLayout handles PUT /api/v1/dashboards/{id}/layout (widgets-only, owner-only).
//
// @Summary  Save a dashboard's widget layout (owner only)
// @Tags     dashboards
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string                  true "Dashboard ID"
// @Param    body body dtoV1.SaveLayoutRequest  true "Ordered widgets"
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.DashboardResponse]
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  404 {object} dtoV1.ErrorResponse
// @Router   /dashboards/{id}/layout [put]
func (h *DashboardHandler) SaveLayout(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.SaveLayoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	updated, err := h.service.SaveLayout(r.Context(), userIDFromContext(r), chi.URLParam(r, "id"), widgetsFromDTO(req.Widgets))
	if err != nil {
		h.mapDashboardError(w, r, err)
		return
	}
	respond(w, http.StatusOK, mapDashboard(updated))
}

// Delete handles DELETE /api/v1/dashboards/{id} (owner-only).
//
// @Summary  Delete a dashboard (owner only)
// @Tags     dashboards
// @Security BearerAuth
// @Param    id path string true "Dashboard ID"
// @Success  204
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  404 {object} dtoV1.ErrorResponse
// @Router   /dashboards/{id} [delete]
func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	err := h.service.Delete(r.Context(), userIDFromContext(r), chi.URLParam(r, "id"))
	if err != nil {
		h.mapDashboardError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DashboardHandler) mapDashboardError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrDashboardNotFound):
		respondError(w, r, http.StatusNotFound, "DASHBOARD_NOT_FOUND", "dashboard not found")
	case errors.Is(err, service.ErrDashboardForbidden):
		respondError(w, r, http.StatusForbidden, "FORBIDDEN", "only the owner can modify this dashboard")
	case errors.Is(err, service.ErrDashboardValidation):
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
	default:
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "dashboard request failed")
	}
}

// --- domain ↔ DTO mappers ---

func mapDashboard(d *domain.Dashboard) dtoV1.DashboardResponse {
	widgets := make([]dtoV1.WidgetInstance, len(d.Widgets))
	for i, w := range d.Widgets {
		widgets[i] = dtoV1.WidgetInstance{ID: w.ID, WidgetTypeID: w.WidgetTypeID, Position: w.Position, Title: w.Title, Config: w.Config}
	}
	return dtoV1.DashboardResponse{
		ID:   d.ID,
		Name: d.Name,
		Scope: dtoV1.DashboardScope{
			Mode: d.Scope.Mode,
			Payload: dtoV1.DashboardScopePayload{
				TagIDs: d.Scope.Payload.TagIDs, ComponentIDs: d.Scope.Payload.ComponentIDs,
				Types: d.Scope.Payload.Types, ResourceIDs: d.Scope.Payload.ResourceIDs,
			},
		},
		Widgets:          widgets,
		DefaultTimeRange: d.DefaultTimeRange,
		RefreshInterval:  d.RefreshInterval,
		Visibility:       d.Visibility,
		OwnerID:          d.OwnerID,
		OwnerName:        d.OwnerName,
		CreatedAt:        d.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        d.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func scopeFromDTO(s dtoV1.DashboardScope) domain.DashboardScope {
	return domain.DashboardScope{
		Mode: s.Mode,
		Payload: domain.DashboardScopePayload{
			TagIDs: s.Payload.TagIDs, ComponentIDs: s.Payload.ComponentIDs,
			Types: s.Payload.Types, ResourceIDs: s.Payload.ResourceIDs,
		},
	}
}

func widgetsFromDTO(in []dtoV1.WidgetInstance) []domain.WidgetInstance {
	out := make([]domain.WidgetInstance, len(in))
	for i, w := range in {
		out[i] = domain.WidgetInstance{ID: w.ID, WidgetTypeID: w.WidgetTypeID, Position: w.Position, Title: w.Title, Config: w.Config}
	}
	return out
}
