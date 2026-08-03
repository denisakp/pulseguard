package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/denisakp/ogoune/internal/config"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
)

// ComponentService manages logical components and their derived status/notifications.
type ComponentService struct {
	components    port.ComponentRepository
	resources     port.ResourceRepository
	channels      port.NotificationChannelRepository
	cfg           *config.Config
	pendingTimers sync.Map // componentID -> *time.Timer
}

// NewComponentService creates a new ComponentService.
func NewComponentService(
	components port.ComponentRepository,
	resources port.ResourceRepository,
	channels port.NotificationChannelRepository,
) *ComponentService {
	return &ComponentService{
		components: components,
		resources:  resources,
		channels:   channels,
	}
}

// NewComponentServiceWithConfig creates a ComponentService with smart alerting configuration.
func NewComponentServiceWithConfig(
	components port.ComponentRepository,
	resources port.ResourceRepository,
	channels port.NotificationChannelRepository,
	cfg *config.Config,
) *ComponentService {
	svc := &ComponentService{
		components: components,
		resources:  resources,
		channels:   channels,
		cfg:        cfg,
	}
	return svc
}

// CreateComponent creates a component with at least one resource and returns its DTO representation.
func (s *ComponentService) CreateComponent(ctx context.Context, payload *dto.CreateComponentPayload) (*dto.ComponentResponse, error) {
	if payload == nil || payload.Name == "" {
		return nil, fmt.Errorf("%w: component name is required", ErrValidationFailed)
	}

	if len(payload.ResourceIDs) == 0 {
		return nil, fmt.Errorf("%w: component must have at least one resource", ErrValidationFailed)
	}

	// Validate all resources exist
	for _, resourceID := range payload.ResourceIDs {
		if _, err := s.resources.FindByID(ctx, resourceID); err != nil {
			if err == repository.ErrNotFound {
				return nil, fmt.Errorf("%w: resource %s not found", ErrValidationFailed, resourceID)
			}
			return nil, err
		}
	}

	component := &domain.Component{
		Name:        payload.Name,
		Description: payload.Description,
	}

	if payload.GroupingWindowSeconds != nil {
		if err := validateGroupingWindow(*payload.GroupingWindowSeconds); err != nil {
			return nil, err
		}
		component.GroupingWindowSeconds = *payload.GroupingWindowSeconds
	}

	created, err := s.components.Create(ctx, component)
	if err != nil {
		return nil, err
	}

	// Assign resources to the new component
	for _, resourceID := range payload.ResourceIDs {
		resource, _ := s.resources.FindByID(ctx, resourceID)
		resource.ComponentID = &created.ID
		if err := s.resources.Update(ctx, resource); err != nil {
			// Rollback: delete the created component if resource assignment fails
			_ = s.components.Delete(ctx, created.ID)
			return nil, fmt.Errorf("failed to assign resource %s: %w", resourceID, err)
		}
	}

	return s.toComponentResponse(ctx, created)
}

// UpdateComponent updates component metadata.
func (s *ComponentService) UpdateComponent(ctx context.Context, id string, payload *dto.UpdateComponentPayload) (*dto.ComponentResponse, error) {
	component, err := s.components.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if payload.Name != nil {
		if *payload.Name == "" {
			return nil, fmt.Errorf("%w: component name is required", ErrValidationFailed)
		}
		component.Name = *payload.Name
	}
	if payload.Description != nil {
		component.Description = payload.Description
	}

	if payload.GroupingWindowSeconds != nil {
		if err := validateGroupingWindow(*payload.GroupingWindowSeconds); err != nil {
			return nil, err
		}
		component.GroupingWindowSeconds = *payload.GroupingWindowSeconds
	}

	if err := s.components.Update(ctx, component); err != nil {
		return nil, err
	}

	return s.toComponentResponse(ctx, component)
}

// DeleteComponent removes a component if no resources are attached.
func (s *ComponentService) DeleteComponent(ctx context.Context, id string) error {
	count, err := s.resources.CountByComponentID(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: component still has %d resources", ErrValidationFailed, count)
	}
	return s.components.Delete(ctx, id)
}

// ListComponents returns components with derived status.
func (s *ComponentService) ListComponents(ctx context.Context, limit, offset int) ([]*dto.ComponentResponse, error) {
	components, err := s.components.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.ComponentResponse, 0, len(components))
	for _, c := range components {
		resp, err := s.toComponentResponse(ctx, c)
		if err != nil {
			return nil, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// GetComponent returns one component.
func (s *ComponentService) GetComponent(ctx context.Context, id string) (*dto.ComponentResponse, error) {
	component, err := s.components.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toComponentResponse(ctx, component)
}

func (s *ComponentService) toComponentResponse(ctx context.Context, component *domain.Component) (*dto.ComponentResponse, error) {
	if component == nil {
		return nil, fmt.Errorf("component cannot be nil")
	}

	resources := component.Resources
	// If not preloaded, fetch
	if resources == nil {
		var err error
		resources, err = s.resources.FindByComponentID(ctx, component.ID)
		if err != nil {
			return nil, err
		}
	}

	status, _ := deriveComponentStatus(resources)

	snaps := make([]dto.ComponentResourceSnapshot, 0, len(resources))
	impacted := make([]dto.ComponentResourceSnapshot, 0)
	for _, r := range resources {
		snap := dto.ComponentResourceSnapshot{ID: r.ID, Name: r.Name, Status: r.Status}
		snaps = append(snaps, snap)
		if r.Status != domain.StatusUp {
			impacted = append(impacted, snap)
		}
	}

	return &dto.ComponentResponse{
		ID:                    component.ID,
		Name:                  component.Name,
		Description:           component.Description,
		Status:                status,
		ImpactedResources:     impacted,
		Resources:             snaps,
		GroupingWindowSeconds: component.GroupingWindowSeconds,
		CreatedAt:             component.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:             component.UpdatedAt.UTC().Format(time.RFC3339),
	}, nil
}


// validateGroupingWindow validates the grouping_window_seconds value.
// 0 means disabled; non-zero must be between 10 and 300 seconds.
func validateGroupingWindow(seconds int) error {
	if seconds != 0 && (seconds < 10 || seconds > 300) {
		return fmt.Errorf("%w: grouping_window_seconds must be 0 (disabled) or between 10 and 300 (got %d)", ErrValidationFailed, seconds)
	}
	return nil
}
