package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	sq "github.com/Masterminds/squirrel"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

// ResourceRepositorySQLC implements port.ResourceRepository via sqlc.
//
// PR1 of US1 (spec 048) — CRUD only, no M2M, no preloads. The following
// methods are stubs that return an "unimplemented" error and will land in
// follow-up PRs:
//
//   - Update     : implements main columns only; Tags / NotificationChannels
//                  diff is deferred to PR2 / PR3.
//   - FindByTag  : requires resource_tags JOIN (PR2).
//   - UpdateMonitoringState / UpdateMetadata : require dynamic SQL (PR4).
//
// Returned `domain.Resource` values have `Tags = nil`, `Component = nil`,
// `NotificationChannels = nil`, `Credential = nil`. Callers that need
// preloads must continue using the GORM impl until later PRs land.
type ResourceRepositorySQLC struct {
	pgQ      *pgsqlc.Queries
	sqliteQ  *sqlitesqlc.Queries
	pgPool   *pgxpool.Pool
	sqliteDB *sql.DB
}

func NewResourceRepositorySQLC(rt SqlcRuntime) port.ResourceRepository {
	r := &ResourceRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgPool = pool
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteDB = db
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *ResourceRepositorySQLC) unconfigured() error {
	return fmt.Errorf("resource_repository_sqlc: unconfigured runtime")
}

// ---------- Public surface (port.ResourceRepository) ----------

