// Package handler — public status endpoints.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/repository"
)

// PublicStatusProvider is the minimum surface the handler needs from the
// service layer. Defined inline to keep the handler trivially testable.
type PublicStatusProvider interface {
	GetCurrent(ctx context.Context) (*dto.PublicStatus, error)
	GetIncidents(ctx context.Context, from, to time.Time, componentID string) (*dto.PublicIncidentsResponse, error)
	GetUptime(ctx context.Context, componentID string, from, to time.Time) (*dto.PublicUptimeResponse, error)
	GetIncidentDetail(ctx context.Context, incidentID string) (*dto.PublicIncidentDetail, error)
	GetResourceWindows(ctx context.Context, resourceID string) (*dto.PublicResourceWindowsResponse, error)
}

type PublicStatusHandler struct {
	svc PublicStatusProvider
}

func NewPublicStatusHandler(svc PublicStatusProvider) *PublicStatusHandler {
	return &PublicStatusHandler{svc: svc}
}

// GetCurrent — GET /status.
//
//	@Summary	Public status page snapshot
//	@Tags		public-status
//	@Produce	json
//	@Success	200	{object}	dto.PublicStatus
//	@Failure	500	{object}	dto.ProblemDetail
//	@Router		/status [get]
func (h *PublicStatusHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.GetCurrent(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// GetIncidents — GET /status/incidents?from=&to=&component_id=.
//
//	@Summary	Incident archive grouped by month
//	@Tags		public-status
//	@Produce	json
//	@Param		from			query		string	false	"ISO date (default: 90 days ago)"
//	@Param		to				query		string	false	"ISO date (default: now)"
//	@Param		component_id	query		string	false	"filter by component"
//	@Success	200	{object}	dto.PublicIncidentsResponse
//	@Failure	422	{object}	dto.ProblemDetail
//	@Failure	500	{object}	dto.ProblemDetail
//	@Router		/status/incidents [get]
func (h *PublicStatusHandler) GetIncidents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to time.Time
	if v := q.Get("from"); v != "" {
		t, err := parseFlexibleDate(v)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "invalid_from", "from must be an ISO date")
			return
		}
		from = t
	}
	if v := q.Get("to"); v != "" {
		t, err := parseFlexibleDate(v)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "invalid_to", "to must be an ISO date")
			return
		}
		to = t
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid_range", "from must be <= to")
		return
	}

	data, err := h.svc.GetIncidents(r.Context(), from, to, q.Get("component_id"))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// GetUptime — GET /status/uptime?from=&to=&component_id=.
//
//	@Summary	Daily uptime aggregates over a range
//	@Tags		public-status
//	@Produce	json
//	@Param		from			query		string	true	"ISO date"
//	@Param		to				query		string	true	"ISO date"
//	@Param		component_id	query		string	false	"filter by component"
//	@Success	200	{object}	dto.PublicUptimeResponse
//	@Failure	422	{object}	dto.ProblemDetail
//	@Failure	500	{object}	dto.ProblemDetail
//	@Router		/status/uptime [get]
func (h *PublicStatusHandler) GetUptime(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to time.Time
	if v := q.Get("from"); v != "" {
		t, err := parseFlexibleDate(v)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "invalid_from", "from must be an ISO date")
			return
		}
		from = t
	}
	if v := q.Get("to"); v != "" {
		t, err := parseFlexibleDate(v)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "invalid_to", "to must be an ISO date")
			return
		}
		to = t
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid_range", "from must be <= to")
		return
	}
	if !from.IsZero() && !to.IsZero() && to.Sub(from) > 366*24*time.Hour {
		writeProblem(w, http.StatusUnprocessableEntity, "range_too_long", "max span is 1 year")
		return
	}

	data, err := h.svc.GetUptime(r.Context(), q.Get("component_id"), from, to)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// GetIncidentDetail — GET /status/incidents/{id}.
//
//	@Summary	Single incident with its lifecycle updates timeline
//	@Tags		public-status
//	@Produce	json
//	@Param		id	path		string	true	"Incident ID"
//	@Success	200	{object}	dto.PublicIncidentDetail
//	@Failure	404	{object}	dto.ProblemDetail
//	@Failure	500	{object}	dto.ProblemDetail
//	@Router		/status/incidents/{id} [get]
func (h *PublicStatusHandler) GetIncidentDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeProblem(w, http.StatusBadRequest, "missing_id", "incident id is required")
		return
	}
	data, err := h.svc.GetIncidentDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// GetResourceWindows — GET /status/resource/{id}/windows.
//
//	@Summary	Per-resource 24h / 7d / 30d / 90d windows + 30-day daily + recent incidents
//	@Tags		public-status
//	@Produce	json
//	@Param		id	path		string	true	"Resource ID"
//	@Success	200	{object}	dto.PublicResourceWindowsResponse
//	@Failure	404	{object}	dto.ProblemDetail
//	@Failure	500	{object}	dto.ProblemDetail
//	@Router		/status/resource/{id}/windows [get]
func (h *PublicStatusHandler) GetResourceWindows(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeProblem(w, http.StatusBadRequest, "missing_id", "resource id is required")
		return
	}
	data, err := h.svc.GetResourceWindows(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// parseFlexibleDate accepts either YYYY-MM-DD or RFC 3339.
func parseFlexibleDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  http.StatusText(status),
		"status": status,
		"code":   code,
		"detail": detail,
	})
}
