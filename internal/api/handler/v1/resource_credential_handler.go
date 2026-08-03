package v1

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/denisakp/ogoune/pkg/safenet"
	"github.com/go-chi/chi/v5"
)

const (
	codeResourceNotFound = "RESOURCE_NOT_FOUND"
	msgResourceNotFound  = "resource not found"
)

// CredentialV1ServiceInterface is the slice of *ResourceCredentialService used by the handler.
type CredentialV1ServiceInterface interface {
	Get(ctx context.Context, resourceID string) (*domain.ResourceCredential, error)
	Set(ctx context.Context, resourceID, username string, password []byte, options []byte) (created bool, err error)
	Delete(ctx context.Context, resourceID string) error
}

// CredentialTester runs a live credential check without persisting the credential.
// Implementations: dispatch to the protocol strategy with a transient *domain.Resource
// + *domain.ResourceCredential and return the resulting CheckResult.
type CredentialTester interface {
	Test(ctx context.Context, resourceID string, username string, password []byte, options []byte) (domain.CheckResult, error)
}

// ResourceCredentialHandler exposes /api/v1/resources/{id}/credentials and the
// rate-limited /credentials/test endpoint.
type ResourceCredentialHandler struct {
	service CredentialV1ServiceInterface
	tester  CredentialTester
}

func NewResourceCredentialHandler(svc CredentialV1ServiceInterface, tester CredentialTester) *ResourceCredentialHandler {
	return &ResourceCredentialHandler{
		service: svc,
		tester:  tester,
	}
}

func mapCredentialResponse(c *domain.ResourceCredential) dtoV1.CredentialResponse {
	return dtoV1.CredentialResponse{
		ResourceID:     c.ResourceID,
		HasCredentials: true,
		Username:       c.Username,
		Password:       dtoV1.PasswordMask,
		CreatedAt:      c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// Get handles GET /api/v1/resources/{id}/credentials.
//
// @Summary  Get the credential metadata for a resource (password masked)
// @Tags     resource-credentials
// @Security BearerAuth
// @Produce  json
// @Param    id path string true "Resource ID"
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.CredentialResponse]
// @Failure  404 {object} dtoV1.ErrorResponse
// @Router   /monitors/{id}/credentials [get]
func (h *ResourceCredentialHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cred, err := h.service.Get(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrResourceNotFound):
			respondError(w, r, http.StatusNotFound, codeResourceNotFound, msgResourceNotFound)
		case errors.Is(err, service.ErrCredentialNotFound):
			respondError(w, r, http.StatusNotFound, "CREDENTIAL_NOT_FOUND", "no credentials configured for this resource")
		default:
			respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get credentials")
		}
		return
	}
	respond(w, http.StatusOK, mapCredentialResponse(cred))
}

// Set handles POST /api/v1/resources/{id}/credentials.
//
// @Summary  Create or replace credentials for a resource
// @Tags     resource-credentials
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string                       true "Resource ID"
// @Param    body body dtoV1.CredentialCreateRequest true "Credential payload"
// @Success  201 {object} dtoV1.SingleResponse[dtoV1.CredentialResponse]
// @Success  200 {object} dtoV1.SingleResponse[dtoV1.CredentialResponse]
// @Failure  403 {object} dtoV1.ErrorResponse
// @Failure  404 {object} dtoV1.ErrorResponse
// @Failure  422 {object} dtoV1.ErrorResponse
// @Router   /monitors/{id}/credentials [post]
func (h *ResourceCredentialHandler) Set(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	req, ok := decodeCredentialBody(w, r)
	if !ok {
		return
	}

	optsBytes, ok := marshalOptions(w, r, req.Options)
	if !ok {
		return
	}

	created, err := h.service.Set(r.Context(), id, req.Username, []byte(req.Password), optsBytes)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrResourceNotFound):
			respondError(w, r, http.StatusNotFound, codeResourceNotFound, msgResourceNotFound)
		case errors.Is(err, service.ErrCredentialInvalid):
			respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid credential payload")
		default:
			respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store credentials")
		}
		return
	}

	// Re-fetch to populate timestamps; this also confirms the encryption round-trip.
	cred, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "credential stored but could not be re-read")
		return
	}

	h.audit(r.Context(), "credential.create", id)

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	respond(w, status, mapCredentialResponse(cred))
}

