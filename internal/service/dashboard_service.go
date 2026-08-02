package service

import (
	"context"
	"errors"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
)

// Dashboard service errors. Handlers map these to HTTP status codes.
var (
	ErrDashboardNotFound   = errors.New("dashboard not found")
	ErrDashboardForbidden  = errors.New("dashboard: not the owner")
	ErrDashboardValidation = errors.New("dashboard: invalid")
)

// validWidgetTypes / scope modes / enums for validation (FR-014).
var (
	validWidgetTypes = map[string]bool{
		domain.WidgetTypeUptimeStat: true, domain.WidgetTypeIncidentsList: true,
		domain.WidgetTypeResponseTime: true, domain.WidgetTypeResourceStatusGrid: true,
	}
	validScopeModes = map[string]bool{
		domain.DashboardScopeModeTag: true, domain.DashboardScopeModeComponent: true,
		domain.DashboardScopeModeType: true, domain.DashboardScopeModeManual: true,
	}
	validTimeRanges = map[string]bool{"24h": true, "7d": true, "30d": true, "90d": true}
	validRefresh    = map[string]bool{"off": true, "30s": true, "1m": true, "5m": true}
	validVisibility = map[string]bool{"private": true, "team": true, "public": true}
)

// DashboardService manages custom dashboards. Reads are instance-wide;
// mutations are owner-only.
type DashboardService struct {
	repo port.DashboardRepository
}

func NewDashboardService(repo port.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// DashboardUpdate is a partial patch — nil fields are left unchanged.
type DashboardUpdate struct {
	Name             *string
	Scope            *domain.DashboardScope
	Widgets          []domain.WidgetInstance // non-nil = replace
	WidgetsSet       bool
	DefaultTimeRange *string
	RefreshInterval  *string
	Visibility       *string
}

// List returns all dashboards, newest-updated first (instance-wide read).
func (s *DashboardService) List(ctx context.Context, limit, offset int) ([]*domain.Dashboard, error) {
	return s.repo.List(ctx, limit, offset)
}

// Get returns one dashboard or ErrDashboardNotFound.
func (s *DashboardService) Get(ctx context.Context, id string) (*domain.Dashboard, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDashboardNotFound
		}
		return nil, err
	}
	return d, nil
}

// Create persists a new dashboard owned by userID.
func (s *DashboardService) Create(ctx context.Context, userID string, d *domain.Dashboard) (*domain.Dashboard, error) {
	d.OwnerID = userID
	if err := validateDashboard(d); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, d)
}

// Update applies a partial patch; only the owner may mutate.
func (s *DashboardService) Update(ctx context.Context, userID, id string, patch DashboardUpdate) (*domain.Dashboard, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != userID {
		return nil, ErrDashboardForbidden
	}
	if patch.Name != nil {
		existing.Name = *patch.Name
	}
	if patch.Scope != nil {
		existing.Scope = *patch.Scope
	}
	if patch.WidgetsSet {
		existing.Widgets = patch.Widgets
	}
	if patch.DefaultTimeRange != nil {
		existing.DefaultTimeRange = *patch.DefaultTimeRange
	}
	if patch.RefreshInterval != nil {
		existing.RefreshInterval = *patch.RefreshInterval
	}
	if patch.Visibility != nil {
		existing.Visibility = *patch.Visibility
	}
	if err := validateDashboard(existing); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDashboardNotFound
		}
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// SaveLayout replaces only the widget list (owner-only).
func (s *DashboardService) SaveLayout(ctx context.Context, userID, id string, widgets []domain.WidgetInstance) (*domain.Dashboard, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != userID {
		return nil, ErrDashboardForbidden
	}
	if err := validateWidgets(widgets); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateWidgets(ctx, id, widgets, time.Now()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDashboardNotFound
		}
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Delete removes a dashboard (owner-only).
func (s *DashboardService) Delete(ctx context.Context, userID, id string) error {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if existing.OwnerID != userID {
		return ErrDashboardForbidden
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrDashboardNotFound
		}
		return err
	}
	return nil
}

func validateDashboard(d *domain.Dashboard) error {
	if d.Name == "" {
		return errWrap("name is required")
	}
	if !validScopeModes[d.Scope.Mode] {
		return errWrap("invalid scope mode")
	}
	if d.DefaultTimeRange != "" && !validTimeRanges[d.DefaultTimeRange] {
		return errWrap("invalid time range")
	}
	if d.RefreshInterval != "" && !validRefresh[d.RefreshInterval] {
		return errWrap("invalid refresh interval")
	}
	if d.Visibility != "" && !validVisibility[d.Visibility] {
		return errWrap("invalid visibility")
	}
	return validateWidgets(d.Widgets)
}

func validateWidgets(widgets []domain.WidgetInstance) error {
	for _, w := range widgets {
		if !validWidgetTypes[w.WidgetTypeID] {
			return errWrap("invalid widget type: " + w.WidgetTypeID)
		}
		if w.Position < 0 {
			return errWrap("widget position must be >= 0")
		}
	}
	return nil
}

func errWrap(msg string) error {
	return errors.Join(ErrDashboardValidation, errors.New(msg))
}
