package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sq "github.com/Masterminds/squirrel"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

// IncidentRepositorySQLC implements port.IncidentRepository via sqlc.
//
// US2 of spec 048. Read paths attach Resource + IncidentDiagnostics via
// controlled-N+1 (1 + R round-trips, here R = up to 2). GetIncidentStats is
// dialect-divergent: CTE one-pass on PG, two correlated sub-queries on SQLite.
type IncidentRepositorySQLC struct {
	pgQ      *pgsqlc.Queries
	sqliteQ  *sqlitesqlc.Queries
	pgPool   *pgxpool.Pool
	sqliteDB *sql.DB
}

func NewIncidentRepositorySQLC(rt SqlcRuntime) port.IncidentRepository {
	r := &IncidentRepositorySQLC{}
	if pool := rt.PgxPool(); pool != nil {
		r.pgPool = pool
		r.pgQ = pgsqlc.New(pool)
	} else if db := rt.SQLiteDB(); db != nil {
		r.sqliteDB = db
		r.sqliteQ = sqlitesqlc.New(db)
	}
	return r
}

func (r *IncidentRepositorySQLC) unconfigured() error {
	return fmt.Errorf("incident_repository_sqlc: unconfigured runtime")
}

// ---------- Public surface (port.IncidentRepository) ----------

