package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

// HostHandler exposes operator host management, credential lifecycle, metric
// history, and the monitor↔host link.
type HostHandler struct {
	hosts   *service.HostService
	metrics *service.HostMetricsService
}

func NewHostHandler(hosts *service.HostService, metrics *service.HostMetricsService) *HostHandler {
	return &HostHandler{hosts: hosts, metrics: metrics}
}

func mapHostResponse(h *domain.Host) dtoV1.HostResponse {
	resp := dtoV1.HostResponse{
		ID:           h.ID,
		Name:         h.Name,
		OS:           h.OS,
		AgentVersion: h.AgentVersion,
		Online:       h.Online,
		LastCPUPct:   h.LastCPUPct,
		LastMemPct:   h.LastMemPct,
		LastDiskPct:  h.LastDiskPct,
		LastNetIn:    h.LastNetIn,
		LastNetOut:   h.LastNetOut,
		LastDisks:    mapDisks(h.LastDisks),
		CreatedAt:    h.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    h.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if h.LastSeenAt != nil {
		s := h.LastSeenAt.UTC().Format(time.RFC3339)
		resp.LastSeenAt = &s
	}
	return resp
}

func mapDisks(in []domain.DiskUsage) []dtoV1.DiskUsageDTO {
	out := make([]dtoV1.DiskUsageDTO, 0, len(in))
	for _, d := range in {
		out = append(out, dtoV1.DiskUsageDTO{Mount: d.Mount, UsedPct: d.UsedPct})
	}
	return out
}

func mapHostMetricSample(s *domain.HostMetricSample) dtoV1.HostMetricSampleResponse {
	return dtoV1.HostMetricSampleResponse{
		SampledAt: s.SampledAt.UTC().Format(time.RFC3339),
		CPUPct:    s.CPUPct,
		MemPct:    s.MemPct,
		NetIn:     s.NetIn,
		NetOut:    s.NetOut,
		Disks:     mapDisks(s.Disks),
	}
}

// Register handles POST /api/v1/hosts
//
// @Summary     Register a host
// @Tags        hosts
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.RegisterHostRequest true "Host payload"
// @Success     201 {object} dtoV1.SingleResponse[dtoV1.RegisterHostResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /hosts [post]
func (h *HostHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.RegisterHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed",
			dtoV1.FieldError{Field: "name", Message: "required"})
		return
	}
	host, raw, prefix, err := h.hosts.Register(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed",
				dtoV1.FieldError{Field: "name", Message: "invalid"})
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to register host")
		return
	}
	respond(w, http.StatusCreated, dtoV1.RegisterHostResponse{
		Host:       mapHostResponse(host),
		Credential: raw,
		Prefix:     prefix,
	})
}

// List handles GET /api/v1/hosts
//
// @Summary     List hosts
// @Tags        hosts
// @Security    BearerAuth
// @Produce     json
// @Param       page     query int false "Page number (default 1)"
// @Param       per_page query int false "Items per page (1-100, default 20)"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} dtoV1.ErrorResponse
// @Router      /hosts [get]
func (h *HostHandler) List(w http.ResponseWriter, r *http.Request) {
	params, errs := parsePagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}
	offset := (params.Page - 1) * params.PerPage
	hosts, total, err := h.hosts.List(r.Context(), params.PerPage, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list hosts")
		return
	}
	data := make([]dtoV1.HostResponse, 0, len(hosts))
	for _, host := range hosts {
		data = append(data, mapHostResponse(host))
	}
	respondPaginated(w, data, dtoV1.MetaResponse{Page: params.Page, PerPage: params.PerPage, Total: int(total)})
}

// Get handles GET /api/v1/hosts/{id}
//
// @Summary     Get a host
// @Tags        hosts
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Host ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.HostResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /hosts/{id} [get]
func (h *HostHandler) Get(w http.ResponseWriter, r *http.Request) {
	host, err := h.hosts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "host not found")
		return
	}
	respond(w, http.StatusOK, mapHostResponse(host))
}

