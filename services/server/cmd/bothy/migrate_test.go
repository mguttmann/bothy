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
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
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
		testPublicURL,
	}
	// An extra entry replaces the default of the same variable; the loader refuses duplicates.
	for _, e := range extra {
		name, _, _ := strings.Cut(e, "=")
		env = slices.DeleteFunc(env, func(have string) bool { return strings.HasPrefix(have, name+"=") })
		env = append(env, e)
	}
	return env
}

// embeddedMigration is one file of the embedded set: its version and the slug of its name.
type embeddedMigration struct {
	version int
	slug    string
}

// embeddedMigrations returns a copy of the embedded migration set and its files in version
// order. Tests derive expected output and the versions of their own extra migrations from it, so
// a ticket that adds a migration does not have to touch them (M1-26).
func embeddedMigrations(t *testing.T) (fstest.MapFS, []embeddedMigration) {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	set := fstest.MapFS{}
	list := make([]embeddedMigration, 0, len(names))
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		set[n] = &fstest.MapFile{Data: b}
		num, slug, _ := strings.Cut(strings.TrimSuffix(n, ".sql"), "_")
		v, err := strconv.Atoi(num)
		if err != nil {
			t.Fatalf("migration %s: %v", n, err)
		}
		list = append(list, embeddedMigration{version: v, slug: slug})
	}
	slices.SortFunc(list, func(a, b embeddedMigration) int { return a.version - b.version })
	if len(list) == 0 {
		t.Fatal("no embedded migrations")
	}
	return set, list
}

// migrationFile names a migration file of version v.
func migrationFile(v int, slug string) string { return fmt.Sprintf("%05d_%s.sql", v, slug) }

// riverVersion returns the highest River migration version this binary carries.
func riverVersion(t *testing.T) int {
	t.Helper()
	n, err := jobs.CheckEmbeddedMigrations()
	if err != nil || n < 1 {
		t.Fatalf("CheckEmbeddedMigrations = %d, %v", n, err)
	}
	return n
}

