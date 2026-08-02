package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

// MonitorV1ServiceInterface defines the resource service methods used by the v1 monitor handler.
type MonitorV1ServiceInterface interface {
	ListActiveResources(ctx context.Context, limit, offset int) ([]*domain.Resource, error)
	ListAll(ctx context.Context) ([]*domain.Resource, error)
	ListByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error)
	GetResourceByID(ctx context.Context, id string) (*domain.Resource, error)
	CreateResource(ctx context.Context, payload *dto.CreateResourcePayload) (*domain.Resource, error)
	UpdateResource(ctx context.Context, id string, payload *dto.UpdateResourcePayload) (*domain.Resource, error)
	DeleteResource(ctx context.Context, resourceID string) error
	PauseMonitoring(ctx context.Context, resourceID string) error
	ResumeMonitoring(ctx context.Context, resourceID string) error
}

// MonitorHandler handles v1 CRUD and lifecycle endpoints for monitors.
type MonitorHandler struct {
	service MonitorV1ServiceInterface
}

// NewMonitorHandler creates a new MonitorHandler with the given service.
func NewMonitorHandler(svc MonitorV1ServiceInterface) *MonitorHandler {
	return &MonitorHandler{service: svc}
}

// mapMonitorResponse maps a domain.Resource to a v1 MonitorResponse.
func mapMonitorResponse(r *domain.Resource) dtoV1.MonitorResponse {
	tags := make([]string, 0, len(r.Tags))
	for _, t := range r.Tags {
		if t != nil {
			tags = append(tags, t.Name)
		}
	}
	var lastCheckedAt interface{}
	if r.LastChecked != nil {
		lastCheckedAt = r.LastChecked.UTC().Format(time.RFC3339)
	}
	return dtoV1.MonitorResponse{
		ID:            r.ID,
		Name:          r.Name,
		Type:          string(r.Type),
		Target:        r.Target,
		Interval:      r.Interval,
		Timeout:       r.Timeout,
		Status:        string(r.Status),
		LastCheckedAt: lastCheckedAt,
		ComponentID:   r.ComponentID,
		HostID:        r.HostID,
		Tags:          tags,
		CreatedAt:     r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// List handles GET /api/v1/monitors
//
// @Summary     List monitors
// @Tags        monitors
// @Security    BearerAuth
// @Produce     json
// @Param       page      query int    false "Page number (default 1)"
// @Param       per_page  query int    false "Items per page (1-100, default 20)"
// @Param       tag       query string false "Filter by exact tag name"
// @Param       type      query string false "Filter by monitor type (http|tcp|dns|icmp|keyword|protocol|heartbeat)"
// @Param       is_active query bool   false "Filter by active state (default true)"
// @Param       q         query string false "Substring search on name + target (case-insensitive)"
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} dtoV1.ErrorResponse
// @Failure     401 {object} dtoV1.ErrorResponse
// @Failure     422 {object} dtoV1.ErrorResponse
// @Router      /monitors [get]
func (h *MonitorHandler) List(w http.ResponseWriter, r *http.Request) {
	params, errs := parsePagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}

	filter, ferrs := parseMonitorFilter(r)
	if len(ferrs) > 0 {
		respondError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "invalid filter parameters", ferrs...)
		return
	}

	items, total, err := h.service.ListByFilter(r.Context(), filter, params.Page, params.PerPage)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list monitors")
		return
	}

	data := make([]dtoV1.MonitorResponse, 0, len(items))
	for _, res := range items {
		data = append(data, mapMonitorResponse(res))
	}

	respondPaginated(w, data, dtoV1.MetaResponse{
		Page:    params.Page,
		PerPage: params.PerPage,
		Total:   total,
	})
}

// Get handles GET /api/v1/monitors/{id}
//
// @Summary     Get a monitor by ID
// @Tags        monitors
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Monitor ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MonitorResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id} [get]
func (h *MonitorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := h.service.GetResourceByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get monitor")
		return
	}
	respond(w, http.StatusOK, mapMonitorResponse(res))
}

// Create handles POST /api/v1/monitors
//
// @Summary     Create a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.CreateMonitorRequest true "Monitor payload"
// @Success     201 {object} dtoV1.SingleResponse[dtoV1.MonitorResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors [post]
func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.CreateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	var fieldErrs []dtoV1.FieldError
	if strings.TrimSpace(req.Name) == "" {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "name", Message: "required"})
	}
	if strings.TrimSpace(req.Type) == "" {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "type", Message: "required"})
	}
	if strings.TrimSpace(req.Target) == "" {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "target", Message: "required"})
	}
	if req.Interval <= 0 {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "interval", Message: "must be greater than 0"})
	}
	if req.Timeout <= 0 {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "timeout", Message: "must be greater than 0"})
	}
	if len(fieldErrs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed", fieldErrs...)
		return
	}

	payload := &dto.CreateResourcePayload{
		Name:         req.Name,
		Type:         domain.ResourceType(req.Type),
		Target:       req.Target,
		Interval:     req.Interval,
		Timeout:      req.Timeout,
		Tags:         req.Tags,
		ComponentID:  req.ComponentID,
		Keyword:      req.Keyword,
		ProtocolType: req.ProtocolType,
		ProtocolPort: req.ProtocolPort,
	}

	created, err := h.service.CreateResource(r.Context(), payload)
	if err != nil {
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create monitor")
		return
	}
	respond(w, http.StatusCreated, mapMonitorResponse(created))
}

// Update handles PUT /api/v1/monitors/{id}
//
// @Summary     Update a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Monitor ID"
// @Param       body body dtoV1.UpdateMonitorRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MonitorResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id} [put]
func (h *MonitorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dtoV1.UpdateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	payload := &dto.UpdateResourcePayload{
		Name:         req.Name,
		Target:       req.Target,
		Interval:     req.Interval,
		Timeout:      req.Timeout,
		ComponentID:  req.ComponentID,
		Keyword:      req.Keyword,
		ProtocolType: req.ProtocolType,
		ProtocolPort: req.ProtocolPort,
	}
	if req.Type != nil {
		t := domain.ResourceType(*req.Type)
		payload.Type = &t
	}
	if req.Tags != nil {
		payload.Tags = &req.Tags
	}

	updated, err := h.service.UpdateResource(r.Context(), id, payload)
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update monitor")
		return
	}
	respond(w, http.StatusOK, mapMonitorResponse(updated))
}

// Delete handles DELETE /api/v1/monitors/{id}
//
// @Summary     Delete a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Param       id path string true "Monitor ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id} [delete]
func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteResource(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete monitor")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Pause handles POST /api/v1/monitors/{id}/pause
//
// @Summary     Pause a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Param       id path string true "Monitor ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MonitorResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/pause [post]
func (h *MonitorHandler) Pause(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.PauseMonitoring(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to pause monitor")
		return
	}
	res, err := h.service.GetResourceByID(r.Context(), id)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get updated monitor")
		return
	}
	respond(w, http.StatusOK, mapMonitorResponse(res))
}

// Resume handles POST /api/v1/monitors/{id}/resume
//
// @Summary     Resume a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Param       id path string true "Monitor ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MonitorResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/resume [post]
func (h *MonitorHandler) Resume(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.ResumeMonitoring(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resume monitor")
		return
	}
	res, err := h.service.GetResourceByID(r.Context(), id)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get updated monitor")
		return
	}
	respond(w, http.StatusOK, mapMonitorResponse(res))
}
