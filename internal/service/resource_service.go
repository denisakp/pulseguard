package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/denisakp/ogoune/internal/config"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	icmppkg "github.com/denisakp/ogoune/internal/icmp"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/google/uuid"
)

const (
	defaultConfirmationChecks   = 2
	defaultConfirmationInterval = 30
	errWrapFmt                  = "%w: %v"
)

// ResourceService orchestrates resource-related operations using repository interfaces.
// This service demonstrates the dependency injection pattern and serves as an example
// of how to compose repository operations while maintaining clean boundaries.
type ResourceService struct {
	resources          port.ResourceRepository
	incidents          port.IncidentRepository
	tags               port.TagsRepository
	channels           port.NotificationChannelRepository
	scheduler          port.ResourceScheduler
	monitoringActivity port.MonitoringActivityRepository
	enrichment         *EnrichmentService
	components         *ComponentService
}

// NewResourceService creates a new ResourceService with the given repository dependencies.
func NewResourceService(
	resources port.ResourceRepository,
	incidents port.IncidentRepository,
	tags port.TagsRepository,
	channels port.NotificationChannelRepository,
	scheduler port.ResourceScheduler,
	monitoringActivity port.MonitoringActivityRepository,
	enrichment *EnrichmentService,
	components *ComponentService,
) *ResourceService {
	return &ResourceService{
		resources:          resources,
		incidents:          incidents,
		tags:               tags,
		channels:           channels,
		scheduler:          scheduler,
		monitoringActivity: monitoringActivity,
		enrichment:         enrichment,
		components:         components,
	}
}

