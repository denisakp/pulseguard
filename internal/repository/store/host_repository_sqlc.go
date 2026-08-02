package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

type HostRepositorySQLC struct {
	pgQ     *pgsqlc.Queries
	sqliteQ *sqlitesqlc.Queries
}

func NewHostRepositorySQLC(rt SqlcRuntime) port.HostRepository {
	r := &HostRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *HostRepositorySQLC) unconfigured() error {
	return fmt.Errorf("host_repository_sqlc: unconfigured runtime")
}

func (r *HostRepositorySQLC) Create(ctx context.Context, h *domain.Host) error {
	h.EnsureID()
	now := time.Now()
	if h.CreatedAt.IsZero() {
		h.CreatedAt = now
	}
	if h.UpdatedAt.IsZero() {
		h.UpdatedAt = now
	}
	disks, err := marshalDisks(h.LastDisks)
	if err != nil {
		return err
	}
	switch {
	case r.pgQ != nil:
		return r.pgQ.CreateHost(ctx, pgsqlc.CreateHostParams{
			ID:           h.ID,
			CreatedAt:    pgtype.Timestamptz{Time: h.CreatedAt, Valid: true},
			UpdatedAt:    pgtype.Timestamptz{Time: h.UpdatedAt, Valid: true},
			Name:         h.Name,
			Os:           pgTextFromPtr(h.OS),
			AgentVersion: pgTextFromPtr(h.AgentVersion),
			LastSeenAt:   pgTimestampFromPtr(h.LastSeenAt),
			LastCpuPct:   pgFloat8FromPtr(h.LastCPUPct),
			LastMemPct:   pgFloat8FromPtr(h.LastMemPct),
			LastDiskPct:  pgFloat8FromPtr(h.LastDiskPct),
			LastNetIn:    pgInt8FromPtr(h.LastNetIn),
			LastNetOut:   pgInt8FromPtr(h.LastNetOut),
			LastDisks:    disks,
		})
	case r.sqliteQ != nil:
		return r.sqliteQ.CreateHost(ctx, sqlitesqlc.CreateHostParams{
			ID:           h.ID,
			CreatedAt:    h.CreatedAt,
			UpdatedAt:    h.UpdatedAt,
			Name:         h.Name,
			Os:           nullStringFromPtr(h.OS),
			AgentVersion: nullStringFromPtr(h.AgentVersion),
			LastSeenAt:   nullTimeFromPtr(h.LastSeenAt),
			LastCpuPct:   nullFloatFromPtr(h.LastCPUPct),
			LastMemPct:   nullFloatFromPtr(h.LastMemPct),
			LastDiskPct:  nullFloatFromPtr(h.LastDiskPct),
			LastNetIn:    nullInt64FromPtr(h.LastNetIn),
			LastNetOut:   nullInt64FromPtr(h.LastNetOut),
			LastDisks:    nullStringFromBytes(disks),
		})
	default:
		return r.unconfigured()
	}
}

func (r *HostRepositorySQLC) FindByID(ctx context.Context, id string) (*domain.Host, error) {
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindHostByID(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find host by id: %w", err)
		}
		return hostFromPG(row)
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindHostByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find host by id: %w", err)
		}
		return hostFromSQLite(row)
	default:
		return nil, r.unconfigured()
	}
}

func (r *HostRepositorySQLC) List(ctx context.Context, limit, offset int) ([]*domain.Host, error) {
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListHosts(ctx, pgsqlc.ListHostsParams{Limit: int32(limit), Offset: int32(offset)})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list hosts: %w", err)
		}
		out := make([]*domain.Host, 0, len(rows))
		for _, row := range rows {
			h, err := hostFromPG(row)
			if err != nil {
				return nil, err
			}
			out = append(out, h)
		}
		return out, nil
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListHosts(ctx, sqlitesqlc.ListHostsParams{Limit: int64(limit), Offset: int64(offset)})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list hosts: %w", err)
		}
		out := make([]*domain.Host, 0, len(rows))
		for _, row := range rows {
			h, err := hostFromSQLite(row)
			if err != nil {
				return nil, err
			}
			out = append(out, h)
		}
		return out, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *HostRepositorySQLC) Count(ctx context.Context) (int64, error) {
	switch {
	case r.pgQ != nil:
		return r.pgQ.CountHosts(ctx)
	case r.sqliteQ != nil:
		return r.sqliteQ.CountHosts(ctx)
	default:
		return 0, r.unconfigured()
	}
}

