package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/denisakp/ogoune/internal/service"
)

// stubResourceRepo implements port.ResourceRepository — only FindByID is used.
type stubResourceRepo struct {
	byID map[string]*domain.Resource
}

func (s *stubResourceRepo) Create(context.Context, *domain.Resource) (*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) SetResourceHostID(context.Context, string, *string) error { return nil }
func (s *stubResourceRepo) ClearResourceHostIDByHost(context.Context, string) error  { return nil }
func (s *stubResourceRepo) FindByID(_ context.Context, id string) (*domain.Resource, error) {
	r, ok := s.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return r, nil
}
func (s *stubResourceRepo) FindByHeartbeatSlug(context.Context, string) (*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) List(context.Context, int, int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) Update(context.Context, *domain.Resource) error { return nil }
func (s *stubResourceRepo) Delete(context.Context, string) error           { return nil }
func (s *stubResourceRepo) FindActive(context.Context, int, int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) FindByTag(context.Context, string, int, int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) FindByComponentID(context.Context, string) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) CountByComponentID(context.Context, string) (int64, error) {
	return 0, nil
}
func (s *stubResourceRepo) FindMissedHeartbeats(context.Context, time.Time, int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) UpdateLastPingAt(context.Context, string, time.Time) error { return nil }
func (s *stubResourceRepo) UpdateStatus(context.Context, string, domain.ResourceStatus) error {
	return nil
}
func (s *stubResourceRepo) UpdateMonitoringState(context.Context, string, port.UpdateMonitoringStateRequest) error {
	return nil
}
func (s *stubResourceRepo) UpdateMetadata(context.Context, string, port.UpdateMetadataRequest) error {
	return nil
}
func (s *stubResourceRepo) FindScheduledResources(context.Context) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) ListResourcesByFilter(context.Context, dynquery.MonitorFilter, int, int) ([]*domain.Resource, int, error) {
	return nil, 0, nil
}

