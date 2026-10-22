// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
)

// TestBackupRunsRole holds brief M7-02 rule 16 and design note 3: bothy_backup can insert,
// update and read backup_runs and nothing else: it can neither delete a run nor read or write any
// other table.
func TestBackupRunsRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	backup := db.Pool(t, pg.RoleBackup)

	allowed := []string{
		`INSERT INTO backup_runs (id, kind) VALUES ('0192f4a0-7b1c-7000-8a00-000000000001', 'pg_full')`,
		`UPDATE backup_runs SET state = 'failed', finished_at = now(), error = 'canceled' WHERE id = '0192f4a0-7b1c-7000-8a00-000000000001'`,
		`SELECT count(*) FROM backup_runs`,
	}
	for _, stmt := range allowed {
		if _, err := backup.Exec(ctx, stmt); err != nil {
			t.Errorf("bothy_backup: %s: %v", stmt, err)
		}
	}
	denied := []string{
		`DELETE FROM backup_runs`,
		`TRUNCATE backup_runs`,
		`INSERT INTO instance (instance_id, schema_version, sync_epoch) VALUES (gen_random_uuid(), 1, gen_random_uuid())`,
		`INSERT INTO principals (id, kind, handle, display_name) VALUES (gen_random_uuid(), 'user', 'sam', 'Sam')`,
		`UPDATE sequence_counters SET value = value + 1`,
		`SELECT count(*) FROM instance`,
		`SELECT count(*) FROM messages`,
		`SELECT count(*) FROM audit_events`,
		`CREATE TABLE backup_extra (id int)`,
	}
	for _, stmt := range denied {
		_, err := backup.Exec(ctx, stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("bothy_backup: %s: %v, want insufficient_privilege (42501)", stmt, err)
		}
	}
}

// newRuns returns Runs over the role bothy_backup of a migrated database and a fake clock.
func newRuns(t *testing.T) (*postgres.Runs, *clock.Fake, *pgtest.DB) {
	t.Helper()
	db := pgtest.New(t)
	clk := clock.NewFake(time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC))
	return postgres.NewRuns(db.Pool(t, pg.RoleBackup), clk, ids.NewGenerator(clk, rand.Reader)), clk, db
}

// storedRun is a backup_runs row as the tests read it.
type storedRun struct {
	kind, state      string
	started          time.Time
	finished         *time.Time
	size             *int64
	manifest, checks map[string]any
	code             *string
}

// readRun reads the row id as superuser.
func readRun(t *testing.T, db *pgtest.DB, id ids.ID) storedRun {
	t.Helper()
	var r storedRun
	if err := db.SuperConn(t).QueryRow(context.Background(), `SELECT kind, state, started_at, finished_at, size_bytes,
		manifest, checks, error FROM backup_runs WHERE id = $1`, id.String()).
		Scan(&r.kind, &r.state, &r.started, &r.finished, &r.size, &r.manifest, &r.checks, &r.code); err != nil {
		t.Fatal(err)
	}
	return r
}

