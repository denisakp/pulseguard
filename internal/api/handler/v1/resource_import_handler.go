package v1

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service/resourceimport"
)

// maxManifestBytes bounds the request body for import (defensive; row cap is enforced after parse).
const maxManifestBytes = 5 << 20 // 5 MiB

// ResourceImportServiceInterface defines the importer/exporter methods used by the handler.
type ResourceImportServiceInterface interface {
	Import(ctx context.Context, raw []byte, opts dtoV1.ImportOptions) (*dtoV1.ImportReport, error)
	ExportYAML(ctx context.Context) ([]byte, error)
}

// ResourceImportHandler handles v1 bulk import/export of monitors.
type ResourceImportHandler struct {
	service ResourceImportServiceInterface
}

// NewResourceImportHandler creates a new ResourceImportHandler.
func NewResourceImportHandler(svc ResourceImportServiceInterface) *ResourceImportHandler {
	return &ResourceImportHandler{service: svc}
}

// Import handles POST /api/v1/monitors/import
//
// @Summary     Bulk-import monitors from a YAML manifest
// @Tags        monitors
// @Security    BearerAuth
// @Accept      plain
// @Produce     json
// @Param       dryRun          query bool   false "Validate only, write nothing"
// @Param       duplicatePolicy query string false "How to treat existing names (skip|error, default skip)"
// @Param       body body string true "YAML manifest"
// @Success     200 {object} dtoV1.SingleResponse[dtoV1.ImportReport]
// @Failure     400 {object} dtoV1.ErrorResponse
// @Failure     403 {object} dtoV1.ErrorResponse
// @Failure     422 {object} dtoV1.SingleResponse[dtoV1.ImportReport]
// @Router      /monitors/import [post]
func (h *ResourceImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	raw, err := readManifestBody(r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	opts := dtoV1.ImportOptions{
		DryRun:          r.URL.Query().Get("dryRun") == "true",
		DuplicatePolicy: parseDuplicatePolicy(r.URL.Query().Get("duplicatePolicy")),
	}

	report, err := h.service.Import(r.Context(), raw, opts)
	if err != nil {
		switch {
		case errors.Is(err, resourceimport.ErrValidationFailed):
			// Per-row errors: return the report body with 422.
			respond(w, http.StatusUnprocessableEntity, report)
		case errors.Is(err, resourceimport.ErrManifestTooLarge):
			respondError(w, r, http.StatusBadRequest, "MANIFEST_TOO_LARGE", err.Error())
		default:
			var parseErr *resourceimport.ParseError
			if errors.As(err, &parseErr) {
				respondError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", parseErr.Error())
				return
			}
			respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to import manifest")
		}
		return
	}

	respond(w, http.StatusOK, report)
}

// Export handles GET /api/v1/monitors/export
//
// @Summary     Export all monitors as a YAML manifest
// @Tags        monitors
// @Security    BearerAuth
// @Produce     text/yaml
// @Success     200 {string} string "YAML manifest"
// @Router      /monitors/export [get]
func (h *ResourceImportHandler) Export(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.ExportYAML(r.Context())
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to export monitors")
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ogoune-monitors.yaml"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// readManifestBody reads the manifest from either a raw body or a multipart file field "manifest".
func readManifestBody(r *http.Request) ([]byte, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, _, err := r.FormFile("manifest")
		if err != nil {
			return nil, errors.New("missing multipart file field 'manifest'")
		}
		defer file.Close()
		return io.ReadAll(io.LimitReader(file, maxManifestBytes))
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxManifestBytes))
	if err != nil {
		return nil, errors.New("failed to read request body")
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("empty manifest body")
	}
	return data, nil
}

func parseDuplicatePolicy(v string) dtoV1.DuplicatePolicy {
	if v == string(dtoV1.DuplicatePolicyError) {
		return dtoV1.DuplicatePolicyError
	}
	return dtoV1.DuplicatePolicySkip
}