// CreateResource creates a new resource using domain validation and persistence.
// After successful creation, it schedules monitoring for the resource and triggers
// asynchronous metadata enrichment so the HTTP request is not blocked by SSL/WHOIS lookups.
func (s *ResourceService) CreateResource(ctx context.Context, payload *dto.CreateResourcePayload) (*domain.Resource, error) {
	// Gate ICMP monitor creation: requires ENABLE_ICMP and runtime capability.
	if payload.Type == domain.ResourceICMP {
		cfg := config.Load()
		if !cfg.EnableICMP {
			return nil, ErrICMPUnavailable
		}
		if cap := icmppkg.Detect(); !cap.Available {
			return nil, ErrICMPUnavailable
		}
	}

	// Validate target format
	if payload.Type != domain.ResourceHeartbeat {
		if err := domain.ValidateResourceTarget(payload.Target, payload.Type); err != nil {
			return nil, fmt.Errorf(errWrapFmt, ErrValidationFailed, err)
		}
	}

	defaultChecks, defaultInterval := confirmationDefaults()
	resolvedChecks, resolvedInterval := domain.ResolveConfirmationDefaults(
		payload.ConfirmationChecks,
		payload.ConfirmationInterval,
		defaultChecks,
		defaultInterval,
	)
	if payload.ConfirmationInterval == nil && payload.Interval > 1 && resolvedInterval >= payload.Interval {
		resolvedInterval = payload.Interval - 1
	}
	if err := domain.ValidateConfirmationSettings(payload.Interval, resolvedChecks, resolvedInterval); err != nil {
		return nil, fmt.Errorf(errWrapFmt, ErrValidationFailed, err)
	}

	resource := &domain.Resource{
		Name:                  payload.Name,
		Type:                  payload.Type,
		Interval:              payload.Interval,
		Timeout:               payload.Timeout,
		Target:                payload.Target,
		IsActive:              true,
		Status:                domain.StatusPending,
		ConfirmationChecks:    resolvedChecks,
		ConfirmationInterval:  resolvedInterval,
		ExpiryAlertThresholds: payload.ExpiryAlertThresholds,
	}

	if payload.Type == domain.ResourceHeartbeat {
		if payload.HeartbeatInterval == nil || payload.HeartbeatGrace == nil {
			return nil, fmt.Errorf("%w: heartbeat_interval and heartbeat_grace are required", ErrValidationFailed)
		}
		if err := domain.ValidateHeartbeatSettings(*payload.HeartbeatInterval, *payload.HeartbeatGrace); err != nil {
			return nil, err
		}
		slug := uuid.NewString()
		resource.HeartbeatSlug = &slug
		resource.HeartbeatInterval = payload.HeartbeatInterval
		resource.HeartbeatGrace = payload.HeartbeatGrace
		resource.Status = domain.StatusUp
		if resource.Target == "" {
			resource.Target = "heartbeat"
		}
	}

	if payload.Type == domain.ResourceKeyword {
		if err := validateKeywordFields(payload.Keyword, payload.KeywordMode); err != nil {
			return nil, err
		}
		resource.Keyword = payload.Keyword
		defaultMode := "contains"
		if payload.KeywordMode != nil {
			resource.KeywordMode = payload.KeywordMode
		} else {
			resource.KeywordMode = &defaultMode
		}
	}

	if payload.Type == domain.ResourceProtocol {
		if err := validateProtocolFields(payload.ProtocolType, payload.ProtocolPort, payload.Target); err != nil {
			return nil, err
		}
		resource.ProtocolType = payload.ProtocolType
		resource.ProtocolPort = payload.ProtocolPort
	}

	// Optional component assignment
	// Apply smart alerting config defaults and per-resource overrides
	cfg := config.Load()
	resource.FlapDetectionEnabled = cfg.FlapDetectionEnabled
	resource.FlapThreshold = cfg.FlapThreshold
	resource.FlapWindowSeconds = cfg.FlapWindowSeconds
	resource.FlapMaxDurationMinutes = cfg.FlapMaxDurationMinutes
	resource.ReminderIntervalMinutes = cfg.ReminderIntervalMinutes
	if payload.FlapDetectionEnabled != nil {
		resource.FlapDetectionEnabled = *payload.FlapDetectionEnabled
	}
	if payload.FlapThreshold != nil {
		resource.FlapThreshold = *payload.FlapThreshold
	}
	if payload.FlapWindowSeconds != nil {
		resource.FlapWindowSeconds = *payload.FlapWindowSeconds
	}
	if payload.FlapMaxDurationMinutes != nil {
		resource.FlapMaxDurationMinutes = *payload.FlapMaxDurationMinutes
	}
	if payload.ReminderIntervalMinutes != nil {
		resource.ReminderIntervalMinutes = *payload.ReminderIntervalMinutes
	}
	if err := validateSmartAlertingFields(resource.FlapThreshold, resource.FlapWindowSeconds, resource.FlapMaxDurationMinutes, resource.ReminderIntervalMinutes); err != nil {
		return nil, err
	}

	// Optional component assignment
	if payload.ComponentID != nil && *payload.ComponentID != "" {
		if s.components == nil {
			return nil, fmt.Errorf("%w: component support is not configured", ErrValidationFailed)
		}
		if _, err := s.components.GetComponent(ctx, *payload.ComponentID); err != nil {
			return nil, fmt.Errorf("%w: invalid component reference", ErrValidationFailed)
		}
		resource.ComponentID = payload.ComponentID
	}

	// Find or create tags by name if provided
	if len(payload.Tags) > 0 {
		tags, err := s.findOrCreateTags(ctx, payload.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed to process tags: %w", err)
		}
		resource.Tags = tags
	}

	// Resolve notification channels by name (lookup-only; never created here).
	// Used by the bulk import path. Missing channel is a validation error.
	if len(payload.NotificationChannelNames) > 0 {
		channels, err := s.resolveChannelsByName(ctx, payload.NotificationChannelNames)
		if err != nil {
			return nil, err
		}
		resource.NotificationChannels = channels
	}

	// Create resource in database
	created, err := s.resources.Create(ctx, resource)
	if err != nil {
		return nil, err
	}

	// Schedule monitoring for the newly created resource
	if err := s.scheduler.Schedule(ctx, created); err != nil {
		// Log the error but don't fail the entire operation
		// The resource was created successfully, monitoring scheduling failed
		return created, fmt.Errorf(errWrapFmt, ErrSchedulerSync, err)
	}

	// Mark metadata as pending for API consumers until enrichment completes
	created.MetadataPending = true

	// Kick off async metadata enrichment (SSL/WHOIS) so the HTTP request returns quickly
	go s.asyncEnrichAndPersist(created)

	// Sync component state if applicable (best-effort)
	if created.ComponentID != nil && s.components != nil {
		_ = s.components.RecalculateAndNotify(ctx, *created.ComponentID)
	}

	return created, nil
}

// GetResourceByID retrieves a resource by its ID.
func (s *ResourceService) GetResourceByID(ctx context.Context, id string) (*domain.Resource, error) {
	resource, err := s.resources.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}

	// load incidents
	// load incidents and convert to the expected value slice type
	incidentPtrs, err := s.incidents.FindByResource(ctx, id, 100, 0) // default limit 100
	if err != nil {
		return nil, fmt.Errorf("failed to load incidents for resource: %w", err)
	}
	incidents := make([]domain.Incident, 0, len(incidentPtrs))
	for _, inc := range incidentPtrs {
		if inc == nil {
			continue
		}
		incidents = append(incidents, *inc)
	}
	resource.Incidents = incidents

	return resource, nil

}

