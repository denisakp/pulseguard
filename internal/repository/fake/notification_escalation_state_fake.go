package fake

import (
	"context"
	"sync"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
)

// NotificationEscalationStateFake is an in-memory single-row escalation-state
// store for tests (spec 083 US2).
type NotificationEscalationStateFake struct {
	mu    sync.Mutex
	state *domain.NotificationEscalationState
}

func NewNotificationEscalationStateFake() *NotificationEscalationStateFake {
	return &NotificationEscalationStateFake{}
}

func (f *NotificationEscalationStateFake) Get(_ context.Context) (*domain.NotificationEscalationState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state == nil {
		return nil, repository.ErrNotFound
	}
	cp := *f.state
	return &cp, nil
}

func (f *NotificationEscalationStateFake) Upsert(_ context.Context, s *domain.NotificationEscalationState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s.ID == "" {
		s.ID = domain.NotificationEscalationStateID
	}
	cp := *s
	f.state = &cp
	return nil
}

var _ port.NotificationEscalationStateRepository = (*NotificationEscalationStateFake)(nil)
