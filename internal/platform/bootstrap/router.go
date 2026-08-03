package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/denisakp/ogoune/internal/api"
	"github.com/denisakp/ogoune/internal/api/handler"
	v1handler "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/denisakp/ogoune/internal/metrics"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/denisakp/ogoune/internal/service/resourceimport"
	"github.com/go-chi/chi/v5"
)

// asCacheRecorder lifts the nil-typed PublicStatusMetrics back to the
// interface the middleware expects, returning nil when metrics are
// disabled so the typed nil doesn't escape into the middleware chain.
func asCacheRecorder(m *metricsModule) middleware.PublicStatusCacheRecorder {
	if m == nil {
		return nil
	}
	return m
}

// metricsModule is a type alias so we don't have to import `metrics` here.
type metricsModule = metrics.PublicStatusMetrics

// InitRouter creates handlers, builds the Chi router, and mounts static files.
func InitRouter(app *App) {
	cfg := app.Cfg
	slog.Info("initializing API server")

	// Initialize API services used only by handlers
	activityService := service.NewMonitoringActivityService(app.MonitoringActivityRepo, app.UptimeDailyAggRepo)
	tagService := service.NewTagService(app.TagsRepo)
	statusPageSettingsService := service.NewStatusPageSettingsService(app.StatusPageSettingsRepo)
	statusPageSettingsService.Configure("status.ogoune.app", cfg.SSLProvider)
	statusPageService := service.NewStatusPageService(app.ResourceRepo, app.IncidentRepo, app.MonitoringActivityRepo, app.MaintenanceRepo, app.StatusPageSettingsRepo, app.ComponentRepo)
	incidentAPIService := service.NewIncidentService(app.IncidentRepo, app.IncidentEventStepRepo)
	liveSnapshotService := service.NewLiveSnapshotService(app.ResourceService, activityService, incidentAPIService)
	notificationService := service.NewNotificationService(app.ResourceRepo, app.NotificationChannelRepo)
	notificationService.SetEventsRepo(app.NotificationRepo)
	maintenanceAPIService := service.NewMaintenanceService(app.MaintenanceRepo, app.MaintenanceScheduler)
	statsService := service.NewStatsService(app.MonitoringActivityRepo, app.IncidentRepo)

	// Initialize handlers
	pingHandler := handler.NewPingHandler(app.ResourceService)
	activityHandler := handler.NewMonitoringActivityHandler(activityService)
	statusPageHandler := handler.NewStatusPageHandler(statusPageService)
	publicStatusHandler := handler.NewPublicStatusHandler(app.PublicStatusService)
	statusPageSettingsHandler := handler.NewStatusPageSettingsHandler(statusPageSettingsService)
	incidentHandler := handler.NewIncidentHandler(incidentAPIService)
	incidentUpdateHandler := handler.NewIncidentUpdateHandler(app.IncidentUpdateService)
	notificationHandler := handler.NewNotificationHandler(notificationService)
	maintenanceHandler := handler.NewMaintenanceHandler(maintenanceAPIService)
	statsHandler := handler.NewStatsHandler(statsService)
	systemHandler := handler.NewSystemHandler()
	runtimeConfigHandler := handler.NewRuntimeConfigHandler(cfg, AppVersion)
	authHandler := handler.NewAuthHandler(app.AuthService, app.JWTManager)
	accountHandler := handler.NewAccountHandler(app.AuthService, app.APIKeyService)
	sessionHandler := handler.NewSessionHandler(app.SessionService)
	componentHandler := handler.NewComponentHandler(app.ComponentService)

	// V1 handlers
	monitorV1Handler := v1handler.NewMonitorHandler(app.ResourceService, liveSnapshotService, activityService)
	incidentV1Handler := v1handler.NewIncidentHandler(incidentAPIService)
	searchV1Handler := v1handler.NewSearchHandler(service.NewSearchService(app.ResourceRepo, app.IncidentRepo))
	channelV1Handler := v1handler.NewNotificationChannelHandler(notificationService)
	componentV1Handler := v1handler.NewComponentHandler(app.ComponentRepo)
	tagV1Handler := v1handler.NewTagHandler(tagService)
	statusPageV1Handler := v1handler.NewStatusPageV1Handler(app.ComponentRepo)
	heartbeatV1Handler := v1handler.NewHeartbeatV1Handler(app.ResourceService)
	twoFactorV1Handler := v1handler.NewTwoFactorHandler(app.TwoFactorService, app.AuthService)
	escalationV1Handler := v1handler.NewEscalationHandler(app.EscalationService)

	credentialService := service.NewResourceCredentialService(app.ResourceCredentialRepo, app.ResourceRepo)
	credentialTester := service.NewResourceCredentialTester(app.ResourceRepo, BuildStrategies())
	credentialV1Handler := v1handler.NewResourceCredentialHandler(credentialService, credentialTester)

	toolboxService := service.NewToolboxService(app.ResourceRepo, 10*time.Second)
	toolboxV1Handler := v1handler.NewToolboxHandler(toolboxService)

	notificationFeedV1Handler := v1handler.NewNotificationFeedHandler(app.NotificationFeedService)
	dashboardV1Handler := v1handler.NewDashboardHandler(app.DashboardService)
	reportV1Handler := v1handler.NewReportHandler(app.ReportService)
	announcementV1Handler := v1handler.NewAnnouncementHandler(app.AnnouncementService)
	integrationsV1Handler := v1handler.NewIntegrationsHandler(app.IntegrationsService)

	// Bulk resource import/export
	resourceImportService := resourceimport.NewService(app.ResourceService, app.ComponentRepo, app.NotificationChannelRepo)
	resourceImportV1Handler := v1handler.NewResourceImportHandler(resourceImportService)

	// Agent device monitoring
	hostV1Handler := v1handler.NewHostHandler(app.HostService, app.HostMetricsService)
	agentStreamV1Handler := v1handler.NewAgentStreamHandler(app.HostMetricsService)

	// Set APP_VERSION env
	err := os.Setenv("APP_VERSION", AppVersion)
	if err != nil {
		return
	}

	apiHandler := api.NewRouter(pingHandler, activityHandler, componentHandler, statusPageHandler, publicStatusHandler, asCacheRecorder(app.PublicStatusCacheMetr), statusPageSettingsHandler, incidentHandler, incidentUpdateHandler, notificationHandler, maintenanceHandler, statsHandler, systemHandler, runtimeConfigHandler, authHandler, accountHandler, app.AuthService, app.APIKeyService, app.SessionService, sessionHandler, twoFactorV1Handler, escalationV1Handler, monitorV1Handler, incidentV1Handler, searchV1Handler, channelV1Handler, componentV1Handler, tagV1Handler, statusPageV1Handler, heartbeatV1Handler, credentialV1Handler, toolboxV1Handler, notificationFeedV1Handler, dashboardV1Handler, reportV1Handler, announcementV1Handler, integrationsV1Handler, resourceImportV1Handler, hostV1Handler, agentStreamV1Handler, app.HostCredentialService, cfg.EnableSwagger, cfg)

	// Root router
	rootRouter := chi.NewRouter()
	rootRouter.Mount("/api", apiHandler)
	rootRouter.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	if cfg.MetricsEnabled && app.MetricsRegistry != nil {
		rootRouter.Handle("/metrics", metrics.NewHandler(cfg.MetricsToken, app.MetricsRegistry))
		slog.Info("/metrics route registered")
	}

	// Serve static files if available
	staticDir := cfg.StaticDir
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		slog.Info("serving static files", "dir", staticDir)
		serveStaticFiles(rootRouter, staticDir)
	} else {
		slog.Warn("static directory not found, frontend will not be served", "dir", staticDir)
	}

	// Spec 060 / US6 — Host router: dispatch requests reaching the custom
	// status-page hostname to the public bundle instead of the admin app.
	// The HostRouter wraps the full rootRouter so any unmatched custom-host
	// request also lands on the status bundle.
	var topHandler http.Handler = rootRouter
	if info, err := os.Stat(cfg.StaticDir); err == nil && info.IsDir() {
		statusBundle := handler.NewStaticStatusHandler(cfg.StaticDir, app.PublicStatusService)
		hostRouter := middleware.NewHostRouter(statusBundle)
		// Seed the cache from current settings (if any) and wire the refresh
		// callback so the middleware reflects future saves / verifications.
		if app.StatusPageSettingsRepo != nil {
			if s, err := app.StatusPageSettingsRepo.Get(context.Background()); err == nil && s != nil {
				hostRouter.Set(s.CustomDomain, string(s.CustomDomainStatus))
			}
		}
		statusPageSettingsHandler.SetDomainRefresh(hostRouter.Set)
		topHandler = hostRouter.Middleware(rootRouter)
	}

	app.RootRouter = rootRouter
	app.Server = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      topHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
