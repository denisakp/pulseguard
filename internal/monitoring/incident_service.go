package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/pkg/notifier"
	"github.com/hibiken/asynq"
)

// IncidentUpdateSeeder is the minimal surface the incident lifecycle needs
// to seed user-facing status updates. Defined inline to
// avoid pulling internal/service into the monitoring package.
type IncidentUpdateSeeder interface {
	AutoSeedOnDetect(ctx context.Context, incidentID, message string)
	AutoSeedOnResolve(ctx context.Context, incidentID, message string)
}

// IncidentService manages incident creation and resolution with dynamic notification dispatch.
// It creates incidents only after 3 consecutive failures and tracks the full lifecycle.
// Notifications are sent via user-configured notification channels (SMTP, Webhook, etc.).
type IncidentService struct {
	incidents            port.IncidentRepository
	eventSteps           port.IncidentEventStepRepository
	notifications        port.NotificationRepository
	notificationChannels port.NotificationChannelRepository
	diagnostics          port.IncidentDiagnosticsRepository
	components           port.ComponentRepository
	updates              IncidentUpdateSeeder
	notificationEmitter  NotificationEmitter
	client               *asynq.Client
}

// NotificationEmitter is the optional in-app notification-feed producer.
// Implemented by *service.NotificationFeedService. Emission is fire-and-forget.
type NotificationEmitter interface {
	Emit(ctx context.Context, n domain.EmittedNotification) error
}

// NewIncidentService creates a new incident service with the given dependencies.
func NewIncidentService(
	incidents port.IncidentRepository,
	eventSteps port.IncidentEventStepRepository,
	notifications port.NotificationRepository,
	notificationChannels port.NotificationChannelRepository,
	diagnostics port.IncidentDiagnosticsRepository,
	client *asynq.Client,
) *IncidentService {
	return &IncidentService{
		incidents:            incidents,
		eventSteps:           eventSteps,
		notifications:        notifications,
		notificationChannels: notificationChannels,
		diagnostics:          diagnostics,
		client:               client,
	}
}

// SetUpdateSeeder wires the optional incident-update auto-seeder. When nil,
// the lifecycle silently skips public status updates.
func (s *IncidentService) SetUpdateSeeder(seeder IncidentUpdateSeeder) {
	s.updates = seeder
}

// SetComponentRepository sets the component repository for notification resolution
func (s *IncidentService) SetComponentRepository(repo port.ComponentRepository) {
	s.components = repo
}

// SetNotificationEmitter wires the optional in-app notification-feed producer.
// When nil, no feed notifications are emitted.
func (s *IncidentService) SetNotificationEmitter(e NotificationEmitter) {
	s.notificationEmitter = e
}

// emitFeedNotification publishes an incident feed notification fire-and-forget:
// it never blocks or fails incident handling; errors are logged only (SC-004).
func (s *IncidentService) emitFeedNotification(incident *domain.Incident, severity, title string) {
	if s.notificationEmitter == nil {
		return
	}
	deepLink := "/incidents/" + incident.ID
	payload, _ := json.Marshal(map[string]string{"incident_id": incident.ID, "resource_id": incident.ResourceID})
	n := domain.EmittedNotification{
		Category:   domain.NotificationCategoryIncident,
		Severity:   severity,
		Title:      title,
		DeepLink:   &deepLink,
		Payload:    payload,
		OccurredAt: time.Now(),
	}
	emitter := s.notificationEmitter
	go func() {
		if err := emitter.Emit(context.Background(), n); err != nil {
			slog.Warn("failed to emit incident feed notification", "incident_id", incident.ID, "error", err)
		}
	}()
}

