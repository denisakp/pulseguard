package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

type HostMetricRepositorySQLC struct {
	pgQ     *pgsqlc.Queries
	sqliteQ *sqlitesqlc.Queries
}

func NewHostMetricRepositorySQLC(rt SqlcRuntime) port.HostMetricsRepository {
	r := &HostMetricRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *HostMetricRepositorySQLC) unconfigured() error {
	return fmt.Errorf("host_metric_repository_sqlc: unconfigured runtime")
}

func (r *HostMetricRepositorySQLC) Insert(ctx context.Context, s *domain.HostMetricSample) error {
	s.EnsureID()
	if s.SampledAt.IsZero() {
		s.SampledAt = time.Now()
	}
	disks, err := marshalDisks(s.Disks)
	if err != nil {
		return err
	}
	switch {
	case r.pgQ != nil:
		return r.pgQ.InsertHostMetric(ctx, pgsqlc.InsertHostMetricParams{
			ID:        s.ID,
			HostID:    s.HostID,
			SampledAt: pgtype.Timestamptz{Time: s.SampledAt, Valid: true},
			CpuPct:    s.CPUPct,
			MemPct:    s.MemPct,
			NetIn:     s.NetIn,
			NetOut:    s.NetOut,
			Disks:     disks,
		})
	case r.sqliteQ != nil:
		return r.sqliteQ.InsertHostMetric(ctx, sqlitesqlc.InsertHostMetricParams{
			ID:        s.ID,
			HostID:    s.HostID,
			SampledAt: s.SampledAt,
			CpuPct:    s.CPUPct,
			MemPct:    s.MemPct,
			NetIn:     s.NetIn,
			NetOut:    s.NetOut,
			Disks:     nullStringFromBytes(disks),
		})
	default:
		return r.unconfigured()
	}
}

func (r *HostMetricRepositorySQLC) ListInRange(ctx context.Context, hostID string, from, to time.Time) ([]*domain.HostMetricSample, error) {
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListHostMetricsInRange(ctx, pgsqlc.ListHostMetricsInRangeParams{
			HostID:      hostID,
			SampledAt:   pgtype.Timestamptz{Time: from, Valid: true},
			SampledAt_2: pgtype.Timestamptz{Time: to, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list host metrics: %w", err)
		}
		out := make([]*domain.HostMetricSample, 0, len(rows))
		for _, row := range rows {
			s, err := hostMetricFromPG(row)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListHostMetricsInRange(ctx, sqlitesqlc.ListHostMetricsInRangeParams{
			HostID:      hostID,
			SampledAt:   from,
			SampledAt_2: to,
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list host metrics: %w", err)
		}
		out := make([]*domain.HostMetricSample, 0, len(rows))
		for _, row := range rows {
			s, err := hostMetricFromSQLite(row)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *HostMetricRepositorySQLC) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	switch {
	case r.pgQ != nil:
		return r.pgQ.DeleteHostMetricsOlderThan(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	case r.sqliteQ != nil:
		return r.sqliteQ.DeleteHostMetricsOlderThan(ctx, cutoff)
	default:
		return 0, r.unconfigured()
	}
}

func (r *HostMetricRepositorySQLC) DeleteByHost(ctx context.Context, hostID string) error {
	switch {
	case r.pgQ != nil:
		return r.pgQ.DeleteHostMetricsByHost(ctx, hostID)
	case r.sqliteQ != nil:
		return r.sqliteQ.DeleteHostMetricsByHost(ctx, hostID)
	default:
		return r.unconfigured()
	}
}

func (r *HostMetricRepositorySQLC) Decimate(ctx context.Context, cutoff time.Time) (int64, error) {
	switch {
	case r.pgQ != nil:
		return r.pgQ.DecimateHostMetrics(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	case r.sqliteQ != nil:
		return r.sqliteQ.DecimateHostMetrics(ctx, cutoff)
	default:
		return 0, r.unconfigured()
	}
}

// ---------- mappers ----------

func hostMetricFromPG(row pgsqlc.HostMetric) (*domain.HostMetricSample, error) {
	disks, err := unmarshalDisks(row.Disks)
	if err != nil {
		return nil, err
	}
	return &domain.HostMetricSample{
		Base:      domain.Base{ID: row.ID},
		HostID:    row.HostID,
		SampledAt: row.SampledAt.Time,
		CPUPct:    row.CpuPct,
		MemPct:    row.MemPct,
		NetIn:     row.NetIn,
		NetOut:    row.NetOut,
		Disks:     disks,
	}, nil
}

func hostMetricFromSQLite(row sqlitesqlc.HostMetric) (*domain.HostMetricSample, error) {
	disks, err := unmarshalDisks([]byte(row.Disks.String))
	if err != nil {
		return nil, err
	}
	return &domain.HostMetricSample{
		Base:      domain.Base{ID: row.ID},
		HostID:    row.HostID,
		SampledAt: row.SampledAt,
		CPUPct:    row.CpuPct,
		MemPct:    row.MemPct,
		NetIn:     row.NetIn,
		NetOut:    row.NetOut,
		Disks:     disks,
	}, nil
}
