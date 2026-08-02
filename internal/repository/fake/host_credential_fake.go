package fake

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
)

// HostCredentialFake is an in-memory HostCredentialRepository for tests.
type HostCredentialFake struct {
	mu   sync.RWMutex
	byID map[string]*domain.HostCredential
}

func NewHostCredentialFake() *HostCredentialFake {
	return &HostCredentialFake{byID: make(map[string]*domain.HostCredential)}
}

func (r *HostCredentialFake) Create(ctx context.Context, c *domain.HostCredential) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c.EnsureID()
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}
	if _, exists := r.byID[c.ID]; exists {
		return ErrDuplicate
	}
	cp := *c
	r.byID[c.ID] = &cp
	return nil
}

func (r *HostCredentialFake) FindActiveByHash(ctx context.Context, hash string) (*domain.HostCredential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, c := range r.byID {
		if c.Hash == hash && c.IsActive {
			cp := *c
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (r *HostCredentialFake) ListByHost(ctx context.Context, hostID string) ([]*domain.HostCredential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*domain.HostCredential, 0)
	for _, c := range r.byID {
		if c.HostID == hostID {
			cp := *c
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *HostCredentialFake) DeactivateByID(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.byID[id]
	if !ok {
		return ErrNotFound
	}
	c.IsActive = false
	c.UpdatedAt = time.Now()
	return nil
}

func (r *HostCredentialFake) DeactivateAllForHost(ctx context.Context, hostID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.byID {
		if c.HostID == hostID && c.IsActive {
			c.IsActive = false
			c.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (r *HostCredentialFake) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.byID[id]
	if !ok {
		return ErrNotFound
	}
	c.LastUsedAt = &at
	return nil
}

func (r *HostCredentialFake) DeleteByHost(ctx context.Context, hostID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, c := range r.byID {
		if c.HostID == hostID {
			delete(r.byID, id)
		}
	}
	return nil
}
