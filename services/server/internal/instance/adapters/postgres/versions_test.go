// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestVersionsReadsPermissionVersion: instance.Versions reads the committed rights version
// without a phase and without a lock (M1-28), on the pool and in an ambient unit of work.
func TestVersionsReadsPermissionVersion(t *testing.T) {
	e := newEnv(t, 2)
	ctx := context.Background()
	if _, err := e.db.SuperConn(t).Exec(ctx, "UPDATE sequence_counters SET value = 42 WHERE name = 'permission_version'"); err != nil {
		t.Fatal(err)
	}
	v := app.NewVersions(postgres.NewRepository(e.pool))
	got, err := v.PermissionVersion(ctx)
	if err != nil || got != 42 {
		t.Fatalf("PermissionVersion = %d, %v; want 42", got, err)
	}
	uow := e.uow(txn.NewRegistry())
	err = uow.Do(ctx, func(ctx context.Context) error {
		n, err := v.PermissionVersion(ctx)
		if err == nil && n != 42 {
			t.Errorf("inside a unit: %d", n)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestVersionsReadsEventSeq: instance.Versions reads the committed counter event_seq without a
// phase and without a lock (M1-63, the head of catch-up and snapshot): Counters would refuse the
// call outside the phase hooks (M1-15).
func TestVersionsReadsEventSeq(t *testing.T) {
	e := newEnv(t, 2)
	ctx := context.Background()
	if _, err := e.db.SuperConn(t).Exec(ctx, "UPDATE sequence_counters SET value = 77 WHERE name = 'event_seq'"); err != nil {
		t.Fatal(err)
	}
	got, err := app.NewVersions(postgres.NewRepository(e.pool)).EventSeq(ctx)
	if err != nil || got != 77 {
		t.Fatalf("EventSeq = %d, %v; want 77", got, err)
	}
}
