// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is what generated sqlc code (sql_package pgx/v5) runs its statements on: a transaction
// or the pool. Repositories get it from Querier, so a read outside a unit of work uses the pool
// and every statement inside one uses the ambient transaction.
type DBTX interface {
	// Exec runs a statement without result rows.
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	// Query runs a statement and returns its rows; the caller closes them.
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	// QueryRow runs a statement that returns at most one row; its error appears at Scan.
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Compile-time checks: both sides of Querier are a DBTX.
var (
	_ DBTX = (*Pool)(nil)
	_ DBTX = pgx.Tx(nil)
)

// Exec runs sql on a connection of the pool, waiting at most database.acquire_timeout for it;
// the connection goes back right after. Errors: those of Acquire, then the statement's.
func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c, err := p.Acquire(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer c.Release()
	return c.Exec(ctx, sql, args...)
}

// Query runs sql on a connection of the pool, waiting at most database.acquire_timeout for it.
// The connection goes back when the rows are closed or read to the end, so the caller must
// close them. Errors: those of Acquire, then the statement's.
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c, err := p.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := c.Query(ctx, sql, args...)
	if err != nil {
		if rows != nil {
			rows.Close()
		}
		c.Release()
		return nil, err
	}
	return &poolRows{Rows: rows, conn: c}, nil
}

// QueryRow runs sql on a connection of the pool, waiting at most database.acquire_timeout for
// it. The connection goes back at Scan, so the caller must call Scan; an error of Acquire also
// appears there.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c, err := p.Acquire(ctx)
	if err != nil {
		return errRow{err: err}
	}
	return &poolRow{row: c.QueryRow(ctx, sql, args...), conn: c}
}

// poolRows releases its connection when the rows end.
type poolRows struct {
	pgx.Rows
	conn *pgxpool.Conn
}

// Next advances to the next row and releases the connection after the last one.
func (r *poolRows) Next() bool {
	if r.Rows.Next() {
		return true
	}
	r.Close()
	return false
}

// Close closes the rows and releases the connection; it may be called more than once.
func (r *poolRows) Close() {
	r.Rows.Close()
	if r.conn != nil {
		r.conn.Release()
		r.conn = nil
	}
}

// poolRow releases its connection at Scan.
type poolRow struct {
	row  pgx.Row
	conn *pgxpool.Conn
}

// Scan reads the row into dest and releases the connection.
func (r *poolRow) Scan(dest ...any) error {
	defer r.conn.Release()
	return r.row.Scan(dest...)
}

// errRow is the row of a failed Acquire.
type errRow struct{ err error }

// Scan returns the error of Acquire.
func (r errRow) Scan(...any) error { return r.err }
