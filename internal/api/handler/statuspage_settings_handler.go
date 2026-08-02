package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/denisakp/ogoune/internal/api/response"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

// StatusPageSettingsHandler handles HTTP requests for status page settings.
// Spec 059 fold: now also owns the custom-domain DNS lifecycle.
type StatusPageSettingsHandler struct {
	service       *service.StatusPageSettingsService
	domainRefresh func(domain, status string)
}

// NewStatusPageSettingsHandler creates a new handler instance.
func NewStatusPageSettingsHandler(svc *service.StatusPageSettingsService) *StatusPageSettingsHandler {
	return &StatusPageSettingsHandler{service: svc}
}

// SetDomainRefresh wires the callback invoked after every successful save /
// verify so the HostRouter middleware can refresh its in-memory cache
func (h *StatusPageSettingsHandler) SetDomainRefresh(fn func(domain, status string)) {
	h.domainRefresh = fn
}

func (h *StatusPageSettingsHandler) notifyDomainCache(domain, status string) {
	if h.domainRefresh != nil {
		h.domainRefresh(domain, status)
	}
}

// RegisterRoutes registers the settings routes.
func (h *StatusPageSettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/statuspage", h.GetSettings)
	r.Put("/settings/statuspage", h.UpdateSettings)
	r.Post("/settings/statuspage/verify-domain", h.VerifyDomain)
	// Spec 060 / US5 — branding logo upload/delete.
	r.Post("/settings/statuspage/logo", h.UploadLogo)
	r.Delete("/settings/statuspage/logo", h.DeleteLogo)
}

func toSettingsResponse(s *domain.StatusPageSettings) dto.StatusPageSettingsResponse {
	records := s.CustomDomainDNS
	if records == nil {
		records = []domain.DNSRecord{}
	}
	status := string(s.CustomDomainStatus)
	if status == "" {
		status = string(domain.DomainStatusPending)
	}
	ssl := string(s.CustomDomainSSL)
	if ssl == "" {
		ssl = string(domain.DomainSSLStatusNone)
	}
	return dto.StatusPageSettingsResponse{
		ID:                     s.ID,
		Name:                   s.Name,
		HomepageURL:            s.HomepageURL,
		CustomDomain:           s.CustomDomain,
		UmamiWebsiteID:         s.UmamiWebsiteID,
		UmamiScriptURL:         s.UmamiScriptURL,
		EnableDetailsPage:      s.EnableDetailsPage,
		ShowUptimePercentage:   s.ShowUptimePercentage,
		HidePausedMonitors:     s.HidePausedMonitors,
		ShowIncidentHistory:    s.ShowIncidentHistory,
		CustomDomainStatus:     status,
		CustomDomainSSLStatus:  ssl,
		CustomDomainDNSRecords: records,
		LogoURLLight:           s.LogoURLLight,
		LogoURLDark:            s.LogoURLDark,
		FaviconURL:             s.FaviconURL,
		PrimaryColor:           s.PrimaryColor,
		ThemeOverrides:         themeOrEmpty(s.ThemeOverrides),
		CreatedAt:              s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:              s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func themeOrEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// GetSettings retrieves the current status page settings.
func (h *StatusPageSettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve settings")
		return
	}
	response.JSON(w, http.StatusOK, toSettingsResponse(settings))
}

// UpdateSettings updates the status page settings (and seeds DNS records when
// the custom domain changes).
func (h *StatusPageSettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req dto.StatusPageSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings := &domain.StatusPageSettings{
		Name:                 req.Name,
		HomepageURL:          req.HomepageURL,
		CustomDomain:         req.CustomDomain,
		UmamiWebsiteID:       req.UmamiWebsiteID,
		UmamiScriptURL:       req.UmamiScriptURL,
		EnableDetailsPage:    req.EnableDetailsPage,
		ShowUptimePercentage: req.ShowUptimePercentage,
		HidePausedMonitors:   req.HidePausedMonitors,
		ShowIncidentHistory:  req.ShowIncidentHistory,
		LogoURLLight:         req.LogoURLLight,
		LogoURLDark:          req.LogoURLDark,
		FaviconURL:           req.FaviconURL,
		PrimaryColor:         req.PrimaryColor,
		ThemeOverrides:       req.ThemeOverrides,
	}

	if err := h.service.UpdateSettings(r.Context(), settings); err != nil {
		switch {
		case errors.Is(err, service.ErrCustomDomainInvalidHostname):
			response.Error(w, http.StatusUnprocessableEntity, "Invalid hostname")
		case errors.Is(err, service.ErrInvalidHexColor):
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_HEX_COLOR: "+err.Error())
		case errors.Is(err, service.ErrInvalidThemeKey):
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_THEME_KEY: "+err.Error())
		case errors.Is(err, service.ErrInvalidThemeValue):
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_THEME_VALUE: "+err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "Failed to update settings")
		}
		return
	}

	updated, err := h.service.GetSettings(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve updated settings")
		return
	}
	h.notifyDomainCache(updated.CustomDomain, string(updated.CustomDomainStatus))
	response.JSON(w, http.StatusOK, toSettingsResponse(updated))
}

// VerifyDomain re-checks the seeded DNS records.
func (h *StatusPageSettingsHandler) VerifyDomain(w http.ResponseWriter, r *http.Request) {
	updated, err := h.service.VerifyDomain(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to verify domain")
		return
	}
	h.notifyDomainCache(updated.CustomDomain, string(updated.CustomDomainStatus))
	response.JSON(w, http.StatusOK, toSettingsResponse(updated))
}
