// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// DatabaseConfig binds the database.* keys of v into the pool configuration of a process and
// names its connections appName, by convention "bothy-<role>" (bothy-api, bothy-worker,
// bothy-migrate), so pg_stat_activity shows which process holds a connection.
//
// The error names the key whose binding failed (config.Bind), never a value.
func DatabaseConfig(v *config.Values, appName string) (pg.Config, error) {
	var c pg.Config
	if err := config.Bind(v, &c); err != nil {
		return pg.Config{}, err
	}
	return c.WithAppName(appName), nil
}

// MigrateConfig holds the settings of bothy migrate besides the connection (brief M1-12 D3).
type MigrateConfig struct {
	// LockTimeout is migrate.lock_timeout, the lock_timeout of every migration step.
	LockTimeout time.Duration `config:"migrate.lock_timeout"`
	// DependencyWait is startup.dependency_wait, how long migrate waits for the database.
	DependencyWait time.Duration `config:"startup.dependency_wait"`
	// BootstrapUser is migrate.bootstrap_user, the superuser of --bootstrap.
	BootstrapUser string `config:"migrate.bootstrap_user"`
	// BootstrapPasswordRef is migrate.bootstrap_password_ref, its password.
	BootstrapPasswordRef string `config:"migrate.bootstrap_password_ref"`
	// OwnerPasswordRef is migrate.owner_password_ref.
	OwnerPasswordRef string `config:"migrate.owner_password_ref"`
	// AppPasswordRef is migrate.app_password_ref.
	AppPasswordRef string `config:"migrate.app_password_ref"`
	// MaintPasswordRef is migrate.maint_password_ref.
	MaintPasswordRef string `config:"migrate.maint_password_ref"`
	// MonitorPasswordRef is migrate.monitor_password_ref.
	MonitorPasswordRef string `config:"migrate.monitor_password_ref"`
}

// MigrateSettings binds the keys of bothy migrate from v.
//
// The error names the key whose binding failed (config.Bind), never a value.
func MigrateSettings(v *config.Values) (MigrateConfig, error) {
	var c MigrateConfig
	if err := config.Bind(v, &c); err != nil {
		return MigrateConfig{}, err
	}
	return c, nil
}

// PasswordRefs maps each role to the secret reference of the password --bootstrap sets for it.
func (c MigrateConfig) PasswordRefs() map[pg.Role]string {
	return map[pg.Role]string{
		pg.RoleOwner: c.OwnerPasswordRef, pg.RoleApp: c.AppPasswordRef,
		pg.RoleMaint: c.MaintPasswordRef, pg.RoleMonitor: c.MonitorPasswordRef,
	}
}

// MigrationSteps returns the steps bothy migrate up runs around the goose migrations: River's
// migrations before them (M1-14) and the instance row with schema_version and
// min_server_version after them (M1-15). Each of those tickets adds its step here.
func MigrationSteps() pg.MigratorOptions {
	return pg.MigratorOptions{}
}