// CreateIncident creates a new incident when a resource reaches 3 consecutive failures.
// It checks for existing active incidents, creates event steps, and dispatches notifications
// to all configured notification channels associated with the resource.
func (s *IncidentService) CreateIncident(ctx context.Context, r *domain.Resource, result domain.CheckResult) error {
	if r == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	// Check if there's already an active incident for this resource (ResolvedAt is nil)
	incidents, err := s.incidents.FindByResource(ctx, r.ID, 1, 0)
	if err != nil {
		return fmt.Errorf("failed to check for existing incidents: %w", err)
	}

	// Look for unresolved incidents (where ResolvedAt is nil)
	for _, incident := range incidents {
		if incident.ResolvedAt == nil {
			slog.Info("active incident already exists, skipping creation",
				"incident_id", incident.ID, "resource_id", r.ID, "started_at", incident.StartedAt.Format(time.RFC3339))
			return nil // Active incident already exists, avoid duplicates
		}
	}

	slog.Info("creating new incident", "resource_id", r.ID, "failure_count", r.FailureCount)

	// Extract cause from result - this is the structured failure reason
	cause := extractCause(result)

	// Create new incident
	incident := &domain.Incident{
		ResourceID: r.ID,
		Resource:   *r,
		Cause:      cause,
		ResolvedAt: nil, // nil means active
		StartedAt:  time.Now(),
		Details:    []byte(result.ResponseData),
	}

	if _, err := s.incidents.Create(ctx, incident); err != nil {
		return fmt.Errorf("failed to create incident: %w", err)
	}

	slog.Info("incident created", "incident_id", incident.ID, "resource_id", r.ID, "cause", cause)

	// Spec 072: surface in the in-app feed (fire-and-forget, never blocks).
	s.emitFeedNotification(incident, domain.NotificationSeverityError, fmt.Sprintf("%s is down", r.Name))

	// Persist incident diagnostics immediately after creation
	// This captures error details, network timing, and other technical context
	diag := s.buildIncidentDiagnostics(incident.ID, result, r)
	if _, err := s.diagnostics.Create(ctx, diag); err != nil {
		slog.Warn("failed to persist incident diagnostics", "incident_id", incident.ID, "error", err)
		// Don't fail incident creation if diagnostics fail - they're supplementary
	} else {
		slog.Debug("persisted incident diagnostics", "incident_id", incident.ID)
	}

	// Step 1: Create "detected" event step
	detectedStep := &domain.IncidentEventStep{
		IncidentID: incident.ID,
		Step:       domain.IncidentEventStepDetected,
		Message:    stringPtr(fmt.Sprintf("Incident detected: %s", humanizeCause(cause))),
	}

	if _, err := s.eventSteps.Create(ctx, detectedStep); err != nil {
		slog.Warn("failed to create detected event step", "incident_id", incident.ID, "error", err)
	}

	// Seed the first public-facing status update (US7).
	if s.updates != nil {
		s.updates.AutoSeedOnDetect(ctx, incident.ID, "")
	}

	if _, err := s.eventSteps.FindLastByIncidentAndStep(ctx, incident.ID, domain.IncidentEventStepDownAlert); err == nil {
		slog.Info("down alert already exists, skipping duplicate dispatch", "incident_id", incident.ID)
		return nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("failed to verify prior down alert event step: %w", err)
	}

	// Fetch notification channels associated with this resource using the resolution hierarchy
	channels := s.resolveNotificationChannels(ctx, r)
	if len(channels) == 0 {
		slog.Warn("incident created but no notification channels configured", "incident_id", incident.ID, "resource_id", r.ID)
		return nil
	}

	// Dispatch notifications to all configured channels
	for _, channel := range channels {
		notificationEvent := &domain.NotificationEvent{
			IncidentID: incident.ID,
			Type:       domain.NotificationEventTypeDown,
			Status:     domain.NotificationEventStatusPending,
		}
		eventCreated := false
		if err := s.notifications.Create(ctx, notificationEvent); err != nil {
			slog.Warn("failed to create pending notification event", "incident_id", incident.ID, "error", err)
		} else {
			eventCreated = true
		}

		err := s.dispatchNotification(ctx, notifier.NotificationPayload{Incident: incident}, channel)

		// Create event step for notification attempt (regardless of success/failure)
		statusMsg := "sent"
		if err != nil {
			statusMsg = fmt.Sprintf("failed: %v", err)
			slog.Warn("failed to dispatch down notification", "channel_id", channel.ID, "channel_type", channel.Type, "incident_id", incident.ID, "error", err)
		}

		alertStep := &domain.IncidentEventStep{
			IncidentID: incident.ID,
			Step:       domain.IncidentEventStepDownAlert,
			Message:    stringPtr(fmt.Sprintf("Down notification %s via %s (%s): %s", statusMsg, channel.Type, channel.Name, humanizeCause(incident.Cause))),
		}
		if _, err := s.eventSteps.Create(ctx, alertStep); err != nil {
			slog.Warn("failed to create alert event step", "incident_id", incident.ID, "error", err)
		}

		if eventCreated {
			processedAt := time.Now()
			if err != nil {
				if markErr := s.notifications.MarkAsFailed(ctx, notificationEvent.ID, err.Error(), processedAt); markErr != nil {
					slog.Warn("failed to mark notification event as failed", "notification_id", notificationEvent.ID, "error", markErr)
				}
				if markErr := s.notificationChannels.MarkFailure(ctx, channel.ID, processedAt); markErr != nil {
					slog.Warn("failed to bump channel failure counter", "channel_id", channel.ID, "error", markErr)
				}
			} else {
				if markErr := s.notifications.MarkAsSent(ctx, notificationEvent.ID, processedAt); markErr != nil {
					slog.Warn("failed to mark notification event as sent", "notification_id", notificationEvent.ID, "error", markErr)
				}
				if markErr := s.notificationChannels.MarkSent(ctx, channel.ID, processedAt); markErr != nil {
					slog.Warn("failed to bump channel last_sent_at", "channel_id", channel.ID, "error", markErr)
				}
			}
		}
	}

	return nil
}

