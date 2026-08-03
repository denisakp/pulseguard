package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/api/handler"
	"github.com/denisakp/ogoune/internal/config"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/stretchr/testify/assert"
)

type mockRouterPingService struct{}

func (m *mockRouterPingService) GetResourceByHeartbeatSlug(ctx context.Context, slug string) (*domain.Resource, error) {
	return nil, service.ErrResourceNotFound
}

func (m *mockRouterPingService) MarkHeartbeatPing(ctx context.Context, resourceID string, at time.Time) error {
	return nil
}

func (m *mockRouterPingService) HandleHeartbeatRecovery(ctx context.Context, resource *domain.Resource) error {
	return nil
}

func TestNewRouter_PingIsPublicAndRootResourcesRemoved(t *testing.T) {
	pingHandler := handler.NewPingHandler(&mockRouterPingService{})
	activityHandler := handler.NewMonitoringActivityHandler(nil)
	statusPageHandler := handler.NewStatusPageHandler(nil)
	publicStatusHandler := handler.NewPublicStatusHandler(nil)
	statusPageSettingsHandler := handler.NewStatusPageSettingsHandler(nil)
	maintenanceHandler := handler.NewMaintenanceHandler(nil)
	statsHandler := handler.NewStatsHandler(nil)
	systemHandler := handler.NewSystemHandler()
	runtimeConfigHandler := handler.NewRuntimeConfigHandler(&config.Config{SSLProvider: "external"}, "test")
	authHandler := handler.NewAuthHandler(nil, nil)
	accountHandler := handler.NewAccountHandler(nil, nil)

	router := NewRouter(
		pingHandler,
		activityHandler,
		statusPageHandler,
		publicStatusHandler,
		nil, // publicCacheMetrics
		statusPageSettingsHandler,
		maintenanceHandler,
		statsHandler,
		systemHandler,
		runtimeConfigHandler,
		authHandler,
		accountHandler,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil, // incidentUpdateV1Handler (spec 086 US2)
		nil, // searchV1Handler (spec 084)
		nil, // hostV1Handler
		nil, // agentStreamV1Handler
		nil, // hostCredentialService
		false,
		&config.Config{
			RateLimitAuth:         10,
			RateLimitAuthWindow:   1 * time.Minute,
			RateLimitGlobal:       100,
			RateLimitGlobalWindow: 1 * time.Minute,
		},
	)

	pingReq := httptest.NewRequest(http.MethodGet, "/ping/550e8400-e29b-41d4-a716-446655440111", nil)
	pingRec := httptest.NewRecorder()
	router.ServeHTTP(pingRec, pingReq)
	assert.Equal(t, http.StatusNotFound, pingRec.Code)

	// Root /resources was migrated to /api/v1/monitors and deleted (spec 085
	// Phase 2b): the route no longer exists, so it 404s instead of 401.
	resourcesReq := httptest.NewRequest(http.MethodGet, "/resources/", nil)
	resourcesRec := httptest.NewRecorder()
	router.ServeHTTP(resourcesRec, resourcesReq)
	assert.Equal(t, http.StatusNotFound, resourcesRec.Code)
}
