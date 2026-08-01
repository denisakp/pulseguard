package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/denisakp/ogoune/internal/service"
	"github.com/denisakp/ogoune/pkg/problemdetail"
)

// HostCredentialAuth authenticates the agent ingestion route by a per-host
// bearer credential (ag_live_…). On success it binds host_id + auth_method to
// the request context. Missing/invalid/revoked → 401. Operator tokens (JWT or
// pk_live_ API keys) are not in ag_live_ format and are therefore rejected.
func HostCredentialAuth(svc *service.HostCredentialService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := extractBearer(r)
			if raw == "" || svc == nil {
				writeAgentUnauthorized(w)
				return
			}
			cred, err := svc.Authenticate(r.Context(), raw)
			if err != nil {
				writeAgentUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyHostID, cred.HostID)
			ctx = context.WithValue(ctx, "auth_method", "host_credential")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ctxKeyHostID is the context key carrying the authenticated agent's host id.
const ctxKeyHostID = "host_id"

// HostIDFromContext returns the authenticated host id set by HostCredentialAuth.
func HostIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyHostID).(string)
	return v, ok && v != ""
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

func writeAgentUnauthorized(w http.ResponseWriter) {
	problemdetail.Write(w, problemdetail.New(problemUnauthorized, "Unauthorized", http.StatusUnauthorized, "invalid or revoked host credential"))
}
