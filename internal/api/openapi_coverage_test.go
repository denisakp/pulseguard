package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/api/handler"
	"github.com/denisakp/ogoune/internal/config"
	"github.com/go-chi/chi/v5"
)

// buildRouterForCoverage builds the full router with placeholder handlers — enough
// to enumerate registered routes via chi.Walk (handlers are never invoked here).
func buildRouterForCoverage() http.Handler {
	return NewRouter(
		handler.NewPingHandler(&mockRouterPingService{}),
		handler.NewMonitoringActivityHandler(nil),
		handler.NewComponentHandler(nil),
		handler.NewStatusPageHandler(nil),
		handler.NewPublicStatusHandler(nil),
		nil, // publicCacheMetrics
		handler.NewStatusPageSettingsHandler(nil),
		handler.NewNotificationHandler(nil),
		handler.NewMaintenanceHandler(nil),
		handler.NewStatsHandler(nil),
		handler.NewSystemHandler(),
		handler.NewRuntimeConfigHandler(&config.Config{SSLProvider: "external"}, "test"),
		handler.NewAuthHandler(nil, nil),
		handler.NewAccountHandler(nil, nil),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, // incidentUpdateV1Handler (spec 086 US2)
		nil, nil, nil, // hostV1Handler, agentStreamV1Handler, hostCredentialService
		false,
		&config.Config{
			RateLimitAuth:         10,
			RateLimitAuthWindow:   1 * time.Minute,
			RateLimitGlobal:       100,
			RateLimitGlobalWindow: 1 * time.Minute,
		},
	)
}

// TestOpenAPICoverage_AllV1RoutesInContract — FR-003: every registered /v1 route
// MUST appear in the generated contract (guard against frontend-blind endpoints).
// Chi routes are `/v1/...` (mounted at /api by bootstrap); the contract uses the
// @BasePath /api/v1, so its paths are basePath-relative — we strip `/v1`.
func TestOpenAPICoverage_AllV1RoutesInContract(t *testing.T) {
	specData, err := os.ReadFile("../../api/openapi/v1.json")
	if err != nil {
		t.Fatalf("read contract (run `make openapi`): %v", err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(specData, &spec); err != nil {
		t.Fatalf("parse contract: %v", err)
	}

	router, ok := buildRouterForCoverage().(chi.Routes)
	if !ok {
		t.Fatal("router is not chi.Routes")
	}

	var missing []string
	seen := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// Only public v1 API routes; skip the unversioned internal surface.
		if !strings.HasPrefix(route, "/v1/") {
			return nil
		}
		// Non-endpoint mounts (docs UI, spec file) are not contract operations.
		if strings.Contains(route, "/docs") || strings.HasSuffix(route, "openapi.json") {
			return nil
		}
		// The agent metrics ingestion route is a WebSocket upgrade,
		// not a REST operation, so it is not part of the OpenAPI contract.
		if strings.HasPrefix(route, "/v1/agent/stream") {
			return nil
		}
		contractPath := strings.TrimPrefix(route, "/v1")     // /v1/monitors → /monitors
		contractPath = strings.TrimSuffix(contractPath, "/") // chi trailing slash
		if contractPath == "" {
			return nil
		}
		key := method + " " + contractPath
		if seen[key] {
			return nil
		}
		seen[key] = true
		if _, present := spec.Paths[contractPath]; !present {
			missing = append(missing, key)
		}
		return nil
	}
	if err := chi.Walk(router, walk); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	if len(missing) > 0 {
		t.Fatalf("v1 routes missing from the OpenAPI contract (add swaggo annotations + run `make openapi`):\n  %s",
			strings.Join(missing, "\n  "))
	}
}
