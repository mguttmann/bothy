// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/migrations"
)

func TestDatabaseConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := wire.DatabaseConfig(loadValues(t, schema(t)), "bothy-api")
	if err != nil {
		t.Fatal(err)
	}
	want := pg.Config{ // #nosec G101 -- a secret reference, not a credential
		Host: "localhost", Port: 5432, Name: "bothy", User: "bothy_app",
		PasswordRef: "file:/run/bothy/secrets/db_app_password", SSLMode: "verify-full", MaxConns: 20,
		AcquireTimeout: 2 * time.Second, StatementTimeout: 5 * time.Second, LockTimeout: 2 * time.Second,
		IdleInTxTimeout: 10 * time.Second,
	}.WithAppName("bothy-api")
	if cfg != want {
		t.Errorf("DatabaseConfig = %+v\nwant %+v", cfg, want)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("the defaults do not validate: %v", err)
	}
}

func TestDatabaseConfigBindsValues(t *testing.T) {
	t.Parallel()
	v := loadValues(t, schema(t),
		"BOTHY_DATABASE__HOST=/var/run/postgresql", "BOTHY_DATABASE__PORT=6543", "BOTHY_DATABASE__SSLMODE=disable",
		"BOTHY_DATABASE__USER=bothy_owner", "BOTHY_DATABASE__PASSWORD_REF=file:/run/bothy/secrets/db_owner_password",
		"BOTHY_DATABASE__SSLROOTCERT=/etc/ssl/db-ca.pem", "BOTHY_DATABASE__MAX_CONNS=3")
	cfg, err := wire.DatabaseConfig(v, "bothy-migrate")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "/var/run/postgresql" || cfg.Port != 6543 || cfg.SSLMode != "disable" || cfg.User != "bothy_owner" ||
		cfg.PasswordRef != "file:/run/bothy/secrets/db_owner_password" || cfg.SSLRootCert != "/etc/ssl/db-ca.pem" ||
		cfg.MaxConns != 3 || cfg.AppName() != "bothy-migrate" {
		t.Errorf("DatabaseConfig = %+v", cfg)
	}
}

func TestDatabaseSSLModeOnlyDisableOrVerifyFull(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"require", "prefer", "verify-ca", "allow"} {
		_, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL, "BOTHY_DATABASE__SSLMODE=" + mode}})
		if err == nil || !strings.Contains(err.Error(), "database.sslmode") {
			t.Errorf("sslmode %s: %v, want an error naming database.sslmode", mode, err)
		}
	}
}

func TestMigrateSettingsBinds(t *testing.T) {
	t.Parallel()
	s, err := wire.MigrateSettings(loadValues(t, schema(t), "BOTHY_MIGRATE__LOCK_TIMEOUT=750ms", "BOTHY_SECRETS__DIR=/srv/secrets"))
	if err != nil {
		t.Fatal(err)
	}
	want := wire.MigrateConfig{ // #nosec G101 -- secret references, not credentials
		LockTimeout: 750 * time.Millisecond, DependencyWait: time.Minute, BootstrapUser: "postgres",
		BootstrapPasswordRef: "file:/srv/secrets/db_superuser_password",
		RequireRecentBackup:  true, RecentBackupMaxAge: 24 * time.Hour,
	}
	refs := map[pg.Role]string{
		pg.RoleOwner: "file:/srv/secrets/db_owner_password", pg.RoleApp: "file:/srv/secrets/db_app_password",
		pg.RoleMaint: "file:/srv/secrets/db_maint_password", pg.RoleMonitor: "file:/srv/secrets/db_monitor_password",
		pg.RoleBackup: "file:/srv/secrets/db_backup_password",
	}
	if s.LockTimeout != want.LockTimeout || s.DependencyWait != want.DependencyWait || s.BootstrapUser != want.BootstrapUser ||
		s.BootstrapPasswordRef != want.BootstrapPasswordRef || s.RequireRecentBackup != want.RequireRecentBackup ||
		s.RecentBackupMaxAge != want.RecentBackupMaxAge || len(s.PasswordRefs()) != len(pg.Roles()) {
		t.Fatalf("MigrateSettings = %+v\nwant %+v", s, want)
	}
	for r, ref := range refs {
		if got := s.PasswordRefs()[r]; got != ref {
			t.Errorf("password of %s: %s, want %s", r, got, ref)
		}
	}
	s, err = wire.MigrateSettings(loadValues(t, schema(t), "BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP=false", "BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE=36h"))
	if err != nil {
		t.Fatal(err)
	}
	if s.RequireRecentBackup || s.RecentBackupMaxAge != 36*time.Hour {
		t.Errorf("RequireRecentBackup %v, RecentBackupMaxAge %s; want false, 36h", s.RequireRecentBackup, s.RecentBackupMaxAge)
	}
}

