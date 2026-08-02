package store

import (
	"context"
	"fmt"
	"time"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

// HostAlertStateRepositorySQLC is the sqlc-backed store for per-host agent-down
// alert state (spec 083).
type HostAlertStateRepositorySQLC struct {
	pgQ     *pgsqlc.Queries
	sqliteQ *sqlitesqlc.Queries
}

func NewHostAlertStateRepositorySQLC(rt SqlcRuntime) port.HostAlertStateRepository {
	r := &HostAlertStateRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *HostAlertStateRepositorySQLC) unconfigured() error {
	return fmt.Errorf("host_alert_state_repository_sqlc: unconfigured runtime")
}

// Get returns the alert state for a host, or repository.ErrNotFound if none exists yet.
func (r *HostAlertStateRepositorySQLC) Get(ctx context.Context, hostID string) (*domain.HostAlertState, error) {
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.GetHostAlertState(ctx, hostID)
		if err != nil {
			if isNoRows(err) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: get host alert state: %w", err)
		}
		return hostAlertStateFromPG(row), nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.GetHostAlertState(ctx, hostID)
		if err != nil {
			if isNoRows(err) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: get host alert state: %w", err)
		}
		return hostAlertStateFromSQLite(row), nil
	default:
		return nil, r.unconfigured()
	}
}

// Upsert inserts or updates the alert state row for a host.
func (r *HostAlertStateRepositorySQLC) Upsert(ctx context.Context, s *domain.HostAlertState) error {
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}
	switch {
	case r.pgQ != nil:
		return r.pgQ.UpsertHostAlertState(ctx, pgsqlc.UpsertHostAlertStateParams{
			HostID:       s.HostID,
			State:        s.State,
			OfflineSince: pgTimestampFromPtr(s.OfflineSince),
			Alerted:      s.Alerted,
			UpdatedAt:    pgTimestampFromPtr(&s.UpdatedAt),
		})
	case r.sqliteQ != nil:
		alerted := int64(0)
		if s.Alerted {
			alerted = 1
		}
		return r.sqliteQ.UpsertHostAlertState(ctx, sqlitesqlc.UpsertHostAlertStateParams{
			HostID:       s.HostID,
			State:        s.State,
			OfflineSince: nullTimeFromPtr(s.OfflineSince),
			Alerted:      alerted,
			UpdatedAt:    s.UpdatedAt,
		})
	default:
		return r.unconfigured()
	}
}

func hostAlertStateFromPG(row pgsqlc.HostAlertState) *domain.HostAlertState {
	return &domain.HostAlertState{
		HostID:       row.HostID,
		State:        row.State,
		OfflineSince: pgtzPtr(row.OfflineSince),
		Alerted:      row.Alerted,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func hostAlertStateFromSQLite(row sqlitesqlc.HostAlertState) *domain.HostAlertState {
	return &domain.HostAlertState{
		HostID:       row.HostID,
		State:        row.State,
		OfflineSince: nullTimePtr(row.OfflineSince),
		Alerted:      row.Alerted != 0,
		UpdatedAt:    row.UpdatedAt,
	}
}
