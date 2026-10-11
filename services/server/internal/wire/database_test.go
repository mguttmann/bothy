// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/wire"
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
		_, err := config.Load(schema(t), config.Sources{Env: []string{"BOTHY_DATABASE__SSLMODE=" + mode}})
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
	}
	refs := map[pg.Role]string{
		pg.RoleOwner: "file:/srv/secrets/db_owner_password", pg.RoleApp: "file:/srv/secrets/db_app_password",
		pg.RoleMaint: "file:/srv/secrets/db_maint_password", pg.RoleMonitor: "file:/srv/secrets/db_monitor_password",
	}
	if s.LockTimeout != want.LockTimeout || s.DependencyWait != want.DependencyWait || s.BootstrapUser != want.BootstrapUser ||
		s.BootstrapPasswordRef != want.BootstrapPasswordRef || len(s.PasswordRefs()) != 4 {
		t.Fatalf("MigrateSettings = %+v\nwant %+v", s, want)
	}
	for r, ref := range refs {
		if got := s.PasswordRefs()[r]; got != ref {
			t.Errorf("password of %s: %s, want %s", r, got, ref)
		}
	}
}

func TestMigrationSteps(t *testing.T) {
	t.Parallel()
	// M1-14 (River) and M1-15 (instance row) add their steps here; until then there are none.
	opts := wire.MigrationSteps()
	if len(opts.Before) != 0 || len(opts.After) != 0 {
		t.Errorf("MigrationSteps = %+v", opts)
	}
}
