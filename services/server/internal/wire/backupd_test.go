// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// backupdEnv is the configuration of bothy-backupd for db: the role bothy_backup for
// backup_runs, the superuser of the test cluster as migrate.bootstrap_user and the password
// files of pgtest.
func backupdEnv(t *testing.T, db *pgtest.DB, extra ...string) *config.Values {
	t.Helper()
	cfg, super := db.Config(pg.RoleBackup), db.SuperConfig()
	dir := db.SecretsDir()
	env := []string{
		testPublicURL,
		"BOTHY_SECRETS__DIR=" + dir,
		"BOTHY_DATABASE__HOST=" + cfg.Host,
		"BOTHY_DATABASE__PORT=" + strconv.Itoa(cfg.Port),
		"BOTHY_DATABASE__NAME=" + db.Name(),
		"BOTHY_DATABASE__USER=" + string(pg.RoleBackup),
		"BOTHY_DATABASE__PASSWORD_REF=file:" + dir + "/db_backup_password",
		"BOTHY_DATABASE__SSLMODE=disable",
		"BOTHY_MIGRATE__BOOTSTRAP_USER=" + super.User,
		"BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF=file:" + dir + "/db_superuser_password",
		"BOTHY_STARTUP__DEPENDENCY_WAIT=" + testkit.PatienceSetting,
		"BOTHY_FILE__FS__ROOT=" + t.TempDir(),
	}
	for _, e := range extra {
		name, _, _ := strings.Cut(e, "=")
		env = slices.DeleteFunc(env, func(have string) bool { return strings.HasPrefix(have, name+"=") })
		env = append(env, e)
	}
	v, err := config.Load(schema(t), config.Sources{Env: env})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return v
}

// TestBackupDaemonRecordsRuns: the daemon built from the configuration reaches backup_runs as
// bothy_backup, refuses an unknown kind, warns without an off-site target and records a run
// whose tool is missing as failed with a code (no binary is on PATH, so nothing runs).
func TestBackupDaemonRecordsRuns(t *testing.T) {
	db := pgtest.New(t)
	v := backupdEnv(t, db)
	t.Setenv("PATH", t.TempDir())
	ctx := testkit.Context(t)
	d, err := wire.NewBackupDaemon(ctx, v, wire.BackupDaemonOptions{DumpDir: t.TempDir(), ServerVersion: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	if err := d.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	if got := d.Warnings(); len(got) != 1 || !strings.Contains(got[0], "local backup only") {
		t.Errorf("Warnings = %q", got)
	}
	if err := d.Prime(ctx); err != nil {
		t.Errorf("Prime: %v", err)
	}
	d.Sample(ctx)
	if err := d.Run(ctx, "weekly"); !errors.Is(err, wire.ErrUnknownBackupKind) {
		t.Errorf("Run(weekly) = %v, want ErrUnknownBackupKind", err)
	}
	if wire.IsBackupKind("weekly") || !wire.IsBackupKind("logical") {
		t.Error("IsBackupKind")
	}
	if err := d.Run(ctx, "logical"); err == nil {
		t.Error("Run(logical) without pg_dump and restic succeeded")
	}
	var status, code string
	if err := db.SuperConn(t).QueryRow(ctx, "SELECT state, error FROM backup_runs WHERE kind = 'logical'").Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !strings.HasSuffix(code, "_not_found") {
		t.Errorf("row %s %q, want failed with a <tool>_not_found code", status, code)
	}
	if err := d.ExpireOffsite(ctx, t.TempDir()); err == nil {
		t.Error("ExpireOffsite without an off-site target succeeded")
	}
	env, err := d.PgBackRestEnv(ctx)
	if err != nil || !slices.Equal(env, []string{"PGBACKREST_EXPIRE_AUTO=n"}) {
		t.Errorf("PgBackRestEnv = %q, %v", env, err)
	}
	start := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	s := d.NewScheduler(start)
	if got := s.Tick(ctx, start.Add(30*time.Second)); len(got) != 0 {
		t.Errorf("Tick at 12:00:30 started %q", got)
	}
	s.Wait()
}

// TestBackupDaemonOffsiteMirror: with an off-site target and the driver s3 the daemon builds the
// object mirror and repo2: no "local backup only" warning, and the pgBackRest environment
// carries the off-site key of secrets/offsite/ (nothing is contacted).
func TestBackupDaemonOffsiteMirror(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	dir := t.TempDir()
	for _, name := range []string{"db_backup_password", "db_superuser_password"} {
		b, err := os.ReadFile(filepath.Join(db.SecretsDir(), name)) // #nosec G304 -- a file of the test cluster
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil { //nolint:gosec // G703: a fixed file name in the test's own directory
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "pgbackrest_cipher_pass"), []byte("EXAMPLECIPHER\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "offsite"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, val := range map[string]string{"access_key_id": "EXAMPLEKEYID\n", "secret_access_key": "EXAMPLESECRET\n"} {
		if err := os.WriteFile(filepath.Join(dir, "offsite", name), []byte(val), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	v := backupdEnv(t, db, "BOTHY_SECRETS__DIR="+dir,
		"BOTHY_DATABASE__PASSWORD_REF=file:"+dir+"/db_backup_password",
		"BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF=file:"+dir+"/db_superuser_password",
		"BOTHY_BACKUP__OFFSITE__TARGET=s3:https://s3.example.com/bothy-backup/instance-1",
		"BOTHY_FILE__STORAGE__DRIVER=s3", "BOTHY_FILE__S3__ENDPOINT=https://files.example.com",
		"BOTHY_FILE__S3__BUCKET=bothy-files")
	ctx := testkit.Context(t)
	d, err := wire.NewBackupDaemon(ctx, v, wire.BackupDaemonOptions{DumpDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	if got := d.Warnings(); len(got) != 0 {
		t.Errorf("Warnings = %q, want none", got)
	}
	env, err := d.PgBackRestEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "PGBACKREST_REPO2_S3_KEY=EXAMPLEKEYID") || !slices.Contains(env, "PGBACKREST_EXPIRE_AUTO=n") {
		t.Errorf("PgBackRestEnv = %q", env)
	}
}
