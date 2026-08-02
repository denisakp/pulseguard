package fake

import (
	"context"
	"sort"
	"sync"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
)

// IncidentFake provides an in-memory implementation of IncidentRepository for testing.
type IncidentFake struct {
	mu        sync.RWMutex
	incidents map[string]*domain.Incident
}

// NewIncidentFake creates a new in-memory IncidentRepository fake.
func NewIncidentFake() *IncidentFake {
	return &IncidentFake{
		incidents: make(map[string]*domain.Incident),
	}
}

func (r *IncidentFake) Create(ctx context.Context, incident *domain.Incident) (*domain.Incident, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Assign a ULID if not set (pure, no *gorm.DB dependency).
	incident.EnsureID()

	if _, exists := r.incidents[incident.ID]; exists {
		return nil, ErrDuplicate
	}

	// Store a copy to avoid external mutations
	copy := *incident
	r.incidents[incident.ID] = &copy

	return &copy, nil
}

func (r *IncidentFake) FindByID(ctx context.Context, id string) (*domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	incident, exists := r.incidents[id]
	if !exists {
		return nil, ErrNotFound
	}

	// Return a copy to avoid external mutations
	copy := *incident
	return &copy, nil
}

func (r *IncidentFake) List(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Convert to slice and sort by created_at DESC
	var incidents []*domain.Incident
	for _, inc := range r.incidents {
		copy := *inc
		incidents = append(incidents, &copy)
	}

	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].CreatedAt.After(incidents[j].CreatedAt)
	})

	// Apply pagination
	if offset >= len(incidents) {
		return []*domain.Incident{}, nil
	}

	end := offset + limit
	if end > len(incidents) {
		end = len(incidents)
	}

	return incidents[offset:end], nil
}

func (r *IncidentFake) Update(ctx context.Context, incident *domain.Incident) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.incidents[incident.ID]; !exists {
		return ErrNotFound
	}

	// Store a copy
	copy := *incident
	r.incidents[incident.ID] = &copy

	return nil
}

func (r *IncidentFake) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.incidents[id]; !exists {
		return ErrNotFound
	}

	delete(r.incidents, id)
	return nil
}

func (r *IncidentFake) FindUnresolved(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Filter unresolved incidents (where ResolvedAt is nil)
	var unresolved []*domain.Incident
	for _, inc := range r.incidents {
		if inc.ResolvedAt == nil {
			copy := *inc
			unresolved = append(unresolved, &copy)
		}
	}

	// Sort by started_at DESC
	sort.Slice(unresolved, func(i, j int) bool {
		return unresolved[i].StartedAt.After(unresolved[j].StartedAt)
	})

	// Apply pagination
	if offset >= len(unresolved) {
		return []*domain.Incident{}, nil
	}

	end := offset + limit
	if end > len(unresolved) {
		end = len(unresolved)
	}

	return unresolved[offset:end], nil
}

func (r *IncidentFake) FindByResource(ctx context.Context, resourceID string, limit, offset int) ([]*domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Filter incidents by resource ID
	var forResource []*domain.Incident
	for _, inc := range r.incidents {
		if inc.ResourceID == resourceID {
			copy := *inc
			forResource = append(forResource, &copy)
		}
	}

	// Sort by started_at DESC
	sort.Slice(forResource, func(i, j int) bool {
		return forResource[i].StartedAt.After(forResource[j].StartedAt)
	})

	// Apply pagination
	if offset >= len(forResource) {
		return []*domain.Incident{}, nil
	}

	end := offset + limit
	if end > len(forResource) {
		end = len(forResource)
	}

	return forResource[offset:end], nil
}

func (r *IncidentFake) GetIncidentStats(ctx context.Context, hours int) (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// This is a simplified implementation for testing
	// In a real scenario, you would filter by time range
	totalIncidents := len(r.incidents)

	// Count unique resource IDs
	resourceMap := make(map[string]bool)
	for _, inc := range r.incidents {
		resourceMap[inc.ResourceID] = true
	}
	affectedMonitors := len(resourceMap)

	return totalIncidents, affectedMonitors, nil
}

func (r *IncidentFake) HasActiveIncident(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, inc := range r.incidents {
		if inc.ResolvedAt == nil {
			return true, nil
		}
	}
	return false, nil
}

func (r *IncidentFake) FindLastResolved(ctx context.Context) (*domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest *domain.Incident
	for _, inc := range r.incidents {
		if inc.ResolvedAt == nil {
			continue
		}
		if latest == nil || inc.ResolvedAt.After(*latest.ResolvedAt) {
			copy := *inc
			latest = &copy
		}
	}

	if latest == nil {
		return nil, ErrNotFound
	}
	return latest, nil
}

func (r *IncidentFake) CountByResourceID(ctx context.Context, resourceID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, inc := range r.incidents {
		if inc.ResourceID == resourceID {
			count++
		}
	}
	return count, nil
}

func (r *IncidentFake) FindActiveByResourceID(ctx context.Context, resourceID string) (*domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest *domain.Incident
	for _, inc := range r.incidents {
		if inc.ResourceID != resourceID || inc.ResolvedAt != nil {
			continue
		}
		if latest == nil || inc.StartedAt.After(latest.StartedAt) {
			copy := *inc
			latest = &copy
		}
	}

	if latest == nil {
		return nil, ErrNotFound
	}
	return latest, nil
}

// ListIncidentsByFilter applies the dynamic filter in memory.
func (r *IncidentFake) ListIncidentsByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]*domain.Incident, 0)
	for _, inc := range r.incidents {
		if f.Status != nil {
			isOpen := inc.ResolvedAt == nil
			switch *f.Status {
			case "open":
				if !isOpen {
					continue
				}
			case "resolved":
				if isOpen {
					continue
				}
			}
		}
		if f.MonitorID != nil && inc.ResourceID != *f.MonitorID {
			continue
		}
		if f.From != nil && inc.StartedAt.Before(*f.From) {
			continue
		}
		if f.To != nil && inc.StartedAt.After(*f.To) {
			continue
		}
		copy := *inc
		matched = append(matched, &copy)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	total := len(matched)
	offset := (page - 1) * perPage
	if offset >= total {
		return []*domain.Incident{}, total, nil
	}
	end := offset + perPage
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}
