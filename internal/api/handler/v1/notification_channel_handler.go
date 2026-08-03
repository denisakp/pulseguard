package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

// ChannelV1ServiceInterface defines the service methods used by the v1 notification channel handler.
type ChannelV1ServiceInterface interface {
	ListNotificationChannels(ctx context.Context, limit, offset int) ([]*domain.NotificationChannel, error)
	GetNotificationChannel(ctx context.Context, id string) (*domain.NotificationChannel, error)
	CreateNotificationChannel(ctx context.Context, payload *dto.CreateNotificationChannelPayload) (*domain.NotificationChannel, error)
	UpdateNotificationChannel(ctx context.Context, id string, payload *dto.UpdateNotificationChannelPayload) (*domain.NotificationChannel, error)
	DeleteNotificationChannel(ctx context.Context, id string) error
	TestNotificationChannel(ctx context.Context, id string) error
	ValidateAndTestChannelConfig(ctx context.Context, channelType domain.NotificationChannelType, config json.RawMessage) error
	Stats(ctx context.Context) (*service.NotificationStats, error)
}

// NotificationChannelHandler handles v1 CRUD + test/stats endpoints for notification channels.
type NotificationChannelHandler struct {
	service ChannelV1ServiceInterface
}

// NewNotificationChannelHandler creates a new NotificationChannelHandler.
func NewNotificationChannelHandler(svc ChannelV1ServiceInterface) *NotificationChannelHandler {
	return &NotificationChannelHandler{service: svc}
}

// sensitiveConfigKeys are config keys that MUST NOT be echoed back in responses.
var sensitiveConfigKeys = []string{"password", "auth_token", "token", "account_sid", "secret"}

// maskChannelResponse builds the rich (root-compatible) channel response but with
// the sensitive config values stripped — v1 never leaks secrets in cleartext.
func maskChannelResponse(ch *domain.NotificationChannel) (*dto.NotificationChannelResponse, error) {
	// ToNotificationChannelResponse unmarshals the config; guard an empty/nil one.
	if len(ch.Config) == 0 {
		c := *ch
		c.Config = []byte("{}")
		ch = &c
	}
	resp, err := dto.ToNotificationChannelResponse(ch)
	if err != nil {
		return nil, err
	}
	for _, key := range sensitiveConfigKeys {
		delete(resp.Config, key)
	}
	return resp, nil
}

// mergePreservedSecrets restores sensitive config values the incoming update omits
// (or leaves empty) from the stored config. Because responses are masked, the
// frontend re-sends a config without secrets on edit; this keeps the stored secret
// unless the operator explicitly supplied a new non-empty value.
func mergePreservedSecrets(existing, incoming []byte) json.RawMessage {
	if len(incoming) == 0 {
		return nil
	}
	var inMap map[string]any
	if err := json.Unmarshal(incoming, &inMap); err != nil {
		return incoming
	}
	var exMap map[string]any
	_ = json.Unmarshal(existing, &exMap)
	for _, key := range sensitiveConfigKeys {
		v, present := inMap[key]
		empty := !present
		if s, ok := v.(string); ok && s == "" {
			empty = true
		}
		if empty {
			if ev, ok := exMap[key]; ok {
				inMap[key] = ev
			}
		}
	}
	merged, err := json.Marshal(inMap)
	if err != nil {
		return incoming
	}
	return merged
}

// List handles GET /api/v1/notification-channels
//
// @Summary     List notification channels
// @Tags        notification-channels
// @Security    BearerAuth
// @Produce     json
// @Param       page     query int false "Page number (default 1)"
// @Param       per_page query int false "Items per page (1-100, default 20)"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} dtoV1.ErrorResponse
// @Router      /notification-channels [get]
func (h *NotificationChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	params, errs := parsePagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}

	offset := (params.Page - 1) * params.PerPage
	items, err := h.service.ListNotificationChannels(r.Context(), params.PerPage, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list channels")
		return
	}

	allItems, err := h.service.ListNotificationChannels(r.Context(), 10000, 0)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to count channels")
		return
	}

	data := make([]*dto.NotificationChannelResponse, 0, len(items))
	for _, ch := range items {
		resp, err := maskChannelResponse(ch)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to map channel")
			return
		}
		data = append(data, resp)
	}

	respondPaginated(w, data, dtoV1.MetaResponse{
		Page:    params.Page,
		PerPage: params.PerPage,
		Total:   len(allItems),
	})
}

// Get handles GET /api/v1/notification-channels/{id}
//
// @Summary     Get a notification channel by ID
// @Tags        notification-channels
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Channel ID"
// @Success     200 {object} dtoV1.SingleResponse[dto.NotificationChannelResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /notification-channels/{id} [get]
func (h *NotificationChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch, err := h.service.GetNotificationChannel(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "channel not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get channel")
		return
	}
	if ch == nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "channel not found")
		return
	}
	resp, err := maskChannelResponse(ch)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to map channel")
		return
	}
	respond(w, http.StatusOK, resp)
}

