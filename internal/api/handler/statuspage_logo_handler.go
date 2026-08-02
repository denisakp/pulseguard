// Package handler — status page logo upload.
package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/denisakp/ogoune/internal/api/response"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/service"
)

const maxLogoBytes = 500 * 1024 // 500 KB per spec 060 contract

// allowedLogoMimes is the closed set of accepted MIME types for logo uploads.
var allowedLogoMimes = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/svg+xml": ".svg",
	"image/webp":    ".webp",
}

// UploadLogo — POST /api/settings/statuspage/logo?slot={light|dark|favicon}.
// multipart/form-data with a "file" part.
func (h *StatusPageSettingsHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	slot := r.URL.Query().Get("slot")
	if err := service.ValidateLogoSlot(slot); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "UNKNOWN_SLOT: "+err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBytes+1024)
	if err := r.ParseMultipartForm(maxLogoBytes + 1024); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "LOGO_TOO_LARGE: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "missing 'file' part")
		return
	}
	defer file.Close()

	if header.Size > maxLogoBytes {
		response.Error(w, http.StatusUnprocessableEntity, "LOGO_TOO_LARGE: exceeds 500 KB")
		return
	}

	mime := header.Header.Get("Content-Type")
	ext, ok := allowedLogoMimes[mime]
	if !ok {
		response.Error(w, http.StatusUnprocessableEntity, "LOGO_INVALID_MIME: "+mime)
		return
	}

	dir := uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create upload dir")
		return
	}
	filename := fmt.Sprintf("%s-%s%s", slot, ulid.Make().String(), ext)
	abs := filepath.Join(dir, filename)
	out, err := os.Create(abs)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to write upload")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to copy upload")
		return
	}

	url := "/static/uploads/statuspage/" + filename
	updated, err := h.service.SetLogoURL(r.Context(), slot, url)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to persist logo URL")
		return
	}
	response.JSON(w, http.StatusOK, toSettingsResponse(updated))
}

// DeleteLogo — DELETE /api/settings/statuspage/logo?slot=...
func (h *StatusPageSettingsHandler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	slot := r.URL.Query().Get("slot")
	if err := service.ValidateLogoSlot(slot); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "UNKNOWN_SLOT: "+err.Error())
		return
	}

	existing, err := h.service.GetSettings(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	current := readLogoSlot(existing, slot)
	if current == "" {
		response.Error(w, http.StatusNotFound, "slot is empty")
		return
	}

	// Best-effort delete of the on-disk asset; never block on this.
	if file := localPathFromURL(current); file != "" {
		_ = os.Remove(file)
	}

	if _, err := h.service.ClearLogo(r.Context(), slot); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to clear logo")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readLogoSlot(s *domain.StatusPageSettings, slot string) string {
	if s == nil {
		return ""
	}
	switch slot {
	case "light":
		return s.LogoURLLight
	case "dark":
		return s.LogoURLDark
	case "favicon":
		return s.FaviconURL
	}
	return ""
}

func localPathFromURL(url string) string {
	prefix := "/static/uploads/statuspage/"
	if !strings.HasPrefix(url, prefix) {
		return ""
	}
	return filepath.Join(uploadDir(), strings.TrimPrefix(url, prefix))
}

func uploadDir() string {
	if d := os.Getenv("STATIC_DIR"); d != "" {
		return filepath.Join(d, "uploads", "statuspage")
	}
	return filepath.Join("static", "uploads", "statuspage")
}
