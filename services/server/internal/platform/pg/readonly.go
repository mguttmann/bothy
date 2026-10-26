// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// sqlstateReadOnly is read_only_sql_transaction: a write in a READ ONLY transaction.
const sqlstateReadOnly = "25006"

// IsReadOnlyRejection reports whether err carries SQLSTATE 25006, the error PostgreSQL gives a
// write inside a READ ONLY transaction.
func IsReadOnlyRejection(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == sqlstateReadOnly
}

// readOnlyTx is the transaction of a unit of work in maintenance mode (UnitConfig.ReadOnly). It
// passes every call on and records whether PostgreSQL rejected a write with 25006, wherever the
// error surfaces (the call itself, a row, the rows or a batch), so the unit fails with 503
// maintenance even when a repository mapped the error or a use case swallowed it.
type readOnlyTx struct {
	pgx.Tx
	rejected *atomic.Bool
}

// newReadOnlyTx wraps tx; a nested transaction shares the record of its parent.
func newReadOnlyTx(tx pgx.Tx, rejected *atomic.Bool) *readOnlyTx {
	return &readOnlyTx{Tx: tx, rejected: rejected}
}

// note records err and returns it unchanged.
func (t *readOnlyTx) note(err error) error {
	if IsReadOnlyRejection(err) {
		t.rejected.Store(true)
	}
	return err
}

// Begin starts a savepoint whose statements are recorded too.
func (t *readOnlyTx) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := t.Tx.Begin(ctx)
	if err != nil {
		return nil, t.note(err)
	}
	return newReadOnlyTx(tx, t.rejected), nil
}

// Exec passes on and records the error.
func (t *readOnlyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := t.Tx.Exec(ctx, sql, args...)
	return tag, t.note(err)
}

// Query passes on and records the error of the call and of the rows.
func (t *readOnlyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := t.Tx.Query(ctx, sql, args...)
	if err != nil {
		return rows, t.note(err)
	}
	return &readOnlyRows{Rows: rows, tx: t}, nil
}

// QueryRow passes on and records the error of Scan.
func (t *readOnlyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return readOnlyRow{row: t.Tx.QueryRow(ctx, sql, args...), tx: t}
}

// CopyFrom passes on and records the error.
func (t *readOnlyTx) CopyFrom(ctx context.Context, table pgx.Identifier, columns []string, src pgx.CopyFromSource) (int64, error) {
	n, err := t.Tx.CopyFrom(ctx, table, columns, src)
	return n, t.note(err)
}

// SendBatch passes on and records the errors of the results.
func (t *readOnlyTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return readOnlyBatch{BatchResults: t.Tx.SendBatch(ctx, b), tx: t}
}

// readOnlyRows records the error of rows once they end.
type readOnlyRows struct {
	pgx.Rows
	tx *readOnlyTx
}

// Err passes on and records.
func (r *readOnlyRows) Err() error { return r.tx.note(r.Rows.Err()) }

// Close closes the rows and records their error.
func (r *readOnlyRows) Close() {
	r.Rows.Close()
	_ = r.tx.note(r.Rows.Err())
}

// readOnlyRow records the error of Scan.
type readOnlyRow struct {
	row pgx.Row
	tx  *readOnlyTx
}

// Scan passes on and records.
func (r readOnlyRow) Scan(dest ...any) error { return r.tx.note(r.row.Scan(dest...)) }

// readOnlyBatch records the errors of a batch.
type readOnlyBatch struct {
	pgx.BatchResults
	tx *readOnlyTx
}

// Exec passes on and records.
func (b readOnlyBatch) Exec() (pgconn.CommandTag, error) {
	tag, err := b.BatchResults.Exec()
	return tag, b.tx.note(err)
}

// Query passes on and records.
func (b readOnlyBatch) Query() (pgx.Rows, error) {
	rows, err := b.BatchResults.Query()
	if err != nil {
		return rows, b.tx.note(err)
	}
	return &readOnlyRows{Rows: rows, tx: b.tx}, nil
}

// QueryRow passes on and records.
func (b readOnlyBatch) QueryRow() pgx.Row {
	return readOnlyRow{row: b.BatchResults.QueryRow(), tx: b.tx}
}

// Close passes on and records.
func (b readOnlyBatch) Close() error { return b.tx.note(b.BatchResults.Close()) }
