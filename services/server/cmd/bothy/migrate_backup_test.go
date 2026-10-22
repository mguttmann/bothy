// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/migrations"
)

// backupNow is the fixed clock of the recent-backup tests.
var backupNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// recentBackupHint is the failure of migrate up with one pending migration and no recent backup.
const recentBackupHint = `bothy migrate: 1 pending migrations and no successful backup (pg_full, pg_diff or logical) younger than migrate.recent_backup_max_age; run "bothy-backupd backup pg_full" in the service backup first, or set migrate.require_recent_backup = false` + "\n"

// fakeHistory is a backupHistory that answers at, found and err and counts its calls.
type fakeHistory struct {
	at    time.Time
	found bool
	err   error
	calls int
}

func (f *fakeHistory) last(_ context.Context, _ *pg.Pool) (time.Time, bool, error) {
	f.calls++
	return f.at, f.found, f.err
}

// freshBackup is a backupHistory with a success one minute before backupNow, for tests that
// migrate an existing schema and are not about the recent-backup check.
func freshBackup(context.Context, *pg.Pool) (time.Time, bool, error) {
	return backupNow.Add(-time.Minute), true, nil
}

// TestMigrateRequiresRecentBackup holds docs/spec/16-operations.md 10 item 1 (R-OP-13): with pending
// migrations on an existing schema, migrate up applies nothing, River's step included, unless a
// pg_full, pg_diff or logical backup succeeded within migrate.recent_backup_max_age. An empty
// database, a schema without backup_runs and migrate.require_recent_backup = false are exempt.
func TestMigrateRequiresRecentBackup(t *testing.T) {
	t.Parallel()
	base := embeddedMigrationSet(t)
	list := embeddedMigrations(t)
	next := list[len(list)-1].version + 1
	withPending := embeddedMigrationSet(t)
	withPending[migrationFile(next, "after_backup")] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE after_backup (id int);\n")}

	// up runs migrate up against db with history and extra environment entries.
	up := func(t *testing.T, db *pgtest.DB, history *fakeHistory, extra ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		e := defaultMigrateEnv(migrateEnvFor(db, pg.RoleOwner, extra...), &stdout, &stderr)
		e.fsys, e.backupHistory, e.now = withPending, history.last, func() time.Time { return backupNow }
		code := migrateCommand(t.Context(), []string{"up"}, e)
		return code, stdout.String(), stderr.String()
	}
	// applied reports whether the pending migration ran.
	applied := func(t *testing.T, db *pgtest.DB) bool {
		t.Helper()
		var ok bool
		if err := db.SuperConn(t).QueryRow(context.Background(), "SELECT to_regclass('public.after_backup') IS NOT NULL").Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}

	for _, c := range []struct {
		name    string
		history fakeHistory
	}{
		{"no successful run", fakeHistory{}},
		{"the last success is too old", fakeHistory{at: backupNow.Add(-25 * time.Hour), found: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			db := pgtest.New(t, pgtest.WithMigrations(base))
			h := c.history
			code, stdout, stderr := up(t, db, &h)
			if code != 1 || stdout != "" || stderr != recentBackupHint {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1, empty and the hint", code, stdout, stderr)
			}
			if h.calls != 1 || applied(t, db) {
				t.Errorf("history read %d times (want 1), pending migration applied %v (want false)", h.calls, applied(t, db))
			}
		})
	}

	t.Run("a fresh run", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithMigrations(base))
		h := fakeHistory{at: backupNow.Add(-23 * time.Hour), found: true}
		code, stdout, stderr := up(t, db, &h)
		if code != 0 || !strings.Contains(stdout, "applied 1\n") || !applied(t, db) || h.calls != 1 {
			t.Errorf("exit %d, stdout %q, stderr %q, history calls %d", code, stdout, stderr, h.calls)
		}
	})

	t.Run("a larger maximum age", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithMigrations(base))
		h := fakeHistory{at: backupNow.Add(-25 * time.Hour), found: true}
		code, _, stderr := up(t, db, &h, "BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE=26h")
		if code != 0 || !applied(t, db) {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})

	t.Run("empty database", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithoutMigrations())
		h := fakeHistory{}
		code, _, stderr := up(t, db, &h)
		if code != 0 || !applied(t, db) || h.calls != 0 {
			t.Errorf("exit %d, stderr %q, history calls %d (want 0)", code, stderr, h.calls)
		}
	})

	t.Run("nothing pending", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithMigrations(withPending))
		h := fakeHistory{}
		code, stdout, stderr := up(t, db, &h)
		if code != 0 || !strings.HasSuffix(stdout, "applied 0\n") || h.calls != 0 {
			t.Errorf("exit %d, stdout %q, stderr %q, history calls %d (want 0)", code, stdout, stderr, h.calls)
		}
	})

	t.Run("require_recent_backup false", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithMigrations(base))
		h := fakeHistory{}
		code, _, stderr := up(t, db, &h, "BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP=false")
		if code != 0 || !applied(t, db) || h.calls != 0 {
			t.Errorf("exit %d, stderr %q, history calls %d (want 0)", code, stderr, h.calls)
		}
	})

	t.Run("no backup_runs table yet", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithMigrations(base))
		h := fakeHistory{err: wire.ErrNoBackupTable}
		code, stdout, stderr := up(t, db, &h)
		if code != 0 || !strings.HasPrefix(stdout, "no backup_runs table yet; the recent-backup check starts with the next upgrade\n") || !applied(t, db) {
			t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
		}
	})

	t.Run("the history cannot be read", func(t *testing.T) {
		t.Parallel()
		db := pgtest.New(t, pgtest.WithMigrations(base))
		h := fakeHistory{err: errors.New("EXAMPLE read failure")}
		code, stdout, stderr := up(t, db, &h)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "EXAMPLE read failure") || applied(t, db) {
			t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
		}
	})
}

// embeddedMigrationSet copies migrations.FS into a map, so a test can add a pending migration.
func embeddedMigrationSet(t *testing.T) fstest.MapFS {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	set := fstest.MapFS{}
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		set[n] = &fstest.MapFile{Data: b}
	}
	if len(set) == 0 {
		t.Fatal("no embedded migrations")
	}
	return set
}

// migrationFile is the file name of migration v with the given slug.
func migrationFile(v int64, slug string) string { return fmt.Sprintf("%05d_%s.sql", v, slug) }