// TestMigrate_RiverInOwnSchema: with jobs.schema = jobs_it, migrate up creates River's tables in
// jobs_it and none in the domain schema, runs River before goose, changes nothing on a second
// run, and status and validate report River (upgrade.md rule M11, R-AR-09).
func TestMigrate_RiverInOwnSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner, "BOTHY_JOBS__SCHEMA=jobs_it")
	river := riverVersion(t)
	_, list := embeddedMigrations(t)
	highest := list[len(list)-1].version
	code, stdout, stderr := runBothy(t, env, "migrate", "up")
	want := fmt.Sprintf("river schema jobs_it version %d, applied %d\n", river, river)
	for _, m := range list {
		want += fmt.Sprintf("applied %d %s\n", m.version, m.slug)
	}
	want += fmt.Sprintf("database version %d, applied %d\n", highest, len(list))
	if code != 0 || stdout != want {
		t.Fatalf("first up: exit %d, stderr %q, stdout\n%s\nwant\n%s", code, stderr, stdout, want)
	}
	super := db.SuperConn(t)
	var inSchema, inPublic int
	if err := super.QueryRow(ctx, `SELECT count(*) FILTER (WHERE table_schema = 'jobs_it'),
		count(*) FILTER (WHERE table_schema = 'public' AND table_name LIKE 'river%')
		FROM information_schema.tables`).Scan(&inSchema, &inPublic); err != nil {
		t.Fatal(err)
	}
	if inSchema == 0 || inPublic != 0 {
		t.Fatalf("tables in jobs_it %d (want some), river tables in public %d (want 0)", inSchema, inPublic)
	}
	for _, tbl := range []string{"river_job", "river_leader", "river_queue", "river_migration"} {
		var ok bool
		if err := super.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "jobs_it."+tbl).Scan(&ok); err != nil || !ok {
			t.Errorf("jobs_it.%s missing (%v)", tbl, err)
		}
	}
	var riverLast, gooseFirst time.Time
	if err := super.QueryRow(ctx, "SELECT max(created_at) FROM jobs_it.river_migration").Scan(&riverLast); err != nil {
		t.Fatal(err)
	}
	if err := super.QueryRow(ctx, "SELECT min(tstamp) FROM goose_db_version WHERE version_id > 0").Scan(&gooseFirst); err != nil {
		t.Fatal(err)
	}
	if !riverLast.Before(gooseFirst) {
		t.Fatalf("River's last migration at %s is not before goose's first at %s", riverLast, gooseFirst)
	}
	// bothy_app has the rights River needs at run time, also on tables of a later River migration.
	app := db.Pool(t, pg.RoleApp)
	if _, err := app.Exec(ctx, "SELECT count(*) FROM jobs_it.river_job"); err != nil {
		t.Fatalf("bothy_app reads river_job: %v", err)
	}
	code, stdout, stderr = runBothy(t, env, "migrate", "up")
	if want := fmt.Sprintf("river schema jobs_it version %d, applied 0\ndatabase version %d, applied 0\n", river, highest); code != 0 || stdout != want {
		t.Fatalf("second up: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	code, stdout, stderr = runBothy(t, env, "migrate", "status")
	if want := fmt.Sprintf("river schema jobs_it version %d, binary requires %d, pending 0\n", river, river); code != 0 || !strings.HasSuffix(stdout, want) {
		t.Fatalf("status: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	if code, stdout, stderr := runBothy(t, env, "migrate", "validate"); code != 0 || !strings.Contains(stdout, "river migrations ok") {
		t.Fatalf("validate: exit %d, stderr %q, stdout %q", code, stderr, stdout)
	}
}

func TestMigrateUpBootstrap(t *testing.T) {
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner)

	_, list := embeddedMigrations(t)
	highest := list[len(list)-1].version

	code, stdout, stderr := runBothy(t, env, "migrate", "up", "--bootstrap")
	river := riverVersion(t)
	want := "bootstrap: roles and grants are in place\n" + fmt.Sprintf("river schema jobs version %d, applied %d\n", river, river)
	for _, m := range list {
		want += fmt.Sprintf("applied %d %s\n", m.version, m.slug)
	}
	want += fmt.Sprintf("database version %d, applied %d\n", highest, len(list))
	if code != 0 || stdout != want {
		t.Fatalf("first run: exit %d, stderr %q, stdout\n%s\nwant\n%s", code, stderr, stdout, want)
	}
	code, stdout, stderr = runBothy(t, env, "migrate", "up", "--bootstrap")
	want = fmt.Sprintf("bootstrap: roles and grants are in place\nriver schema jobs version %d, applied 0\ndatabase version %d, applied 0\n", river, highest)
	if code != 0 || stdout != want {
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
	_, list := embeddedMigrations(t)
	highest := list[len(list)-1].version
	want, applied := "", "^"
	for _, m := range list {
		want += fmt.Sprintf("%d %s pending -\n", m.version, m.slug)
		applied += fmt.Sprintf(`%d %s applied \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\n`, m.version, regexp.QuoteMeta(m.slug))
	}
	want += fmt.Sprintf("database version 0, binary requires %d, pending %d\n", highest, len(list))
	applied += fmt.Sprintf(`database version %d, binary requires %d, pending 0\n`, highest, highest)
	code, stdout, stderr := runBothy(t, env, "migrate", "status")
	river := riverVersion(t)
	want += fmt.Sprintf("river schema jobs version 0, binary requires %d, pending %d\n", river, river)
	if code != 0 || stdout != want {
		t.Fatalf("before: exit %d, stderr %q, stdout\n%s\nwant\n%s", code, stderr, stdout, want)
	}
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("up: exit %d, stderr %q", code, stderr)
	}
	// bothy_app may read the status as well.
	code, stdout, stderr = runBothy(t, migrateEnvFor(db, pg.RoleApp), "migrate", "status")
	pattern := regexp.MustCompile(applied + fmt.Sprintf("river schema jobs version %d, binary requires %d, pending 0\n$", river, river))
	if code != 0 || !pattern.MatchString(stdout) {
		t.Fatalf("after: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
}

func TestMigrateNewerSchemaNotice(t *testing.T) {
	t.Parallel()
	newer, list := embeddedMigrations(t)
	newer[migrationFile(list[len(list)-1].version+1, "from_a_newer_binary")] = &fstest.MapFile{
		Data: []byte("-- +goose Up\nCREATE TABLE newer_marker (id int);\n"),
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
	if want := fmt.Sprintf("migrations ok\nriver migrations ok, version %d\n", riverVersion(t)); code != 0 || stdout != want || stderr != "" {
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
	fsys, list := embeddedMigrations(t)
	first, _ := embeddedMigrations(t)
	next := list[len(list)-1].version + 1
	locked := &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE locked_table (id int);\n")}
	fsys[migrationFile(next, "locked_table")] = locked
	first[migrationFile(next, "locked_table")] = locked
	fsys[migrationFile(next+1, "alter_locked")] = &fstest.MapFile{
		Data: []byte("-- +goose Up\nALTER TABLE locked_table ADD COLUMN extra int;\n"),
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