// Create handles POST /api/v1/notification-channels
//
// @Summary     Create a notification channel
// @Tags        notification-channels
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.CreateChannelRequest true "Channel payload"
// @Success     201 {object} dtoV1.SingleResponse[dto.NotificationChannelResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /notification-channels [post]
func (h *NotificationChannelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.CreateChannelRequest
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
	if len(fieldErrs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed", fieldErrs...)
		return
	}

	channelType := domain.NotificationChannelType(req.Type)
	if !channelType.IsValid() {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid channel type",
			dtoV1.FieldError{Field: "type", Message: "must be smtp, slack, or sms"})
		return
	}

	payload := &dto.CreateNotificationChannelPayload{
		Name:             req.Name,
		Type:             channelType,
		Config:           req.Config,
		EnabledByDefault: req.EnabledByDefault,
	}

	created, err := h.service.CreateNotificationChannel(r.Context(), payload)
	if err != nil {
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create channel")
		return
	}
	resp, err := maskChannelResponse(created)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to map channel")
		return
	}
	respond(w, http.StatusCreated, resp)
}

// Update handles PUT /api/v1/notification-channels/{id}
//
// @Summary     Update a notification channel
// @Tags        notification-channels
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Channel ID"
// @Param       body body dtoV1.UpdateChannelRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dto.NotificationChannelResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /notification-channels/{id} [put]
func (h *NotificationChannelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dtoV1.UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	// Load the stored channel so an update that omits secrets (because responses
	// are masked) preserves them instead of wiping the stored config.
	existing, err := h.service.GetNotificationChannel(r.Context(), id)
	if err != nil || existing == nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "channel not found")
		return
	}

	payload := &dto.UpdateNotificationChannelPayload{
		Name:             req.Name,
		EnabledByDefault: req.EnabledByDefault,
	}
	if req.Type != nil {
		t := domain.NotificationChannelType(*req.Type)
		payload.Type = &t
	}
	if len(req.Config) > 0 {
		payload.Config = mergePreservedSecrets(existing.Config, req.Config)
	}

	updated, err := h.service.UpdateNotificationChannel(r.Context(), id, payload)
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "channel not found")
			return
		}
		if errors.Is(err, service.ErrValidationFailed) {
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update channel")
		return
	}
	resp, err := maskChannelResponse(updated)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to map channel")
		return
	}
	respond(w, http.StatusOK, resp)
}

// Patch handles PATCH /api/v1/notification-channels/{id} — partial update, alias of Update.
//
// @Summary     Update a notification channel (partial)
// @Tags        notification-channels
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Channel ID"
// @Param       body body dtoV1.UpdateChannelRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dto.NotificationChannelResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /notification-channels/{id} [patch]
func (h *NotificationChannelHandler) Patch(w http.ResponseWriter, r *http.Request) {
	h.Update(w, r)
}

// Delete handles DELETE /api/v1/notification-channels/{id}
//
// @Summary     Delete a notification channel
// @Tags        notification-channels
// @Security    BearerAuth
// @Param       id path string true "Channel ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /notification-channels/{id} [delete]
func (h *NotificationChannelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteNotificationChannel(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "channel not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete channel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /api/v1/notification-channels/{id}/test — sends a real test
// notification through a saved channel.
//
// @Summary     Send a test notification through a saved channel
// @Tags        notification-channels
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Channel ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MessageResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /notification-channels/{id}/test [post]
func (h *NotificationChannelHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.TestNotificationChannel(r.Context(), id); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "TEST_FAILED", err.Error())
		return
	}
	respond(w, http.StatusOK, dtoV1.MessageResponse{Message: "test notification sent"})
}

// TestConfig handles POST /api/v1/notification-channels/test-config — validates and
// tests an unsaved channel config.
//
// @Summary     Validate and test an unsaved channel config
// @Tags        notification-channels
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.TestChannelConfigRequest true "Config to test"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.MessageResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /notification-channels/test-config [post]
func (h *NotificationChannelHandler) TestConfig(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.TestChannelConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if err := h.service.ValidateAndTestChannelConfig(r.Context(), domain.NotificationChannelType(req.Type), req.Config); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "TEST_FAILED", err.Error())
		return
	}
	respond(w, http.StatusOK, dtoV1.MessageResponse{Message: "configuration validated and tested"})
}

// Stats handles GET /api/v1/notifications/stats — notification counters for the header.
//
// @Summary     Notification stats counters
// @Tags        notification-channels
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.NotificationStatsResponse]
// @Failure     500 {object} dtoV1.ErrorResponse
// @Router      /notifications/stats [get]
func (h *NotificationChannelHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Stats(r.Context())
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load notification stats")
		return
	}
	respond(w, http.StatusOK, dtoV1.NotificationStatsResponse{
		Sent30d:   stats.Sent30d,
		Pending:   stats.Pending,
		Failed24h: stats.Failed24h,
	})
}