func (s *IncidentService) NotifyFlapping(ctx context.Context, r *domain.Resource, transitionCount, windowSeconds, maxDurationMinutes int) error {
	if r == nil {
		return fmt.Errorf("resource cannot be nil")
	}
	channels := s.resolveNotificationChannels(ctx, r)
	for _, channel := range channels {
		if err := s.dispatchNotification(ctx, notifier.NotificationPayload{Flapping: &notifier.FlappingNotification{
			Resource:           *r,
			TransitionCount:    transitionCount,
			WindowSeconds:      windowSeconds,
			MaxDurationMinutes: maxDurationMinutes,
			FlapStartedAt:      r.FlapStartedAt,
			TriggeredAt:        time.Now(),
		}}, channel); err != nil {
			slog.Warn("failed to dispatch flapping notification", "channel_id", channel.ID, "channel_type", channel.Type, "resource_id", r.ID, "error", err)
		}
	}
	return nil
}

func (s *IncidentService) NotifyStabilized(ctx context.Context, r *domain.Resource, finalStatus domain.ResourceStatus) error {
	if r == nil {
		return fmt.Errorf("resource cannot be nil")
	}
	channels := s.resolveNotificationChannels(ctx, r)
	for _, channel := range channels {
		if err := s.dispatchNotification(ctx, notifier.NotificationPayload{Flapping: &notifier.FlappingNotification{
			Resource:    *r,
			Stabilized:  true,
			FinalStatus: finalStatus,
			TriggeredAt: time.Now(),
		}}, channel); err != nil {
			slog.Warn("failed to dispatch stabilization notification", "channel_id", channel.ID, "channel_type", channel.Type, "resource_id", r.ID, "error", err)
		}
	}
	return nil
}

