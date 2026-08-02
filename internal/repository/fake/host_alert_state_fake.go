package fake

import (
	"context"
	"sync"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
)

// HostAlertStateFake is an in-memory per-host agent-down alert state store for
// tests (spec 083).
type HostAlertStateFake struct {
	mu    sync.Mutex
	items map[string]domain.HostAlertState // keyed by host_id
}

func NewHostAlertStateFake() *HostAlertStateFake {
	return &HostAlertStateFake{items: map[string]domain.HostAlertState{}}
}

func (f *HostAlertStateFake) Get(_ context.Context, hostID string) (*domain.HostAlertState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.items[hostID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := s
	return &cp, nil
}

func (f *HostAlertStateFake) Upsert(_ context.Context, s *domain.HostAlertState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[s.HostID] = *s
	return nil
}

var _ port.HostAlertStateRepository = (*HostAlertStateFake)(nil)