// Delete handles DELETE /api/v1/resources/{id}/credentials.
//
// @Summary  Remove credentials for a resource (revert to no-auth behavior)
// @Tags     resource-credentials
// @Security BearerAuth
// @Param    id path string true "Resource ID"
// @Success  204
// @Failure  404 {object} dtoV1.ErrorResponse
// @Router   /monitors/{id}/credentials [delete]
func (h *ResourceCredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.service.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrResourceNotFound):
			respondError(w, r, http.StatusNotFound, codeResourceNotFound, msgResourceNotFound)
		case errors.Is(err, service.ErrCredentialNotFound):
			respondError(w, r, http.StatusNotFound, "CREDENTIAL_NOT_FOUND", "no credentials configured for this resource")
		default:
			respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete credentials")
		}
		return
	}
	h.audit(r.Context(), "credential.delete", id)
	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /api/v1/resources/{id}/credentials/test — rate-limited live check
// against the supplied credential without persisting it.
//
// @Summary  Live-test credentials without persisting (rate-limited 10/min)
// @Tags     resource-credentials
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string                       true "Resource ID"
// @Param    body body dtoV1.CredentialCreateRequest true "Credential payload (not persisted)"
// @Success  200 {object} dtoV1.CredentialTestResponse
// @Failure  404 {object} dtoV1.ErrorResponse
// @Failure  422 {object} dtoV1.ErrorResponse
// @Failure  429 {object} dtoV1.ErrorResponse
// @Router   /monitors/{id}/credentials/test [post]
func (h *ResourceCredentialHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	req, ok := decodeCredentialBody(w, r)
	if !ok {
		return
	}
	optsBytes, ok := marshalOptions(w, r, req.Options)
	if !ok {
		return
	}

	start := time.Now()
	result, err := h.tester.Test(r.Context(), id, req.Username, []byte(req.Password), optsBytes)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			respondError(w, r, http.StatusNotFound, codeResourceNotFound, msgResourceNotFound)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to run live test")
		return
	}

	resp := dtoV1.CredentialTestResponse{LatencyMs: latency}
	if result.Status == string(domain.StatusUp) {
		resp.Status = "ok"
	} else {
		resp.Status = "failed"
		if result.Cause != nil {
			resp.Cause = causeToWireString(*result.Cause)
		}
	}
	respond(w, http.StatusOK, resp)
}

func decodeCredentialBody(w http.ResponseWriter, r *http.Request) (dtoV1.CredentialCreateRequest, bool) {
	var req dtoV1.CredentialCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON body")
		return req, false
	}
	if req.Password == "" {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "password is required",
			dtoV1.FieldError{Field: "password", Message: "must not be empty"})
		return req, false
	}
	return req, true
}

func marshalOptions(w http.ResponseWriter, r *http.Request, opts map[string]any) ([]byte, bool) {
	if len(opts) == 0 {
		return nil, true
	}
	b, err := json.Marshal(opts)
	if err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "options is not valid JSON")
		return nil, false
	}
	return b, true
}

// audit emits a structured log entry recording the credential mutation.
// FR-019: includes user_id, monitor_id, action, ts. Never the password value.
func (h *ResourceCredentialHandler) audit(ctx context.Context, action, resourceID string) {
	userID, _ := ctx.Value("user_id").(string)
	slog.Default().LogAttrs(ctx, slog.LevelInfo, "audit",
		slog.String("category", "credential"),
		slog.String("action", action),
		slog.String("user_id", userID),
		slog.String("resource_id", resourceID),
	)
}

// causeToWireString maps internal CheckFailureCause to the stable string set
// surfaced by /credentials/test responses (matches contracts/credentials-v1.yaml).
func causeToWireString(cause domain.CheckFailureCause) string {
	switch cause {
	case domain.ProtocolAuthFailed:
		return "auth_failed"
	case domain.ProtocolTLSHandshakeFailed:
		return "tls_handshake_failed"
	case domain.ProtocolDecryptFailed:
		return "decrypt_failed"
	case domain.ConnectionTimeout:
		return "connection_timeout"
	case domain.ConnectionRefused:
		return "connection_refused"
	case domain.ProtocolHandshakeFailed, domain.ProtocolUnexpectedResponse:
		return "protocol_mismatch"
	default:
		return string(cause)
	}
}

// Ensure unused-import safety net for safenet (used elsewhere in transitive callers).
var _ = safenet.SafeDial