func (s *IncidentService) SendReminderIfDue(ctx context.Context, r *domain.Resource) error {
	if r == nil || r.ReminderIntervalMinutes <= 0 {
		return nil
	}
	activeIncident, err := s.findActiveIncident(ctx, r.ID)
	if err != nil || activeIncident == nil || activeIncident.ResolvedAt != nil {
		return err
	}
	lastStep, err := s.eventSteps.FindLastByIncidentAndStep(ctx, activeIncident.ID, domain.IncidentEventStepDownAlert)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("failed to fetch last down alert step: %w", err)
	}
	elapsed := int(time.Since(lastStep.CreatedAt).Minutes())
	if elapsed < r.ReminderIntervalMinutes {
		return nil
	}
	channels := s.resolveNotificationChannels(ctx, r)
	for _, channel := range channels {
		notificationEvent := &domain.NotificationEvent{
			IncidentID: activeIncident.ID,
			Type:       domain.NotificationEventTypeReminder,
			Status:     domain.NotificationEventStatusPending,
		}
		eventCreated := false
		if err := s.notifications.Create(ctx, notificationEvent); err != nil {
			slog.Warn("failed to create reminder notification event", "incident_id", activeIncident.ID, "error", err)
		} else {
			eventCreated = true
		}

		if err := s.dispatchNotification(ctx, notifier.NotificationPayload{Reminder: &notifier.ReminderNotification{
			Resource:       *r,
			Incident:       *activeIncident,
			ElapsedMinutes: elapsed,
			TriggeredAt:    time.Now(),
		}}, channel); err != nil {
			slog.Warn("failed to dispatch reminder notification", "channel_id", channel.ID, "channel_type", channel.Type, "incident_id", activeIncident.ID, "error", err)
			if eventCreated {
				now := time.Now()
				if markErr := s.notifications.MarkAsFailed(ctx, notificationEvent.ID, err.Error(), now); markErr != nil {
					slog.Warn("failed to mark reminder notification event as failed", "notification_id", notificationEvent.ID, "error", markErr)
				}
				if markErr := s.notificationChannels.MarkFailure(ctx, channel.ID, now); markErr != nil {
					slog.Warn("failed to bump channel failure counter", "channel_id", channel.ID, "error", markErr)
				}
			}
		} else if eventCreated {
			now := time.Now()
			if markErr := s.notifications.MarkAsSent(ctx, notificationEvent.ID, now); markErr != nil {
				slog.Warn("failed to mark reminder notification event as sent", "notification_id", notificationEvent.ID, "error", markErr)
			}
			if markErr := s.notificationChannels.MarkSent(ctx, channel.ID, now); markErr != nil {
				slog.Warn("failed to bump channel last_sent_at", "channel_id", channel.ID, "error", markErr)
			}
		}
	}
	reminderMessage := "Reminder notification sent"
	if _, err := s.eventSteps.Create(ctx, &domain.IncidentEventStep{IncidentID: activeIncident.ID, Step: domain.IncidentEventStepReminder, Message: &reminderMessage}); err != nil {
		slog.Warn("failed to create reminder event step", "incident_id", activeIncident.ID, "error", err)
	}
	anchorMessage := "Reminder anchor written"
	if _, err := s.eventSteps.Create(ctx, &domain.IncidentEventStep{IncidentID: activeIncident.ID, Step: domain.IncidentEventStepDownAlert, Message: &anchorMessage}); err != nil {
		slog.Warn("failed to create reminder down-alert anchor", "incident_id", activeIncident.ID, "error", err)
	}
	return nil
}

func (s *IncidentService) findActiveIncident(ctx context.Context, resourceID string) (*domain.Incident, error) {
	incidents, err := s.incidents.FindByResource(ctx, resourceID, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find incidents: %w", err)
	}
	var activeIncident *domain.Incident
	for _, incident := range incidents {
		if incident.ResolvedAt == nil && (activeIncident == nil || incident.StartedAt.After(activeIncident.StartedAt)) {
			activeIncident = incident
		}
	}
	return activeIncident, nil
}

// FindLatestIncidentForResource returns the latest incident (resolved or active) for a resource.
func (s *IncidentService) FindLatestIncidentForResource(ctx context.Context, resourceID string) (*domain.Incident, error) {
	incidents, err := s.incidents.FindByResource(ctx, resourceID, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find incidents: %w", err)
	}
	if len(incidents) == 0 {
		return nil, repository.ErrNotFound
	}
	return incidents[0], nil
}

