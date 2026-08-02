package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

const (
	notifDefaultPerPage = 50
	notifMaxPerPage     = 200
)

// NotificationFeedV1ServiceInterface is the slice of *NotificationFeedService used by the handler.
type NotificationFeedV1ServiceInterface interface {
	ListForUser(ctx context.Context, userID string, category *string, limit, offset int) ([]*domain.FeedNotification, int64, error)
	MarkRead(ctx context.Context, id string) error
	MarkAllRead(ctx context.Context, userID string, before time.Time) (int64, error)
}

// NotificationFeedHandler exposes /api/v1/notifications.
type NotificationFeedHandler struct {
	service NotificationFeedV1ServiceInterface
}

func NewNotificationFeedHandler(svc NotificationFeedV1ServiceInterface) *NotificationFeedHandler {
	return &NotificationFeedHandler{service: svc}
}

func mapNotificationItem(n *domain.FeedNotification) dtoV1.NotificationItem {
	item := dtoV1.NotificationItem{
		ID:         n.ID,
		Category:   n.Category,
		Severity:   n.Severity,
		Title:      n.Title,
		OccurredAt: n.OccurredAt.UTC().Format(time.RFC3339),
		Unread:     n.ReadAt == nil,
	}
	if n.Description != nil {
		item.Description = *n.Description
	}
	if n.DeepLink != nil {
		item.DeepLink = *n.DeepLink
	}
	return item
}

func userIDFromContext(r *http.Request) string {
	id, _ := r.Context().Value("user_id").(string)
	return id
}

// List handles GET /api/v1/notifications.
//
// @Summary  List the in-app notification feed (newest-first, paginated)
// @Tags     notifications
// @Security BearerAuth
// @Produce  json
// @Param    page     query int    false "Page number (default 1)"
// @Param    per_page query int    false "Items per page (1-200, default 50)"
// @Param    category query string false "Filter: incident|system|general"
// @Success  200 {object} map[string]interface{}
// @Failure  401 {object} dtoV1.ErrorResponse
// @Failure  422 {object} dtoV1.ErrorResponse
// @Router   /notifications [get]
func (h *NotificationFeedHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, errs := parseNotifPagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}

	var category *string
	if c := r.URL.Query().Get("category"); c != "" {
		switch c {
		case domain.NotificationCategoryIncident, domain.NotificationCategorySystem, domain.NotificationCategoryGeneral:
			category = &c
		default:
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category",
				dtoV1.FieldError{Field: "category", Message: "must be incident, system, or general"})
			return
		}
	}

	offset := (page - 1) * perPage
	items, total, err := h.service.ListForUser(r.Context(), userIDFromContext(r), category, perPage, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list notifications")
		return
	}

	data := make([]dtoV1.NotificationItem, len(items))
	for i, n := range items {
		data[i] = mapNotificationItem(n)
	}
	respondPaginated(w, data, dtoV1.MetaResponse{Page: page, PerPage: perPage, Total: int(total)})
}

// MarkRead handles POST /api/v1/notifications/{id}/read.
//
// @Summary  Mark a single notification read
// @Tags     notifications
// @Security BearerAuth
// @Param    id path string true "Notification ID"
// @Success  204
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  404 {object} dtoV1.ErrorResponse
// @Router   /notifications/{id}/read [post]
func (h *NotificationFeedHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.service.MarkRead(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotificationNotFound) {
			respondError(w, r, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "notification not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark read")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead handles POST /api/v1/notifications/read-all.
//
// @Summary  Mark all visible notifications read (optional before_timestamp cursor)
// @Tags     notifications
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.MarkAllReadResponse]
// @Failure  403 {object} dtoV1.ErrorResponse
// @Router   /notifications/read-all [post]
func (h *NotificationFeedHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	var before time.Time
	// Optional body { "before_timestamp": "<RFC3339>" }; absent → now (in service).
	if r.Body != nil {
		var body struct {
			BeforeTimestamp string `json:"before_timestamp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.BeforeTimestamp != "" {
			if t, perr := time.Parse(time.RFC3339, body.BeforeTimestamp); perr == nil {
				before = t
			}
		}
	}
	marked, err := h.service.MarkAllRead(r.Context(), userIDFromContext(r), before)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark all read")
		return
	}
	respond(w, http.StatusOK, dtoV1.MarkAllReadResponse{Marked: marked})
}

func parseNotifPagination(r *http.Request) (page, perPage int, errs []dtoV1.FieldError) {
	page, perPage = 1, notifDefaultPerPage
	if raw := r.URL.Query().Get("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			errs = append(errs, dtoV1.FieldError{Field: "page", Message: "must be a positive integer"})
		} else {
			page = v
		}
	}
	if raw := r.URL.Query().Get("per_page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			errs = append(errs, dtoV1.FieldError{Field: "per_page", Message: "must be a positive integer"})
		} else {
			if v > notifMaxPerPage {
				v = notifMaxPerPage
			}
			perPage = v
		}
	}
	return page, perPage, errs
}