func (r *HostRepositorySQLC) Delete(ctx context.Context, id string) error {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.DeleteHost(ctx, id)
		if err != nil {
			return fmt.Errorf("sqlc: delete host: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.DeleteHost(ctx, id)
		if err != nil {
			return fmt.Errorf("sqlc: delete host: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

func (r *HostRepositorySQLC) UpdateSnapshot(ctx context.Context, h *domain.Host) error {
	now := time.Now()
	h.UpdatedAt = now
	disks, err := marshalDisks(h.LastDisks)
	if err != nil {
		return err
	}
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.UpdateHostSnapshot(ctx, pgsqlc.UpdateHostSnapshotParams{
			ID:           h.ID,
			LastSeenAt:   pgTimestampFromPtr(h.LastSeenAt),
			LastCpuPct:   pgFloat8FromPtr(h.LastCPUPct),
			LastMemPct:   pgFloat8FromPtr(h.LastMemPct),
			LastDiskPct:  pgFloat8FromPtr(h.LastDiskPct),
			LastNetIn:    pgInt8FromPtr(h.LastNetIn),
			LastNetOut:   pgInt8FromPtr(h.LastNetOut),
			LastDisks:    disks,
			Os:           pgTextFromPtr(h.OS),
			AgentVersion: pgTextFromPtr(h.AgentVersion),
			UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("sqlc: update host snapshot: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.UpdateHostSnapshot(ctx, sqlitesqlc.UpdateHostSnapshotParams{
			ID:           h.ID,
			LastSeenAt:   nullTimeFromPtr(h.LastSeenAt),
			LastCpuPct:   nullFloatFromPtr(h.LastCPUPct),
			LastMemPct:   nullFloatFromPtr(h.LastMemPct),
			LastDiskPct:  nullFloatFromPtr(h.LastDiskPct),
			LastNetIn:    nullInt64FromPtr(h.LastNetIn),
			LastNetOut:   nullInt64FromPtr(h.LastNetOut),
			LastDisks:    nullStringFromBytes(disks),
			Os:           nullStringFromPtr(h.OS),
			AgentVersion: nullStringFromPtr(h.AgentVersion),
			UpdatedAt:    now,
		})
		if err != nil {
			return fmt.Errorf("sqlc: update host snapshot: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

// ---------- mappers ----------

func hostFromPG(row pgsqlc.Host) (*domain.Host, error) {
	disks, err := unmarshalDisks(row.LastDisks)
	if err != nil {
		return nil, err
	}
	return &domain.Host{
		Base:         domain.Base{ID: row.ID, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time},
		Name:         row.Name,
		OS:           ptrStringFromPGText(row.Os),
		AgentVersion: ptrStringFromPGText(row.AgentVersion),
		LastSeenAt:   pgtzPtr(row.LastSeenAt),
		LastCPUPct:   ptrFloatFromPGFloat8(row.LastCpuPct),
		LastMemPct:   ptrFloatFromPGFloat8(row.LastMemPct),
		LastDiskPct:  ptrFloatFromPGFloat8(row.LastDiskPct),
		LastNetIn:    ptrInt64FromPGInt8(row.LastNetIn),
		LastNetOut:   ptrInt64FromPGInt8(row.LastNetOut),
		LastDisks:    disks,
	}, nil
}

func hostFromSQLite(row sqlitesqlc.Host) (*domain.Host, error) {
	disks, err := unmarshalDisks([]byte(row.LastDisks.String))
	if err != nil {
		return nil, err
	}
	return &domain.Host{
		Base:         domain.Base{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt},
		Name:         row.Name,
		OS:           ptrStringFromNullString(row.Os),
		AgentVersion: ptrStringFromNullString(row.AgentVersion),
		LastSeenAt:   nullTimePtr(row.LastSeenAt),
		LastCPUPct:   ptrFloatFromNullFloat(row.LastCpuPct),
		LastMemPct:   ptrFloatFromNullFloat(row.LastMemPct),
		LastDiskPct:  ptrFloatFromNullFloat(row.LastDiskPct),
		LastNetIn:    ptrInt64FromNullInt64(row.LastNetIn),
		LastNetOut:   ptrInt64FromNullInt64(row.LastNetOut),
		LastDisks:    disks,
	}, nil
}

// ---------- disks JSON helpers ----------

func marshalDisks(d []domain.DiskUsage) ([]byte, error) {
	if len(d) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("marshal disks: %w", err)
	}
	return b, nil
}

func unmarshalDisks(b []byte) ([]domain.DiskUsage, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var out []domain.DiskUsage
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("unmarshal disks: %w", err)
	}
	return out, nil
}

func nullStringFromBytes(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

// ---------- shared numeric ptr helpers (host domain, spec 079) ----------

func pgFloat8FromPtr(p *float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *p, Valid: true}
}

func nullFloatFromPtr(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}

func pgInt8FromPtr(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}

func nullInt64FromPtr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func ptrFloatFromPGFloat8(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func ptrFloatFromNullFloat(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func ptrInt64FromPGInt8(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

func ptrInt64FromNullInt64(i sql.NullInt64) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}
