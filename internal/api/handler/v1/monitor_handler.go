package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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
	GetResourceByIDWithResponseTimes(ctx context.Context, id string, limit int) (*dto.ResourceResponse, error)
	CreateResource(ctx context.Context, payload *dto.CreateResourcePayload) (*domain.Resource, error)
	UpdateResource(ctx context.Context, id string, payload *dto.UpdateResourcePayload) (*domain.Resource, error)
	DeleteResource(ctx context.Context, resourceID string) error
	PauseMonitoring(ctx context.Context, resourceID string) error
	ResumeMonitoring(ctx context.Context, resourceID string) error
	// spec 085 — parity with the root resources API
	AddTagsToResource(ctx context.Context, resourceID string, tagIDs []string) error
	RemoveTagFromResource(ctx context.Context, resourceID, tagID string) error
}

// monitorLiveProvider is the live-snapshot service subset (spec 085 parity).
type monitorLiveProvider interface {
	GetLiveSnapshot(ctx context.Context, resourceID string) (*dto.LiveSnapshotResponse, error)
}

// monitorUptimeProvider is the uptime-stats service subset (spec 085 parity).
type monitorUptimeProvider interface {
	GetUptimeStats(ctx context.Context, resourceID string) ([]dto.UptimeStatResponse, error)
}

// MonitorHandler handles v1 CRUD and lifecycle endpoints for monitors.
type MonitorHandler struct {
	service MonitorV1ServiceInterface
	live    monitorLiveProvider
	uptime  monitorUptimeProvider
}

// NewMonitorHandler creates a new MonitorHandler. live/uptime may be nil (their
// endpoints then return 503); bootstrap injects the real services.
func NewMonitorHandler(svc MonitorV1ServiceInterface, live monitorLiveProvider, uptime monitorUptimeProvider) *MonitorHandler {
	return &MonitorHandler{service: svc, live: live, uptime: uptime}
}

// mapMonitorResponse maps a domain.Resource to a thin v1 MonitorResponse. Still used
// by the monitor↔host link/unlink endpoints (host_handler.go); the CRUD/list endpoints
// now return the rich dto.ResourceResponse (spec 085 Phase 2a).
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

	data := make([]dto.ResourceResponse, 0, len(items))
	for _, res := range items {
		if res != nil {
			data = append(data, dto.ToResourceListResponse(*res))
		}
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
// @Param       limit query int false "Number of recent response-time points to include (default 60)"
// @Success     200 {object} dtoV1.SingleResponse[dto.ResourceResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id} [get]
func (h *MonitorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit := 60
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	res, err := h.service.GetResourceByIDWithResponseTimes(r.Context(), id, limit)
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get monitor")
		return
	}
	respond(w, http.StatusOK, res)
}

// Create handles POST /api/v1/monitors
//
// @Summary     Create a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dto.CreateResourcePayload true "Monitor payload"
// @Success     201 {object} dtoV1.SingleResponse[dto.ResourceResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors [post]
func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var payload dto.CreateResourcePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	var fieldErrs []dtoV1.FieldError
	if strings.TrimSpace(payload.Name) == "" {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "name", Message: "required"})
	}
	if strings.TrimSpace(string(payload.Type)) == "" {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "type", Message: "required"})
	}
	// target is required except for heartbeat monitors (parity with the root handler).
	if strings.TrimSpace(payload.Target) == "" && payload.Type != domain.ResourceHeartbeat {
		fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "target", Message: "required"})
	}
	if payload.ExpiryAlertThresholds != nil {
		if err := validateExpiryThresholds(*payload.ExpiryAlertThresholds); err != nil {
			fieldErrs = append(fieldErrs, dtoV1.FieldError{Field: "expiry_alert_thresholds", Message: err.Error()})
		}
	}
	if len(fieldErrs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed", fieldErrs...)
		return
	}

	created, err := h.service.CreateResource(r.Context(), &payload)
	if err != nil {
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create monitor")
		return
	}
	respond(w, http.StatusCreated, dto.ToResourceDetailResponse(*created))
}

// Update handles PUT /api/v1/monitors/{id} (full-shape update; partial semantics).
//
// @Summary     Update a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Monitor ID"
// @Param       body body dto.UpdateResourcePayload true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dto.ResourceResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id} [put]
func (h *MonitorHandler) Update(w http.ResponseWriter, r *http.Request) { h.applyUpdate(w, r) }

// Patch handles PATCH /api/v1/monitors/{id} — partial update; only supplied fields
// change, unset fields are preserved (spec 085 parity with the root PATCH).
//
// @Summary     Partially update a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Monitor ID"
// @Param       body body dto.UpdateResourcePayload true "Partial update payload"
// @Success     200 {object} dtoV1.SingleResponse[dto.ResourceResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Failure     422 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id} [patch]
func (h *MonitorHandler) Patch(w http.ResponseWriter, r *http.Request) { h.applyUpdate(w, r) }

