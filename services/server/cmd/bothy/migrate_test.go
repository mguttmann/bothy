// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"io/fs"
	"net"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// migrateEnvFor returns the environment that points bothy migrate at db as role, with the
// secrets directory of pgtest; extra entries ("NAME=value") are added or replace a default.
func migrateEnvFor(db *pgtest.DB, role pg.Role, extra ...string) []string {
	cfg := db.Config(role)
	env := []string{
		"BOTHY_SECRETS__DIR=" + db.SecretsDir(),
		"BOTHY_DATABASE__HOST=" + cfg.Host,
		"BOTHY_DATABASE__PORT=" + strconv.Itoa(cfg.Port),
		"BOTHY_DATABASE__NAME=" + db.Name(),
		"BOTHY_DATABASE__USER=" + string(role),
		"BOTHY_DATABASE__PASSWORD_REF=" + cfg.PasswordRef,
		"BOTHY_DATABASE__SSLMODE=disable",
		"BOTHY_STARTUP__DEPENDENCY_WAIT=10s",
	}
	// An extra entry replaces the default of the same variable; the loader refuses duplicates.
	for _, e := range extra {
		name, _, _ := strings.Cut(e, "=")
		env = slices.DeleteFunc(env, func(have string) bool { return strings.HasPrefix(have, name+"=") })
		env = append(env, e)
	}
	return env
}

func TestMigrateUpBootstrap(t *testing.T) {
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner)

	code, stdout, stderr := runBothy(t, env, "migrate", "up", "--bootstrap")
	want := "bootstrap: roles and grants are in place\napplied 1 base_privileges\n"
	if code != 0 || !strings.HasPrefix(stdout, want) || !strings.HasSuffix(stdout, "database version 1, applied 1\n") {
		t.Fatalf("first run: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	code, stdout, stderr = runBothy(t, env, "migrate", "up", "--bootstrap")
	if code != 0 || stdout != "bootstrap: roles and grants are in place\ndatabase version 1, applied 0\n" {
		t.Fatalf("second run: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	// The roles still log in with the passwords of their secret files.
	for _, r := range pg.Roles() {
		if err := db.Pool(t, r).Ping(context.Background()); err != nil {
			t.Errorf("role %s after the bootstrap: %v", r, err)
		}
	}
}

func TestMigrateUpAsAppFails(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	code, stdout, stderr := runBothy(t, migrateEnvFor(db, pg.RoleApp), "migrate", "up")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "bothy migrate: migrate must run as the schema owner; set database.user\n") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestMigrateStatusOutput(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner)
	code, stdout, stderr := runBothy(t, env, "migrate", "status")
	if want := "1 base_privileges pending -\ndatabase version 0, binary requires 1, pending 1\n"; code != 0 || stdout != want {
		t.Fatalf("before: exit %d, stderr %q, stdout\n%s\nwant\n%s", code, stderr, stdout, want)
	}
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("up: exit %d, stderr %q", code, stderr)
	}
	// bothy_app may read the status as well.
	code, stdout, stderr = runBothy(t, migrateEnvFor(db, pg.RoleApp), "migrate", "status")
	pattern := regexp.MustCompile(`^1 base_privileges applied \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\ndatabase version 1, binary requires 1, pending 0\n$`)
	if code != 0 || !pattern.MatchString(stdout) {
		t.Fatalf("after: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
}

func TestMigrateNewerSchemaNotice(t *testing.T) {
	t.Parallel()
	newer := fstest.MapFS{"00002_from_a_newer_binary.sql": {Data: []byte("-- +goose Up\nCREATE TABLE newer_marker (id int);\n")}}
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		newer[n] = &fstest.MapFile{Data: b}
	}
	db := pgtest.New(t, pgtest.WithMigrations(newer))
	code, stdout, stderr := runBothy(t, migrateEnvFor(db, pg.RoleOwner), "migrate", "up")
	if code != 0 || stdout != newerSchemaNotice {
		t.Errorf("exit %d, stderr %q, stdout %q", code, stderr, stdout)
	}
}

func TestMigrateValidate(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runBothy(t, nil, "migrate", "validate")
	if code != 0 || stdout != "migrations ok\n" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	var out, errOut bytes.Buffer
	e := defaultMigrateEnv(nil, &out, &errOut)
	e.fsys = fstest.MapFS{"00002_gap.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n")}}
	if code := migrateCommand(t.Context(), []string{"validate"}, e); code != 1 || !strings.Contains(errOut.String(), "version 1 is missing") {
		t.Errorf("invalid set: exit %d, stderr %q", code, errOut.String())
	}
}

func TestMigrateUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"migrate"}, {"migrate", "down"}, {"migrate", "status", "--bootstrap"}, {"migrate", "validate", "--config", "x.yaml"},
		{"migrate", "up", "extra"}, {"migrate", "up", "--config="}, {"migrate", "up", "--bootstrap=maybe"},
	} {
		code, stdout, stderr := runBothy(t, nil, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, migrateUsage) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	for _, args := range [][]string{{"migrate", "help"}, {"migrate", "--help"}, {"migrate", "up", "-h"}} {
		code, stdout, stderr := runBothy(t, nil, args...)
		if code != 0 || stdout != migrateUsage || stderr != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestMigrateConfigErrorExit2(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runBothy(t, []string{"BOTHY_DATABASE__SSLMODE=require"}, "migrate", "status")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "database.sslmode") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestMigrateUnreachableDatabase(t *testing.T) {
	t.Parallel()
	l, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	_ = l.Close()
	dir := t.TempDir()
	writeSecret(t, dir, "db_owner_password", "unused-EXAMPLE\n", 0o600)
	env := []string{
		"BOTHY_SECRETS__DIR=" + dir, "BOTHY_DATABASE__HOST=127.0.0.1", "BOTHY_DATABASE__PORT=" + port,
		"BOTHY_DATABASE__SSLMODE=disable", "BOTHY_DATABASE__USER=bothy_owner",
		"BOTHY_DATABASE__PASSWORD_REF=file:" + filepath.Join(dir, "db_owner_password"),
		"BOTHY_STARTUP__DEPENDENCY_WAIT=300ms", "BOTHY_DATABASE__ACQUIRE_TIMEOUT=100ms",
	}
	code, stdout, stderr := runBothy(t, env, "migrate", "up")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "127.0.0.1:"+port) || strings.Contains(stderr, "unused-EXAMPLE") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// TestMigrateUpHonoursLockTimeout holds rule M6 of upgrade.md at the command: a migration that
// waits for a lock fails after migrate.lock_timeout, it does not block the application.
func TestMigrateUpHonoursLockTimeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fsys := fstest.MapFS{
		"00002_locked_table.sql": {Data: []byte("-- +goose Up\nCREATE TABLE locked_table (id int);\n")},
		"00003_alter_locked.sql": {Data: []byte("-- +goose Up\nALTER TABLE locked_table ADD COLUMN extra int;\n")},
	}
	first := fstest.MapFS{"00002_locked_table.sql": fsys["00002_locked_table.sql"]}
	for _, set := range []fstest.MapFS{fsys, first} {
		names, err := fs.Glob(migrations.FS, "*.sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range names {
			b, err := fs.ReadFile(migrations.FS, n)
			if err != nil {
				t.Fatal(err)
			}
			set[n] = &fstest.MapFile{Data: b}
		}
	}
	db := pgtest.New(t, pgtest.WithMigrations(first))
	tx, err := db.SuperConn(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "LOCK TABLE locked_table IN ACCESS SHARE MODE"); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	e := defaultMigrateEnv(migrateEnvFor(db, pg.RoleOwner, "BOTHY_MIGRATE__LOCK_TIMEOUT=300ms"), &stdout, &stderr)
	e.fsys = fsys
	code := migrateCommand(ctx, []string{"up"}, e)
	if code != 1 || !strings.Contains(stderr.String(), "55P03") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 with lock_not_available (55P03)", code, stdout.String(), stderr.String())
	}
}

// TestMigrateMissingDatabaseIsNamed: when migrate gives up, its error names the cause the
// database reported, here the missing database.name, not the end of the wait.
func TestMigrateMissingDatabaseIsNamed(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner, "BOTHY_DATABASE__NAME=no_such_database", "BOTHY_STARTUP__DEPENDENCY_WAIT=1s")
	for _, args := range [][]string{{"migrate", "up", "--bootstrap"}, {"migrate", "up"}} {
		code, stdout, stderr := runBothy(t, env, args...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, `database "no_such_database" does not exist`) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}
