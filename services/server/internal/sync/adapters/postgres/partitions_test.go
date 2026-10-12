// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	stdsync "sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// partition is one partition of event_log with its owner.
type partition struct {
	name  string
	owner string
}

// partitions lists the partitions of event_log in bound order.
func partitions(t *testing.T, e *env) []partition {
	t.Helper()
	rows, err := e.db.SuperConn(t).Query(context.Background(), `SELECT c.relname, pg_get_userbyid(c.relowner)
		FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'public.event_log'::regclass ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (partition, error) {
		var p partition
		err := r.Scan(&p.name, &p.owner)
		return p, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// names returns the partition names for the lower bounds.
func names(lowers ...int64) []string {
	out := make([]string, len(lowers))
	for i, l := range lowers {
		out[i] = domain.PartitionName(l)
	}
	return out
}

// partitionNames returns the names of the partitions of e.
func partitionNames(t *testing.T, e *env) []string {
	t.Helper()
	var out []string
	for _, p := range partitions(t, e) {
		out = append(out, p.name)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// smallPartitions is a configuration with 10 numbers per partition.
var smallPartitions = app.PartitionConfig{Size: 10, Ahead: 3, Retention: 24 * time.Hour, LockTimeout: 2 * time.Second}

func TestPartitions_CreatedAhead(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{partitions: smallPartitions})
	if got, want := partitionNames(t, e), names(1, 11, 21); !equal(got, want) {
		t.Fatalf("partitions of an empty log = %v, want %v (three partitions ahead)", got, want)
	}
	e.mustAppend(t, 3)
	e.mustAppend(t, 3)
	e.mustAppend(t, 3)
	e.mustAppend(t, 3) // head 12, inside [11, 21)
	res, err := e.maintenance(t, smallPartitions).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Head != 12 || res.Created != 2 || res.Dropped != 0 {
		t.Fatalf("result = %+v, want head 12 and two partitions created, so three stand above [11, 21)", res)
	}
	if got, want := partitionNames(t, e), names(1, 11, 21, 31, 41); !equal(got, want) {
		t.Fatalf("partitions = %v, want %v", got, want)
	}
}

func TestPartitions_Idempotent(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{partitions: smallPartitions, maxConns: 8})
	m := e.maintenance(t, smallPartitions)
	res, err := m.Run(context.Background())
	if err != nil || res.Created != 0 || res.Dropped != 0 {
		t.Fatalf("second run = %+v, %v; want nothing to do", res, err)
	}
	bigger := smallPartitions
	bigger.Ahead = 6
	var wg stdsync.WaitGroup
	results := make([]app.MaintenanceResult, 4)
	errs := make([]error, 4)
	for i := range 4 {
		wg.Go(func() { results[i], errs[i] = e.maintenance(t, bigger).Run(context.Background()) })
	}
	wg.Wait()
	created := 0
	for i := range 4 {
		if errs[i] != nil {
			t.Fatalf("concurrent run %d: %v", i, errs[i])
		}
		created += results[i].Created
	}
	if created != 3 {
		t.Fatalf("concurrent runs created %d partitions together, want 3", created)
	}
	if got, want := partitionNames(t, e), names(1, 11, 21, 31, 41, 51); !equal(got, want) {
		t.Fatalf("partitions = %v, want %v", got, want)
	}
}

func TestPartitions_RunAsAppRole(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{partitions: smallPartitions})
	var user string
	if err := e.pool.QueryRow(context.Background(), "SELECT current_user").Scan(&user); err != nil || user != string(pg.RoleApp) {
		t.Fatalf("the test pool runs as %q (%v), want bothy_app", user, err)
	}
	for _, p := range partitions(t, e) {
		if p.owner != string(pg.RoleOwner) {
			t.Errorf("partition %s belongs to %s, want bothy_owner", p.name, p.owner)
		}
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE"} {
			var ok bool
			if err := e.db.SuperConn(t).QueryRow(context.Background(),
				"SELECT has_table_privilege('bothy_app', $1, $2)", "public."+p.name, priv).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Errorf("bothy_app holds %s on partition %s; it reaches rows only through event_log", priv, p.name)
			}
		}
	}
	var create bool
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT has_schema_privilege('bothy_app', 'public', 'CREATE')").Scan(&create); err != nil || create {
		t.Fatalf("bothy_app may create in public (%v); DDL must go through the functions only", err)
	}
}

func TestPartitions_DetachedAfterRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{partitions: smallPartitions})
	m := e.maintenance(t, smallPartitions)
	for range 6 {
		e.mustAppend(t, 5)
	}
	if _, err := m.Run(ctx); err != nil { // head 30: partitions up to [51, 61)
		t.Fatal(err)
	}
	e.mustAppend(t, 5) // head 35, inside [31, 41)
	res, err := m.Run(ctx)
	if err != nil || res.Dropped != 0 {
		t.Fatalf("run inside the retention = %+v, %v; want nothing dropped", res, err)
	}
	e.clock.Set(time.Now().Add(smallPartitions.Retention + time.Hour))
	res, err = m.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Dropped != 3 {
		t.Fatalf("result = %+v, want the three partitions below the head dropped", res)
	}
	got := partitionNames(t, e)
	if got[0] != domain.PartitionName(31) {
		t.Fatalf("partitions = %v; the partition holding the head (31) must remain", got)
	}
	if minSeq, err := e.repo.MinRetainedSeq(ctx); err != nil || minSeq != 31 {
		t.Fatalf("MinRetainedSeq = %d, %v; want 31", minSeq, err)
	}
	if head, err := e.repo.Head(ctx); err != nil || head != 35 {
		t.Fatalf("Head = %d, %v; want 35", head, err)
	}
	e.clock.Set(time.Now().Add(100 * smallPartitions.Retention))
	if res, err := m.Run(ctx); err != nil || res.Dropped != 0 {
		t.Fatalf("run far after the retention = %+v, %v; the head partition is never dropped", res, err)
	}
	if head, _ := e.repo.Head(ctx); head != 35 {
		t.Fatalf("Head = %d after maintenance; it must stay the counter value", head)
	}
}

// TestPartitions_EnsureDoesNotBlockInserts holds the lock level of the README: while a
// transaction that attached a new partition is still open (it holds SHARE UPDATE EXCLUSIVE on
// event_log), units of work keep appending events.
func TestPartitions_EnsureDoesNotBlockInserts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{partitions: smallPartitions, maxConns: 6})
	owner := e.db.Pool(t, pg.RoleOwner)
	tx, err := owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var created int
	if err := tx.QueryRow(ctx, "SELECT sync_event_log_ensure_partitions(10, 5, 0)").Scan(&created); err != nil || created != 2 {
		t.Fatalf("ensure in the open transaction = %d, %v", created, err)
	}
	actx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := e.appendUnit(actx, testEvent("thread.during_attach")); err != nil {
		t.Fatalf("an append waited for the open attach: %v", err)
	}
}

// TestPartitions_BlockedDetachKeepsCreatedPartitions: a long reader (pg_dump, a long catch-up
// read) holds ACCESS SHARE on event_log while an expired partition waits for DETACH. The drop is
// deferred, but the partitions created ahead in the same run stay, so appends continue.
func TestPartitions_BlockedDetachKeepsCreatedPartitions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := smallPartitions
	cfg.LockTimeout = 200 * time.Millisecond
	e := newEnv(t, envOptions{partitions: cfg, maxConns: 6})
	m := e.maintenance(t, cfg)
	for range 5 {
		e.mustAppend(t, 5) // head 25
	}
	if _, err := m.Run(ctx); err != nil { // partitions up to [51, 61)
		t.Fatal(err)
	}
	for range 7 {
		e.mustAppend(t, 5) // head 60, the last partition is full
	}
	owner := e.db.Pool(t, pg.RoleOwner)
	tx, err := owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "LOCK TABLE event_log IN ACCESS SHARE MODE"); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(time.Now().Add(cfg.Retention + time.Hour))
	res, err := m.Run(ctx)
	if !errors.Is(err, app.ErrDropDeferred) || res.Created == 0 {
		t.Fatalf("maintenance under a long reader = %+v, %v; want partitions created and the drop deferred", res, err)
	}
	if _, err := e.appendUnit(ctx, testEvent("thread.after_maintenance")); err != nil {
		t.Fatalf("append after the deferred drop: %v", err)
	}
}
