// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

func TestPgtestImagePinned(t *testing.T) {
	t.Parallel()
	if !regexp.MustCompile(`^postgres:18\.[0-9]+-[a-z]+@sha256:[0-9a-f]{64}$`).MatchString(Image) {
		t.Fatalf("Image %q is not a PostgreSQL 18 image pinned by digest", Image)
	}
	// The same pin as the database of the local SonarQube, so Renovate moves both together.
	compose, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "tools", "sonarqube", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compose), "image: "+Image+"\n") {
		t.Errorf("tools/sonarqube/compose.yaml does not pin %s", Image)
	}
}

func TestPgtestBindsLoopbackOnly(t *testing.T) {
	t.Parallel()
	db := New(t, WithoutMigrations())
	info, err := db.c.ctr.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.HostConfig == nil || len(info.HostConfig.PortBindings) == 0 {
		t.Fatal("the container has no port bindings")
	}
	for port, bindings := range info.HostConfig.PortBindings {
		for _, b := range bindings {
			if b.HostIP.String() != "127.0.0.1" {
				t.Errorf("port %s is bound on %q, want 127.0.0.1", port, b.HostIP)
			}
		}
	}
	published := 0
	for port, bindings := range info.NetworkSettings.Ports {
		for _, b := range bindings {
			published++
			if b.HostIP.String() != "127.0.0.1" {
				t.Errorf("port %s is published on %q, want 127.0.0.1", port, b.HostIP)
			}
			if b.HostPort == "" || b.HostPort == "5432" {
				t.Errorf("port %s is published on host port %q, want a random one", port, b.HostPort)
			}
		}
	}
	if published == 0 {
		t.Error("no published port")
	}
	if info.Config.Labels[label] != labelValue {
		t.Errorf("label %s = %q, want %q", label, info.Config.Labels[label], labelValue)
	}
}

func TestNewRequiresMain(t *testing.T) {
	// Not parallel: it switches the package state that New reads.
	setActive(false)
	_, err := sharedCluster()
	setActive(true)
	if err == nil || !strings.Contains(err.Error(), "pgtest.Main") {
		t.Fatalf("New without Main: error %v, want one naming pgtest.Main", err)
	}
}

func TestNewDatabasesAreIsolated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a, b := New(t), New(t)
	if a.Name() == b.Name() {
		t.Fatalf("two databases share the name %s", a.Name())
	}
	ca := a.SuperConn(t)
	if _, err := ca.Exec(ctx, "CREATE TABLE only_in_a (id int)"); err != nil {
		t.Fatal(err)
	}
	var found bool
	if err := b.SuperConn(t).QueryRow(ctx, "SELECT to_regclass('only_in_a') IS NOT NULL").Scan(&found); err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("a table of one database is visible in another")
	}
	// The template is migrated: the version table is there and bothy_app may read it.
	p := a.Pool(t, pg.RoleApp)
	var version int64
	if err := p.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	c, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	if err := c.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version < 1 {
		t.Errorf("schema version %d, want the migrated template", version)
	}
}

func TestDatabaseIsDroppedAtCleanup(t *testing.T) {
	t.Parallel()
	var name string
	t.Run("user", func(t *testing.T) {
		name = New(t, WithoutMigrations()).Name()
	})
	var exists bool
	keeper := New(t, WithoutMigrations())
	if err := keeper.SuperConn(t).QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Errorf("database %s still exists after the cleanup of its test", name)
	}
}

// TestFailedTemplateIsDropped: a migration set that fails leaves no template behind, so a second
// attempt reports the migration error again instead of "database already exists".
func TestFailedTemplateIsDropped(t *testing.T) {
	t.Parallel()
	broken := fstest.MapFS{"00001_broken.sql": {Data: []byte("-- +goose Up\nSELECT * FROM no_such_table;\n")}}
	c, err := sharedCluster()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for attempt := 1; attempt <= 2; attempt++ {
		err := c.createDatabase(ctx, "t_"+randomHex(8), broken)
		if err == nil {
			t.Fatalf("attempt %d: broken migration set accepted", attempt)
		}
		if strings.Contains(err.Error(), "already exists") {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	key, err := hashFS(broken)
	if err != nil {
		t.Fatal(err)
	}
	var exists bool
	keeper := New(t, WithoutMigrations())
	if err := keeper.SuperConn(t).QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", "tpl_"+key[:16]).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("the template of the failed migration set still exists")
	}
}

func TestSecretsDirHoldsRolePasswords(t *testing.T) {
	t.Parallel()
	db := New(t, WithoutMigrations())
	for _, name := range []string{"db_owner_password", "db_app_password", "db_maint_password", "db_monitor_password", "db_superuser_password"} {
		fi, err := os.Stat(filepath.Join(db.SecretsDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s has mode %o, want 0600", name, fi.Mode().Perm())
		}
	}
	for _, r := range pg.Roles() {
		if got := db.Config(r).User; got != string(r) {
			t.Errorf("Config(%s).User = %s", r, got)
		}
		if err := db.Pool(t, r).Ping(context.Background()); err != nil {
			t.Errorf("role %s cannot log in with its secret file: %v", r, err)
		}
	}
}
