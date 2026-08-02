package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
)

// HostService orchestrates host CRUD, the monitor↔host link, and host deletion
// (which unlinks monitors and purges credentials + metrics). Credential issue/
// rotate/revoke are delegated to HostCredentialService so the raw token path
// lives in one place.
type HostService struct {
	hosts     port.HostRepository
	creds     *HostCredentialService
	metrics   port.HostMetricsRepository
	resources port.ResourceRepository
	freshness time.Duration
	now       func() time.Time
}

func NewHostService(
	hosts port.HostRepository,
	creds *HostCredentialService,
	metrics port.HostMetricsRepository,
	resources port.ResourceRepository,
	freshness time.Duration,
) *HostService {
	if freshness <= 0 {
		freshness = 45 * time.Second
	}
	return &HostService{
		hosts:     hosts,
		creds:     creds,
		metrics:   metrics,
		resources: resources,
		freshness: freshness,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SetNow overrides the clock. Tests only.
func (s *HostService) SetNow(fn func() time.Time) { s.now = fn }

// Register creates a host and issues its first credential. Returns the host and
// the raw credential (shown once) + its prefix.
func (s *HostService) Register(ctx context.Context, name string) (*domain.Host, string, string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > 200 {
		return nil, "", "", ErrValidationFailed
	}
	host := &domain.Host{Name: trimmed}
	if err := s.hosts.Create(ctx, host); err != nil {
		return nil, "", "", err
	}
	raw, prefix, err := s.creds.Issue(ctx, host.ID)
	if err != nil {
		return nil, "", "", err
	}
	host.Online = false
	return host, raw, prefix, nil
}

// List returns hosts with their derived online state, plus the total count.
func (s *HostService) List(ctx context.Context, limit, offset int) ([]*domain.Host, int64, error) {
	hosts, err := s.hosts.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	now := s.now()
	for _, h := range hosts {
		h.Online = h.IsOnline(now, s.freshness)
	}
	total, err := s.hosts.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return hosts, total, nil
}

// Get returns a host with its derived online state.
func (s *HostService) Get(ctx context.Context, id string) (*domain.Host, error) {
	host, err := s.hosts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}
	host.Online = host.IsOnline(s.now(), s.freshness)
	return host, nil
}

// Delete removes a host: unlinks its monitors (host_id → nil, monitors intact),
// purges its credentials and metrics, then deletes the host row.
func (s *HostService) Delete(ctx context.Context, id string) error {
	if _, err := s.hosts.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrHostNotFound
		}
		return err
	}
	if err := s.resources.ClearResourceHostIDByHost(ctx, id); err != nil {
		return err
	}
	if err := s.creds.repo.DeleteByHost(ctx, id); err != nil {
		return err
	}
	if err := s.metrics.DeleteByHost(ctx, id); err != nil {
		return err
	}
	return s.hosts.Delete(ctx, id)
}

// RotateCredential rotates the host's credential (old deactivated, new issued).
func (s *HostService) RotateCredential(ctx context.Context, hostID string) (string, string, error) {
	if _, err := s.Get(ctx, hostID); err != nil {
		return "", "", err
	}
	return s.creds.Rotate(ctx, hostID)
}

// RevokeCredential deactivates every active credential for the host.
func (s *HostService) RevokeCredential(ctx context.Context, hostID string) error {
	if _, err := s.Get(ctx, hostID); err != nil {
		return err
	}
	return s.creds.Revoke(ctx, hostID)
}

// LinkMonitor links a monitor to a host (both must exist) and returns the
// updated monitor.
func (s *HostService) LinkMonitor(ctx context.Context, monitorID, hostID string) (*domain.Resource, error) {
	if _, err := s.Get(ctx, hostID); err != nil {
		return nil, err
	}
	if _, err := s.resources.FindByID(ctx, monitorID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}
	hid := hostID
	if err := s.resources.SetResourceHostID(ctx, monitorID, &hid); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}
	return s.resources.FindByID(ctx, monitorID)
}

// UnlinkMonitor clears a monitor's host link.
func (s *HostService) UnlinkMonitor(ctx context.Context, monitorID string) error {
	if err := s.resources.SetResourceHostID(ctx, monitorID, nil); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrResourceNotFound
		}
		return err
	}
	return nil
}
