// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestUnitOfWork_CancelDuringCommitStillCommits: once COMMIT is sent, the end of the caller's
// context no longer aborts it (like the rollback, it runs without the cancellation, bounded by
// database.statement_timeout), so the unit reports the outcome of the COMMIT and its AfterCommit
// functions run. A deferred constraint trigger keeps the COMMIT busy while the context ends.
func TestUnitOfWork_CancelDuringCommitStillCommits(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	for _, q := range []string{
		"CREATE FUNCTION slow_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.6); RETURN NULL; END $$",
		"CREATE CONSTRAINT TRIGGER slow_commit AFTER INSERT ON items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION slow_commit()",
	} {
		if _, err := e.db.SuperConn(t).Exec(context.Background(), q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	uow := e.uow(txn.NewRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var after atomic.Bool
	err := uow.Do(ctx, func(ctx context.Context) error {
		if err := insert(ctx, 61, "x"); err != nil {
			return err
		}
		txn.AfterCommit(ctx, func() { after.Store(true) })
		time.AfterFunc(200*time.Millisecond, cancel)
		return nil
	})
	if err != nil {
		t.Fatalf("Do = %v, want the committed unit", err)
	}
	if !e.rowExists(t, 61) || !after.Load() {
		t.Fatalf("row committed %v, AfterCommit ran %v, want both", e.rowExists(t, 61), after.Load())
	}
}