// TestRunsStartFinish holds Runs: Start inserts a running row with started_at from the clock and
// a UUIDv7 id; Finish records succeeded with size and manifest, or failed with the code, NULL
// size and the empty manifest; a second Finish of the same run fails and changes nothing.
func TestRunsStartFinish(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runs, clk, db := newRuns(t)
	start := clk.Now()

	ok, err := runs.Start(ctx, status.BackupPGFull)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ids.Parse(ok.String()); err != nil || !ok.Time().Equal(start) {
		t.Errorf("id %s is not a UUIDv7 of the clock: %v", ok, err)
	}
	if r := readRun(t, db, ok); r.kind != "pg_full" || r.state != "running" || !r.started.Equal(start) ||
		r.finished != nil || r.size != nil || len(r.manifest) != 0 || r.code != nil {
		t.Errorf("started row %+v", r)
	}

	clk.Advance(7 * time.Minute)
	manifest := []byte(`{"schema":"backup_manifest.v1","kind":"pg_full"}`)
	if err := runs.Finish(ctx, status.FinishedRun{ID: ok, Succeeded: true, SizeBytes: 4096, Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	r := readRun(t, db, ok)
	var want map[string]any
	if err := json.Unmarshal(manifest, &want); err != nil {
		t.Fatal(err)
	}
	if r.state != "succeeded" || r.finished == nil || !r.finished.Equal(clk.Now()) || r.size == nil || *r.size != 4096 ||
		!reflect.DeepEqual(r.manifest, want) || r.code != nil || len(r.checks) != 0 {
		t.Errorf("succeeded row %+v", r)
	}
	if err := runs.Finish(ctx, status.FinishedRun{ID: ok, Error: "canceled"}); err == nil {
		t.Error("a second Finish succeeded")
	}
	if r := readRun(t, db, ok); r.state != "succeeded" {
		t.Errorf("the second Finish changed the row: %+v", r)
	}

	failed, err := runs.Start(ctx, status.BackupLogical)
	if err != nil {
		t.Fatal(err)
	}
	if err := runs.Finish(ctx, status.FinishedRun{ID: failed, SizeBytes: -1, Error: "pg_dump_exit_1"}); err != nil {
		t.Fatal(err)
	}
	if r := readRun(t, db, failed); r.state != "failed" || r.size != nil || len(r.manifest) != 0 || r.code == nil || *r.code != "pg_dump_exit_1" {
		t.Errorf("failed row %+v", r)
	}
	if err := runs.Finish(ctx, status.FinishedRun{ID: ids.New(), Succeeded: true}); err == nil {
		t.Error("Finish of an unknown run succeeded")
	}
}

// TestRunsLastSuccess holds LastSuccess: the newest started_at of the succeeded rows of the given
// kinds; failed and running rows and other kinds do not count; none gives false.
func TestRunsLastSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runs, clk, _ := newRuns(t)

	if _, found, err := runs.LastSuccess(ctx, status.BackupPGFull, status.BackupPGDiff, status.BackupLogical); err != nil || found {
		t.Fatalf("LastSuccess on an empty table = %v, %v; want false", found, err)
	}
	record := func(kind status.BackupKind, succeeded bool) time.Time {
		t.Helper()
		clk.Advance(time.Hour)
		at := clk.Now()
		id, err := runs.Start(ctx, kind)
		if err != nil {
			t.Fatal(err)
		}
		run := status.FinishedRun{ID: id, Succeeded: succeeded, SizeBytes: 1, Manifest: []byte(`{}`)}
		if !succeeded {
			run = status.FinishedRun{ID: id, SizeBytes: -1, Error: "restic_exit_1"}
		}
		if err := runs.Finish(ctx, run); err != nil {
			t.Fatal(err)
		}
		return at
	}
	full := record(status.BackupPGFull, true)
	diff := record(status.BackupPGDiff, true)
	record(status.BackupPGFull, false)
	objects := record(status.BackupObjects, true)
	clk.Advance(time.Hour)
	if _, err := runs.Start(ctx, status.BackupPGDiff); err != nil { // still running
		t.Fatal(err)
	}

	for _, c := range []struct {
		kinds []status.BackupKind
		want  time.Time
		found bool
	}{
		{[]status.BackupKind{status.BackupPGFull}, full, true},
		{[]status.BackupKind{status.BackupPGFull, status.BackupPGDiff}, diff, true},
		{[]status.BackupKind{status.BackupPGFull, status.BackupPGDiff, status.BackupLogical}, diff, true},
		{[]status.BackupKind{status.BackupObjects}, objects, true},
		{[]status.BackupKind{status.BackupLogical}, time.Time{}, false},
	} {
		got, found, err := runs.LastSuccess(ctx, c.kinds...)
		if err != nil || found != c.found || !got.Equal(c.want) {
			t.Errorf("LastSuccess(%v) = %v, %v, %v; want %v, %v", c.kinds, got, found, err, c.want, c.found)
		}
	}
}

// TestRunsNoBackupTable holds ErrNoBackupTable: a schema migrated before M7-02 has no
// backup_runs, and LastSuccess says so instead of failing with a database error.
func TestRunsNoBackupTable(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	clk := clock.NewFake(time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC))
	runs := postgres.NewRuns(db.Pool(t, pg.RoleOwner), clk, ids.NewGenerator(clk, rand.Reader))
	if _, _, err := runs.LastSuccess(context.Background(), status.BackupPGFull); !errors.Is(err, status.ErrNoBackupTable) {
		t.Errorf("LastSuccess = %v, want ErrNoBackupTable", err)
	}
}

// TestRunsLatestSucceededAndChecks holds the two additions of M7-03: LatestSucceeded returns the
// newest succeeded row of the kinds with its id and manifest (failed rows and other kinds do not
// count), and Finish stores the checks of a restore test.
func TestRunsLatestSucceededAndChecks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runs, clk, db := newRuns(t)
	if _, found, err := runs.LatestSucceeded(ctx, status.BackupPGFull, status.BackupPGDiff); err != nil || found {
		t.Fatalf("LatestSucceeded on an empty table = %v, %v; want false", found, err)
	}
	record := func(kind status.BackupKind, succeeded bool, manifest string) ids.ID {
		t.Helper()
		clk.Advance(time.Hour)
		id, err := runs.Start(ctx, kind)
		if err != nil {
			t.Fatal(err)
		}
		run := status.FinishedRun{ID: id, Succeeded: succeeded, SizeBytes: 1, Manifest: []byte(manifest)}
		if !succeeded {
			run = status.FinishedRun{ID: id, SizeBytes: -1, Error: "pgbackrest_exit_1"}
		}
		if err := runs.Finish(ctx, run); err != nil {
			t.Fatal(err)
		}
		return id
	}
	record(status.BackupPGFull, true, `{"n":1}`)
	diff := record(status.BackupPGDiff, true, `{"n":2}`)
	record(status.BackupPGFull, false, "")
	record(status.BackupLogical, true, `{"n":3}`)
	got, found, err := runs.LatestSucceeded(ctx, status.BackupPGFull, status.BackupPGDiff)
	if err != nil || !found || got.ID != diff || got.Kind != status.BackupPGDiff || string(got.Manifest) != `{"n": 2}` {
		t.Errorf("LatestSucceeded = %+v (%s), %v, %v; want the differential", got, got.Manifest, found, err)
	}

	test, err := runs.Start(ctx, status.BackupRestoreTest)
	if err != nil {
		t.Fatal(err)
	}
	checks := []byte(`{"schema":"restore_test_checks.v1","checks":[]}`)
	if err := runs.Finish(ctx, status.FinishedRun{ID: test, SizeBytes: -1, Checks: checks, Error: "checks_failed"}); err != nil {
		t.Fatal(err)
	}
	if r := readRun(t, db, test); r.state != "failed" || r.checks["schema"] != "restore_test_checks.v1" || len(r.manifest) != 0 {
		t.Errorf("restore test row %+v", r)
	}
}