// applyUpdate decodes the full pointer-shaped UpdateResourcePayload and applies it as
// a partial update (shared by PUT + PATCH). Only supplied fields change.
func (h *MonitorHandler) applyUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var payload dto.UpdateResourcePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if payload.ExpiryAlertThresholds != nil {
		if err := validateExpiryThresholds(*payload.ExpiryAlertThresholds); err != nil {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed",
				dtoV1.FieldError{Field: "expiry_alert_thresholds", Message: err.Error()})
			return
		}
	}

	updated, err := h.service.UpdateResource(r.Context(), id, &payload)
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
	respond(w, http.StatusOK, dto.ToResourceDetailResponse(*updated))
}

// validateExpiryThresholds mirrors the root handler's check: CSV of integers 1–365.
func validateExpiryThresholds(s string) error {
	if s == "" {
		return nil
	}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("invalid threshold value %q: must be an integer", p)
		}
		if v <= 0 || v > 365 {
			return fmt.Errorf("threshold value %d is out of range (must be 1–365)", v)
		}
	}
	return nil
}

// GetLive handles GET /api/v1/monitors/{id}/live — live snapshot (spec 085 parity).
//
// @Summary     Live snapshot of a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Monitor ID"
// @Success     200 {object} dtoV1.SingleResponse[dto.LiveSnapshotResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/live [get]
func (h *MonitorHandler) GetLive(w http.ResponseWriter, r *http.Request) {
	if h.live == nil {
		respondError(w, r, http.StatusServiceUnavailable, "UNAVAILABLE", "live snapshot service not available")
		return
	}
	snapshot, err := h.live.GetLiveSnapshot(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load live snapshot")
		return
	}
	respond(w, http.StatusOK, snapshot)
}

// GetUptimeStats handles GET /api/v1/monitors/{id}/uptime-stats (spec 085 parity).
//
// @Summary     Hourly uptime statistics for a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Monitor ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MonitorUptimeStatsResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/uptime-stats [get]
func (h *MonitorHandler) GetUptimeStats(w http.ResponseWriter, r *http.Request) {
	if h.uptime == nil {
		respondError(w, r, http.StatusServiceUnavailable, "UNAVAILABLE", "uptime service not available")
		return
	}
	id := chi.URLParam(r, "id")
	stats, err := h.uptime.GetUptimeStats(r.Context(), id)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load uptime stats")
		return
	}
	out := dtoV1.MonitorUptimeStatsResponse{ResourceID: id, Stats: make([]dtoV1.MonitorUptimeStat, 0, len(stats))}
	for _, s := range stats {
		out.Stats = append(out.Stats, dtoV1.MonitorUptimeStat{
			Hour: s.Hour, UptimePercent: s.UptimePercent, SuccessfulCount: s.SuccessfulCount, TotalCount: s.TotalCount,
		})
	}
	respond(w, http.StatusOK, out)
}

// AddTag handles POST /api/v1/monitors/{id}/tags (spec 085 parity).
//
// @Summary     Attach tags to a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Param       id   path string true "Monitor ID"
// @Param       body body dtoV1.AddTagsRequest true "Tag IDs"
// @Success     204
// @Failure     403 {object} dtoV1.ErrorResponse
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/tags [post]
func (h *MonitorHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dtoV1.AddTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if err := h.service.AddTagsToResource(r.Context(), id, req.TagIDs); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to attach tags")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveTag handles DELETE /api/v1/monitors/{id}/tags/{tagID} (spec 085 parity).
//
// @Summary     Detach a tag from a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Param       id    path string true "Monitor ID"
// @Param       tagID path string true "Tag ID"
// @Success     204
// @Failure     403 {object} dtoV1.ErrorResponse
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/tags/{tagID} [delete]
func (h *MonitorHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveTagFromResource(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "tagID")); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor or tag not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to detach tag")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
// @Success     200 {object} dtoV1.SingleResponse[dto.ResourceResponse]
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
	respond(w, http.StatusOK, dto.ToResourceDetailResponse(*res))
}

// Resume handles POST /api/v1/monitors/{id}/resume
//
// @Summary     Resume a monitor
// @Tags        monitors
// @Security    BearerAuth
// @Param       id path string true "Monitor ID"
// @Success     200 {object} dtoV1.SingleResponse[dto.ResourceResponse]
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
	respond(w, http.StatusOK, dto.ToResourceDetailResponse(*res))
}