func (r *ResourceRepositorySQLC) Create(ctx context.Context, res *domain.Resource) (*domain.Resource, error) {
	if res.ID == "" {
		res.EnsureID()
	}
	now := time.Now()
	if res.CreatedAt.IsZero() {
		res.CreatedAt = now
	}
	if res.UpdatedAt.IsZero() {
		res.UpdatedAt = now
	}
	if res.Status == "" {
		res.Status = domain.StatusPending
	}
	if res.Interval == 0 {
		res.Interval = 300
	}
	if res.Timeout == 0 {
		res.Timeout = 10
	}
	tagIDs := tagIDsFromResource(res)
	channelIDs := channelIDsFromResource(res)
	switch {
	case r.pgQ != nil:
		err := pgsqlc.WithTx(ctx, r.pgPool, func(q *pgsqlc.Queries) error {
			if err := q.CreateResource(ctx, resourceToPGCreate(res)); err != nil {
				return err
			}
			for _, tid := range tagIDs {
				if err := q.LinkResourceTag(ctx, pgsqlc.LinkResourceTagParams{
					ResourceID: res.ID, TagID: tid,
				}); err != nil {
					return err
				}
			}
			for _, cid := range channelIDs {
				if err := q.LinkResourceChannel(ctx, pgsqlc.LinkResourceChannelParams{
					ResourceID: res.ID, NotificationChannelID: cid,
				}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: create resource: %w", err)
		}
		return res, nil
	case r.sqliteQ != nil:
		err := sqlitesqlc.WithTx(ctx, r.sqliteDB, func(q *sqlitesqlc.Queries) error {
			if err := q.CreateResource(ctx, resourceToSQLiteCreate(res)); err != nil {
				return err
			}
			for _, tid := range tagIDs {
				if err := q.LinkResourceTag(ctx, sqlitesqlc.LinkResourceTagParams{
					ResourceID: res.ID, TagID: tid,
				}); err != nil {
					return err
				}
			}
			for _, cid := range channelIDs {
				if err := q.LinkResourceChannel(ctx, sqlitesqlc.LinkResourceChannelParams{
					ResourceID: res.ID, NotificationChannelID: cid,
				}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: create resource: %w", err)
		}
		return res, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *ResourceRepositorySQLC) FindByID(ctx context.Context, id string) (*domain.Resource, error) {
	var out *domain.Resource
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindResourceByID(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find resource by id: %w", err)
		}
		out = resourceFromPG(row)
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindResourceByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find resource by id: %w", err)
		}
		out = resourceFromSQLite(row)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, []*domain.Resource{out}); err != nil {
		return nil, err
	}
	if err := r.attachUptimeStats(ctx, []*domain.Resource{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResourceRepositorySQLC) FindByHeartbeatSlug(ctx context.Context, slug string) (*domain.Resource, error) {
	var out *domain.Resource
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindResourceByHeartbeatSlug(ctx, pgtype.Text{String: slug, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find resource by heartbeat slug: %w", err)
		}
		out = resourceFromPG(row)
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindResourceByHeartbeatSlug(ctx, sql.NullString{String: slug, Valid: true})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find resource by heartbeat slug: %w", err)
		}
		out = resourceFromSQLite(row)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, []*domain.Resource{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResourceRepositorySQLC) List(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
	var out []*domain.Resource
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListResources(ctx, pgsqlc.ListResourcesParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list resources: %w", err)
		}
		out = resourcesFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListResources(ctx, sqlitesqlc.ListResourcesParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list resources: %w", err)
		}
		out = resourcesFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachTagsOnly(ctx, out); err != nil {
		return nil, err
	}
	if err := r.attachIncidentCounts(ctx, out); err != nil {
		return nil, err
	}
	if err := r.attachUptimeStats(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResourceRepositorySQLC) Update(ctx context.Context, res *domain.Resource) error {
	res.UpdatedAt = time.Now()
	targetTagIDs := tagIDsFromResource(res)
	targetChannelIDs := channelIDsFromResource(res)
	switch {
	case r.pgQ != nil:
		return pgsqlc.WithTx(ctx, r.pgPool, func(q *pgsqlc.Queries) error {
			if err := r.updateMainPG(ctx, q, res); err != nil {
				return err
			}
			// Tags diff
			currentTagIDs, err := q.ListTagIDsByResourceID(ctx, res.ID)
			if err != nil {
				return err
			}
			tagAdd, tagRemove := diffJunctionSets(currentTagIDs, targetTagIDs)
			for _, tid := range tagRemove {
				if err := q.UnlinkResourceTag(ctx, pgsqlc.UnlinkResourceTagParams{
					ResourceID: res.ID, TagID: tid,
				}); err != nil {
					return err
				}
			}
			for _, tid := range tagAdd {
				if err := q.LinkResourceTag(ctx, pgsqlc.LinkResourceTagParams{
					ResourceID: res.ID, TagID: tid,
				}); err != nil {
					return err
				}
			}
			// Channels diff
			currentChannelIDs, err := q.ListChannelIDsByResourceID(ctx, res.ID)
			if err != nil {
				return err
			}
			chAdd, chRemove := diffJunctionSets(currentChannelIDs, targetChannelIDs)
			for _, cid := range chRemove {
				if err := q.UnlinkResourceChannel(ctx, pgsqlc.UnlinkResourceChannelParams{
					ResourceID: res.ID, NotificationChannelID: cid,
				}); err != nil {
					return err
				}
			}
			for _, cid := range chAdd {
				if err := q.LinkResourceChannel(ctx, pgsqlc.LinkResourceChannelParams{
					ResourceID: res.ID, NotificationChannelID: cid,
				}); err != nil {
					return err
				}
			}
			return nil
		})
	case r.sqliteQ != nil:
		return sqlitesqlc.WithTx(ctx, r.sqliteDB, func(q *sqlitesqlc.Queries) error {
			if err := r.updateMainSQLite(ctx, q, res); err != nil {
				return err
			}
			currentTagIDs, err := q.ListTagIDsByResourceID(ctx, res.ID)
			if err != nil {
				return err
			}
			tagAdd, tagRemove := diffJunctionSets(currentTagIDs, targetTagIDs)
			for _, tid := range tagRemove {
				if err := q.UnlinkResourceTag(ctx, sqlitesqlc.UnlinkResourceTagParams{
					ResourceID: res.ID, TagID: tid,
				}); err != nil {
					return err
				}
			}
			for _, tid := range tagAdd {
				if err := q.LinkResourceTag(ctx, sqlitesqlc.LinkResourceTagParams{
					ResourceID: res.ID, TagID: tid,
				}); err != nil {
					return err
				}
			}
			currentChannelIDs, err := q.ListChannelIDsByResourceID(ctx, res.ID)
			if err != nil {
				return err
			}
			chAdd, chRemove := diffJunctionSets(currentChannelIDs, targetChannelIDs)
			for _, cid := range chRemove {
				if err := q.UnlinkResourceChannel(ctx, sqlitesqlc.UnlinkResourceChannelParams{
					ResourceID: res.ID, NotificationChannelID: cid,
				}); err != nil {
					return err
				}
			}
			for _, cid := range chAdd {
				if err := q.LinkResourceChannel(ctx, sqlitesqlc.LinkResourceChannelParams{
					ResourceID: res.ID, NotificationChannelID: cid,
				}); err != nil {
					return err
				}
			}
			return nil
		})
	default:
		return r.unconfigured()
	}
}

func (r *ResourceRepositorySQLC) updateMainPG(ctx context.Context, q *pgsqlc.Queries, res *domain.Resource) error {
	if _, err := q.UpdateResourceMain(ctx, pgsqlc.UpdateResourceMainParams{
		ID:                      res.ID,
		Name:                    res.Name,
		Type:                    string(res.Type),
		Target:                  res.Target,
		Interval:                int32(res.Interval),
		Timeout:                 int32(res.Timeout),
		IsActive:                res.IsActive,
		ConfirmationChecks:      int32(res.ConfirmationChecks),
		ConfirmationInterval:    int32(res.ConfirmationInterval),
		ComponentID:             pgTextFromPtr(res.ComponentID),
		ExpiryAlertThresholds:   pgTextFromPtr(res.ExpiryAlertThresholds),
		FlapDetectionEnabled:    res.FlapDetectionEnabled,
		FlapThreshold:           int32(res.FlapThreshold),
		FlapWindowSeconds:       int32(res.FlapWindowSeconds),
		FlapMaxDurationMinutes:  int32(res.FlapMaxDurationMinutes),
		ReminderIntervalMinutes: int32(res.ReminderIntervalMinutes),
		HeartbeatInterval:       pgInt4FromPtr(res.HeartbeatInterval),
		HeartbeatGrace:          pgInt4FromPtr(res.HeartbeatGrace),
		UpdatedAt:               pgtype.Timestamptz{Time: res.UpdatedAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("sqlc: update resource: %w", err)
	}
	return nil
}

func (r *ResourceRepositorySQLC) updateMainSQLite(ctx context.Context, q *sqlitesqlc.Queries, res *domain.Resource) error {
	if _, err := q.UpdateResourceMain(ctx, sqlitesqlc.UpdateResourceMainParams{
		ID:                      res.ID,
		Name:                    res.Name,
		Type:                    string(res.Type),
		Target:                  res.Target,
		Interval:                int64(res.Interval),
		Timeout:                 int64(res.Timeout),
		IsActive:                boolToInt64(res.IsActive),
		ConfirmationChecks:      int64(res.ConfirmationChecks),
		ConfirmationInterval:    int64(res.ConfirmationInterval),
		ComponentID:             nullStringFromPtr(res.ComponentID),
		ExpiryAlertThresholds:   nullStringFromPtr(res.ExpiryAlertThresholds),
		FlapDetectionEnabled:    boolToInt64(res.FlapDetectionEnabled),
		FlapThreshold:           int64(res.FlapThreshold),
		FlapWindowSeconds:       int64(res.FlapWindowSeconds),
		FlapMaxDurationMinutes:  int64(res.FlapMaxDurationMinutes),
		ReminderIntervalMinutes: int64(res.ReminderIntervalMinutes),
		HeartbeatInterval:       nullInt64FromPtrInt(res.HeartbeatInterval),
		HeartbeatGrace:          nullInt64FromPtrInt(res.HeartbeatGrace),
		UpdatedAt:               res.UpdatedAt,
	}); err != nil {
		return fmt.Errorf("sqlc: update resource: %w", err)
	}
	return nil
}

func (r *ResourceRepositorySQLC) Delete(ctx context.Context, id string) error {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.SoftDeleteResource(ctx, id)
		if err != nil {
			return fmt.Errorf("sqlc: delete resource: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.SoftDeleteResource(ctx, id)
		if err != nil {
			return fmt.Errorf("sqlc: delete resource: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

func (r *ResourceRepositorySQLC) FindActive(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
	var out []*domain.Resource
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListActiveResources(ctx, pgsqlc.ListActiveResourcesParams{
			Limit: int32(limit), Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find active resources: %w", err)
		}
		out = resourcesFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListActiveResources(ctx, sqlitesqlc.ListActiveResourcesParams{
			Limit: int64(limit), Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find active resources: %w", err)
		}
		out = resourcesFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResourceRepositorySQLC) FindByTag(ctx context.Context, tagName string, limit, offset int) ([]*domain.Resource, error) {
	switch {
	case r.pgQ != nil:
		ids, err := r.pgQ.FindResourceIDsByTagName(ctx, pgsqlc.FindResourceIDsByTagNameParams{
			Name: tagName, Limit: int32(limit), Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find resource ids by tag: %w", err)
		}
		if len(ids) == 0 {
			return []*domain.Resource{}, nil
		}
		rows, err := r.pgQ.FindResourcesByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("sqlc: find resources by ids: %w", err)
		}
		out := resourcesInOrder(resourcesFromPG(rows), ids)
		if err := r.attachPreloads(ctx, out); err != nil {
			return nil, err
		}
		return out, nil
	case r.sqliteQ != nil:
		ids, err := r.sqliteQ.FindResourceIDsByTagName(ctx, sqlitesqlc.FindResourceIDsByTagNameParams{
			Name: tagName, Limit: int64(limit), Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find resource ids by tag: %w", err)
		}
		if len(ids) == 0 {
			return []*domain.Resource{}, nil
		}
		rows, err := r.sqliteQ.FindResourcesByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("sqlc: find resources by ids: %w", err)
		}
		out := resourcesInOrder(resourcesFromSQLite(rows), ids)
		if err := r.attachPreloads(ctx, out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *ResourceRepositorySQLC) FindByComponentID(ctx context.Context, componentID string) ([]*domain.Resource, error) {
	var out []*domain.Resource
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.FindResourcesByComponentID(ctx, pgtype.Text{String: componentID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find resources by component: %w", err)
		}
		out = resourcesFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.FindResourcesByComponentID(ctx, sql.NullString{String: componentID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find resources by component: %w", err)
		}
		out = resourcesFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResourceRepositorySQLC) CountByComponentID(ctx context.Context, componentID string) (int64, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.CountResourcesByComponentID(ctx, pgtype.Text{String: componentID, Valid: true})
		if err != nil {
			return 0, fmt.Errorf("sqlc: count resources by component: %w", err)
		}
		return n, nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.CountResourcesByComponentID(ctx, sql.NullString{String: componentID, Valid: true})
		if err != nil {
			return 0, fmt.Errorf("sqlc: count resources by component: %w", err)
		}
		return n, nil
	default:
		return 0, r.unconfigured()
	}
}

func (r *ResourceRepositorySQLC) FindMissedHeartbeats(ctx context.Context, now time.Time, limit int) ([]*domain.Resource, error) {
	if limit <= 0 {
		limit = 1000
	}
	var out []*domain.Resource
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.FindMissedHeartbeatsPG(ctx, pgsqlc.FindMissedHeartbeatsPGParams{
			NowUnix:  float64(now.Unix()),
			RowLimit: int32(limit),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find missed heartbeats: %w", err)
		}
		out = resourcesFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.FindMissedHeartbeatsSQLite(ctx, sqlitesqlc.FindMissedHeartbeatsSQLiteParams{
			NowUnix:  now.Unix(),
			RowLimit: int64(limit),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find missed heartbeats: %w", err)
		}
		out = resourcesFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResourceRepositorySQLC) UpdateLastPingAt(ctx context.Context, id string, at time.Time) error {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.UpdateResourceLastPingAt(ctx, pgsqlc.UpdateResourceLastPingAtParams{
			ID:         id,
			LastPingAt: pgtype.Timestamp{Time: at, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("sqlc: update last_ping_at: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.UpdateResourceLastPingAt(ctx, sqlitesqlc.UpdateResourceLastPingAtParams{
			ID:         id,
			LastPingAt: sql.NullTime{Time: at, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("sqlc: update last_ping_at: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

func (r *ResourceRepositorySQLC) UpdateStatus(ctx context.Context, id string, status domain.ResourceStatus) error {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.UpdateResourceStatus(ctx, pgsqlc.UpdateResourceStatusParams{
			ID: id, Status: string(status),
		})
		if err != nil {
			return fmt.Errorf("sqlc: update status: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.UpdateResourceStatus(ctx, sqlitesqlc.UpdateResourceStatusParams{
			ID: id, Status: string(status),
		})
		if err != nil {
			return fmt.Errorf("sqlc: update status: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

// UpdateMonitoringState writes only the columns whose pointer field is
// non-nil. Hand-built dynamic SQL — sqlc-generated static queries cannot
// express "skip column N" without exploding to 2^N variants. Column names
// are hardcoded; only values cross the driver via parameterized SQL.
func (r *ResourceRepositorySQLC) UpdateMonitoringState(ctx context.Context, id string, req port.UpdateMonitoringStateRequest) error {
	b := newSetBuilder()
	if req.Status != nil {
		b.add("status", string(*req.Status))
	}
	if req.FailureCount != nil {
		b.add("failure_count", *req.FailureCount)
	}
	if req.LastChecked != nil {
		b.add("last_checked", derefTimePtr(req.LastChecked))
	}
	if req.LastStatusTransition != nil {
		b.add("last_status_transition", derefTimePtr(req.LastStatusTransition))
	}
	if req.FlapStartedAt != nil {
		b.add("flap_started_at", derefTimePtr(req.FlapStartedAt))
	}
	return r.execDynamicUpdate(ctx, id, b)
}

func (r *ResourceRepositorySQLC) UpdateMetadata(ctx context.Context, id string, req port.UpdateMetadataRequest) error {
	b := newSetBuilder()
	if req.SSLExpirationDate != nil {
		b.add("ssl_expiration_date", derefTimePtr(req.SSLExpirationDate))
	}
	if req.SSLIssuer != nil {
		b.add("ssl_issuer", *req.SSLIssuer)
	}
	if req.DomainExpirationDate != nil {
		b.add("domain_expiration_date", derefTimePtr(req.DomainExpirationDate))
	}
	if req.DomainRegistrar != nil {
		b.add("domain_registrar", *req.DomainRegistrar)
	}
	return r.execDynamicUpdate(ctx, id, b)
}

// execDynamicUpdate finalises a SET-builder against the `resources` table.
// No-op when no columns were added (caller passed an all-nil request).
func (r *ResourceRepositorySQLC) execDynamicUpdate(ctx context.Context, id string, b *setBuilder) error {
	if b.empty() {
		return nil
	}
	switch {
	case r.pgPool != nil:
		sqlStr, args := b.buildPG(id)
		ct, err := r.pgPool.Exec(ctx, sqlStr, args...)
		if err != nil {
			return fmt.Errorf("sqlc: dynamic update resources: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteDB != nil:
		sqlStr, args := b.buildSQLite(id)
		res, err := r.sqliteDB.ExecContext(ctx, sqlStr, args...)
		if err != nil {
			return fmt.Errorf("sqlc: dynamic update resources: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

// setBuilder accumulates (column, value) pairs for a dynamic UPDATE.
type setBuilder struct {
	cols []string
	vals []any
}

func newSetBuilder() *setBuilder { return &setBuilder{} }

func (b *setBuilder) add(col string, v any) {
	b.cols = append(b.cols, col)
	b.vals = append(b.vals, v)
}

func (b *setBuilder) empty() bool { return len(b.cols) == 0 }

// buildPG returns `UPDATE resources SET c1=$1, c2=$2, ... WHERE id=$N` and
// the args slice (values + id at the tail).
func (b *setBuilder) buildPG(id string) (string, []any) {
	var sb strings.Builder
	sb.WriteString("UPDATE resources SET ")
	for i, col := range b.cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(col)
		sb.WriteString(" = $")
		sb.WriteString(strconv.Itoa(i + 1))
	}
	sb.WriteString(" WHERE id = $")
	sb.WriteString(strconv.Itoa(len(b.cols) + 1))
	args := append([]any{}, b.vals...)
	args = append(args, id)
	return sb.String(), args
}

// buildSQLite returns `UPDATE resources SET c1=?, c2=?, ... WHERE id=?` and
// the args slice (values + id at the tail).
func (b *setBuilder) buildSQLite(id string) (string, []any) {
	var sb strings.Builder
	sb.WriteString("UPDATE resources SET ")
	for i, col := range b.cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(col)
		sb.WriteString(" = ?")
	}
	sb.WriteString(" WHERE id = ?")
	args := append([]any{}, b.vals...)
	args = append(args, id)
	return sb.String(), args
}

// derefTimePtr returns the inner *time.Time, preserving nil for "set NULL".
// Drivers (pgx and database/sql) translate nil → NULL.
func derefTimePtr(pp **time.Time) any {
	if pp == nil || *pp == nil {
		return nil
	}
	return **pp
}

func (r *ResourceRepositorySQLC) FindScheduledResources(ctx context.Context) ([]*domain.Resource, error) {
	var out []*domain.Resource
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListScheduledResources(ctx)
		if err != nil {
			return nil, fmt.Errorf("sqlc: list scheduled resources: %w", err)
		}
		out = resourcesFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListScheduledResources(ctx)
		if err != nil {
			return nil, fmt.Errorf("sqlc: list scheduled resources: %w", err)
		}
		out = resourcesFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetResourceHostID links (or unlinks, when hostID is nil) a monitor to a host.
func (r *ResourceRepositorySQLC) SetResourceHostID(ctx context.Context, resourceID string, hostID *string) error {
	now := time.Now()
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.SetResourceHostID(ctx, pgsqlc.SetResourceHostIDParams{
			ID:        resourceID,
			HostID:    pgTextFromPtr(hostID),
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("sqlc: set resource host_id: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.SetResourceHostID(ctx, sqlitesqlc.SetResourceHostIDParams{
			ID:        resourceID,
			HostID:    nullStringFromPtr(hostID),
			UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("sqlc: set resource host_id: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

// ClearResourceHostIDByHost unlinks every monitor pointing at the given host.
func (r *ResourceRepositorySQLC) ClearResourceHostIDByHost(ctx context.Context, hostID string) error {
	switch {
	case r.pgQ != nil:
		return r.pgQ.ClearResourceHostIDByHost(ctx, pgTextFromPtr(&hostID))
	case r.sqliteQ != nil:
		return r.sqliteQ.ClearResourceHostIDByHost(ctx, nullStringFromPtr(&hostID))
	default:
		return r.unconfigured()
	}
}

// ---------- Mapping helpers ----------

func resourceToPGCreate(r *domain.Resource) pgsqlc.CreateResourceParams {
	return pgsqlc.CreateResourceParams{
		ID:                      r.ID,
		CreatedAt:               pgtype.Timestamptz{Time: r.CreatedAt, Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: r.UpdatedAt, Valid: true},
		Name:                    r.Name,
		Type:                    string(r.Type),
		Interval:                int32(r.Interval),
		Timeout:                 int32(r.Timeout),
		Target:                  r.Target,
		LastChecked:             pgTimestampFromPtr(r.LastChecked),
		Status:                  string(r.Status),
		IsActive:                r.IsActive,
		FailureCount:            int32(r.FailureCount),
		SslExpirationDate:       pgTimestampFromPtr(metaSSLExpiration(r)),
		SslIssuer:               pgTextFromPtr(metaSSLIssuerPtr(r)),
		DomainExpirationDate:    pgTimestampFromPtr(metaDomainExpiration(r)),
		DomainRegistrar:         pgTextFromPtr(metaDomainRegistrarPtr(r)),
		ComponentID:             pgTextFromPtr(r.ComponentID),
		ConfirmationChecks:      int32(r.ConfirmationChecks),
		ConfirmationInterval:    int32(r.ConfirmationInterval),
		ExpiryAlertThresholds:   pgTextFromPtr(r.ExpiryAlertThresholds),
		FlapDetectionEnabled:    r.FlapDetectionEnabled,
		FlapThreshold:           int32(r.FlapThreshold),
		FlapWindowSeconds:       int32(r.FlapWindowSeconds),
		FlapMaxDurationMinutes:  int32(r.FlapMaxDurationMinutes),
		LastStatusTransition:    pgTimestampFromPtr(r.LastStatusTransition),
		FlapStartedAt:           pgTimestampFromPtr(r.FlapStartedAt),
		ReminderIntervalMinutes: int32(r.ReminderIntervalMinutes),
		HeartbeatSlug:           pgTextFromPtr(r.HeartbeatSlug),
		HeartbeatInterval:       pgInt4FromPtr(r.HeartbeatInterval),
		HeartbeatGrace:          pgInt4FromPtr(r.HeartbeatGrace),
		LastPingAt:              pgPlainTimestampFromPtr(r.LastPingAt),
		Keyword:                 pgTextFromPtr(r.Keyword),
		KeywordMode:             pgTextFromPtr(r.KeywordMode),
		ProtocolType:            pgTextFromPtr(r.ProtocolType),
		ProtocolPort:            pgInt4FromPtr(r.ProtocolPort),
		HostID:                  pgTextFromPtr(r.HostID),
	}
}

func resourceToSQLiteCreate(r *domain.Resource) sqlitesqlc.CreateResourceParams {
	return sqlitesqlc.CreateResourceParams{
		ID:                      r.ID,
		CreatedAt:               r.CreatedAt,
		UpdatedAt:               r.UpdatedAt,
		Name:                    r.Name,
		Type:                    string(r.Type),
		Interval:                int64(r.Interval),
		Timeout:                 int64(r.Timeout),
		Target:                  r.Target,
		LastChecked:             nullTimeFromPtr(r.LastChecked),
		Status:                  string(r.Status),
		IsActive:                boolToInt64(r.IsActive),
		FailureCount:            int64(r.FailureCount),
		SslExpirationDate:       nullTimeFromPtr(metaSSLExpiration(r)),
		SslIssuer:               nullStringFromPtr(metaSSLIssuerPtr(r)),
		DomainExpirationDate:    nullTimeFromPtr(metaDomainExpiration(r)),
		DomainRegistrar:         nullStringFromPtr(metaDomainRegistrarPtr(r)),
		ComponentID:             nullStringFromPtr(r.ComponentID),
		ConfirmationChecks:      int64(r.ConfirmationChecks),
		ConfirmationInterval:    int64(r.ConfirmationInterval),
		ExpiryAlertThresholds:   nullStringFromPtr(r.ExpiryAlertThresholds),
		FlapDetectionEnabled:    boolToInt64(r.FlapDetectionEnabled),
		FlapThreshold:           int64(r.FlapThreshold),
		FlapWindowSeconds:       int64(r.FlapWindowSeconds),
		FlapMaxDurationMinutes:  int64(r.FlapMaxDurationMinutes),
		LastStatusTransition:    nullTimeFromPtr(r.LastStatusTransition),
		FlapStartedAt:           nullTimeFromPtr(r.FlapStartedAt),
		ReminderIntervalMinutes: int64(r.ReminderIntervalMinutes),
		HeartbeatSlug:           nullStringFromPtr(r.HeartbeatSlug),
		HeartbeatInterval:       nullInt64FromPtrInt(r.HeartbeatInterval),
		HeartbeatGrace:          nullInt64FromPtrInt(r.HeartbeatGrace),
		LastPingAt:              nullTimeFromPtr(r.LastPingAt),
		Keyword:                 nullStringFromPtr(r.Keyword),
		KeywordMode:             nullStringFromPtr(r.KeywordMode),
		ProtocolType:            nullStringFromPtr(r.ProtocolType),
		ProtocolPort:            nullInt64FromPtrInt(r.ProtocolPort),
		HostID:                  nullStringFromPtr(r.HostID),
	}
}

func resourceFromPG(row pgsqlc.Resource) *domain.Resource {
	out := &domain.Resource{
		Base: domain.Base{
			ID:        row.ID,
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		},
		Name:                    row.Name,
		Type:                    domain.ResourceType(row.Type),
		Interval:                int(row.Interval),
		Timeout:                 int(row.Timeout),
		Target:                  row.Target,
		LastChecked:             ptrTimeFromPGTimestamptz(row.LastChecked),
		Status:                  domain.ResourceStatus(row.Status),
		IsActive:                row.IsActive,
		FailureCount:            int(row.FailureCount),
		ComponentID:             ptrStringFromPGText(row.ComponentID),
		ConfirmationChecks:      int(row.ConfirmationChecks),
		ConfirmationInterval:    int(row.ConfirmationInterval),
		ExpiryAlertThresholds:   ptrStringFromPGText(row.ExpiryAlertThresholds),
		FlapDetectionEnabled:    row.FlapDetectionEnabled,
		FlapThreshold:           int(row.FlapThreshold),
		FlapWindowSeconds:       int(row.FlapWindowSeconds),
		FlapMaxDurationMinutes:  int(row.FlapMaxDurationMinutes),
		LastStatusTransition:    ptrTimeFromPGTimestamptz(row.LastStatusTransition),
		FlapStartedAt:           ptrTimeFromPGTimestamptz(row.FlapStartedAt),
		ReminderIntervalMinutes: int(row.ReminderIntervalMinutes),
		HeartbeatSlug:           ptrStringFromPGText(row.HeartbeatSlug),
		HeartbeatInterval:       ptrIntFromPGInt4(row.HeartbeatInterval),
		HeartbeatGrace:          ptrIntFromPGInt4(row.HeartbeatGrace),
		LastPingAt:              ptrTimeFromPGTimestamp(row.LastPingAt),
		Keyword:                 ptrStringFromPGText(row.Keyword),
		KeywordMode:             ptrStringFromPGText(row.KeywordMode),
		ProtocolType:            ptrStringFromPGText(row.ProtocolType),
		ProtocolPort:            ptrIntFromPGInt4(row.ProtocolPort),
		HostID:                  ptrStringFromPGText(row.HostID),
	}
	if md := metaFromPG(row); md != nil {
		out.Metadata = md
	}
	return out
}

func resourceFromSQLite(row sqlitesqlc.Resource) *domain.Resource {
	out := &domain.Resource{
		Base: domain.Base{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		},
		Name:                    row.Name,
		Type:                    domain.ResourceType(row.Type),
		Interval:                int(row.Interval),
		Timeout:                 int(row.Timeout),
		Target:                  row.Target,
		LastChecked:             ptrTimeFromNullTime(row.LastChecked),
		Status:                  domain.ResourceStatus(row.Status),
		IsActive:                row.IsActive == 1,
		FailureCount:            int(row.FailureCount),
		ComponentID:             ptrStringFromNullString(row.ComponentID),
		ConfirmationChecks:      int(row.ConfirmationChecks),
		ConfirmationInterval:    int(row.ConfirmationInterval),
		ExpiryAlertThresholds:   ptrStringFromNullString(row.ExpiryAlertThresholds),
		FlapDetectionEnabled:    row.FlapDetectionEnabled == 1,
		FlapThreshold:           int(row.FlapThreshold),
		FlapWindowSeconds:       int(row.FlapWindowSeconds),
		FlapMaxDurationMinutes:  int(row.FlapMaxDurationMinutes),
		LastStatusTransition:    ptrTimeFromNullTime(row.LastStatusTransition),
		FlapStartedAt:           ptrTimeFromNullTime(row.FlapStartedAt),
		ReminderIntervalMinutes: int(row.ReminderIntervalMinutes),
		HeartbeatSlug:           ptrStringFromNullString(row.HeartbeatSlug),
		HeartbeatInterval:       ptrIntFromNullInt64(row.HeartbeatInterval),
		HeartbeatGrace:          ptrIntFromNullInt64(row.HeartbeatGrace),
		LastPingAt:              ptrTimeFromNullTime(row.LastPingAt),
		Keyword:                 ptrStringFromNullString(row.Keyword),
		KeywordMode:             ptrStringFromNullString(row.KeywordMode),
		ProtocolType:            ptrStringFromNullString(row.ProtocolType),
		ProtocolPort:            ptrIntFromNullInt64(row.ProtocolPort),
		HostID:                  ptrStringFromNullString(row.HostID),
	}
	if md := metaFromSQLite(row); md != nil {
		out.Metadata = md
	}
	return out
}

func resourcesFromPG(rows []pgsqlc.Resource) []*domain.Resource {
	out := make([]*domain.Resource, len(rows))
	for i, row := range rows {
		out[i] = resourceFromPG(row)
	}
	return out
}

func resourcesFromSQLite(rows []sqlitesqlc.Resource) []*domain.Resource {
	out := make([]*domain.Resource, len(rows))
	for i, row := range rows {
		out[i] = resourceFromSQLite(row)
	}
	return out
}

// ---------- preload composite ----------

// attachPreloads fetches Tags + NotificationChannels + Component + Credential
// for the given resources via 4 round-trips (controlled N+1, clarification Q1
// and FR-006). Each helper is a no-op when the underlying ID set is empty.
//
// Used by FindByID / FindByHeartbeatSlug / FindActive / FindByTag /
// FindByComponentID / FindMissedHeartbeats / FindScheduledResources — paths
// whose callers consume the full graph (workers checking credentials,
// scheduler reading config, etc.).
//
// For the v1 List path (MonitorHandler.List), use attachTagsOnly: the
// response mapper consumes only Tags, the 3 other preloads were dead code on
// that path and accounted for ~80% of the GORM-vs-sqlc perf gap.
func (r *ResourceRepositorySQLC) attachPreloads(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	if err := r.attachTagsToResources(ctx, resources); err != nil {
		return err
	}
	if err := r.attachChannelsToResources(ctx, resources); err != nil {
		return err
	}
	if err := r.attachComponentsToResources(ctx, resources); err != nil {
		return err
	}
	if err := r.attachCredentialsToResources(ctx, resources); err != nil {
		return err
	}
	return nil
}

// attachTagsOnly is the list-view variant: only Tags are preloaded. Used by
// List + ListResourcesByFilter where the v1 response mapper consumes only
// tag names. Drops 3 round-trips vs attachPreloads. See spec 050 retro for
// the audit confirming Channels/Component/Credential are unused on this path.
func (r *ResourceRepositorySQLC) attachTagsOnly(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	return r.attachTagsToResources(ctx, resources)
}

// attachIncidentCounts populates Resource.IncidentCount30d with the number of
// incidents whose started_at falls within the last 30 days. One round-trip
// total (GROUP BY resource_id). Resources with zero incidents in the window
// get a non-nil pointer to 0 so the JSON field is always present.
func (r *ResourceRepositorySQLC) attachIncidentCounts(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]string, 0, len(resources))
	for _, res := range resources {
		if res != nil {
			ids = append(ids, res.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	since := time.Now().Add(-30 * 24 * time.Hour)

	counts := make(map[string]int, len(resources))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.CountIncidentsPerResourceSince(ctx, pgsqlc.CountIncidentsPerResourceSinceParams{
			StartedAt: pgtype.Timestamptz{Time: since, Valid: true},
			Column2:   ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: count incidents per resource: %w", err)
		}
		for _, row := range rows {
			counts[row.ResourceID] = int(row.IncidentCount)
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.CountIncidentsPerResourceSince(ctx, sqlitesqlc.CountIncidentsPerResourceSinceParams{
			StartedAt:   since,
			ResourceIds: ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: count incidents per resource: %w", err)
		}
		for _, row := range rows {
			counts[row.ResourceID] = int(row.IncidentCount)
		}
	default:
		return r.unconfigured()
	}

	for _, res := range resources {
		if res == nil {
			continue
		}
		c := counts[res.ID]
		res.IncidentCount30d = &c
	}
	return nil
}

// attachUptimeStats populates Resource.Uptime30d and Resource.ResponseTimeAvg
// using two bulk round-trips: SUM(up)/SUM(samples) over uptime_daily_agg for
// the last 30 days, and AVG(response_time) over successful monitoring_activities
// for the same window. Resources with no data leave both pointers nil so the
// JSON fields omit and the frontend renders `-`.
func (r *ResourceRepositorySQLC) attachUptimeStats(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]string, 0, len(resources))
	for _, res := range resources {
		if res != nil {
			ids = append(ids, res.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)
	fromDay30 := now.AddDate(0, 0, -29) // inclusive 30-day window
	fromDay7 := now.AddDate(0, 0, -6)   // inclusive 7-day window

	uptime30 := make(map[string]float64, len(resources))
	uptime7 := make(map[string]float64, len(resources))
	resp := make(map[string]int, len(resources))

	switch {
	case r.pgQ != nil:
		rows30, err := r.pgQ.SumUptimeAggByResourcesSince(ctx, pgsqlc.SumUptimeAggByResourcesSinceParams{
			Day:     pgtype.Date{Time: fromDay30, Valid: true},
			Column2: ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: sum uptime agg 30d: %w", err)
		}
		for _, row := range rows30 {
			if row.SamplesSum > 0 {
				uptime30[row.ResourceID] = float64(row.UpSum) / float64(row.SamplesSum)
			}
		}
		rows7, err := r.pgQ.SumUptimeAggByResourcesSince(ctx, pgsqlc.SumUptimeAggByResourcesSinceParams{
			Day:     pgtype.Date{Time: fromDay7, Valid: true},
			Column2: ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: sum uptime agg 7d: %w", err)
		}
		for _, row := range rows7 {
			if row.SamplesSum > 0 {
				uptime7[row.ResourceID] = float64(row.UpSum) / float64(row.SamplesSum)
			}
		}
		aRows, err := r.pgQ.AvgResponseTimeByResourcesSince(ctx, pgsqlc.AvgResponseTimeByResourcesSinceParams{
			CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
			Column2:   ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: avg response time per resource: %w", err)
		}
		for _, row := range aRows {
			resp[row.ResourceID] = int(row.AvgMs + 0.5)
		}
	case r.sqliteQ != nil:
		rows30, err := r.sqliteQ.SumUptimeAggByResourcesSince(ctx, sqlitesqlc.SumUptimeAggByResourcesSinceParams{
			FromDay:     fromDay30.Format("2006-01-02"),
			ResourceIds: ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: sum uptime agg 30d: %w", err)
		}
		for _, row := range rows30 {
			if row.SamplesSum.Valid && row.SamplesSum.Float64 > 0 && row.UpSum.Valid {
				uptime30[row.ResourceID] = row.UpSum.Float64 / row.SamplesSum.Float64
			}
		}
		rows7, err := r.sqliteQ.SumUptimeAggByResourcesSince(ctx, sqlitesqlc.SumUptimeAggByResourcesSinceParams{
			FromDay:     fromDay7.Format("2006-01-02"),
			ResourceIds: ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: sum uptime agg 7d: %w", err)
		}
		for _, row := range rows7 {
			if row.SamplesSum.Valid && row.SamplesSum.Float64 > 0 && row.UpSum.Valid {
				uptime7[row.ResourceID] = row.UpSum.Float64 / row.SamplesSum.Float64
			}
		}
		aRows, err := r.sqliteQ.AvgResponseTimeByResourcesSince(ctx, sqlitesqlc.AvgResponseTimeByResourcesSinceParams{
			Since:       since,
			ResourceIds: ids,
		})
		if err != nil {
			return fmt.Errorf("sqlc: avg response time per resource: %w", err)
		}
		for _, row := range aRows {
			if row.AvgMs.Valid {
				resp[row.ResourceID] = int(row.AvgMs.Float64 + 0.5)
			}
		}
	default:
		return r.unconfigured()
	}

	for _, res := range resources {
		if res == nil {
			continue
		}
		if v, ok := uptime30[res.ID]; ok {
			res.Uptime30d = &v
		}
		if v, ok := uptime7[res.ID]; ok {
			res.Uptime7d = &v
		}
		if v, ok := resp[res.ID]; ok {
			res.ResponseTimeAvg = &v
		}
	}
	return nil
}

// ---------- M2M (resource_tags) helpers ----------

func tagIDsFromResource(r *domain.Resource) []string {
	if len(r.Tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.Tags))
	for _, t := range r.Tags {
		if t != nil && t.ID != "" {
			out = append(out, t.ID)
		}
	}
	return out
}

// attachTagsToResources fetches resource_tags + tags for a set of resource IDs
// (one round-trip) and attaches the resulting *domain.Tags slices to each
// matching resource in `resources`. Resources with no tags get an empty slice
// (consistent with GORM Preload semantics: nil-vs-empty distinction not
// preserved by the underlying join).
func (r *ResourceRepositorySQLC) attachTagsToResources(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]string, len(resources))
	for i, res := range resources {
		ids[i] = res.ID
	}
	byID := make(map[string][]*domain.Tags, len(resources))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListTagsByResourceIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload tags: %w", err)
		}
		for _, row := range rows {
			t := &domain.Tags{
				Base: domain.Base{
					ID:        row.ID,
					CreatedAt: row.CreatedAt.Time,
					UpdatedAt: row.UpdatedAt.Time,
				},
				Name:        row.Name,
				Color:       ptrStringFromPGText(row.Color),
				Description: ptrStringFromPGText(row.Description),
			}
			byID[row.ResourceID] = append(byID[row.ResourceID], t)
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListTagsByResourceIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload tags: %w", err)
		}
		for _, row := range rows {
			t := &domain.Tags{
				Base: domain.Base{
					ID:        row.ID,
					CreatedAt: row.CreatedAt,
					UpdatedAt: row.UpdatedAt,
				},
				Name:        row.Name,
				Color:       ptrStringFromNullString(row.Color),
				Description: ptrStringFromNullString(row.Description),
			}
			byID[row.ResourceID] = append(byID[row.ResourceID], t)
		}
	default:
		return r.unconfigured()
	}
	for _, res := range resources {
		res.Tags = byID[res.ID]
	}
	return nil
}

// ---------- M2M (resource_notification_channels) helpers ----------

func channelIDsFromResource(r *domain.Resource) []string {
	if len(r.NotificationChannels) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.NotificationChannels))
	for _, c := range r.NotificationChannels {
		if c != nil && c.ID != "" {
			out = append(out, c.ID)
		}
	}
	return out
}

func (r *ResourceRepositorySQLC) attachChannelsToResources(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]string, len(resources))
	for i, res := range resources {
		ids[i] = res.ID
	}
	byID := make(map[string][]*domain.NotificationChannel, len(resources))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListChannelsByResourceIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload channels: %w", err)
		}
		for _, row := range rows {
			cfg, err := decryptChannelConfig(row.Config)
			if err != nil {
				return fmt.Errorf("sqlc: preload channels (decrypt config): %w", err)
			}
			ch := &domain.NotificationChannel{
				Base: domain.Base{
					ID:        row.ID,
					CreatedAt: row.CreatedAt.Time,
					UpdatedAt: row.UpdatedAt.Time,
				},
				Name:             row.Name,
				Type:             domain.NotificationChannelType(row.Type),
				Config:           cfg,
				EnabledByDefault: row.EnabledByDefault,
			}
			byID[row.ResourceID] = append(byID[row.ResourceID], ch)
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListChannelsByResourceIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload channels: %w", err)
		}
		for _, row := range rows {
			cfg, err := decryptChannelConfig(row.Config)
			if err != nil {
				return fmt.Errorf("sqlc: preload channels (decrypt config): %w", err)
			}
			ch := &domain.NotificationChannel{
				Base: domain.Base{
					ID:        row.ID,
					CreatedAt: row.CreatedAt,
					UpdatedAt: row.UpdatedAt,
				},
				Name:             row.Name,
				Type:             domain.NotificationChannelType(row.Type),
				Config:           cfg,
				EnabledByDefault: row.EnabledByDefault == 1,
			}
			byID[row.ResourceID] = append(byID[row.ResourceID], ch)
		}
	default:
		return r.unconfigured()
	}
	for _, res := range resources {
		res.NotificationChannels = byID[res.ID]
	}
	return nil
}

// ---------- 1-to-1 preload (Component) ----------

func (r *ResourceRepositorySQLC) attachComponentsToResources(ctx context.Context, resources []*domain.Resource) error {
	ids := make([]string, 0, len(resources))
	for _, res := range resources {
		if res.ComponentID != nil && *res.ComponentID != "" {
			ids = append(ids, *res.ComponentID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	byID := make(map[string]*domain.Component, len(ids))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListComponentsByIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload components: %w", err)
		}
		for _, row := range rows {
			byID[row.ID] = componentFromPG(row)
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListComponentsByIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload components: %w", err)
		}
		for _, row := range rows {
			byID[row.ID] = componentFromSQLite(row)
		}
	default:
		return r.unconfigured()
	}
	for _, res := range resources {
		if res.ComponentID == nil {
			continue
		}
		if c, ok := byID[*res.ComponentID]; ok {
			res.Component = c
		}
	}
	return nil
}

// ---------- 1-to-1 preload (Credential) ----------

func (r *ResourceRepositorySQLC) attachCredentialsToResources(ctx context.Context, resources []*domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]string, len(resources))
	for i, res := range resources {
		ids[i] = res.ID
	}
	byResourceID := make(map[string]*domain.ResourceCredential, len(resources))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListCredentialsByResourceIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload credentials: %w", err)
		}
		for _, row := range rows {
			pw, err := decryptCredentialPassword(row.Password)
			if err != nil {
				return err
			}
			opts, err := decryptCredentialOptions(row.Options)
			if err != nil {
				return err
			}
			username := ""
			if row.Username.Valid {
				username = row.Username.String
			}
			byResourceID[row.ResourceID] = &domain.ResourceCredential{
				Base: domain.Base{
					ID:        row.ID,
					CreatedAt: row.CreatedAt.Time,
					UpdatedAt: row.UpdatedAt.Time,
				},
				ResourceID: row.ResourceID,
				Username:   username,
				Password:   pw,
				Options:    opts,
			}
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListCredentialsByResourceIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload credentials: %w", err)
		}
		for _, row := range rows {
			pw, err := decryptCredentialPassword(row.Password)
			if err != nil {
				return err
			}
			opts, err := decryptCredentialOptions(row.Options)
			if err != nil {
				return err
			}
			username := ""
			if row.Username.Valid {
				username = row.Username.String
			}
			byResourceID[row.ResourceID] = &domain.ResourceCredential{
				Base: domain.Base{
					ID:        row.ID,
					CreatedAt: row.CreatedAt,
					UpdatedAt: row.UpdatedAt,
				},
				ResourceID: row.ResourceID,
				Username:   username,
				Password:   pw,
				Options:    opts,
			}
		}
	default:
		return r.unconfigured()
	}
	for _, res := range resources {
		if c, ok := byResourceID[res.ID]; ok {
			res.Credential = c
		}
	}
	return nil
}

// resourcesInOrder reorders `loaded` to match the order of `ids`. Used after
// FindResourcesByIDs (which loses ORDER BY) to restore the order produced by
// the upstream FindResourceIDs* query.
func resourcesInOrder(loaded []*domain.Resource, ids []string) []*domain.Resource {
	byID := make(map[string]*domain.Resource, len(loaded))
	for _, r := range loaded {
		byID[r.ID] = r
	}
	out := make([]*domain.Resource, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			out = append(out, r)
		}
	}
	return out
}

// ---------- domain.ResourceMetaData glue ----------

func metaSSLExpiration(r *domain.Resource) *time.Time {
	if r.Metadata == nil {
		return nil
	}
	return r.Metadata.SSLExpirationDate
}

func metaSSLIssuerPtr(r *domain.Resource) *string {
	if r.Metadata == nil || r.Metadata.SSLIssuer == "" {
		return nil
	}
	s := r.Metadata.SSLIssuer
	return &s
}

func metaDomainExpiration(r *domain.Resource) *time.Time {
	if r.Metadata == nil {
		return nil
	}
	return r.Metadata.DomainExpirationDate
}

func metaDomainRegistrarPtr(r *domain.Resource) *string {
	if r.Metadata == nil || r.Metadata.DomainRegistrar == "" {
		return nil
	}
	s := r.Metadata.DomainRegistrar
	return &s
}

func metaFromPG(row pgsqlc.Resource) *domain.ResourceMetaData {
	md := &domain.ResourceMetaData{}
	any := false
	if row.SslExpirationDate.Valid {
		t := row.SslExpirationDate.Time
		md.SSLExpirationDate = &t
		any = true
	}
	if row.SslIssuer.Valid {
		md.SSLIssuer = row.SslIssuer.String
		any = true
	}
	if row.DomainExpirationDate.Valid {
		t := row.DomainExpirationDate.Time
		md.DomainExpirationDate = &t
		any = true
	}
	if row.DomainRegistrar.Valid {
		md.DomainRegistrar = row.DomainRegistrar.String
		any = true
	}
	if !any {
		return nil
	}
	return md
}

func metaFromSQLite(row sqlitesqlc.Resource) *domain.ResourceMetaData {
	md := &domain.ResourceMetaData{}
	any := false
	if row.SslExpirationDate.Valid {
		t := row.SslExpirationDate.Time
		md.SSLExpirationDate = &t
		any = true
	}
	if row.SslIssuer.Valid {
		md.SSLIssuer = row.SslIssuer.String
		any = true
	}
	if row.DomainExpirationDate.Valid {
		t := row.DomainExpirationDate.Time
		md.DomainExpirationDate = &t
		any = true
	}
	if row.DomainRegistrar.Valid {
		md.DomainRegistrar = row.DomainRegistrar.String
		any = true
	}
	if !any {
		return nil
	}
	return md
}

// ---------- shared scalar helpers (local — could move to a helpers file later) ----------

func pgPlainTimestampFromPtr(p *time.Time) pgtype.Timestamp {
	if p == nil {
		return pgtype.Timestamp{}
	}
	return pgtype.Timestamp{Time: *p, Valid: true}
}

func nullInt64FromPtrInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func ptrTimeFromPGTimestamptz(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func ptrTimeFromPGTimestamp(t pgtype.Timestamp) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func ptrTimeFromNullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func ptrStringFromPGText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func ptrStringFromNullString(t sql.NullString) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func ptrIntFromPGInt4(t pgtype.Int4) *int {
	if !t.Valid {
		return nil
	}
	v := int(t.Int32)
	return &v
}

func ptrIntFromNullInt64(t sql.NullInt64) *int {
	if !t.Valid {
		return nil
	}
	v := int(t.Int64)
	return &v
}

// ListResourcesByFilter implements dynamic filtering for the v1 monitors
// list endpoint (spec 051). Builds the ID + COUNT queries via squirrel,
// executes via the raw pool/db handle, then reuses FindResourcesByIDs +
// attachPreloads to materialise full domain rows with preloads.
func (r *ResourceRepositorySQLC) ListResourcesByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error) {
	var ph sq.PlaceholderFormat
	switch {
	case r.pgQ != nil:
		ph = sq.Dollar
	case r.sqliteQ != nil:
		ph = sq.Question
	default:
		return nil, 0, r.unconfigured()
	}

	idSQL, idArgs, err := dynquery.BuildMonitorIDsQuery(f, page, perPage, ph)
	if err != nil {
		return nil, 0, fmt.Errorf("dynquery: build monitor ids: %w", err)
	}
	countSQL, countArgs, err := dynquery.BuildMonitorCountQuery(f, ph)
	if err != nil {
		return nil, 0, fmt.Errorf("dynquery: build monitor count: %w", err)
	}

	var ids []string
	var total int64

	switch {
	case r.pgQ != nil:
		rows, err := r.pgPool.Query(ctx, idSQL, idArgs...)
		if err != nil {
			return nil, 0, fmt.Errorf("sqlc: list monitors by filter (ids): %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, 0, fmt.Errorf("sqlc: scan monitor id: %w", err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := r.pgPool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("sqlc: list monitors by filter (count): %w", err)
		}
		if len(ids) == 0 {
			return []*domain.Resource{}, int(total), nil
		}
		rrows, err := r.pgQ.FindResourcesByIDs(ctx, ids)
		if err != nil {
			return nil, 0, fmt.Errorf("sqlc: find resources by ids: %w", err)
		}
		out := resourcesInOrder(resourcesFromPG(rrows), ids)
		if err := r.attachTagsOnly(ctx, out); err != nil {
			return nil, 0, err
		}
		if err := r.attachIncidentCounts(ctx, out); err != nil {
			return nil, 0, err
		}
		if err := r.attachUptimeStats(ctx, out); err != nil {
			return nil, 0, err
		}
		return out, int(total), nil
	case r.sqliteQ != nil:
		rows, err := r.sqliteDB.QueryContext(ctx, idSQL, idArgs...)
		if err != nil {
			return nil, 0, fmt.Errorf("sqlc: list monitors by filter (ids): %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, 0, fmt.Errorf("sqlc: scan monitor id: %w", err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := r.sqliteDB.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("sqlc: list monitors by filter (count): %w", err)
		}
		if len(ids) == 0 {
			return []*domain.Resource{}, int(total), nil
		}
		rrows, err := r.sqliteQ.FindResourcesByIDs(ctx, ids)
		if err != nil {
			return nil, 0, fmt.Errorf("sqlc: find resources by ids: %w", err)
		}
		out := resourcesInOrder(resourcesFromSQLite(rrows), ids)
		if err := r.attachTagsOnly(ctx, out); err != nil {
			return nil, 0, err
		}
		if err := r.attachIncidentCounts(ctx, out); err != nil {
			return nil, 0, err
		}
		if err := r.attachUptimeStats(ctx, out); err != nil {
			return nil, 0, err
		}
		return out, int(total), nil
	default:
		return nil, 0, r.unconfigured()
	}
}
