// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
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

// embedded is one migration of migrations.FS.
type embedded struct {
	version int64
	name    string
}

// embeddedMigrations lists migrations.FS in version order.
func embeddedMigrations(t *testing.T) []embedded {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]embedded, 0, len(names))
	for _, n := range names {
		v, err := strconv.ParseInt(n[:5], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, embedded{version: v, name: strings.TrimSuffix(n[6:], ".sql")})
	}
	return out
}

// appliedLines is what migrate up prints for every embedded migration.
func appliedLines(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, m := range embeddedMigrations(t) {
		fmt.Fprintf(&b, "applied %d %s\n", m.version, m.name)
	}
	return b.String()
}

// requiredVersion is the highest embedded version.
func requiredVersion(t *testing.T) int64 {
	t.Helper()
	v, err := pg.RequiredSchemaVersion(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// nextMigrationName names the migration offset versions above the embedded ones.
func nextMigrationName(t *testing.T, offset int64, slug string) string {
	t.Helper()
	return fmt.Sprintf("%05d_%s.sql", requiredVersion(t)+offset, slug)
}

func TestMigrateUpBootstrap(t *testing.T) {
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner)

	code, stdout, stderr := runBothy(t, env, "migrate", "up", "--bootstrap")
	want := "bootstrap: roles and grants are in place\n" + appliedLines(t)
	need := requiredVersion(t)
	if code != 0 || stdout != want+fmt.Sprintf("database version %d, applied %d\n", need, need) {
		t.Fatalf("first run: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	code, stdout, stderr = runBothy(t, env, "migrate", "up", "--bootstrap")
	if code != 0 || stdout != fmt.Sprintf("bootstrap: roles and grants are in place\ndatabase version %d, applied 0\n", need) {
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
	need := requiredVersion(t)
	var pending, applied strings.Builder
	for _, m := range embeddedMigrations(t) {
		fmt.Fprintf(&pending, "%d %s pending -\n", m.version, m.name)
		fmt.Fprintf(&applied, `%d %s applied \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\n`, m.version, regexp.QuoteMeta(m.name))
	}
	if want := pending.String() + fmt.Sprintf("database version 0, binary requires %d, pending %d\n", need, need); code != 0 || stdout != want {
		t.Fatalf("before: exit %d, stderr %q, stdout\n%s\nwant\n%s", code, stderr, stdout, want)
	}
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("up: exit %d, stderr %q", code, stderr)
	}
	// bothy_app may read the status as well.
	code, stdout, stderr = runBothy(t, migrateEnvFor(db, pg.RoleApp), "migrate", "status")
	pattern := regexp.MustCompile("^" + applied.String() + fmt.Sprintf(`database version %d, binary requires %d, pending 0\n$`, need, need))
	if code != 0 || !pattern.MatchString(stdout) {
		t.Fatalf("after: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
}

func TestMigrateNewerSchemaNotice(t *testing.T) {
	t.Parallel()
	newer := fstest.MapFS{nextMigrationName(t, 1, "from_a_newer_binary"): {Data: []byte("-- +goose Up\nCREATE TABLE newer_marker (id int);\n")}}
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
	locked, alter := nextMigrationName(t, 1, "locked_table"), nextMigrationName(t, 2, "alter_locked")
	fsys := fstest.MapFS{
		locked: {Data: []byte("-- +goose Up\nCREATE TABLE locked_table (id int);\n")},
		alter:  {Data: []byte("-- +goose Up\nALTER TABLE locked_table ADD COLUMN extra int;\n")},
	}
	first := fstest.MapFS{locked: fsys[locked]}
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