// Delete handles DELETE /api/v1/hosts/{id}
//
// @Summary     Delete a host
// @Tags        hosts
// @Security    BearerAuth
// @Param       id path string true "Host ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /hosts/{id} [delete]
func (h *HostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.hosts.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, service.ErrHostNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "host not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete host")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RotateCredential handles POST /api/v1/hosts/{id}/credential/rotate
//
// @Summary     Rotate a host credential
// @Tags        hosts
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Host ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.CreateHostCredentialResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /hosts/{id}/credential/rotate [post]
func (h *HostHandler) RotateCredential(w http.ResponseWriter, r *http.Request) {
	raw, prefix, err := h.hosts.RotateCredential(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, service.ErrHostNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "host not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to rotate credential")
		return
	}
	respond(w, http.StatusOK, dtoV1.CreateHostCredentialResponse{Credential: raw, Prefix: prefix})
}

// RevokeCredential handles POST /api/v1/hosts/{id}/credential/revoke
//
// @Summary     Revoke a host credential
// @Tags        hosts
// @Security    BearerAuth
// @Param       id path string true "Host ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /hosts/{id}/credential/revoke [post]
func (h *HostHandler) RevokeCredential(w http.ResponseWriter, r *http.Request) {
	if err := h.hosts.RevokeCredential(r.Context(), chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, service.ErrHostNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "host not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke credential")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Metrics handles GET /api/v1/hosts/{id}/metrics
//
// @Summary     Get host metric history
// @Tags        hosts
// @Security    BearerAuth
// @Produce     json
// @Param       id   path  string true  "Host ID"
// @Param       from query string false "RFC3339 start (default now-1h)"
// @Param       to   query string false "RFC3339 end (default now)"
// @Success     200 {object} map[string]interface{}
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /hosts/{id}/metrics [get]
func (h *HostHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.hosts.Get(r.Context(), id); err != nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "host not found")
		return
	}
	var from, to time.Time
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid 'from' timestamp")
			return
		}
		from = t
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid 'to' timestamp")
			return
		}
		to = t
	}
	samples, err := h.metrics.History(r.Context(), id, from, to)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load metrics")
		return
	}
	data := make([]dtoV1.HostMetricSampleResponse, 0, len(samples))
	for _, s := range samples {
		data = append(data, mapHostMetricSample(s))
	}
	respondPaginated(w, data, dtoV1.MetaResponse{Page: 1, PerPage: len(data), Total: len(data)})
}

// LinkMonitor handles POST /api/v1/monitors/{id}/host
//
// @Summary     Link a monitor to a host
// @Tags        monitors
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Monitor ID"
// @Param       body body dtoV1.LinkHostRequest true "Host link payload"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MonitorResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/host [post]
func (h *HostHandler) LinkMonitor(w http.ResponseWriter, r *http.Request) {
	monitorID := chi.URLParam(r, "id")
	var req dtoV1.LinkHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if strings.TrimSpace(req.HostID) == "" {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed",
			dtoV1.FieldError{Field: "host_id", Message: "required"})
		return
	}
	monitor, err := h.hosts.LinkMonitor(r.Context(), monitorID, req.HostID)
	if err != nil {
		if errors.Is(err, service.ErrHostNotFound) || errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor or host not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to link monitor")
		return
	}
	respond(w, http.StatusOK, mapMonitorResponse(monitor))
}

// UnlinkMonitor handles DELETE /api/v1/monitors/{id}/host
//
// @Summary     Unlink a monitor from its host
// @Tags        monitors
// @Security    BearerAuth
// @Param       id path string true "Monitor ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /monitors/{id}/host [delete]
func (h *HostHandler) UnlinkMonitor(w http.ResponseWriter, r *http.Request) {
	if err := h.hosts.UnlinkMonitor(r.Context(), chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "monitor not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to unlink monitor")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
