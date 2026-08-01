package worker

import (
	"context"
	"log/slog"

	mhandler "github.com/denisakp/ogoune/internal/maintenance"
	"github.com/hibiken/asynq"
)

// Processor wraps the Asynq server and provides a unified interface for handling tasks.
type Processor struct {
	server             *asynq.Server
	monitoringHandler  *MonitoringTaskHandler
	maintenanceHandler *mhandler.TaskHandler
	expiryHandler      *ExpiryTaskHandler
	retentionHandler   *NotificationRetentionHandler
	reportHandler      *ReportTaskHandler
	hostRetention      *HostMetricsRetentionHandler
}

// Config controls worker concurrency and queue weights for the hosted Asynq lane.
type Config struct {
	Concurrency            int
	MonitoringQueueWeight  int
	MaintenanceQueueWeight int
}

// NewProcessor creates a new worker processor with the given handlers.
func NewProcessor(
	redisOpt asynq.RedisConnOpt,
	monitoringHandler *MonitoringTaskHandler,
	maintenanceHandler *mhandler.TaskHandler,
	expiryHandler *ExpiryTaskHandler,
	retentionHandler *NotificationRetentionHandler,
	reportHandler *ReportTaskHandler,
	hostRetention *HostMetricsRetentionHandler,
	config Config,
) *Processor {
	if config.Concurrency <= 0 {
		config.Concurrency = 10
	}
	if config.MonitoringQueueWeight <= 0 {
		config.MonitoringQueueWeight = 10
	}
	if config.MaintenanceQueueWeight <= 0 {
		config.MaintenanceQueueWeight = 5
	}

	server := asynq.NewServer(redisOpt, asynq.Config{
		// Configure the server with appropriate settings
		Concurrency: config.Concurrency,
		Queues: map[string]int{
			"monitoring":  config.MonitoringQueueWeight,
			"maintenance": config.MaintenanceQueueWeight,
		},
	})

	return &Processor{
		server:             server,
		monitoringHandler:  monitoringHandler,
		maintenanceHandler: maintenanceHandler,
		expiryHandler:      expiryHandler,
		retentionHandler:   retentionHandler,
		reportHandler:      reportHandler,
		hostRetention:      hostRetention,
	}
}

// Start begins processing tasks from the queues.
func (p *Processor) Start(ctx context.Context) error {
	// Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc("monitoring:check", p.monitoringHandler.ProcessTask)
	mux.HandleFunc("maintenance:start", p.maintenanceHandler.ProcessStart)
	mux.HandleFunc("maintenance:end", p.maintenanceHandler.ProcessEnd)
	mux.HandleFunc(TypeExpiryCheck, p.expiryHandler.ProcessTask)
	if p.retentionHandler != nil {
		mux.HandleFunc(TypeNotificationRetention, p.retentionHandler.ProcessTask)
	}
	if p.reportHandler != nil {
		mux.HandleFunc(TypeReportCheck, p.reportHandler.ProcessTask)
	}
	if p.hostRetention != nil {
		mux.HandleFunc(TypeHostMetricsRetention, p.hostRetention.ProcessTask)
	}
	// Note: notification:send handler removed - notifications are now sent directly by IncidentService

	// Start the server
	slog.Info("starting Asynq worker")
	return p.server.Start(mux)
}

// Stop gracefully shuts down the processor.
func (p *Processor) Stop() {
	if p == nil || p.server == nil {
		return
	}

	slog.Info("shutting down Asynq worker")
	p.server.Stop()
}
