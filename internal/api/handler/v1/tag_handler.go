package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/go-chi/chi/v5"
)

// TagV1ServiceInterface defines the tag service methods used by the v1 tag handler.
type TagV1ServiceInterface interface {
	CreateTag(ctx context.Context, t *domain.Tags) error
	ListTags(ctx context.Context, limit, offset int) ([]*domain.Tags, error)
	GetTagByID(ctx context.Context, id string) (*domain.Tags, error)
	UpdateTag(ctx context.Context, id string, name string, color *string, description *string) (*domain.Tags, error)
	DeleteTag(ctx context.Context, id string) error
}

// TagHandler handles v1 CRUD endpoints for tags.
type TagHandler struct {
	service TagV1ServiceInterface
}

// NewTagHandler creates a new TagHandler.
func NewTagHandler(svc TagV1ServiceInterface) *TagHandler {
	return &TagHandler{service: svc}
}

// mapTagResponse maps a domain.Tags to a v1 TagResponse.
func mapTagResponse(t *domain.Tags) dtoV1.TagResponse {
	return dtoV1.TagResponse{
		ID:          t.ID,
		Name:        t.Name,
		Color:       t.Color,
		Description: t.Description,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// List handles GET /api/v1/tags
//
// @Summary     List tags
// @Tags        tags
// @Security    BearerAuth
// @Produce     json
// @Param       page     query int false "Page number (default 1)"
// @Param       per_page query int false "Items per page (1-100, default 20)"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} dtoV1.ErrorResponse
// @Router      /tags [get]
func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	params, errs := parsePagination(r)
	if len(errs) > 0 {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid pagination parameters", errs...)
		return
	}

	offset := (params.Page - 1) * params.PerPage
	items, err := h.service.ListTags(r.Context(), params.PerPage, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tags")
		return
	}

	all, err := h.service.ListTags(r.Context(), 10000, 0)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to count tags")
		return
	}

	data := make([]dtoV1.TagResponse, 0, len(items))
	for _, t := range items {
		data = append(data, mapTagResponse(t))
	}

	respondPaginated(w, data, dtoV1.MetaResponse{
		Page:    params.Page,
		PerPage: params.PerPage,
		Total:   len(all),
	})
}

// Get handles GET /api/v1/tags/{id}
//
// @Summary     Get a tag by ID
// @Tags        tags
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Tag ID"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.TagResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Router      /tags/{id} [get]
func (h *TagHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.service.GetTagByID(r.Context(), id)
	if err != nil || t == nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "tag not found")
		return
	}
	respond(w, http.StatusOK, mapTagResponse(t))
}

// Create handles POST /api/v1/tags
//
// @Summary     Create a tag
// @Tags        tags
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body body dtoV1.CreateTagRequest true "Tag payload"
// @Success     201 {object} dtoV1.SingleResponse[dtoV1.TagResponse]
// @Failure     422 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /tags [post]
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtoV1.CreateTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed",
			dtoV1.FieldError{Field: "name", Message: "required"})
		return
	}

	t := &domain.Tags{
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	}
	if err := h.service.CreateTag(r.Context(), t); err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create tag")
		return
	}
	respond(w, http.StatusCreated, mapTagResponse(t))
}

// Update handles PUT /api/v1/tags/{id}
//
// @Summary     Update a tag
// @Tags        tags
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Tag ID"
// @Param       body body dtoV1.UpdateTagRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.TagResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /tags/{id} [put]
func (h *TagHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.service.GetTagByID(r.Context(), id)
	if err != nil || existing == nil {
		respondError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "tag not found")
		return
	}

	var req dtoV1.UpdateTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid request body")
		return
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	updated, err := h.service.UpdateTag(r.Context(), id, name, req.Color, req.Description)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update tag")
		return
	}
	respond(w, http.StatusOK, mapTagResponse(updated))
}

// Patch handles PATCH /api/v1/tags/{id} — partial update, alias of Update.
// Exposed so the frontend's PATCH edits (inherited from the root API) map 1:1.
//
// @Summary     Update a tag (partial)
// @Tags        tags
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id   path string true "Tag ID"
// @Param       body body dtoV1.UpdateTagRequest true "Update payload"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.TagResponse]
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /tags/{id} [patch]
func (h *TagHandler) Patch(w http.ResponseWriter, r *http.Request) {
	h.Update(w, r)
}

// Delete handles DELETE /api/v1/tags/{id}
//
// @Summary     Delete a tag
// @Tags        tags
// @Security    BearerAuth
// @Param       id path string true "Tag ID"
// @Success     204
// @Failure     404 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Router      /tags/{id} [delete]
func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteTag(r.Context(), id); err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete tag")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
