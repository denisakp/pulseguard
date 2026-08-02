// Package middleware — HostRouter routes requests reaching the configured
// custom status-page hostname to the public status bundle and blocks
// admin endpoints on that host.
package middleware

import (
	"net/http"
	"strings"
	"sync/atomic"
)

// HostRouter intercepts requests before any other routing. When the request's
// Host matches the configured custom_domain AND the domain is verified, the
// request is dispatched to the status-bundle handler instead of the admin
// router. Non-public API paths return 404 on the custom host.
type HostRouter struct {
	cfg         atomic.Pointer[hostRouterCfg]
	serveStatus http.Handler
}

type hostRouterCfg struct {
	domain   string // already normalized
	verified bool
}

// NewHostRouter creates a HostRouter that delegates matching requests to
// serveStatus. serveStatus is the public bundle handler (status.html + assets).
func NewHostRouter(serveStatus http.Handler) *HostRouter {
	h := &HostRouter{serveStatus: serveStatus}
	h.cfg.Store(&hostRouterCfg{})
	return h
}

// Set updates the configured custom domain + verification status. Empty
// domain disables routing. status must equal "verified" to activate.
func (h *HostRouter) Set(domain, status string) {
	domain = normalizeHost(strings.TrimSpace(domain))
	h.cfg.Store(&hostRouterCfg{
		domain:   domain,
		verified: status == "verified",
	})
}

// Middleware returns the chi-compatible middleware.
func (h *HostRouter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := h.cfg.Load()
		if cfg == nil || cfg.domain == "" || !cfg.verified {
			next.ServeHTTP(w, r)
			return
		}
		if normalizeHost(r.Host) != cfg.domain {
			next.ServeHTTP(w, r)
			return
		}
		if !allowedOnCustomHost(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		h.serveStatus.ServeHTTP(w, r)
	})
}

// normalizeHost lowercases, strips port suffix, and strips a trailing dot.
func normalizeHost(h string) string {
	h = strings.ToLower(h)
	if i := strings.LastIndex(h, ":"); i >= 0 {
		h = h[:i]
	}
	h = strings.TrimSuffix(h, ".")
	return h
}

// allowedOnCustomHost is the closed allow-list of paths the public bundle
// must reach. Everything else (admin API endpoints, etc.) returns 404 on the
// custom host.
func allowedOnCustomHost(path string) bool {
	// Non-API paths → SPA navigation / static assets (handled by serveStatus).
	if !strings.HasPrefix(path, "/api/") {
		return true
	}
	// Public API allow-list.
	switch {
	case path == "/api/status",
		strings.HasPrefix(path, "/api/status/"):
		return true
	case path == "/api/config/runtime":
		return true
	case path == "/api/health":
		return true
	}
	return false
}
