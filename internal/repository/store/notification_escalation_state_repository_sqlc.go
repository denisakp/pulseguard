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

// NotificationEscalationStateRepositorySQLC persists the single-row unread-digest
// high-water mark (spec 083 US2).
type NotificationEscalationStateRepositorySQLC struct {
	pgQ     *pgsqlc.Queries
	sqliteQ *sqlitesqlc.Queries
}

func NewNotificationEscalationStateRepositorySQLC(rt SqlcRuntime) port.NotificationEscalationStateRepository {
	r := &NotificationEscalationStateRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *NotificationEscalationStateRepositorySQLC) unconfigured() error {
	return fmt.Errorf("notification_escalation_state_repository_sqlc: unconfigured runtime")
}

// Get returns the escalation state, or repository.ErrNotFound if unset.
func (r *NotificationEscalationStateRepositorySQLC) Get(ctx context.Context) (*domain.NotificationEscalationState, error) {
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.GetNotificationEscalationState(ctx, domain.NotificationEscalationStateID)
		if err != nil {
			if isNoRows(err) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: get escalation state: %w", err)
		}
		return &domain.NotificationEscalationState{
			ID:                  row.ID,
			LastDigestAt:        pgtzPtr(row.LastDigestAt),
			WatermarkOccurredAt: pgtzPtr(row.WatermarkOccurredAt),
			UpdatedAt:           row.UpdatedAt.Time,
		}, nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.GetNotificationEscalationState(ctx, domain.NotificationEscalationStateID)
		if err != nil {
			if isNoRows(err) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: get escalation state: %w", err)
		}
		return &domain.NotificationEscalationState{
			ID:                  row.ID,
			LastDigestAt:        nullTimePtr(row.LastDigestAt),
			WatermarkOccurredAt: nullTimePtr(row.WatermarkOccurredAt),
			UpdatedAt:           row.UpdatedAt,
		}, nil
	default:
		return nil, r.unconfigured()
	}
}

// Upsert inserts or updates the single escalation-state row.
func (r *NotificationEscalationStateRepositorySQLC) Upsert(ctx context.Context, s *domain.NotificationEscalationState) error {
	if s.ID == "" {
		s.ID = domain.NotificationEscalationStateID
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}
	switch {
	case r.pgQ != nil:
		return r.pgQ.UpsertNotificationEscalationState(ctx, pgsqlc.UpsertNotificationEscalationStateParams{
			ID:                  s.ID,
			LastDigestAt:        pgTimestampFromPtr(s.LastDigestAt),
			WatermarkOccurredAt: pgTimestampFromPtr(s.WatermarkOccurredAt),
			UpdatedAt:           pgTimestampFromPtr(&s.UpdatedAt),
		})
	case r.sqliteQ != nil:
		return r.sqliteQ.UpsertNotificationEscalationState(ctx, sqlitesqlc.UpsertNotificationEscalationStateParams{
			ID:                  s.ID,
			LastDigestAt:        nullTimeFromPtr(s.LastDigestAt),
			WatermarkOccurredAt: nullTimeFromPtr(s.WatermarkOccurredAt),
			UpdatedAt:           s.UpdatedAt,
		})
	default:
		return r.unconfigured()
	}
}
