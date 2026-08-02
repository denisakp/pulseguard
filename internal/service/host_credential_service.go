package service

import (
	"context"
	"errors"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/pkg/hostcred"
)

// HostCredentialService issues, rotates, revokes, and authenticates per-host
// agent credentials. Mirrors APIKeyService: generate → store hash → find-by-hash
// on auth. The raw credential is returned only at issue/rotate time.
type HostCredentialService struct {
	repo port.HostCredentialRepository
	now  func() time.Time
}

func NewHostCredentialService(repo port.HostCredentialRepository) *HostCredentialService {
	return &HostCredentialService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// SetNow overrides the clock. Tests only.
func (s *HostCredentialService) SetNow(fn func() time.Time) { s.now = fn }

// Issue creates a new active credential for the host and returns the raw token
// (shown once) and its non-secret prefix.
func (s *HostCredentialService) Issue(ctx context.Context, hostID string) (string, string, error) {
	raw, hash, prefix, err := hostcred.Generate()
	if err != nil {
		return "", "", err
	}
	cred := &domain.HostCredential{
		HostID:   hostID,
		Hash:     hash,
		Prefix:   prefix,
		IsActive: true,
	}
	if err := s.repo.Create(ctx, cred); err != nil {
		return "", "", err
	}
	return raw, prefix, nil
}

// Rotate deactivates every active credential for the host, then issues a new one.
func (s *HostCredentialService) Rotate(ctx context.Context, hostID string) (string, string, error) {
	if err := s.repo.DeactivateAllForHost(ctx, hostID); err != nil {
		return "", "", err
	}
	return s.Issue(ctx, hostID)
}

// Revoke deactivates every active credential for the host.
func (s *HostCredentialService) Revoke(ctx context.Context, hostID string) error {
	return s.repo.DeactivateAllForHost(ctx, hostID)
}

// Authenticate resolves a raw credential to its active host credential. On
// success it best-effort touches last_used_at. Unknown / revoked / malformed
// → ErrHostCredentialInvalid.
func (s *HostCredentialService) Authenticate(ctx context.Context, raw string) (*domain.HostCredential, error) {
	if !hostcred.IsFormat(raw) {
		return nil, ErrHostCredentialInvalid
	}
	cred, err := s.repo.FindActiveByHash(ctx, hostcred.Hash(raw))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrHostCredentialInvalid
		}
		return nil, err
	}
	// Best-effort usage stamp; never fail auth on this.
	_ = s.repo.TouchLastUsed(ctx, cred.ID, s.now())
	return cred, nil
}
