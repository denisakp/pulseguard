package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

type HostCredentialRepositorySQLC struct {
	pgQ     *pgsqlc.Queries
	sqliteQ *sqlitesqlc.Queries
}

func NewHostCredentialRepositorySQLC(rt SqlcRuntime) port.HostCredentialRepository {
	r := &HostCredentialRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *HostCredentialRepositorySQLC) unconfigured() error {
	return fmt.Errorf("host_credential_repository_sqlc: unconfigured runtime")
}

func (r *HostCredentialRepositorySQLC) Create(ctx context.Context, c *domain.HostCredential) error {
	c.EnsureID()
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}
	switch {
	case r.pgQ != nil:
		return r.pgQ.CreateHostCredential(ctx, pgsqlc.CreateHostCredentialParams{
			ID:         c.ID,
			CreatedAt:  pgtype.Timestamptz{Time: c.CreatedAt, Valid: true},
			UpdatedAt:  pgtype.Timestamptz{Time: c.UpdatedAt, Valid: true},
			HostID:     c.HostID,
			Hash:       c.Hash,
			Prefix:     c.Prefix,
			IsActive:   c.IsActive,
			LastUsedAt: pgTimestampFromPtr(c.LastUsedAt),
		})
	case r.sqliteQ != nil:
		return r.sqliteQ.CreateHostCredential(ctx, sqlitesqlc.CreateHostCredentialParams{
			ID:         c.ID,
			CreatedAt:  c.CreatedAt,
			UpdatedAt:  c.UpdatedAt,
			HostID:     c.HostID,
			Hash:       c.Hash,
			Prefix:     c.Prefix,
			IsActive:   boolToInt64(c.IsActive),
			LastUsedAt: nullTimeFromPtr(c.LastUsedAt),
		})
	default:
		return r.unconfigured()
	}
}

func (r *HostCredentialRepositorySQLC) FindActiveByHash(ctx context.Context, hash string) (*domain.HostCredential, error) {
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindActiveHostCredentialByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find active host credential: %w", err)
		}
		return hostCredentialFromPG(row), nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindActiveHostCredentialByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find active host credential: %w", err)
		}
		return hostCredentialFromSQLite(row), nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *HostCredentialRepositorySQLC) ListByHost(ctx context.Context, hostID string) ([]*domain.HostCredential, error) {
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListHostCredentialsByHost(ctx, hostID)
		if err != nil {
			return nil, fmt.Errorf("sqlc: list host credentials: %w", err)
		}
		out := make([]*domain.HostCredential, len(rows))
		for i, row := range rows {
			out[i] = hostCredentialFromPG(row)
		}
		return out, nil
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListHostCredentialsByHost(ctx, hostID)
		if err != nil {
			return nil, fmt.Errorf("sqlc: list host credentials: %w", err)
		}
		out := make([]*domain.HostCredential, len(rows))
		for i, row := range rows {
			out[i] = hostCredentialFromSQLite(row)
		}
		return out, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *HostCredentialRepositorySQLC) DeactivateByID(ctx context.Context, id string) error {
	now := time.Now()
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.DeactivateHostCredentialByID(ctx, pgsqlc.DeactivateHostCredentialByIDParams{
			ID:        id,
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("sqlc: deactivate host credential: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.DeactivateHostCredentialByID(ctx, sqlitesqlc.DeactivateHostCredentialByIDParams{
			ID:        id,
			UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("sqlc: deactivate host credential: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

func (r *HostCredentialRepositorySQLC) DeactivateAllForHost(ctx context.Context, hostID string) error {
	now := time.Now()
	switch {
	case r.pgQ != nil:
		return r.pgQ.DeactivateAllHostCredentialsForHost(ctx, pgsqlc.DeactivateAllHostCredentialsForHostParams{
			HostID:    hostID,
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		})
	case r.sqliteQ != nil:
		return r.sqliteQ.DeactivateAllHostCredentialsForHost(ctx, sqlitesqlc.DeactivateAllHostCredentialsForHostParams{
			HostID:    hostID,
			UpdatedAt: now,
		})
	default:
		return r.unconfigured()
	}
}

func (r *HostCredentialRepositorySQLC) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	switch {
	case r.pgQ != nil:
		return r.pgQ.TouchHostCredentialLastUsed(ctx, pgsqlc.TouchHostCredentialLastUsedParams{
			ID:         id,
			LastUsedAt: pgtype.Timestamptz{Time: at, Valid: true},
		})
	case r.sqliteQ != nil:
		return r.sqliteQ.TouchHostCredentialLastUsed(ctx, sqlitesqlc.TouchHostCredentialLastUsedParams{
			ID:         id,
			LastUsedAt: sql.NullTime{Time: at, Valid: true},
		})
	default:
		return r.unconfigured()
	}
}

func (r *HostCredentialRepositorySQLC) DeleteByHost(ctx context.Context, hostID string) error {
	switch {
	case r.pgQ != nil:
		return r.pgQ.DeleteHostCredentialsByHost(ctx, hostID)
	case r.sqliteQ != nil:
		return r.sqliteQ.DeleteHostCredentialsByHost(ctx, hostID)
	default:
		return r.unconfigured()
	}
}

// ---------- mappers ----------

func hostCredentialFromPG(row pgsqlc.HostCredential) *domain.HostCredential {
	return &domain.HostCredential{
		Base:       domain.Base{ID: row.ID, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time},
		HostID:     row.HostID,
		Hash:       row.Hash,
		Prefix:     row.Prefix,
		IsActive:   row.IsActive,
		LastUsedAt: pgtzPtr(row.LastUsedAt),
	}
}

func hostCredentialFromSQLite(row sqlitesqlc.HostCredential) *domain.HostCredential {
	return &domain.HostCredential{
		Base:       domain.Base{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt},
		HostID:     row.HostID,
		Hash:       row.Hash,
		Prefix:     row.Prefix,
		IsActive:   row.IsActive != 0,
		LastUsedAt: nullTimePtr(row.LastUsedAt),
	}
}