// ResolveIncident resolves an active incident when a resource recovers.
// It updates the incident, creates event steps, and triggers recovery notifications.
// Notifications are sent via SMTP and webhook (if configured).
func (s *IncidentService) ResolveIncident(ctx context.Context, r *domain.Resource, result domain.CheckResult) error {
	if r == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	// Find the active incident for this resource (ResolvedAt is nil)
	incidents, err := s.incidents.FindByResource(ctx, r.ID, 10, 0)
	if err != nil {
		return fmt.Errorf("failed to find incidents: %w", err)
	}

	// Look for the most recent unresolved incident
	var activeIncident *domain.Incident
	for _, incident := range incidents {
		if incident.ResolvedAt == nil {
			if activeIncident == nil || incident.StartedAt.After(activeIncident.StartedAt) {
				activeIncident = incident
			}
		}
	}

	// No active incident to resolve
	if activeIncident == nil {
		slog.Debug("no active incident found for resource, recovery without prior incident", "resource_id", r.ID)
		return nil
	}

	duration := time.Since(activeIncident.StartedAt)
	slog.Info("resolving incident", "incident_id", activeIncident.ID, "resource_id", r.ID, "duration", duration)

	// Resolve the incident by setting ResolvedAt timestamp
	now := time.Now()
	activeIncident.ResolvedAt = &now

	if err := s.incidents.Update(ctx, activeIncident); err != nil {
		return fmt.Errorf("failed to resolve incident: %w", err)
	}

	// Seed the public "resolved" status update (US7).
	if s.updates != nil {
		s.updates.AutoSeedOnResolve(ctx, activeIncident.ID, "")
	}

	slog.Info("incident resolved", "incident_id", activeIncident.ID, "resource_id", r.ID)

	// Spec 072: surface recovery in the in-app feed (fire-and-forget).
	s.emitFeedNotification(activeIncident, domain.NotificationSeveritySuccess, fmt.Sprintf("%s recovered", r.Name))

	// Step 1: Create "resolved" event step
	resolvedStep := &domain.IncidentEventStep{
		IncidentID: activeIncident.ID,
		Step:       domain.IncidentEventStepResolved,
		Message:    stringPtr("Incident resolved: resource is back up"),
	}

	if _, err := s.eventSteps.Create(ctx, resolvedStep); err != nil {
		slog.Warn("failed to create resolved event step", "incident_id", activeIncident.ID, "error", err)
	}

	// Fetch notification channels associated with this resource using the resolution hierarchy
	channels := s.resolveNotificationChannels(ctx, r)
	if len(channels) == 0 {
		slog.Info("no notification channels resolved for resource", "resource_id", r.ID)
		return nil
	}

	// Dispatch resolution notifications to all configured channels
	for _, channel := range channels {
		notificationEvent := &domain.NotificationEvent{
			IncidentID: activeIncident.ID,
			Type:       domain.NotificationEventTypeUp,
			Status:     domain.NotificationEventStatusPending,
		}
		eventCreated := false
		if err := s.notifications.Create(ctx, notificationEvent); err != nil {
			slog.Warn("failed to create pending notification event", "incident_id", activeIncident.ID, "error", err)
		} else {
			eventCreated = true
		}

		err := s.dispatchNotification(ctx, notifier.NotificationPayload{Incident: activeIncident}, channel)

		// Create event step for notification attempt (regardless of success/failure)
		statusMsg := "sent"
		if err != nil {
			statusMsg = fmt.Sprintf("failed: %v", err)
			slog.Warn("failed to dispatch resolution notification", "channel_id", channel.ID, "channel_type", channel.Type, "incident_id", activeIncident.ID, "error", err)
		}

		upAlertStep := &domain.IncidentEventStep{
			IncidentID: activeIncident.ID,
			Step:       domain.IncidentEventStepUpAlert,
			Message:    stringPtr(fmt.Sprintf("Up notification %s via %s (%s): %s", statusMsg, channel.Type, channel.Name, humanizeCause(activeIncident.Cause))),
		}
		if _, err := s.eventSteps.Create(ctx, upAlertStep); err != nil {
			slog.Warn("failed to create up alert event step", "incident_id", activeIncident.ID, "error", err)
		}

		if eventCreated {
			processedAt := time.Now()
			if err != nil {
				if markErr := s.notifications.MarkAsFailed(ctx, notificationEvent.ID, err.Error(), processedAt); markErr != nil {
					slog.Warn("failed to mark notification event as failed", "notification_id", notificationEvent.ID, "error", markErr)
				}
				if markErr := s.notificationChannels.MarkFailure(ctx, channel.ID, processedAt); markErr != nil {
					slog.Warn("failed to bump channel failure counter", "channel_id", channel.ID, "error", markErr)
				}
			} else {
				if markErr := s.notifications.MarkAsSent(ctx, notificationEvent.ID, processedAt); markErr != nil {
					slog.Warn("failed to mark notification event as sent", "notification_id", notificationEvent.ID, "error", markErr)
				}
				if markErr := s.notificationChannels.MarkSent(ctx, channel.ID, processedAt); markErr != nil {
					slog.Warn("failed to bump channel last_sent_at", "channel_id", channel.ID, "error", markErr)
				}
			}
		}
	}

	return nil
}

