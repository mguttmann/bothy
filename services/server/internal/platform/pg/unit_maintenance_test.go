// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// readOnlyUoW returns a unit of work of e in maintenance mode with retry after d.
func (e *unitEnv) readOnlyUoW(d time.Duration) *pg.UnitOfWork {
	cfg := e.cfg
	cfg.ReadOnly, cfg.ReadOnlyRetryAfter = true, d
	reg := txn.NewRegistry()
	reg.Seal()
	return pg.NewUnitOfWork(e.pool, cfg, reg)
}

// wantMaintenance fails t unless err is 503 maintenance with Retry-After d.
func wantMaintenance(t *testing.T, err error, d time.Duration) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != errs.CodeMaintenance || e.RetryAfter != d {
		t.Fatalf("error %v, want maintenance with Retry-After %v", err, d)
	}
}

// TestUnitOfWork_ReadOnlyInMaintenance: in maintenance mode (ADR 0065 item 5) a unit without the
// exemption runs READ ONLY; its write fails at PostgreSQL with 25006 and comes back as 503
// maintenance with the configured Retry-After, also when the use case swallowed the error of the
// statement or mapped it to another error; reads still work, and nothing is written.
func TestUnitOfWork_ReadOnlyInMaintenance(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.readOnlyUoW(7 * time.Second)
	ctx := context.Background()

	var n int
	if err := uow.Do(ctx, func(ctx context.Context) error {
		return pg.Querier(ctx, nil).QueryRow(ctx, "SELECT count(*) FROM items").Scan(&n)
	}); err != nil || n != 2 {
		t.Fatalf("read in maintenance mode: %d rows, %v", n, err)
	}

	cases := map[string]func(ctx context.Context) error{
		"returned": func(ctx context.Context) error { return insert(ctx, 3, "c") },
		"swallowed": func(ctx context.Context) error {
			_ = insert(ctx, 3, "c")
			return nil
		},
		"mapped": func(ctx context.Context) error {
			if insert(ctx, 3, "c") != nil {
				return errs.New(errs.CodeInternalError, "mapped by a repository")
			}
			return nil
		},
		"returning row": func(ctx context.Context) error {
			tx, err := pg.Tx(ctx)
			if err != nil {
				return err
			}
			var id int
			if tx.QueryRow(ctx, "INSERT INTO items VALUES (3, 'c') RETURNING id").Scan(&id) != nil {
				return errors.New("swallowed by the caller")
			}
			return nil
		},
		"rows": func(ctx context.Context) error {
			tx, err := pg.Tx(ctx)
			if err != nil {
				return err
			}
			rows, err := tx.Query(ctx, "UPDATE items SET v = 'z' RETURNING id")
			if err != nil {
				return nil //nolint:nilerr // the test swallows on purpose
			}
			rows.Close()
			return nil
		},
	}
	for name, fn := range cases {
		wantMaintenance(t, uow.Do(ctx, fn), 7*time.Second)
		if e.rowExists(t, 3) {
			t.Fatalf("%s: the write of a read-only unit committed", name)
		}
	}
	var v string
	if err := pg.Querier(ctx, e.pool).QueryRow(ctx, "SELECT v FROM items WHERE id = 1").Scan(&v); err != nil || v != "a" {
		t.Fatalf("row 1 is %q (%v) after the units, want a", v, err)
	}
}

// TestUnitOfWork_ExemptWritesInMaintenance: a unit whose context carries
// txn.WithWriteInMaintenance writes as usual in maintenance mode (the exceptions of ADR 0065
// item 4), a nested unit joins its transaction, and outside maintenance mode every unit writes.
func TestUnitOfWork_ExemptWritesInMaintenance(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	ctx := context.Background()
	uow := e.readOnlyUoW(time.Second)
	if err := uow.Do(txn.WithWriteInMaintenance(ctx), func(ctx context.Context) error {
		if err := insert(ctx, 3, "c"); err != nil {
			return err
		}
		return uow.Do(ctx, func(ctx context.Context) error { return insert(ctx, 4, "d") })
	}); err != nil {
		t.Fatalf("exempt unit: %v", err)
	}
	if !e.rowExists(t, 3) || !e.rowExists(t, 4) {
		t.Fatal("the rows of the exempt unit are missing")
	}
	if err := e.uow(txn.NewRegistry()).Do(ctx, func(ctx context.Context) error { return insert(ctx, 5, "e") }); err != nil {
		t.Fatalf("unit outside maintenance mode: %v", err)
	}
	if !txn.WritesInMaintenance(txn.Detach(txn.WithWriteInMaintenance(ctx))) {
		t.Fatal("Detach dropped the exemption")
	}
}