// instanceRow reads the instance row as the superuser; found is false when it is missing.
func instanceRow(t *testing.T, db *pgtest.DB) (schemaVersion int64, minServer string, instanceID string, found bool) {
	t.Helper()
	err := db.SuperConn(t).QueryRow(context.Background(),
		"SELECT schema_version, coalesce(min_server_version, ''), instance_id::text FROM instance").Scan(&schemaVersion, &minServer, &instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return schemaVersion, minServer, instanceID, true
}

// runAfterSteps builds the steps for fsys over a pool of bothy_owner on db and runs the after
// steps as bothy migrate up would.
func runAfterSteps(t *testing.T, db *pgtest.DB, fsys fs.FS) {
	t.Helper()
	pool := db.Pool(t, pg.RoleOwner)
	opts, err := wire.MigrationSteps(pool, fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Before) != 0 || len(opts.After) != 1 || opts.After[0].Name() != "instance" {
		t.Fatalf("MigrationSteps = %+v, want the instance step after the migrations", opts)
	}
	sqlDB := pool.OpenSQL()
	defer func() { _ = sqlDB.Close() }()
	if err := opts.After[0].Run(context.Background(), sqlDB); err != nil {
		t.Fatal(err)
	}
}

// TestMigrationSteps: the instance step records the goose version and migrations.MinServerVersion
// when the database is at the newest migration of the set, creates a missing row, and leaves the
// row alone when the database is newer than the set (upgrade.md "Rückweg").
func TestMigrationSteps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	need, err := pg.RequiredSchemaVersion(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	// pgtest.WithSteps(wire.MigrationSteps) in TestMain: the template already has the row.
	db := pgtest.New(t)
	v, minServer, id, found := instanceRow(t, db)
	if !found || v != need || minServer != migrations.MinServerVersion {
		t.Fatalf("row after migrate: found %v, schema %d, min %q", found, v, minServer)
	}
	if _, err := db.SuperConn(t).Exec(ctx, "DELETE FROM instance"); err != nil {
		t.Fatal(err)
	}
	runAfterSteps(t, db, migrations.FS)
	v, minServer, id2, found := instanceRow(t, db)
	if !found || v != need || minServer != migrations.MinServerVersion || id2 == id {
		t.Fatalf("row after the step: found %v, schema %d, min %q, id %s (was %s)", found, v, minServer, id2, id)
	}

	// A set whose newest migration is older than the database: the step changes nothing.
	older := fstest.MapFS{}
	first, err := fs.ReadFile(migrations.FS, "00001_base_privileges.sql")
	if err != nil {
		t.Fatal(err)
	}
	older["00001_base_privileges.sql"] = &fstest.MapFile{Data: first}
	if _, err := db.SuperConn(t).Exec(ctx, "UPDATE instance SET schema_version = 0, min_server_version = '7.0.0'"); err != nil {
		t.Fatal(err)
	}
	runAfterSteps(t, db, older)
	if v, minServer, _, _ := instanceRow(t, db); v != 0 || minServer != "7.0.0" {
		t.Fatalf("a step of an older set changed the row: schema %d, min %q", v, minServer)
	}
}

// TestMigrateUpSteps: bothy migrate up runs River's migrations before goose (M1-14, upgrade.md
// rule M11), the instance step after them (M1-15) and then the partitions of event_log (M1-16,
// upgrade.md rule M10). MigrationSteps, which pgtest.WithSteps uses for the templates, keeps only
// the instance step (docs/DECISIONS.md INT-W5/3, M1-16/7).
func TestMigrateUpSteps(t *testing.T) {
	t.Parallel()
	cfg, err := wire.SyncConfigFrom(syncValues(t))
	if err != nil {
		t.Fatal(err)
	}
	opts, err := wire.MigrateUpSteps(nil, migrations.FS, wire.MigrateUpConfig{JobsSchema: "jobs", Sync: cfg}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Before) != 1 || opts.Before[0].Name() != "river" || len(opts.After) != 2 ||
		opts.After[0].Name() != "instance" || opts.After[1].Name() != "event_log_partitions" {
		t.Errorf("MigrateUpSteps = %+v, want river before, instance and event_log_partitions after the migrations", opts)
	}
	if _, err := wire.MigrateUpSteps(nil, migrations.FS, wire.MigrateUpConfig{JobsSchema: "jobs"}, nil); err == nil {
		t.Error("MigrateUpSteps accepted a zero partition configuration")
	}
}

// TestMigrateUpSteps_PartitionStepCreatesPartitions runs the partition step on a database
// without partitions: afterwards sync.event_log_partitions_ahead partitions stand above the head.
func TestMigrateUpSteps_PartitionStepCreatesPartitions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	cfg, err := wire.SyncConfigFrom(syncValues(t, "BOTHY_SYNC__EVENT_LOG_PARTITIONS_AHEAD=2"))
	if err != nil {
		t.Fatal(err)
	}
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	opts, err := wire.MigrateUpSteps(pool, migrations.FS, wire.MigrateUpConfig{JobsSchema: "jobs", Sync: cfg}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := opts.After[1].Run(ctx, nil); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := db.SuperConn(t).QueryRow(ctx, "SELECT count(*) FROM pg_inherits WHERE inhparent = 'public.event_log'::regclass").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("%d partitions after the step ran twice, want 2", n)
	}
}
