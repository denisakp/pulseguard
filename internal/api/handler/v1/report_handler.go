package v1

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
)

// ReportV1ServiceInterface is the slice of *ReportService used by the handler.
type ReportV1ServiceInterface interface {
	GetSettings(ctx context.Context) (*domain.ReportSettings, error)
	SaveSettings(ctx context.Context, in *domain.ReportSettings) (*domain.ReportSettings, error)
	ListHistory(ctx context.Context, limit int) ([]*domain.ReportHistory, error)
	GeneratePreview(ctx context.Context) (*domain.ReportHistory, error)
}

// ReportHandler exposes /api/v1/reports.
type ReportHandler struct {
	service ReportV1ServiceInterface
}

func NewReportHandler(svc ReportV1ServiceInterface) *ReportHandler {
	return &ReportHandler{service: svc}
}

// GetSettings handles GET /api/v1/reports/settings.
//
// @Summary  Get the monthly-report configuration
// @Tags     reports
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.ReportSettingsResponse]
// @Failure  401 {object} dtoV1.ErrorResponse
// @Router   /reports/settings [get]
func (h *ReportHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.service.GetSettings(r.Context())
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load report settings")
		return
	}
	respond(w, http.StatusOK, mapReportSettings(s))
}

// UpdateSettings handles PUT /api/v1/reports/settings.
//
// @Summary  Update the monthly-report configuration
// @Tags     reports
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body dtoV1.UpdateReportSettingsRequest true "Report settings"
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.ReportSettingsResponse]
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  422 {object} dtoV1.ErrorResponse
// @Router   /reports/settings [put]
func (h *ReportHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.UpdateReportSettingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	in := &domain.ReportSettings{
		Enabled:        req.Enabled,
		RecipientEmail: req.RecipientEmail,
		Schedule:       req.Schedule,
		Scope:          req.Scope,
	}
	saved, err := h.service.SaveSettings(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrReportValidation) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save report settings")
		return
	}
	respond(w, http.StatusOK, mapReportSettings(saved))
}

// History handles GET /api/v1/reports/history?limit=.
//
// @Summary  List generated reports (newest first)
// @Tags     reports
// @Security BearerAuth
// @Produce  json
// @Param    limit query int false "Max entries (default 6, max 50)"
// @Success  200 {object} map[string]interface{}
// @Router   /reports/history [get]
func (h *ReportHandler) History(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	items, err := h.service.ListHistory(r.Context(), limit)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list report history")
		return
	}
	data := make([]dtoV1.ReportHistoryResponse, len(items))
	for i, it := range items {
		data[i] = mapReportHistory(it)
	}
	respond(w, http.StatusOK, data)
}

// Preview handles GET /api/v1/reports/preview.
//
// @Summary  Preview the current in-progress period (not persisted)
// @Tags     reports
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.ReportHistoryResponse]
// @Router   /reports/preview [get]
func (h *ReportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	pv, err := h.service.GeneratePreview(r.Context())
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to build report preview")
		return
	}
	if pv == nil {
		respond(w, http.StatusOK, nil)
		return
	}
	respond(w, http.StatusOK, mapReportHistory(pv))
}

// --- domain ↔ DTO mappers ---

func mapReportSettings(s *domain.ReportSettings) dtoV1.ReportSettingsResponse {
	var last *string
	if s.LastSentAt != nil {
		v := s.LastSentAt.UTC().Format(time.RFC3339)
		last = &v
	}
	return dtoV1.ReportSettingsResponse{
		Enabled:        s.Enabled,
		RecipientEmail: s.RecipientEmail,
		Schedule:       s.Schedule,
		Scope:          s.Scope,
		LastSentAt:     last,
	}
}

func mapReportHistory(h *domain.ReportHistory) dtoV1.ReportHistoryResponse {
	bd := make([]dtoV1.ReportBreakdown, len(h.Breakdown))
	for i, b := range h.Breakdown {
		bd[i] = dtoV1.ReportBreakdown{Name: b.Name, UptimePct: b.UptimePct, Incidents: b.Incidents}
	}
	return dtoV1.ReportHistoryResponse{
		ID:                h.ID,
		Period:            h.Period,
		SentAt:            h.SentAt.UTC().Format(time.RFC3339),
		Status:            string(h.Status),
		UptimePct:         h.UptimePct,
		IncidentCount:     h.IncidentCount,
		DowntimeSeconds:   h.DowntimeSeconds,
		RecipientEmail:    h.RecipientEmail,
		ResourceBreakdown: bd,
	}
}
