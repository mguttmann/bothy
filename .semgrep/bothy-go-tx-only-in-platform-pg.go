// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-tx-only-in-platform-pg (run by `make semgrep`).
package fixtures

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

type unitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

func transactions(ctx context.Context, pool beginner, conn *pgx.Conn, uow unitOfWork) {
	// ruleid: bothy-go-tx-only-in-platform-pg
	_, _ = pool.Begin(ctx)
	// ruleid: bothy-go-tx-only-in-platform-pg
	_, _ = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	// ruleid: bothy-go-tx-only-in-platform-pg
	_ = pgx.BeginFunc(ctx, conn, func(pgx.Tx) error { return nil })
	// ruleid: bothy-go-tx-only-in-platform-pg
	_ = pgx.BeginTxFunc(ctx, conn, pgx.TxOptions{}, func(pgx.Tx) error { return nil })
	// ruleid: bothy-go-tx-only-in-platform-pg
	tx, _ := pool.Begin(ctx)
	// ruleid: bothy-go-tx-only-in-platform-pg
	_ = tx.Commit(ctx)
	// ruleid: bothy-go-tx-only-in-platform-pg
	_ = tx.Rollback(ctx)
	// ruleid: bothy-go-tx-only-in-platform-pg
	unit, _ := txn.NewUnit(nil, txn.Options{})
	// ruleid: bothy-go-tx-only-in-platform-pg
	ctx = txn.WithUnit(ctx, unit)
	// ok: bothy-go-tx-only-in-platform-pg
	_ = uow.Do(ctx, func(context.Context) error { return nil })
	// ok: bothy-go-tx-only-in-platform-pg
	_, _ = conn.Exec(ctx, "SELECT 1")
}
