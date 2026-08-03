package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/denisakp/ogoune/internal/dto"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

// ComponentV1ServiceInterface is the component service surface used by the v1 handler.
// It mirrors the root handler's service dependency so v1 recovers the full behavior
// (derived status, resource membership, grouping window, delete guard, bulk ops).
type ComponentV1ServiceInterface interface {
	CreateComponent(ctx context.Context, payload *dto.CreateComponentPayload) (*dto.ComponentResponse, error)
	UpdateComponent(ctx context.Context, id string, payload *dto.UpdateComponentPayload) (*dto.ComponentResponse, error)
	DeleteComponent(ctx context.Context, id string) error
	GetComponent(ctx context.Context, id string) (*dto.ComponentResponse, error)
	ListComponents(ctx context.Context, limit, offset int) ([]*dto.ComponentResponse, error)
	BulkAssignToComponent(ctx context.Context, componentID string, payload *dto.BulkAssignPayload) error
	BulkRemoveFromComponent(ctx context.Context, payload *dto.BulkRemovePayload) error
}

// ComponentHandler handles v1 CRUD + membership endpoints for components.
type ComponentHandler struct {
	service ComponentV1ServiceInterface
}

// NewComponentHandler creates a new ComponentHandler.
func NewComponentHandler(svc ComponentV1ServiceInterface) *ComponentHandler {
	return &ComponentHandler{service: svc}
}

// List handles GET /api/v1/components
//
// @Summary     List components
// @Tags        components
// @Security    BearerAuth
// @Produce     json
// @Param       page     query int false "Page number (default 1)"
// @Param       per_page query int false "Items per page (1-100, default 20)"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} dtoV1.ErrorResponse
// @Router      /components [get]
func (h *ComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	params, errs := parsePagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}

	// Components are few and status is derived per item; fetch all once (single
	// derivation pass) then paginate in memory.
	all, err := h.service.ListComponents(r.Context(), 10000, 0)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list components")
		return
	}

	total := len(all)
	offset := (params.Page - 1) * params.PerPage
	end := offset + params.PerPage
	if offset > total {
		offset = total
	}
	if end > total {
		end = total
	}
	page := all[offset:end]

	respondPaginated(w, page, dtoV1.MetaResponse{
		Page:    params.Page,
		PerPage: params.PerPage,
		Total:   total,
	})
}

// Get handles GET /api/v1/components/{id}
//
// @Summary     Get a component by ID
// @Tags        components
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Component ID"
// @Success     200 {object} dtoV1.SingleResponse[dto.ComponentResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /components/{id} [get]
func (h *ComponentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.service.GetComponent(r.Context(), id)
	if err != nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "component not found")
		return
	}
	respond(w, http.StatusOK, c)
}

// Create handles POST /api/v1/components
//
// @Summary     Create a component
// @Tags        components
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.CreateComponentRequest true "Component payload"
// @Success     201 {object} dtoV1.SingleResponse[dto.ComponentResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /components [post]
func (h *ComponentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.CreateComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	var fieldErrs []dtoV1.FieldError
	if strings.TrimSpace(req.Name) == "" {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "name", Message: "required"})
	}
	if len(req.ResourceIDs) == 0 {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "resource_ids", Message: "at least one resource is required"})
	}
	if len(fieldErrs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed", fieldErrs...)
		return
	}

	created, err := h.service.CreateComponent(r.Context(), &dto.CreateComponentPayload{
		Name:                  req.Name,
		Description:           req.Description,
		ResourceIDs:           req.ResourceIDs,
		GroupingWindowSeconds: req.GroupingWindowSeconds,
	})
	if err != nil {
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create component")
		return
	}
	respond(w, http.StatusCreated, created)
}

// Update handles PUT /api/v1/components/{id}
//
// @Summary     Update a component
// @Tags        components
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Component ID"
// @Param       body body dtoV1.UpdateComponentRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dto.ComponentResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /components/{id} [put]
func (h *ComponentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dtoV1.UpdateComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	updated, err := h.service.UpdateComponent(r.Context(), id, &dto.UpdateComponentPayload{
		Name:                  req.Name,
		Description:           req.Description,
		GroupingWindowSeconds: req.GroupingWindowSeconds,
	})
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "component not found")
			return
		}
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update component")
		return
	}
	respond(w, http.StatusOK, updated)
}

// Patch handles PATCH /api/v1/components/{id} — partial update, alias of Update.
//
// @Summary     Update a component (partial)
// @Tags        components
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Component ID"
// @Param       body body dtoV1.UpdateComponentRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dto.ComponentResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /components/{id} [patch]
func (h *ComponentHandler) Patch(w http.ResponseWriter, r *http.Request) {
	h.Update(w, r)
}

// Delete handles DELETE /api/v1/components/{id}
//
// @Summary     Delete a component
// @Tags        components
// @Security    BearerAuth
// @Param       id path string true "Component ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     409 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /components/{id} [delete]
func (h *ComponentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteComponent(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "component not found")
			return
		}
		if errors.Is(err, service.ErrValidationFailed) {
			// Delete guard: component still has resources attached.
			respondError(w, r, http.StatusConflict, "COMPONENT_HAS_RESOURCES", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete component")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BulkAssign handles POST /api/v1/components/{id}/resources/bulk-assign
//
// @Summary     Assign resources to a component
// @Tags        components
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Component ID"
// @Param       body body dtoV1.BulkResourceIDsRequest true "Resource IDs"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MessageResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     422 {object} dtoV1.ErrorResponse
// @Router      /components/{id}/resources/bulk-assign [post]
func (h *ComponentHandler) BulkAssign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dtoV1.BulkResourceIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	err := h.service.BulkAssignToComponent(r.Context(), id, &dto.BulkAssignPayload{ResourceIDs: req.ResourceIDs})
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "component not found")
			return
		}
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to assign resources")
		return
	}
	respond(w, http.StatusOK, dtoV1.MessageResponse{Message: "resources assigned"})
}

// BulkRemove handles POST /api/v1/components/resources/bulk-remove
//
// @Summary     Remove resources from their components
// @Tags        components
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.BulkResourceIDsRequest true "Resource IDs"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MessageResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Router      /components/resources/bulk-remove [post]
func (h *ComponentHandler) BulkRemove(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.BulkResourceIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	err := h.service.BulkRemoveFromComponent(r.Context(), &dto.BulkRemovePayload{ResourceIDs: req.ResourceIDs})
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "resource not found")
			return
		}
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove resources")
		return
	}
	respond(w, http.StatusOK, dtoV1.MessageResponse{Message: "resources removed"})
}
