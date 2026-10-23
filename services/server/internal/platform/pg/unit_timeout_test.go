// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestUnitOfWork_StatementTimeoutOption covers txn.StatementTimeout (M3-14 design note 11): the
// outermost unit sets the given statement_timeout instead of database.statement_timeout, a
// statement beyond it fails with an error IsStatementTimeout recognises, and a cancellation by
// the caller is not a statement timeout.
func TestUnitOfWork_StatementTimeoutOption(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	ctx := context.Background()

	show := func(opts ...txn.Option) string {
		t.Helper()
		var v string
		if err := uow.Do(ctx, func(ctx context.Context) error {
			return pg.Querier(ctx, nil).QueryRow(ctx, "SHOW statement_timeout").Scan(&v)
		}, opts...); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := show(txn.StatementTimeout(1234 * time.Millisecond)); got != "1234ms" {
		t.Fatalf("statement_timeout with the option = %q, want 1234ms", got)
	}
	if got, def := show(), e.cfg.StatementTimeout; got == "1234ms" || def == 1234*time.Millisecond {
		t.Fatalf("statement_timeout without the option = %q, want database.statement_timeout %s", got, def)
	}
	// A nested unit joins the ambient transaction; its option does not change the outer deadline.
	var nested string
	if err := uow.Do(ctx, func(ctx context.Context) error {
		return uow.Do(ctx, func(ctx context.Context) error {
			return pg.Querier(ctx, nil).QueryRow(ctx, "SHOW statement_timeout").Scan(&nested)
		}, txn.StatementTimeout(time.Second))
	}, txn.StatementTimeout(1234*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if nested != "1234ms" {
		t.Fatalf("statement_timeout in a nested unit = %q, want the outer 1234ms", nested)
	}

	// pg_sleep far beyond the deadline: the server cancels it, whatever the load of the host.
	err := uow.Do(ctx, func(ctx context.Context) error {
		_, err := pg.Querier(ctx, nil).Exec(ctx, "SELECT pg_sleep(600)")
		return err
	}, txn.StatementTimeout(50*time.Millisecond))
	if err == nil || !pg.IsStatementTimeout(ctx, err) {
		t.Fatalf("err = %v, want a statement timeout", err)
	}
	if pg.IsStatementTimeout(ctx, errors.New("other")) || pg.IsStatementTimeout(ctx, nil) {
		t.Fatal("IsStatementTimeout reported another error")
	}

	// A caller that cancels while the statement runs: PostgreSQL answers 57014 as well, but the
	// cause is the caller, not the deadline.
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- uow.Do(cctx, func(ctx context.Context) error {
			var pid int
			if err := pg.Querier(ctx, nil).QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
				return err
			}
			close(started)
			_, err := pg.Querier(ctx, nil).Exec(ctx, "SELECT pg_sleep(600)")
			return err
		}, txn.StatementTimeout(10*time.Minute))
	}()
	testkit.Receive(t, "the unit started", started)
	testkit.Until(t, "the sleep runs", func() bool {
		var n int
		if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE query = 'SELECT pg_sleep(600)' AND state = 'active'").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	})
	cancel()
	err = testkit.Receive(t, "the unit ended", done)
	if err == nil || pg.IsStatementTimeout(cctx, err) {
		t.Fatalf("err = %v after a cancellation, want an error that is no statement timeout", err)
	}
}