// GetResourceByIDWithResponseTimes retrieves a resource by its ID with recent response times.
func (s *ResourceService) GetResourceByIDWithResponseTimes(ctx context.Context, id string, limit int) (*dto.ResourceResponse, error) {
	// Get the base resource
	resource, err := s.GetResourceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Normalize tags to include only id, name, and color to keep payload focused
	if len(resource.Tags) > 0 {
		trimmed := make([]*domain.Tags, 0, len(resource.Tags))
		for _, t := range resource.Tags {
			if t == nil {
				continue
			}
			trimmed = append(trimmed, &domain.Tags{
				Base:  domain.Base{ID: t.ID},
				Name:  t.Name,
				Color: t.Color,
			})
		}
		resource.Tags = trimmed
	}

	// Get recent response times
	responsePoints, err := s.monitoringActivity.GetRecentResponseTimes(ctx, id, limit)
	if err != nil {
		// Don't fail the entire request if we can't get response times
		// Just log and return resource without response times
		rr := &dto.ResourceResponse{
			Resource:      *resource,
			ResponseTimes: []dto.ResponseTimePoint{},
		}
		dto.EnrichResponseExpiry(rr)
		return rr, nil
	}

	// Map to DTO response times
	responseTimes := make([]dto.ResponseTimePoint, len(responsePoints))
	for i, point := range responsePoints {
		responseTimes[i] = dto.ResponseTimePoint{
			Timestamp:    point.Timestamp,
			ResponseTime: point.ResponseTime,
		}
	}

	rr := &dto.ResourceResponse{
		Resource:      *resource,
		ResponseTimes: responseTimes,
	}
	dto.EnrichResponseExpiry(rr)
	rr.Waiting = resource.IsHeartbeatWaiting()

	return rr, nil
}

// UpdateResource updates an existing resource by ID with the provided payload.
func (s *ResourceService) UpdateResource(ctx context.Context, id string, payload *dto.UpdateResourcePayload) (*domain.Resource, error) {
	resource, err := s.resources.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}
	previousComponentID := resource.ComponentID

	if err := s.applyResourcePayload(ctx, resource, payload); err != nil {
		return nil, err
	}

	if err := s.validateTypeSpecificUpdate(resource, payload); err != nil {
		return nil, err
	}

	resource.MetadataPending = true
	go s.asyncEnrichAndPersist(resource)

	if err := s.resources.Update(ctx, resource); err != nil {
		return nil, err
	}

	if err := s.scheduler.Schedule(ctx, resource); err != nil {
		return nil, fmt.Errorf(errWrapFmt, ErrSchedulerSync, err)
	}

	s.reconcileComponentChange(ctx, resource, previousComponentID)

	return resource, nil
}

// DeleteResource soft deletes a resource and unschedules its monitoring.
func (s *ResourceService) DeleteResource(ctx context.Context, resourceID string) error {
	var componentID *string
	if res, err := s.resources.FindByID(ctx, resourceID); err == nil {
		componentID = res.ComponentID
	}

	// Resolve any active incident before soft-deleting to prevent orphan incidents.
	if incident, err := s.incidents.FindActiveByResourceID(ctx, resourceID); err == nil {
		now := time.Now()
		incident.ResolvedAt = &now
		if err := s.incidents.Update(ctx, incident); err != nil {
			slog.Error("failed to resolve incident for deleted resource", "resource_id", resourceID, "error", err)
		}
	}
	// ErrNotFound from FindActiveByResourceID is expected (no active incident) — proceed silently.

	if err := s.resources.Delete(ctx, resourceID); err != nil {
		return err
	}

	// Unschedule monitoring for the deleted resource
	if err := s.scheduler.Unschedule(ctx, resourceID); err != nil {
		slog.Error("failed to unschedule resource", "resource_id", resourceID, "error", err)
	}

	// Auto-cleanup component if it becomes empty after resource deletion
	if componentID != nil && s.components != nil {
		count, err := s.resources.CountByComponentID(ctx, *componentID)
		if err == nil && count == 0 {
			// Component is now empty - auto-delete it
			_ = s.components.DeleteComponent(ctx, *componentID)
		} else if err == nil {
			// Component still has resources - recalculate status
			_ = s.components.RecalculateAndNotify(ctx, *componentID)
		}
	}

	return nil
}

// ListActiveResources returns all active resources with pagination.
func (s *ResourceService) ListActiveResources(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
	return s.resources.FindActive(ctx, limit, offset)
}

// ListResourcesByTag returns resources filtered by a specific tag.
func (s *ResourceService) ListResourcesByTag(ctx context.Context, tagName string, limit, offset int) ([]*domain.Resource, error) {
	return s.resources.FindByTag(ctx, tagName, limit, offset)
}

// ListUnresolvedIncidents returns unresolved incidents for a specific resource.
func (s *ResourceService) ListUnresolvedIncidents(ctx context.Context, resourceID string) ([]*domain.Incident, error) {
	// First verify resource exists
	_, err := s.resources.FindByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	// Get unresolved incidents for this resource
	return s.incidents.FindByResource(ctx, resourceID, 50, 0) // Default limit of 50
}

// ListAll retrieves all monitoring resources from the repository.
// This method supports listing all resources without pagination for simple use cases.
func (s *ResourceService) ListAll(ctx context.Context) ([]*domain.Resource, error) {
	// Use a large limit to get all resources (can be optimized with proper pagination later)
	return s.resources.List(ctx, 1000, 0)
}

// ListByFilter passes the dynamic filter through to the repo.
func (s *ResourceService) ListByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error) {
	return s.resources.ListResourcesByFilter(ctx, f, page, perPage)
}
