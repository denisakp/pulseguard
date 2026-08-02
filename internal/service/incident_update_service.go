// Package service — incident lifecycle updates.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
)

type IncidentUpdateService struct {
	repo port.IncidentUpdateRepository
}

func NewIncidentUpdateService(repo port.IncidentUpdateRepository) *IncidentUpdateService {
	return &IncidentUpdateService{repo: repo}
}

// ListByIncident returns the timeline for one incident, newest first.
func (s *IncidentUpdateService) ListByIncident(ctx context.Context, incidentID string) ([]*domain.IncidentUpdate, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident_update: missing incident_id")
	}
	return s.repo.ListByIncident(ctx, incidentID)
}

// Create posts a new update on an incident.
func (s *IncidentUpdateService) Create(ctx context.Context, incidentID string, status domain.IncidentUpdateStatus, message, postedBy string) (*domain.IncidentUpdate, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident_update: missing incident_id")
	}
	if !validStatus(status) {
		return nil, fmt.Errorf("incident_update: invalid status %q", status)
	}
	u := &domain.IncidentUpdate{
		IncidentID: incidentID,
		Status:     status,
		Message:    strings.TrimSpace(message),
		PostedBy:   postedBy,
		PostedAt:   time.Now().UTC(),
	}
	return s.repo.Create(ctx, u)
}

// Update edits an existing update.
func (s *IncidentUpdateService) Update(ctx context.Context, id string, status domain.IncidentUpdateStatus, message string) (*domain.IncidentUpdate, error) {
	if !validStatus(status) {
		return nil, fmt.Errorf("incident_update: invalid status %q", status)
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	existing.Status = status
	existing.Message = strings.TrimSpace(message)
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *IncidentUpdateService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// AutoSeedOnDetect creates the first "Investigating" update when an incident
// is detected by the monitoring layer. Idempotent at the call site (caller
// must invoke once per incident); never returns an error to the caller as
// auto-seed failures must not block detection.
func (s *IncidentUpdateService) AutoSeedOnDetect(ctx context.Context, incidentID, message string) {
	if s == nil || s.repo == nil || incidentID == "" {
		return
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "We are currently investigating this issue."
	}
	_, _ = s.repo.Create(ctx, &domain.IncidentUpdate{
		IncidentID: incidentID,
		Status:     domain.IncidentUpdateInvestigating,
		Message:    msg,
		PostedAt:   time.Now().UTC(),
	})
}

// AutoSeedOnResolve appends a "Resolved" update when an incident lifecycle
// closes. Same guarantees as AutoSeedOnDetect.
func (s *IncidentUpdateService) AutoSeedOnResolve(ctx context.Context, incidentID, message string) {
	if s == nil || s.repo == nil || incidentID == "" {
		return
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "This incident has been resolved."
	}
	_, _ = s.repo.Create(ctx, &domain.IncidentUpdate{
		IncidentID: incidentID,
		Status:     domain.IncidentUpdateResolved,
		Message:    msg,
		PostedAt:   time.Now().UTC(),
	})
}

func validStatus(s domain.IncidentUpdateStatus) bool {
	switch s {
	case domain.IncidentUpdateInvestigating,
		domain.IncidentUpdateIdentified,
		domain.IncidentUpdateMonitoring,
		domain.IncidentUpdateResolved:
		return true
	}
	return false
}
