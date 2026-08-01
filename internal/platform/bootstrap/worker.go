package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/maintenance"
	"github.com/denisakp/ogoune/internal/monitoring"
	"github.com/denisakp/ogoune/internal/scheduler"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/denisakp/ogoune/internal/worker"
	"github.com/hibiken/asynq"
)

// InitWorker starts the scheduler runtime, dispatchers, heartbeat detector, and Asynq processor.
func InitWorker(app *App) {
	enrichmentService := service.NewEnrichmentService(30 * time.Second)
	app.ComponentService = service.NewComponentService(app.ComponentRepo, app.ResourceRepo, app.NotificationChannelRepo)
	pendingRetryService := service.NewPendingNotificationRetryService(
		app.NotificationRepo,
		app.IncidentRepo,
		app.NotificationChannelRepo,
		app.ComponentRepo,
		"",
		24*time.Hour,
	)

	if err := app.RuntimeScheduler.Start(context.Background(), NewResourceRepositoryAdapter(app.ResourceRepo)); err != nil {
		slog.Error("failed to start scheduler runtime", "error", err)
		os.Exit(1)
	}

	if app.SchedulerCfg.Mode == scheduler.ModeTimingWheel {
		initTimingWheelWorker(app, enrichmentService)
	} else {
		bootstrapAsynqScheduling(app)
	}

	runStartupPendingNotificationRetry(context.Background(), pendingRetryService)

	heartbeatDetector := service.NewHeartbeatDetectorService(app.ResourceRepo, app.DetectorIncidentSvc)
	if err := startHeartbeatDetector(context.Background(), heartbeatDetector, 60*time.Second); err != nil {
		slog.Error("failed to start heartbeat detector", "error", err)
		os.Exit(1)
	}
	slog.Info("heartbeat detector started", "interval", "60s")

	if app.SchedulerCfg.Mode == scheduler.ModeAsynq {
		initAsynqProcessor(app, enrichmentService)
	} else {
		slog.Info("skipping Asynq worker initialization (TimingWheel mode)")
	}
}

// initTimingWheelWorker sets up the in-process dispatcher and daily expiry check for TimingWheel mode.
func initTimingWheelWorker(app *App, enrichmentService *service.EnrichmentService) {
	slog.Info("TimingWheel scheduler started")

	activeResources, err := app.ResourceRepo.FindActive(context.Background(), 10000, 0)
	if err == nil {
		slog.Info("TimingWheel loaded active resources", "count", len(activeResources))
	}

	tw, ok := app.RuntimeScheduler.(*scheduler.TimingWheelScheduler)
	if !ok {
		return
	}

	strategies := BuildStrategies()
	executor := domain.NewCheckExecutor(strategies, app.MetricsRecorder)

	incidentService := monitoring.NewIncidentService(
		app.IncidentRepo,
		app.IncidentEventStepRepo,
		app.NotificationRepo,
		app.NotificationChannelRepo,
		app.IncidentDiagnosticsRepo,
		nil,
	)
	if app.IncidentUpdateService != nil {
		incidentService.SetUpdateSeeder(app.IncidentUpdateService)
	}
	if app.NotificationFeedService != nil {
		incidentService.SetNotificationEmitter(app.NotificationFeedService)
	}
	app.DetectorIncidentSvc = incidentService

	monitoringHandler := worker.NewMonitoringTaskHandler(app.ResourceRepo, app.MonitoringActivityRepo, app.MaintenanceRepo, app.IncidentDiagnosticsRepo, executor, incidentService, app.ComponentService, app.ConfirmationScheduler)

	startTimingWheelDispatcher(tw, monitoringHandler, app.SchedulerCfg.TimingWheel.MaxWorkers)
	startTimingWheelExpiryCheck(app, enrichmentService)
	startTimingWheelNotificationRetention(app)
	startTimingWheelReportCheck(app)
	startTimingWheelHostMetricsRetention(app)
}

// startTimingWheelHostMetricsRetention runs the daily host-metrics retention
// (decimate + purge) in-process (spec 079): once at startup, then daily.
func startTimingWheelHostMetricsRetention(app *App) {
	if app.HostMetricsService == nil {
		return
	}
	handler := worker.NewHostMetricsRetentionHandler(app.HostMetricsService)
	_ = handler.ProcessTask(context.Background(), asynq.NewTask(worker.TypeHostMetricsRetention, nil)) // startup catch-up
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := handler.ProcessTask(context.Background(), asynq.NewTask(worker.TypeHostMetricsRetention, nil)); err != nil {
				slog.Error("TimingWheel host:metrics:retention failed", "error", err)
			}
		}
	}()
	slog.Info("TimingWheel daily host metrics retention scheduled")
}

