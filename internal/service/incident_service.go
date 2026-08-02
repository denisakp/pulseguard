package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
)

// IncidentService provides business logic for incident management operations.
type IncidentService struct {
	incidents  port.IncidentRepository
	eventSteps port.IncidentEventStepRepository
}

// NewIncidentService creates a new IncidentService with the given repository dependencies.
func NewIncidentService(
	incidents port.IncidentRepository,
	eventSteps port.IncidentEventStepRepository,
) *IncidentService {
	return &IncidentService{
		incidents:  incidents,
		eventSteps: eventSteps,
	}
}

// ListAll retrieves all incidents with pagination.
// Limit defaults to 25 if not provided or invalid.
// Offset defaults to 0 if negative.
func (s *IncidentService) ListAll(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	if limit <= 0 {
		limit = 25
	}

	if offset < 0 {
		offset = 0
	}

	incidents, err := s.incidents.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list incidents: %w", err)
	}

	return incidents, nil
}

// ListByFilter passes the dynamic filter through to the repo.
func (s *IncidentService) ListByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error) {
	return s.incidents.ListIncidentsByFilter(ctx, f, page, perPage)
}

// ListUnresolved retrieves all unresolved incidents with pagination.
func (s *IncidentService) ListUnresolved(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	if limit <= 0 {
		limit = 25
	}

	if offset < 0 {
		offset = 0
	}

	incidents, err := s.incidents.FindUnresolved(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list unresolved incidents: %w", err)
	}

	return incidents, nil
}

// GetIncidentByID retrieves a single incident by its ID with all related event steps.
func (s *IncidentService) GetIncidentByID(ctx context.Context, id string) (*domain.Incident, error) {
	incident, err := s.incidents.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("%w: incident not found", ErrResourceNotFound)
		}
		return nil, fmt.Errorf("failed to get incident: %w", err)
	}

	// Fetch all event steps for this incident
	eventSteps, err := s.eventSteps.List(ctx, 1000, 0) // Get all steps (no pagination for now)
	if err != nil {
		// Don't fail the entire request if we can't get event steps
		// Just log and return incident without steps
		return incident, nil
	}

	// Filter event steps for this incident
	var incidentSteps []domain.IncidentEventStep
	for _, step := range eventSteps {
		if step.IncidentID == incident.ID {
			incidentSteps = append(incidentSteps, *step)
		}
	}

	incident.EventStep = incidentSteps

	return incident, nil
}

// GetIncidentsByResource retrieves all incidents for a specific resource with pagination.
func (s *IncidentService) GetIncidentsByResource(ctx context.Context, resourceID string, limit, offset int) ([]*domain.Incident, error) {
	if limit <= 0 {
		limit = 25
	}

	if offset < 0 {
		offset = 0
	}

	incidents, err := s.incidents.FindByResource(ctx, resourceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get incidents for resource: %w", err)
	}

	return incidents, nil
}

// GetEventStepsForIncident retrieves all event steps for a specific incident.
func (s *IncidentService) GetEventStepsForIncident(ctx context.Context, incidentID string) ([]domain.IncidentEventStep, error) {
	// Get all event steps (considering a small limit for now, can be optimized)
	steps, err := s.eventSteps.List(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get event steps: %w", err)
	}

	// Filter steps for this incident
	var result []domain.IncidentEventStep
	for _, step := range steps {
		if step.IncidentID == incidentID {
			result = append(result, *step)
		}
	}

	return result, nil
}

// GetActiveIncident returns the most recent unresolved incident for a resource.
// When no active incident exists, it returns (nil, nil).
func (s *IncidentService) GetActiveIncident(ctx context.Context, resourceID string) (*dto.LiveActiveIncident, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource_id is required")
	}

	incident, err := s.incidents.FindActiveByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active incident for resource %s: %w", resourceID, err)
	}

	return &dto.LiveActiveIncident{
		ID:        incident.ID,
		StartedAt: incident.StartedAt,
		Cause:     incident.Cause,
	}, nil
}
