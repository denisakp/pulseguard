package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/repository"
)

// IncidentUpdateProvider is the incident-update service surface used by the v1 handler.
type IncidentUpdateProvider interface {
	ListByIncident(ctx context.Context, incidentID string) ([]*domain.IncidentUpdate, error)
	Create(ctx context.Context, incidentID string, status domain.IncidentUpdateStatus, message, postedBy string) (*domain.IncidentUpdate, error)
	Update(ctx context.Context, id string, status domain.IncidentUpdateStatus, message string) (*domain.IncidentUpdate, error)
	Delete(ctx context.Context, id string) error
}

// IncidentUpdateHandler handles the v1 incident-update (timeline) endpoints.
type IncidentUpdateHandler struct {
	svc IncidentUpdateProvider
}

// NewIncidentUpdateHandler creates a new v1 IncidentUpdateHandler.
func NewIncidentUpdateHandler(svc IncidentUpdateProvider) *IncidentUpdateHandler {
	return &IncidentUpdateHandler{svc: svc}
}

// mapIncidentUpdateResponse maps a domain.IncidentUpdate to a v1 IncidentUpdateResponse.
func mapIncidentUpdateResponse(u *domain.IncidentUpdate) dtoV1.IncidentUpdateResponse {
	return dtoV1.IncidentUpdateResponse{
		ID:         u.ID,
		IncidentID: u.IncidentID,
		Status:     string(u.Status),
		Message:    u.Message,
		PostedBy:   u.PostedBy,
		PostedAt:   u.PostedAt.UTC().Format(time.RFC3339),
		CreatedAt:  u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// List handles GET /api/v1/incidents/{id}/updates
//
// @Summary     List an incident's timeline updates
// @Tags        incidents
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Incident ID"
// @Success     200 {object} map[string]interface{}
// @Failure     500 {object} dtoV1.ErrorResponse
// @Router      /incidents/{id}/updates [get]
func (h *IncidentUpdateHandler) List(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, err := h.svc.ListByIncident(r.Context(), id)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list incident updates")
		return
	}
	out := make([]dtoV1.IncidentUpdateResponse, 0, len(rows))
	for _, u := range rows {
		out = append(out, mapIncidentUpdateResponse(u))
	}
	respond(w, http.StatusOK, out)
}

// Create handles POST /api/v1/incidents/{id}/updates
//
// @Summary     Add an update to an incident's timeline
// @Tags        incidents
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Incident ID"
// @Param       body body dtoV1.IncidentUpdateRequest true "Update payload"
// @Success     201 {object} dtoV1.SingleResponse[dtoV1.IncidentUpdateResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /incidents/{id}/updates [post]
func (h *IncidentUpdateHandler) Create(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dtoV1.IncidentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	out, err := h.svc.Create(r.Context(), id, domain.IncidentUpdateStatus(req.Status), req.Message, userIDFromContext(r))
	if err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
		return
	}
	respond(w, http.StatusCreated, mapIncidentUpdateResponse(out))
}

// Update handles PATCH /api/v1/incidents/{id}/updates/{updateID}
//
// @Summary     Edit an incident timeline update
// @Tags        incidents
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id       path string true "Incident ID"
// @Param       updateID path string true "Update ID"
// @Param       body     body dtoV1.IncidentUpdateRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.IncidentUpdateResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     422 {object} dtoV1.ErrorResponse
// @Router      /incidents/{id}/updates/{updateID} [patch]
func (h *IncidentUpdateHandler) Update(w http.ResponseWriter, r *http.Request) {
	updateID := chi.URLParam(r, "updateID")
	var req dtoV1.IncidentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	out, err := h.svc.Update(r.Context(), updateID, domain.IncidentUpdateStatus(req.Status), req.Message)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "update not found")
			return
		}
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
		return
	}
	respond(w, http.StatusOK, mapIncidentUpdateResponse(out))
}

// Delete handles DELETE /api/v1/incidents/{id}/updates/{updateID}
//
// @Summary     Delete an incident timeline update
// @Tags        incidents
// @Security    BearerAuth
// @Param       id       path string true "Incident ID"
// @Param       updateID path string true "Update ID"
// @Success     204
// @Failure     500 {object} dtoV1.ErrorResponse
// @Router      /incidents/{id}/updates/{updateID} [delete]
func (h *IncidentUpdateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	updateID := chi.URLParam(r, "updateID")
	if err := h.svc.Delete(r.Context(), updateID); err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete incident update")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