// startTimingWheelReportCheck runs the monthly report catch-up scan in-process
// (spec 076): once at startup (self-heals a missed month on restart) then daily.
func startTimingWheelReportCheck(app *App) {
	if app.ReportService == nil {
		return
	}
	handler := worker.NewReportTaskHandler(app.ReportService)
	_ = handler.ProcessTask(context.Background(), asynq.NewTask(worker.TypeReportCheck, nil)) // startup catch-up
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = handler.ProcessTask(context.Background(), asynq.NewTask(worker.TypeReportCheck, nil))
		}
	}()
	slog.Info("TimingWheel daily report check scheduled")
}

// startTimingWheelNotificationRetention runs the daily feed-retention prune in-process (spec 072).
func startTimingWheelNotificationRetention(app *App) {
	if app.NotificationFeedRepo == nil {
		return
	}
	handler := worker.NewNotificationRetentionHandler(app.NotificationFeedRepo, app.Cfg.NotificationRetentionDays)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := handler.ProcessTask(context.Background(), asynq.NewTask(worker.TypeNotificationRetention, nil)); err != nil {
				slog.Error("TimingWheel notification:retention failed", "error", err)
			}
		}
	}()
	slog.Info("TimingWheel daily notification retention scheduled", "retention_days", app.Cfg.NotificationRetentionDays)
}

func startTimingWheelDispatcher(tw *scheduler.TimingWheelScheduler, handler *worker.MonitoringTaskHandler, maxWorkers int) {
	workers := maxWorkers
	if workers <= 0 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		go func() {
			for job := range tw.CheckJobs() {
				if job == nil || job.ResourceID == "" {
					continue
				}
				processTimingWheelJob(job, handler)
			}
		}()
	}

	slog.Info("TimingWheel check dispatcher started", "workers", workers)
}

func processTimingWheelJob(job *scheduler.CheckJob, handler *worker.MonitoringTaskHandler) {
	payload, err := json.Marshal(map[string]string{"resource_id": job.ResourceID})
	if err != nil {
		slog.Error("failed to marshal timingwheel payload", "resource_id", job.ResourceID, "error", err)
		return
	}

	task := asynq.NewTask("monitoring:check", payload)
	if err := handler.ProcessTask(context.Background(), task); err != nil {
		slog.Error("TimingWheel check failed", "resource_id", job.ResourceID, "error", err)
	}
}

func startTimingWheelExpiryCheck(app *App, enrichmentService *service.EnrichmentService) {
	twExpiryNotificationLogRepo := app.ExpiryNotificationLogRepo
	twExpiryService := service.NewExpiryNotificationService(
		twExpiryNotificationLogRepo,
		app.NotificationChannelRepo,
		service.ParseGlobalThresholds(app.Cfg.ExpiryAlertThresholds),
	)
	twExpiryHandler := worker.NewExpiryTaskHandler(app.ResourceRepo, app.NotificationChannelRepo, enrichmentService, twExpiryService)
	go func() {
		runExpiry := func() {
			if err := twExpiryHandler.ProcessTask(context.Background(), asynq.NewTask(worker.TypeExpiryCheck, nil)); err != nil {
				slog.Error("TimingWheel expiry:check failed", "error", err)
			}
		}
		// Startup catch-up so freshly-renewed certificates are refreshed without
		// waiting up to 24h for the first tick.
		runExpiry()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			runExpiry()
		}
	}()
	slog.Info("TimingWheel daily expiry check scheduled")
}

// bootstrapAsynqScheduling schedules all active resources via the Asynq adapter at startup.
func bootstrapAsynqScheduling(app *App) {
	slog.Info("bootstrapping: scheduling active resources with Asynq")

	bootstrapCtx := context.Background()
	activeResources, err := app.ResourceRepo.FindActive(bootstrapCtx, 10000, 0)
	if err != nil {
		slog.Error("failed to fetch active resources during bootstrap", "error", err)
		os.Exit(1)
	}

	slog.Info("found active resources to schedule", "count", len(activeResources))

	successCount := 0
	failureCount := 0

	for _, resource := range activeResources {
		slog.Debug("scheduling resource", "name", resource.Name, "resource_id", resource.ID)

		if err := app.SchedulerAdapter.Schedule(bootstrapCtx, resource); err != nil {
			slog.Error("failed to schedule resource", "resource_id", resource.ID, "error", err)
			failureCount++
		} else {
			slog.Debug("successfully scheduled resource", "resource_id", resource.ID)
			successCount++
		}
	}

	slog.Info("bootstrap completed",
		"total", len(activeResources),
		"success", successCount,
		"failed", failureCount,
	)

	if failureCount > 0 {
		slog.Warn("some resources failed to schedule", "failed", failureCount)
	}
}