// extractCause extracts a structured cause from the monitoring result.
// This provides consistent failure categorization.
func extractCause(result domain.CheckResult) string {
	if result.Cause != nil {
		return string(*result.Cause) // DNSResolutionFailed, ConnectionTimeOut, etc...
	}

	if result.Status == "down" && len(result.ResponseData) > 0 {
		data := strings.ToLower(result.ResponseData)

		if contains(data, "timeout") {
			return "connection_timeout"
		}
		if contains(data, "refused") {
			return "connection_refused"
		}
		if contains(data, "dns") || contains(data, "resolve") || contains(data, "no such host") {
			return "dns_resolution_failure"
		}
		if contains(data, "ssl") || contains(data, "tls") || contains(data, "certificate") {
			return "ssl_certificate_error"
		}
		if contains(data, "status") || contains(data, "code") {
			return "invalid_status_code"
		}

	}

	// Fallback
	// Map status to structured causes
	switch result.Status {
	case "down":
		return "health_check_failed"
	case "error":
		return "check_execution_error"
	default:
		return "unknown_failure"
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) &&
			(findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// dispatchNotification sends notifications via the given notification channel.
// It unmarshals the channel config and instantiates the appropriate notifier.
func (s *IncidentService) dispatchNotification(ctx context.Context, payload notifier.NotificationPayload, channel *domain.NotificationChannel) error {
	var n notifier.Notifier
	var err error

	// Instantiate the appropriate notifier based on channel type
	switch channel.Type {
	case "smtp":
		n, err = notifier.NewSMTPNotifierFromConfig(string(channel.Config))
		if err != nil {
			return fmt.Errorf("failed to build SMTP notifier: %w", err)
		}
	case "webhook":
		n, err = notifier.NewWebhookNotifierFromConfig(string(channel.Config))
		if err != nil {
			return fmt.Errorf("failed to build webhook notifier: %w", err)
		}
	default:
		return fmt.Errorf("unknown notification channel type: %s", channel.Type)
	}

	// Send the notification
	if err := n.Send(ctx, payload); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	slog.Info("notification sent", "channel_type", channel.Type, "channel_id", channel.ID)
	return nil
}

// resolveNotificationChannels implements the notification channel resolution hierarchy:
// 1. Resource-specific channels (highest priority)
// 2. Component-level channels (if resource belongs to a component)
// 3. Global default channels (lowest priority, fallback)
func (s *IncidentService) resolveNotificationChannels(ctx context.Context, r *domain.Resource) []*domain.NotificationChannel {
	var channels []*domain.NotificationChannel

	// Step 1: Try resource-specific channels
	resourceChannels, err := s.notificationChannels.FindByResourceID(ctx, r.ID)
	if err == nil && len(resourceChannels) > 0 {
		slog.Debug("resolved notification channels from resource", "count", len(resourceChannels), "resource_id", r.ID)
		return resourceChannels
	}

	// Step 2: If resource belongs to a component, try component-level channels
	if r.ComponentID != nil && s.components != nil {
		component, err := s.components.FindByID(ctx, *r.ComponentID)
		if err == nil && component != nil {
			componentChannels, err := s.notificationChannels.FindByComponentID(ctx, component.ID)
			if err == nil && len(componentChannels) > 0 {
				slog.Debug("resolved notification channels from component", "count", len(componentChannels), "component_id", component.ID)
				return componentChannels
			}
		}
	}

	// Step 3: Fall back to default/global channels
	defaultChannels, err := s.notificationChannels.FindDefaultChannels(ctx)
	if err == nil && len(defaultChannels) > 0 {
		slog.Debug("resolved global default notification channels", "count", len(defaultChannels))
		return defaultChannels
	}

	slog.Debug("no notification channels found for resource", "resource_id", r.ID)
	return channels
}

// stringPtr is a helper to create string pointers.
func stringPtr(s string) *string {
	return &s
}

// buildIncidentDiagnostics constructs an IncidentDiagnostics record from a CheckResult.
// This captures rich diagnostic information to help users debug issues.
// Note: This mirrors the logic in worker/diagnostics_builder.go but is kept here for convenience.
func (s *IncidentService) buildIncidentDiagnostics(incidentID string, result domain.CheckResult, resource *domain.Resource) *domain.IncidentDiagnostics {
	diag := &domain.IncidentDiagnostics{
		IncidentID:        incidentID,
		RequestMethod:     result.RequestMethod,
		RequestURL:        result.RequestURL,
		RequestHeaders:    sanitizeRequestHeaders(result.RequestHeaders),
		RequestTimeout:    resource.Timeout,
		HTTPStatusCode:    result.HTTPStatusCode,
		ResponseHeaders:   result.ResponseHeaders,
		TotalDuration:     int(result.ResponseTime.Milliseconds()),
		DNSDuration:       int(result.DNSDuration.Milliseconds()),
		TLSDuration:       int(result.TLSDuration.Milliseconds()),
		FirstByteDuration: int(result.FirstByteDuration.Milliseconds()),
	}

	// Set error context if available
	if result.Cause != nil {
		diag.FailureType = string(*result.Cause)
	}

	if result.ErrorMessage != "" {
		diag.ErrorMessage = result.ErrorMessage
	}

	// Store response data for context (response body is captured separately)
	if result.ResponseBody != "" {
		diag.ResponseBody = result.ResponseBody
		diag.ResponseSize = len(result.ResponseBody)
	}

	return diag
}

func humanizeCause(cause string) string {
	known := map[string]string{
		"connection_timeout":                 "Connection timeout",
		"connection_refused":                 "Connection refused",
		"invalid_status_code":                "Invalid HTTP status code",
		"dns_resolution_failure":             "DNS resolution failed",
		"ssl_certificate_error":              "SSL certificate error",
		"health_check_failed":                "Health check failed",
		"check_execution_error":              "Check execution error",
		"unknown_failure":                    "Unknown failure",
		string(domain.ConnectionTimeout):     "Connection timeout",
		string(domain.ConnectionRefused):     "Connection refused",
		string(domain.DNSResolutionFailed):   "DNS resolution failed",
		string(domain.HTTPInvalidStatusCode): "Invalid HTTP status code",
		string(domain.HTTPRequestFailed):     "HTTP request failed",
		string(domain.HTTPSSLError):          "HTTPS handshake error",
		string(domain.InvalidTarget):         "Invalid target",
		string(domain.MissedHeartbeat):       "No ping received within the expected interval + grace period",
	}

	if msg, ok := known[cause]; ok {
		return msg
	}

	if strings.TrimSpace(cause) == "" {
		return "Health check failed"
	}

	return "Health check failed"
}

func sanitizeRequestHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return map[string]string{}
	}

	clean := make(map[string]string, len(headers))
	for k, v := range headers {
		if strings.EqualFold(k, "Authorization") {
			continue
		}
		clean[k] = v
	}

	return clean
}