func (r *IncidentRepositorySQLC) Create(ctx context.Context, inc *domain.Incident) (*domain.Incident, error) {
	if inc.ID == "" {
		inc.EnsureID()
	}
	now := time.Now()
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = now
	}
	if inc.UpdatedAt.IsZero() {
		inc.UpdatedAt = now
	}
	if inc.StartedAt.IsZero() {
		inc.StartedAt = now
	}
	if inc.Cause == "" {
		inc.Cause = "unknown_failure"
	}
	switch {
	case r.pgQ != nil:
		if err := r.pgQ.CreateIncident(ctx, pgsqlc.CreateIncidentParams{
			ID:         inc.ID,
			CreatedAt:  pgtype.Timestamptz{Time: inc.CreatedAt, Valid: true},
			UpdatedAt:  pgtype.Timestamptz{Time: inc.UpdatedAt, Valid: true},
			ResourceID: inc.ResourceID,
			Cause:      inc.Cause,
			ResolvedAt: pgTimestampFromPtr(inc.ResolvedAt),
			StartedAt:  pgtype.Timestamptz{Time: inc.StartedAt, Valid: true},
			Details:    inc.Details,
		}); err != nil {
			return nil, fmt.Errorf("sqlc: create incident: %w", err)
		}
		return inc, nil
	case r.sqliteQ != nil:
		if err := r.sqliteQ.CreateIncident(ctx, sqlitesqlc.CreateIncidentParams{
			ID:         inc.ID,
			CreatedAt:  inc.CreatedAt,
			UpdatedAt:  inc.UpdatedAt,
			ResourceID: inc.ResourceID,
			Cause:      inc.Cause,
			ResolvedAt: nullTimeFromPtr(inc.ResolvedAt),
			StartedAt:  inc.StartedAt,
			Details:    inc.Details,
		}); err != nil {
			return nil, fmt.Errorf("sqlc: create incident: %w", err)
		}
		return inc, nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) FindByID(ctx context.Context, id string) (*domain.Incident, error) {
	var out *domain.Incident
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindIncidentByID(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find incident by id: %w", err)
		}
		out = incidentFromPG(row)
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindIncidentByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find incident by id: %w", err)
		}
		out = incidentFromSQLite(row)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, []*domain.Incident{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *IncidentRepositorySQLC) List(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	var out []*domain.Incident
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListIncidents(ctx, pgsqlc.ListIncidentsParams{
			Limit: int32(limit), Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list incidents: %w", err)
		}
		out = incidentsFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListIncidents(ctx, sqlitesqlc.ListIncidentsParams{
			Limit: int64(limit), Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: list incidents: %w", err)
		}
		out = incidentsFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *IncidentRepositorySQLC) Update(ctx context.Context, inc *domain.Incident) error {
	inc.UpdatedAt = time.Now()
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.UpdateIncident(ctx, pgsqlc.UpdateIncidentParams{
			ID:         inc.ID,
			ResourceID: inc.ResourceID,
			Cause:      inc.Cause,
			ResolvedAt: pgTimestampFromPtr(inc.ResolvedAt),
			StartedAt:  pgtype.Timestamptz{Time: inc.StartedAt, Valid: true},
			Details:    inc.Details,
			UpdatedAt:  pgtype.Timestamptz{Time: inc.UpdatedAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("sqlc: update incident: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.UpdateIncident(ctx, sqlitesqlc.UpdateIncidentParams{
			ID:         inc.ID,
			ResourceID: inc.ResourceID,
			Cause:      inc.Cause,
			ResolvedAt: nullTimeFromPtr(inc.ResolvedAt),
			StartedAt:  inc.StartedAt,
			Details:    inc.Details,
			UpdatedAt:  inc.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("sqlc: update incident: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) Delete(ctx context.Context, id string) error {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.DeleteIncident(ctx, id)
		if err != nil {
			return fmt.Errorf("sqlc: delete incident: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.DeleteIncident(ctx, id)
		if err != nil {
			return fmt.Errorf("sqlc: delete incident: %w", err)
		}
		if n == 0 {
			return repository.ErrNotFound
		}
		return nil
	default:
		return r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) FindUnresolved(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	var out []*domain.Incident
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.FindUnresolvedIncidents(ctx, pgsqlc.FindUnresolvedIncidentsParams{
			Limit: int32(limit), Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find unresolved incidents: %w", err)
		}
		out = incidentsFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.FindUnresolvedIncidents(ctx, sqlitesqlc.FindUnresolvedIncidentsParams{
			Limit: int64(limit), Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find unresolved incidents: %w", err)
		}
		out = incidentsFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *IncidentRepositorySQLC) FindByResource(ctx context.Context, resourceID string, limit, offset int) ([]*domain.Incident, error) {
	var out []*domain.Incident
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.FindIncidentsByResource(ctx, pgsqlc.FindIncidentsByResourceParams{
			ResourceID: resourceID, Limit: int32(limit), Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find incidents by resource: %w", err)
		}
		out = incidentsFromPG(rows)
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.FindIncidentsByResource(ctx, sqlitesqlc.FindIncidentsByResourceParams{
			ResourceID: resourceID, Limit: int64(limit), Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("sqlc: find incidents by resource: %w", err)
		}
		out = incidentsFromSQLite(rows)
	default:
		return nil, r.unconfigured()
	}
	if err := r.attachPreloads(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *IncidentRepositorySQLC) GetIncidentStats(ctx context.Context, hours int) (int, int, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.GetIncidentStatsPG(ctx, pgtype.Timestamptz{Time: since, Valid: true})
		if err != nil {
			return 0, 0, fmt.Errorf("sqlc: get incident stats: %w", err)
		}
		return int(row.TotalIncidents), int(row.AffectedMonitors), nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.GetIncidentStatsSQLite(ctx, since)
		if err != nil {
			return 0, 0, fmt.Errorf("sqlc: get incident stats: %w", err)
		}
		return int(row.TotalIncidents), int(row.AffectedMonitors), nil
	default:
		return 0, 0, r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) FindActiveByResourceID(ctx context.Context, resourceID string) (*domain.Incident, error) {
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindActiveIncidentByResourceID(ctx, resourceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find active incident by resource: %w", err)
		}
		return incidentFromPG(row), nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindActiveIncidentByResourceID(ctx, resourceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find active incident by resource: %w", err)
		}
		return incidentFromSQLite(row), nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) HasActiveIncident(ctx context.Context) (bool, error) {
	switch {
	case r.pgQ != nil:
		v, err := r.pgQ.HasActiveIncident(ctx)
		if err != nil {
			return false, fmt.Errorf("sqlc: has active incident: %w", err)
		}
		return v, nil
	case r.sqliteQ != nil:
		// sqlc v1.30 generates int64 (not bool) for SQLite EXISTS(...) — coerce.
		v, err := r.sqliteQ.HasActiveIncident(ctx)
		if err != nil {
			return false, fmt.Errorf("sqlc: has active incident: %w", err)
		}
		return v != 0, nil
	default:
		return false, r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) FindLastResolved(ctx context.Context) (*domain.Incident, error) {
	switch {
	case r.pgQ != nil:
		row, err := r.pgQ.FindLastResolvedIncident(ctx)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find last resolved incident: %w", err)
		}
		return incidentFromPG(row), nil
	case r.sqliteQ != nil:
		row, err := r.sqliteQ.FindLastResolvedIncident(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, repository.ErrNotFound
			}
			return nil, fmt.Errorf("sqlc: find last resolved incident: %w", err)
		}
		return incidentFromSQLite(row), nil
	default:
		return nil, r.unconfigured()
	}
}

func (r *IncidentRepositorySQLC) CountByResourceID(ctx context.Context, resourceID string) (int64, error) {
	switch {
	case r.pgQ != nil:
		n, err := r.pgQ.CountIncidentsByResourceID(ctx, resourceID)
		if err != nil {
			return 0, fmt.Errorf("sqlc: count incidents by resource: %w", err)
		}
		return n, nil
	case r.sqliteQ != nil:
		n, err := r.sqliteQ.CountIncidentsByResourceID(ctx, resourceID)
		if err != nil {
			return 0, fmt.Errorf("sqlc: count incidents by resource: %w", err)
		}
		return n, nil
	default:
		return 0, r.unconfigured()
	}
}

// ---------- preloads (controlled N+1) ----------

func (r *IncidentRepositorySQLC) attachPreloads(ctx context.Context, incidents []*domain.Incident) error {
	if len(incidents) == 0 {
		return nil
	}
	if err := r.attachResources(ctx, incidents); err != nil {
		return err
	}
	if err := r.attachDiagnostics(ctx, incidents); err != nil {
		return err
	}
	return nil
}

func (r *IncidentRepositorySQLC) attachResources(ctx context.Context, incidents []*domain.Incident) error {
	// Build the set of unique resource IDs.
	idSet := make(map[string]struct{}, len(incidents))
	for _, inc := range incidents {
		if inc.ResourceID != "" {
			idSet[inc.ResourceID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	byID := make(map[string]*domain.Resource, len(ids))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.FindResourcesByIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload incident resources: %w", err)
		}
		for _, row := range rows {
			byID[row.ID] = resourceFromPG(row)
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.FindResourcesByIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload incident resources: %w", err)
		}
		for _, row := range rows {
			byID[row.ID] = resourceFromSQLite(row)
		}
	default:
		return r.unconfigured()
	}
	for _, inc := range incidents {
		if res, ok := byID[inc.ResourceID]; ok {
			inc.Resource = *res
		}
	}
	return nil
}

func (r *IncidentRepositorySQLC) attachDiagnostics(ctx context.Context, incidents []*domain.Incident) error {
	ids := make([]string, len(incidents))
	for i, inc := range incidents {
		ids[i] = inc.ID
	}
	byIncidentID := make(map[string]*domain.IncidentDiagnostics, len(incidents))
	switch {
	case r.pgQ != nil:
		rows, err := r.pgQ.ListIncidentDiagnosticsByIncidentIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload incident diagnostics: %w", err)
		}
		for _, row := range rows {
			d, err := incidentDiagnosticsFromPG(row)
			if err != nil {
				return fmt.Errorf("sqlc: map incident diagnostics: %w", err)
			}
			byIncidentID[row.IncidentID] = d
		}
	case r.sqliteQ != nil:
		rows, err := r.sqliteQ.ListIncidentDiagnosticsByIncidentIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("sqlc: preload incident diagnostics: %w", err)
		}
		for _, row := range rows {
			d, err := incidentDiagnosticsFromSQLite(row)
			if err != nil {
				return fmt.Errorf("sqlc: map incident diagnostics: %w", err)
			}
			byIncidentID[row.IncidentID] = d
		}
	default:
		return r.unconfigured()
	}
	for _, inc := range incidents {
		if d, ok := byIncidentID[inc.ID]; ok {
			inc.IncidentDiagnostics = d
		}
	}
	return nil
}

// ---------- mapping helpers ----------

func incidentFromPG(row pgsqlc.Incident) *domain.Incident {
	return &domain.Incident{
		Base: domain.Base{
			ID:        row.ID,
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		},
		ResourceID: row.ResourceID,
		Cause:      row.Cause,
		ResolvedAt: ptrTimeFromPGTimestamptz(row.ResolvedAt),
		StartedAt:  row.StartedAt.Time,
		Details:    row.Details,
	}
}

func incidentFromSQLite(row sqlitesqlc.Incident) *domain.Incident {
	return &domain.Incident{
		Base: domain.Base{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		},
		ResourceID: row.ResourceID,
		Cause:      row.Cause,
		ResolvedAt: ptrTimeFromNullTime(row.ResolvedAt),
		StartedAt:  row.StartedAt,
		Details:    row.Details,
	}
}

func incidentsFromPG(rows []pgsqlc.Incident) []*domain.Incident {
	out := make([]*domain.Incident, len(rows))
	for i, row := range rows {
		out[i] = incidentFromPG(row)
	}
	return out
}

func incidentsFromSQLite(rows []sqlitesqlc.Incident) []*domain.Incident {
	out := make([]*domain.Incident, len(rows))
	for i, row := range rows {
		out[i] = incidentFromSQLite(row)
	}
	return out
}

// ListIncidentsByFilter implements dynamic filtering for the v1 incidents
// list endpoint. Builds the SELECT + COUNT via squirrel, executes
// via the raw pool/db, and scans the typed sqlc row representations using
// existing mappers. Preloads (Resource, Diagnostics) are NOT attached — the
// v1 handler's mapIncidentResponse only needs incident columns.
func (r *IncidentRepositorySQLC) ListIncidentsByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error) {
	var ph sq.PlaceholderFormat
	switch {
	case r.pgQ != nil:
		ph = sq.Dollar
	case r.sqliteQ != nil:
		ph = sq.Question
	default:
		return nil, 0, r.unconfigured()
	}

	rowsSQL, rowsArgs, err := dynquery.BuildIncidentsQuery(f, page, perPage, ph)
	if err != nil {
		return nil, 0, fmt.Errorf("dynquery: build incidents: %w", err)
	}
	countSQL, countArgs, err := dynquery.BuildIncidentCountQuery(f, ph)
	if err != nil {
		return nil, 0, fmt.Errorf("dynquery: build incidents count: %w", err)
	}

	var total int64
	switch {
	case r.pgQ != nil:
		pgRows, err := r.pgPool.Query(ctx, rowsSQL, rowsArgs...)
		if err != nil {
			return nil, 0, fmt.Errorf("sqlc: list incidents by filter (rows): %w", err)
		}
		var typed []pgsqlc.Incident
		for pgRows.Next() {
			var row pgsqlc.Incident
			if err := pgRows.Scan(
				&row.ID, &row.ResourceID, &row.Cause, &row.ResolvedAt, &row.StartedAt,
				&row.Details, &row.CreatedAt, &row.UpdatedAt,
			); err != nil {
				pgRows.Close()
				return nil, 0, fmt.Errorf("sqlc: scan incident row: %w", err)
			}
			typed = append(typed, row)
		}
		pgRows.Close()
		if err := r.pgPool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("sqlc: list incidents by filter (count): %w", err)
		}
		out := incidentsFromPG(typed)
		if err := r.attachResources(ctx, out); err != nil {
			return nil, 0, err
		}
		return out, int(total), nil

	case r.sqliteQ != nil:
		sqlRows, err := r.sqliteDB.QueryContext(ctx, rowsSQL, rowsArgs...)
		if err != nil {
			return nil, 0, fmt.Errorf("sqlc: list incidents by filter (rows): %w", err)
		}
		var typed []sqlitesqlc.Incident
		for sqlRows.Next() {
			var row sqlitesqlc.Incident
			if err := sqlRows.Scan(
				&row.ID, &row.ResourceID, &row.Cause, &row.ResolvedAt, &row.StartedAt,
				&row.Details, &row.CreatedAt, &row.UpdatedAt,
			); err != nil {
				sqlRows.Close()
				return nil, 0, fmt.Errorf("sqlc: scan incident row: %w", err)
			}
			typed = append(typed, row)
		}
		sqlRows.Close()
		if err := r.sqliteDB.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("sqlc: list incidents by filter (count): %w", err)
		}
		out := incidentsFromSQLite(typed)
		if err := r.attachResources(ctx, out); err != nil {
			return nil, 0, err
		}
		return out, int(total), nil

	default:
		return nil, 0, r.unconfigured()
	}
}
