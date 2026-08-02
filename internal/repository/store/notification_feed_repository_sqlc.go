package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

type NotificationFeedRepositorySQLC struct {
	pgQ     *pgsqlc.Queries
	sqliteQ *sqlitesqlc.Queries
}

func NewNotificationFeedRepositorySQLC(rt SqlcRuntime) port.NotificationFeedRepository {
	r := &NotificationFeedRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *NotificationFeedRepositorySQLC) unconfigured() error {
	return fmt.Errorf("notification_feed_repository_sqlc: unconfigured runtime")
}

func (r *NotificationFeedRepositorySQLC) Create(ctx context.Context, n *domain.FeedNotification) (*domain.FeedNotification, error) {
	n.EnsureID()
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	if n.UpdatedAt.IsZero() {
		n.UpdatedAt = now
	}
	if n.OccurredAt.IsZero() {
		n.OccurredAt = now
	}
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.CreateNotification(ctx, pgsqlc.CreateNotificationParams{
			ID:          n.ID,
			UserID:      pgTextFromPtr(n.UserID),
			Category:    n.Category,
			Severity:    n.Severity,
			Title:       n.Title,
			Description: pgTextFromPtr(n.Description),
			DeepLink:    pgTextFromPtr(n.DeepLink),
			Payload:     n.Payload,
			OccurredAt:  pgtype.Timestamptz{Time: n.OccurredAt, Valid: true},
			ReadAt:      pgTimestampFromPtr(n.ReadAt),
			CreatedAt:   pgtype.Timestamptz{Time: n.CreatedAt, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: n.UpdatedAt, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: create notification: %w", err)
		}
		return feedNotifFromPG(row), nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.CreateNotification(ctx, sqlitesqlc.CreateNotificationParams{
			ID:          n.ID,
			UserID:      nullStringFromPtr(n.UserID),
			Category:    n.Category,
			Severity:    n.Severity,
			Title:       n.Title,
			Description: nullStringFromPtr(n.Description),
			DeepLink:    nullStringFromPtr(n.DeepLink),
			Payload:     payloadToNullString(n.Payload),
			OccurredAt:  n.OccurredAt,
			ReadAt:      nullTimeFromPtr(n.ReadAt),
			CreatedAt:   n.CreatedAt,
			UpdatedAt:   n.UpdatedAt,
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: create notification: %w", err)
		}
		return feedNotifFromSQLite(row), nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *NotificationFeedRepositorySQLC) ListForUser(ctx context.Context, userID string, category *string, limit, offset int) ([]*domain.FeedNotification, error) {
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListNotificationsForUser(ctx, pgsqlc.ListNotificationsForUserParams{
			UserID:   pgtype.Text{String: userID, Valid: true},
			Category: pgTextFromPtr(category),
			Lim:      int32(limit),
			Off:      int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list notifications: %w", err)
		}
		out := make([]*domain.FeedNotification, len(rows))
		for i, row := range rows {
			out[i] = feedNotifFromPG(row)
		}
		return out, nil
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListNotificationsForUser(ctx, sqlitesqlc.ListNotificationsForUserParams{
			UserID:   nullStringFromPtr(&userID),
			Category: sqliteNarg(category),
			Lim:      int64(limit),
			Off:      int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list notifications: %w", err)
		}
		out := make([]*domain.FeedNotification, len(rows))
		for i, row := range rows {
			out[i] = feedNotifFromSQLite(row)
		}
		return out, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *NotificationFeedRepositorySQLC) CountForUser(ctx context.Context, userID string, category *string) (int64, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.CountNotificationsForUser(ctx, pgsqlc.CountNotificationsForUserParams{
			UserID:   pgtype.Text{String: userID, Valid: true},
			Category: pgTextFromPtr(category),
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: count notifications: %w", err)
		}
		return n, nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.CountNotificationsForUser(ctx, sqlitesqlc.CountNotificationsForUserParams{
			UserID:   nullStringFromPtr(&userID),
			Category: sqliteNarg(category),
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: count notifications: %w", err)
		}
		return n, nil
	default:
		return 0, r.unconfigured()
	}
}

func (r *NotificationFeedRepositorySQLC) MarkRead(ctx context.Context, id string, at time.Time) (int64, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.MarkNotificationRead(ctx, pgsqlc.MarkNotificationReadParams{
			ID:        id,
			ReadAt:    pgtype.Timestamptz{Time: at, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: at, Valid: true},
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: mark notification read: %w", err)
		}
		return n, nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.MarkNotificationRead(ctx, sqlitesqlc.MarkNotificationReadParams{
			ID:        id,
			ReadAt:    nullTimeFromPtr(&at),
			UpdatedAt: at,
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: mark notification read: %w", err)
		}
		return n, nil
	default:
		return 0, r.unconfigured()
	}
}

func (r *NotificationFeedRepositorySQLC) MarkAllRead(ctx context.Context, userID string, before, at time.Time) (int64, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.MarkAllNotificationsReadForUser(ctx, pgsqlc.MarkAllNotificationsReadForUserParams{
			ReadAt:    pgtype.Timestamptz{Time: at, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: at, Valid: true},
			BeforeTs:  pgtype.Timestamptz{Time: before, Valid: true},
			UserID:    pgtype.Text{String: userID, Valid: true},
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: mark all read: %w", err)
		}
		return n, nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.MarkAllNotificationsReadForUser(ctx, sqlitesqlc.MarkAllNotificationsReadForUserParams{
			ReadAt:    nullTimeFromPtr(&at),
			UpdatedAt: at,
			BeforeTs:  before,
			UserID:    nullStringFromPtr(&userID),
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: mark all read: %w", err)
		}
		return n, nil
	default:
		return 0, r.unconfigured()
	}
}

func (r *NotificationFeedRepositorySQLC) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.DeleteNotificationsOlderThan(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
		if err != nil {
			return 0, fmt.Errorf("sqlc: delete old notifications: %w", err)
		}
		return n, nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.DeleteNotificationsOlderThan(ctx, cutoff)
		if err != nil {
			return 0, fmt.Errorf("sqlc: delete old notifications: %w", err)
		}
		return n, nil
	default:
		return 0, r.unconfigured()
	}
}

func (r *NotificationFeedRepositorySQLC) ListUnreadForEscalation(ctx context.Context, categories []string, cutoff time.Time, limit int) ([]*domain.FeedNotification, error) {
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListUnreadForEscalation(ctx, pgsqlc.ListUnreadForEscalationParams{
			Cutoff:     pgTimestampFromPtr(&cutoff),
			Categories: categories,
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list unread for escalation: %w", err)
		}
		out := make([]*domain.FeedNotification, len(rows))
		for i, row := range rows {
			out[i] = feedNotifFromPG(row)
		}
		return capFeed(out, limit), nil
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListUnreadForEscalation(ctx, sqlitesqlc.ListUnreadForEscalationParams{
			Cutoff:     cutoff,
			Categories: categories,
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list unread for escalation: %w", err)
		}
		out := make([]*domain.FeedNotification, len(rows))
		for i, row := range rows {
			out[i] = feedNotifFromSQLite(row)
		}
		return capFeed(out, limit), nil
	default:
		return nil, r.unconfigured()
	}
}

// capFeed truncates to the newest `limit` items (the query already orders newest
// first). The LIMIT is applied in Go because SQLite rejects a numbered placeholder
// after a slice-expanded IN(...) list (placeholder-numbering collision).
func capFeed(items []*domain.FeedNotification, limit int) []*domain.FeedNotification {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (r *NotificationFeedRepositorySQLC) CountUnreadForEscalation(ctx context.Context, categories []string, cutoff time.Time) (int, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.CountUnreadForEscalation(ctx, pgsqlc.CountUnreadForEscalationParams{
			Cutoff:     pgTimestampFromPtr(&cutoff),
			Categories: categories,
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: count unread for escalation: %w", err)
		}
		return int(n), nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.CountUnreadForEscalation(ctx, sqlitesqlc.CountUnreadForEscalationParams{
			Cutoff:     cutoff,
			Categories: categories,
		})
		if err != nil {
			return 0, fmt.Errorf("sqlc: count unread for escalation: %w", err)
		}
		return int(n), nil
	default:
		return 0, r.unconfigured()
	}
}

// sqliteNarg returns nil or the string value for a sqlc nullable (interface{}) param.
func sqliteNarg(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// payloadToNullString converts a []byte JSON payload to the sqlite TEXT column type.
func payloadToNullString(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

func feedNotifFromPG(row pgsqlc.Notification) *domain.FeedNotification {
	out := &domain.FeedNotification{
		Base:       domain.Base{ID: row.ID, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time},
		Category:   row.Category,
		Severity:   row.Severity,
		Title:      row.Title,
		Payload:    row.Payload,
		OccurredAt: row.OccurredAt.Time,
	}
	if row.UserID.Valid {
		v := row.UserID.String
		out.UserID = &v
	}
	if row.Description.Valid {
		v := row.Description.String
		out.Description = &v
	}
	if row.DeepLink.Valid {
		v := row.DeepLink.String
		out.DeepLink = &v
	}
	if row.ReadAt.Valid {
		t := row.ReadAt.Time
		out.ReadAt = &t
	}
	return out
}

func feedNotifFromSQLite(row sqlitesqlc.Notification) *domain.FeedNotification {
	out := &domain.FeedNotification{
		Base:       domain.Base{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt},
		Category:   row.Category,
		Severity:   row.Severity,
		Title:      row.Title,
		OccurredAt: row.OccurredAt,
	}
	if row.UserID.Valid {
		v := row.UserID.String
		out.UserID = &v
	}
	if row.Description.Valid {
		v := row.Description.String
		out.Description = &v
	}
	if row.DeepLink.Valid {
		v := row.DeepLink.String
		out.DeepLink = &v
	}
	if row.Payload.Valid && row.Payload.String != "" {
		out.Payload = []byte(row.Payload.String)
	}
	if row.ReadAt.Valid {
		t := row.ReadAt.Time
		out.ReadAt = &t
	}
	return out
}
