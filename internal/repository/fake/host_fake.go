package fake

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
)

// HostFake is an in-memory HostRepository for tests.
type HostFake struct {
	mu   sync.RWMutex
	byID map[string]*domain.Host
}

func NewHostFake() *HostFake {
	return &HostFake{byID: make(map[string]*domain.Host)}
}

func (r *HostFake) Create(ctx context.Context, h *domain.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	h.EnsureID()
	now := time.Now()
	if h.CreatedAt.IsZero() {
		h.CreatedAt = now
	}
	if h.UpdatedAt.IsZero() {
		h.UpdatedAt = now
	}
	if _, exists := r.byID[h.ID]; exists {
		return ErrDuplicate
	}
	cp := *h
	r.byID[h.ID] = &cp
	return nil
}

func (r *HostFake) FindByID(ctx context.Context, id string) (*domain.Host, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *h
	return &cp, nil
}

func (r *HostFake) List(ctx context.Context, limit, offset int) ([]*domain.Host, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]*domain.Host, 0, len(r.byID))
	for _, h := range r.byID {
		cp := *h
		all = append(all, &cp)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	if offset >= len(all) {
		return []*domain.Host{}, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *HostFake) Count(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.byID)), nil
}

func (r *HostFake) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[id]; !ok {
		return ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

func (r *HostFake) UpdateSnapshot(ctx context.Context, h *domain.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.byID[h.ID]
	if !ok {
		return ErrNotFound
	}
	existing.LastSeenAt = h.LastSeenAt
	existing.LastCPUPct = h.LastCPUPct
	existing.LastMemPct = h.LastMemPct
	existing.LastDiskPct = h.LastDiskPct
	existing.LastNetIn = h.LastNetIn
	existing.LastNetOut = h.LastNetOut
	existing.LastDisks = h.LastDisks
	if h.OS != nil {
		existing.OS = h.OS
	}
	if h.AgentVersion != nil {
		existing.AgentVersion = h.AgentVersion
	}
	existing.UpdatedAt = time.Now()
	return nil
}