func TestEscalation_Create_AssignsNextPriority(t *testing.T) {
	repo := fake.NewEscalationRepository()
	svc := service.NewEscalationService(repo, nil)
	ctx := context.Background()

	p, err := svc.Create(ctx, &domain.EscalationPolicy{
		Name: "Critical",
		Scope: domain.EscalationScope{
			Kind:  domain.EscalationScopeComponent,
			Value: "comp-1",
		},
		IsActive: true,
		Steps: []domain.EscalationStep{
			{DelayMinutes: 5, ChannelIDs: []string{"ch-1"}},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Priority == 0 {
		t.Fatalf("create must assign priority > 0, got 0")
	}
}

func TestEscalation_Create_RejectsTooManySteps(t *testing.T) {
	repo := fake.NewEscalationRepository()
	svc := service.NewEscalationService(repo, nil)
	steps := make([]domain.EscalationStep, 6)
	for i := range steps {
		steps[i] = domain.EscalationStep{DelayMinutes: 5, ChannelIDs: []string{"ch"}}
	}
	_, err := svc.Create(context.Background(), &domain.EscalationPolicy{
		Name:  "Bad",
		Scope: domain.EscalationScope{Kind: domain.EscalationScopeComponent, Value: "c"},
		IsActive: true,
		Steps: steps,
	})
	if err != service.ErrEscalationStepsRange {
		t.Fatalf("expected steps range error, got %v", err)
	}
}

func TestEscalation_Create_RejectsZeroChannels(t *testing.T) {
	repo := fake.NewEscalationRepository()
	svc := service.NewEscalationService(repo, nil)
	_, err := svc.Create(context.Background(), &domain.EscalationPolicy{
		Name:     "Bad",
		Scope:    domain.EscalationScope{Kind: domain.EscalationScopeTag, Value: "t"},
		IsActive: true,
		Steps:    []domain.EscalationStep{{DelayMinutes: 5, ChannelIDs: []string{}}},
	})
	if err != service.ErrEscalationChannelsEmpty {
		t.Fatalf("expected channels empty error, got %v", err)
	}
}

func TestEscalation_Reorder_RejectsMissingID(t *testing.T) {
	repo := fake.NewEscalationRepository()
	svc := service.NewEscalationService(repo, nil)
	ctx := context.Background()

	a, _ := svc.Create(ctx, &domain.EscalationPolicy{
		Name: "A", IsActive: true,
		Scope: domain.EscalationScope{Kind: domain.EscalationScopeComponent, Value: "c"},
		Steps: []domain.EscalationStep{{DelayMinutes: 5, ChannelIDs: []string{"x"}}},
	})
	_, _ = svc.Create(ctx, &domain.EscalationPolicy{
		Name: "B", IsActive: true,
		Scope: domain.EscalationScope{Kind: domain.EscalationScopeComponent, Value: "c"},
		Steps: []domain.EscalationStep{{DelayMinutes: 5, ChannelIDs: []string{"x"}}},
	})

	// missing one active ID
	if err := svc.Reorder(ctx, []string{a.ID}); err != service.ErrEscalationReorderMissing {
		t.Fatalf("expected reorder missing error, got %v", err)
	}
}

func TestEscalation_Reorder_RejectsUnknownID(t *testing.T) {
	repo := fake.NewEscalationRepository()
	svc := service.NewEscalationService(repo, nil)
	ctx := context.Background()

	a, _ := svc.Create(ctx, &domain.EscalationPolicy{
		Name: "A", IsActive: true,
		Scope: domain.EscalationScope{Kind: domain.EscalationScopeComponent, Value: "c"},
		Steps: []domain.EscalationStep{{DelayMinutes: 5, ChannelIDs: []string{"x"}}},
	})

	if err := svc.Reorder(ctx, []string{a.ID, "ghost"}); err != service.ErrEscalationReorderMissing {
		// length mismatch beats unknown check
		t.Fatalf("expected mismatch first, got %v", err)
	}

	if err := svc.Reorder(ctx, []string{"ghost"}); err != service.ErrEscalationReorderUnknown {
		t.Fatalf("expected unknown error, got %v", err)
	}
}

func TestEscalation_MatchForResource_LowerPriorityWins(t *testing.T) {
	repo := fake.NewEscalationRepository()
	res := &domain.Resource{Name: "api", Type: domain.ResourceHTTP}
	c := "comp-1"
	res.ComponentID = &c
	resources := &stubResourceRepo{byID: map[string]*domain.Resource{}}
	resources.byID["r1"] = res
	res.Base.ID = "r1"
	svc := service.NewEscalationService(repo, resources)
	ctx := context.Background()

	high, _ := svc.Create(ctx, &domain.EscalationPolicy{
		Name: "high", IsActive: true,
		Scope: domain.EscalationScope{Kind: domain.EscalationScopeComponent, Value: c},
		Steps: []domain.EscalationStep{{DelayMinutes: 5, ChannelIDs: []string{"ch"}}},
	})
	_, _ = svc.Create(ctx, &domain.EscalationPolicy{
		Name: "low", IsActive: true,
		Scope: domain.EscalationScope{Kind: domain.EscalationScopeComponent, Value: c},
		Steps: []domain.EscalationStep{{DelayMinutes: 5, ChannelIDs: []string{"ch"}}},
	})

	matched, err := svc.MatchForResource(ctx, "r1")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if matched == nil || matched.ID != high.ID {
		t.Fatalf("expected lowest priority (first created) to win, got %+v", matched)
	}
}

func TestEscalation_MatchForResource_NoMatchReturnsNil(t *testing.T) {
	repo := fake.NewEscalationRepository()
	res := &domain.Resource{Name: "api", Type: domain.ResourceHTTP}
	res.Base.ID = "r1"
	resources := &stubResourceRepo{byID: map[string]*domain.Resource{"r1": res}}
	svc := service.NewEscalationService(repo, resources)

	matched, err := svc.MatchForResource(context.Background(), "r1")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if matched != nil {
		t.Fatalf("expected no match, got %+v", matched)
	}
}
