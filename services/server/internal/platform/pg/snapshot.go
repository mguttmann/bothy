// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ReadSnapshot runs fn in one READ ONLY transaction with REPEATABLE READ on p, so every statement
// of fn sees the same snapshot of the database, and ends the transaction. It writes nothing and
// runs no phase hook: it is for reads that must agree with each other, such as the counts of a
// backup manifest (brief M7-02 design note 3), never for a use case that writes (those run in
// the unit of work). Opening and ending transactions stays in this package (R-AR-06).
//
// The error is the one of Begin (the pool is unavailable) or of fn; the transaction is rolled
// back in every case, which ends a read-only snapshot like a commit would.
//
// Example:
//
//	err := pg.ReadSnapshot(ctx, pool, func(ctx context.Context, q pg.DBTX) error {
//		return q.QueryRow(ctx, "SELECT count(*) FROM files").Scan(&n)
//	})
func ReadSnapshot(ctx context.Context, p *Pool, fn func(ctx context.Context, q DBTX) error) error {
	tx, err := p.Begin(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	return fn(ctx, tx)
}
