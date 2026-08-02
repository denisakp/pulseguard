// Package internaltest — query-counting DBTX decorators.
//
// Wraps the sqlc-generated DBTX interfaces so tests can assert the exact
// number of round-trips a read path takes (controlled-N+1 = 1+R verification
// per spec 048 §FR-006). Counter increments per DBTX call, not per row.
package internaltest

import (
	"context"
	"database/sql"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
)

// Counter is a thread-safe DBTX-call counter shared between the wrapper and
// the test code. Use Snapshot to read; Reset to zero between subtests.
type Counter struct {
	n atomic.Int64
}

// Snapshot returns the current count atomically.
func (c *Counter) Snapshot() int64 { return c.n.Load() }

// Reset zeroes the counter atomically.
func (c *Counter) Reset() { c.n.Store(0) }

// ---- Postgres ----

// PGQueryCounter wraps a pgsqlc.DBTX and increments a Counter on each call.
// Implements pgsqlc.DBTX so it can be passed to pgsqlc.New(...).
type PGQueryCounter struct {
	inner pgsqlc.DBTX
	c     *Counter
}

// NewPGCounter returns a fresh Counter plus a DBTX that wraps the given pool.
// The returned DBTX can be passed to pgsqlc.New(dbtx) directly.
func NewPGCounter(pool *pgxpool.Pool) (*Counter, pgsqlc.DBTX) {
	c := &Counter{}
	return c, &PGQueryCounter{inner: pool, c: c}
}

func (w *PGQueryCounter) Exec(ctx context.Context, sqlStr string, args ...interface{}) (pgconn.CommandTag, error) {
	w.c.n.Add(1)
	return w.inner.Exec(ctx, sqlStr, args...)
}

func (w *PGQueryCounter) Query(ctx context.Context, sqlStr string, args ...interface{}) (pgx.Rows, error) {
	w.c.n.Add(1)
	return w.inner.Query(ctx, sqlStr, args...)
}

func (w *PGQueryCounter) QueryRow(ctx context.Context, sqlStr string, args ...interface{}) pgx.Row {
	w.c.n.Add(1)
	return w.inner.QueryRow(ctx, sqlStr, args...)
}

// ---- SQLite ----

// SQLiteQueryCounter wraps a sqlitesqlc.DBTX and increments a Counter on each
// call. Implements sqlitesqlc.DBTX.
type SQLiteQueryCounter struct {
	inner sqlitesqlc.DBTX
	c     *Counter
}

// NewSQLiteCounter returns a fresh Counter plus a DBTX that wraps the given
// *sql.DB. The returned DBTX can be passed to sqlitesqlc.New(dbtx) directly.
func NewSQLiteCounter(db *sql.DB) (*Counter, sqlitesqlc.DBTX) {
	c := &Counter{}
	return c, &SQLiteQueryCounter{inner: db, c: c}
}

func (w *SQLiteQueryCounter) ExecContext(ctx context.Context, sqlStr string, args ...interface{}) (sql.Result, error) {
	w.c.n.Add(1)
	return w.inner.ExecContext(ctx, sqlStr, args...)
}

func (w *SQLiteQueryCounter) PrepareContext(ctx context.Context, sqlStr string) (*sql.Stmt, error) {
	w.c.n.Add(1)
	return w.inner.PrepareContext(ctx, sqlStr)
}

func (w *SQLiteQueryCounter) QueryContext(ctx context.Context, sqlStr string, args ...interface{}) (*sql.Rows, error) {
	w.c.n.Add(1)
	return w.inner.QueryContext(ctx, sqlStr, args...)
}

func (w *SQLiteQueryCounter) QueryRowContext(ctx context.Context, sqlStr string, args ...interface{}) *sql.Row {
	w.c.n.Add(1)
	return w.inner.QueryRowContext(ctx, sqlStr, args...)
}
