// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// restoreState is the part of the instance row bothy restore finalize changes, as the superuser
// reads it.
type restoreState struct {
	epoch, system string
	timeline      int64
}

func readRestoreState(t *testing.T, db *pgtest.DB) restoreState {
	t.Helper()
	var s restoreState
	if err := db.SuperConn(t).QueryRow(context.Background(), `SELECT sync_epoch::text, coalesce(pg_system_identifier, ''),
		coalesce(pg_timeline_id, 0) FROM instance`).Scan(&s.epoch, &s.system, &s.timeline); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestRestoreUsage: no subcommand, an unknown one, an unknown flag and an argument are usage
// errors (exit 2); --help prints the help with the option and the exit codes (exit 0).
func TestRestoreUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"restore"}, {"restore", "rollback"}, {"restore", "finalize", "--force"},
		{"restore", "finalize", "extra"}, {"restore", "finalize", "--relocation="}} {
		if code, _, stderr := runBothy(t, nil, args...); code != exitUsage || !strings.Contains(stderr, "Usage: bothy restore finalize") {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr)
		}
	}
	for _, args := range [][]string{{"restore", "--help"}, {"restore", "finalize", "-h"}} {
		code, stdout, _ := runBothy(t, nil, args...)
		if code != exitOK || !strings.Contains(stdout, "--relocation FILE") || !strings.Contains(stdout, "Exit codes: 0 done, 1 error or refused, 2 usage") {
			t.Errorf("%v: exit %d, stdout %q", args, code, stdout)
		}
	}
	// A manifest that cannot be read is exit 1 before the database is touched.
	if code, _, stderr := runBothy(t, nil, "restore", "finalize", "--relocation", filepath.Join(t.TempDir(), "missing.json")); code != exitFailure ||
		!strings.Contains(stderr, "read the manifest") {
		t.Errorf("missing manifest: exit %d, stderr %q", code, stderr)
	}
}

// TestFinalizeRestore_RefusesWithConnectedApp (brief M7-04): an open bothy_app session makes
// bothy restore finalize exit 1 without a change; once it ends, the command exits 0, prints the
// epochs and the identity and sets a new epoch.
func TestFinalizeRestore_RefusesWithConnectedApp(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	env := migrateEnvFor(db, pg.RoleOwner)
	before := readRestoreState(t, db)
	app := db.Pool(t, pg.RoleApp)
	if _, err := app.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runBothy(t, env, "restore", "finalize")
	if code != exitFailure || !strings.Contains(stderr, "bothy_app") || stdout != "" {
		t.Fatalf("with a bothy_app session: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got := readRestoreState(t, db); got != before {
		t.Fatalf("the refusal changed the row: %+v, was %+v", got, before)
	}
	app.Close()
	// Close does not wait for the backend to end; the command counts it until it has.
	super := db.SuperConn(t)
	testkit.Until(t, "no bothy_app session connected", func() bool {
		var n int
		if err := super.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity
			WHERE usename = 'bothy_app' AND datname = current_database()`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	})
	code, stdout, stderr = runBothy(t, env, "restore", "finalize")
	after := readRestoreState(t, db)
	if code != exitOK || after.epoch == before.epoch {
		t.Fatalf("exit %d, stdout %q, stderr %q, row %+v", code, stdout, stderr, after)
	}
	want := regexp.MustCompile(`^sync epoch ` + before.epoch + ` -> ` + after.epoch + `, cursors at head_seq [0-9]+\ndatabase identity [0-9]+ timeline [0-9]+\n$`)
	if !want.MatchString(stdout) {
		t.Fatalf("stdout %q", stdout)
	}
}

// TestRestoreFinalize_RelocationMismatchExits1: a manifest that is not backup_manifest.v1, or
// whose facts differ from the database, is exit 1 and changes nothing.
func TestRestoreFinalize_RelocationMismatchExits1(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	env := migrateEnvFor(db, pg.RoleOwner)
	before := readRestoreState(t, db)
	bad := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(bad, []byte(`{"schema":"other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runBothy(t, env, "restore", "finalize", "--relocation", bad); code != exitFailure || !strings.Contains(stderr, "manifest") {
		t.Fatalf("invalid manifest: exit %d, stderr %q", code, stderr)
	}
	if got := readRestoreState(t, db); got != before {
		t.Fatalf("row changed: %+v, was %+v", got, before)
	}
}