// initAsynqProcessor creates and starts the Asynq worker processor with all task handlers.
func initAsynqProcessor(app *App, enrichmentService *service.EnrichmentService) {
	slog.Info("initializing background worker for Asynq")

	strategies := BuildStrategies()
	executor := domain.NewCheckExecutor(strategies, app.MetricsRecorder)

	incidentService := monitoring.NewIncidentService(
		app.IncidentRepo,
		app.IncidentEventStepRepo,
		app.NotificationRepo,
		app.NotificationChannelRepo,
		app.IncidentDiagnosticsRepo,
		app.AsynqClient,
	)
	if app.IncidentUpdateService != nil {
		incidentService.SetUpdateSeeder(app.IncidentUpdateService)
	}
	if app.NotificationFeedService != nil {
		incidentService.SetNotificationEmitter(app.NotificationFeedService)
	}
	app.DetectorIncidentSvc = incidentService

	monitoringHandler := worker.NewMonitoringTaskHandler(app.ResourceRepo, app.MonitoringActivityRepo, app.MaintenanceRepo, app.IncidentDiagnosticsRepo, executor, incidentService, app.ComponentService, app.ConfirmationScheduler)
	maintenanceTaskHandler := maintenance.NewTaskHandler(app.MaintenanceRepo, &maintenance.AsynqClientAdapter{Client: app.AsynqClient})

	expiryNotificationLogRepo := app.ExpiryNotificationLogRepo
	expiryService := service.NewExpiryNotificationService(
		expiryNotificationLogRepo,
		app.NotificationChannelRepo,
		service.ParseGlobalThresholds(app.Cfg.ExpiryAlertThresholds),
	)
	expiryTaskHandler := worker.NewExpiryTaskHandler(app.ResourceRepo, app.NotificationChannelRepo, enrichmentService, expiryService)

	if _, err := app.AsynqScheduler.Register("@daily", asynq.NewTask(worker.TypeExpiryCheck, nil)); err != nil {
		slog.Error("failed to register expiry:check scheduler", "error", err)
	}

	var retentionHandler *worker.NotificationRetentionHandler
	if app.NotificationFeedRepo != nil {
		retentionHandler = worker.NewNotificationRetentionHandler(app.NotificationFeedRepo, app.Cfg.NotificationRetentionDays)
		if _, err := app.AsynqScheduler.Register("@daily", asynq.NewTask(worker.TypeNotificationRetention, nil)); err != nil {
			slog.Error("failed to register notification:retention scheduler", "error", err)
		}
	}

	// Monthly report catch-up scan (spec 076) — daily tick, idempotent per period.
	var reportHandler *worker.ReportTaskHandler
	if app.ReportService != nil {
		reportHandler = worker.NewReportTaskHandler(app.ReportService)
		if _, err := app.AsynqScheduler.Register("@daily", asynq.NewTask(worker.TypeReportCheck, nil)); err != nil {
			slog.Error("failed to register report:check scheduler", "error", err)
		}
	}

	// Host metrics retention (spec 079) — daily decimate + purge.
	var hostMetricsRetentionHandler *worker.HostMetricsRetentionHandler
	if app.HostMetricsService != nil {
		hostMetricsRetentionHandler = worker.NewHostMetricsRetentionHandler(app.HostMetricsService)
		if _, err := app.AsynqScheduler.Register("@daily", asynq.NewTask(worker.TypeHostMetricsRetention, nil)); err != nil {
			slog.Error("failed to register host:metrics:retention scheduler", "error", err)
		}
	}

	app.Processor = worker.NewProcessor(app.RedisOpt, monitoringHandler, maintenanceTaskHandler, expiryTaskHandler, retentionHandler, reportHandler, hostMetricsRetentionHandler, worker.Config{
		Concurrency: app.SchedulerCfg.Asynq.Concurrency,
	})

	go func() {
		slog.Info("starting Asynq worker server")
		if err := app.Processor.Start(context.Background()); err != nil {
			slog.Error("could not run Asynq worker server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("waiting for worker to start", "delay", "10s")
	time.Sleep(10 * time.Second)
	slog.Info("background worker started")
}
