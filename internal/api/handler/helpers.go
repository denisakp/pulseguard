package handler

import (
	"net/http"

	"github.com/denisakp/ogoune/internal/api/response"
)

// respondJSON writes a JSON response with the given status code and payload.
// Shared across the (non-versioned root) handlers in this package.
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	response.JSON(w, status, payload)
}

// respondError writes a JSON error response ({"error": "message"}) with the given status.
func respondError(w http.ResponseWriter, status int, message string) {
	response.Error(w, status, message)
}
