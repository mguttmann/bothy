// SPDX-License-Identifier: Apache-2.0

package executor_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/status/adapters/executor"
)

// TestRestoreCommands holds the commands of the restore test (brief M7-03): the restore of one
// set from repo1 with recovery target immediate, promotion, archive_mode off and the repository
// host acting as the database host, also in restore_command; verify of repo1; and the restic
// restore of the sampled paths of the newest snapshot of the object backup (tag and path). Requests the commands cannot
// express are refused before a binary runs.
func TestRestoreCommands(t *testing.T) {
	bin := t.TempDir()
	pg := newFake(t, bin, "pgbackrest", "")
	r := resticFake(t, bin, 0)
	e := newExecutor(t, executor.Config{PgBackRest: pg.path, Restic: r.path, SecretsDir: secretsDir(t, false), LocalResticRepo: "/var/lib/example/restic"})
	ctx := context.Background()

	if err := e.PgBackRestRestore(ctx, "20261018-020000F_20261019-020000D", "/var/lib/bothy-backup/restore/pgdata"); err != nil {
		t.Fatal(err)
	}
	if err := e.PgBackRestVerify(ctx); err != nil {
		t.Fatal(err)
	}
	calls := pg.calls(t)
	if len(calls) != 2 {
		t.Fatalf("%d pgbackrest calls, want restore and verify", len(calls))
	}
	wantRestore := []string{"--stanza=bothy", "--repo=1", "--reset-pg1-host", "--pg1-path=/var/lib/bothy-backup/restore/pgdata",
		"--set=20261018-020000F_20261019-020000D", "--type=immediate", "--target-action=promote", "--archive-mode=off",
		`--recovery-option=restore_command=pgbackrest --stanza=bothy --repo=1 --reset-pg1-host --pg1-path=/var/lib/bothy-backup/restore/pgdata archive-get %f "%p"`,
		"restore"}
	if !slices.Equal(calls[0].args, wantRestore) {
		t.Errorf("restore args\n %q\nwant %q", calls[0].args, wantRestore)
	}
	if want := []string{"--stanza=bothy", "--repo=1", "verify"}; !slices.Equal(calls[1].args, want) {
		t.Errorf("verify args %q, want %q", calls[1].args, want)
	}
	checkEnv(t, calls[0], map[string]string{"PGBACKREST_EXPIRE_AUTO": "n"})

	if err := e.ResticRestore(ctx, "objects", "/var/lib/bothy/files", []string{"/var/lib/bothy/files/f/ab/1", "/var/lib/bothy/files/f/cd/2"}, "/var/lib/bothy-backup/restore/objects"); err != nil {
		t.Fatal(err)
	}
	rc := r.calls(t)
	want := []string{"-r", "/var/lib/example/restic", "restore", "latest", "--tag", "objects", "--path", "/var/lib/bothy/files", "--target", "/var/lib/bothy-backup/restore/objects",
		"--include", "/var/lib/bothy/files/f/ab/1", "--include", "/var/lib/bothy/files/f/cd/2"}
	if len(rc) != 1 || !slices.Equal(rc[0].args, want) {
		t.Errorf("restic calls %+v, want one with %q", rc, want)
	}

	for _, err := range []error{
		e.PgBackRestRestore(ctx, "latest", "/var/lib/bothy-backup/restore/pgdata"),
		e.PgBackRestRestore(ctx, "20261018-020000F", "relative/pgdata"),
		e.ResticRestore(ctx, "objects", "/var/lib/bothy/files", nil, "/var/lib/bothy-backup/restore/objects"),
		e.ResticRestore(ctx, "", "/var/lib/bothy/files", []string{"/x"}, "/var/lib/bothy-backup/restore/objects"),
		e.ResticRestore(ctx, "objects", "", []string{"/x"}, "/var/lib/bothy-backup/restore/objects"),
	} {
		if err == nil {
			t.Error("an invalid request ran")
		}
	}
	if len(pg.calls(t)) != 2 || len(r.calls(t)) != 1 {
		t.Error("an invalid request reached a binary")
	}
}
