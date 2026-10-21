// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/moby/moby/api/types/network"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
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
	if info.Config.Labels[watchdogLabel] != watchdogLabelValue {
		t.Errorf("label %s = %q, want %q", watchdogLabel, info.Config.Labels[watchdogLabel], watchdogLabelValue)
	}
	if !info.HostConfig.AutoRemove {
		t.Error("the container lacks AutoRemove, so a stopped container would stay")
	}
}

func TestNewRequiresMain(t *testing.T) {
	// Not parallel: it switches the package state that New reads.
	setActive(false)
	_, err := sharedCluster(clusterKey{})
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
	testkit.Integration(t)
	t.Parallel()
	broken := fstest.MapFS{"00001_broken.sql": {Data: []byte("-- +goose Up\nSELECT * FROM no_such_table;\n")}}
	c, err := sharedCluster(clusterKey{})
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
	for _, name := range []string{"db_owner_password", "db_app_password", "db_maint_password", "db_monitor_password", "db_backup_password", "db_superuser_password"} {
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

// TestDurableKeepsDurabilitySettings: a database from Durable runs with the PostgreSQL defaults
// for fsync, synchronous_commit and full_page_writes (on), the measurement condition of ADR 0063
// point 2; a database without the option keeps the speed flags (off).
func TestDurableKeepsDurabilitySettings(t *testing.T) {
	t.Parallel()
	settings := []string{"fsync", "synchronous_commit", "full_page_writes"}
	for _, tc := range []struct {
		name string
		db   *DB
		want string
	}{
		{"durable", New(t, Durable()), "on"},
		{"default", New(t), "off"},
	} {
		conn := tc.db.SuperConn(t)
		for _, s := range settings {
			var got string
			if err := conn.QueryRow(context.Background(), "SHOW "+s).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("%s database: %s = %s, want %s", tc.name, s, got, tc.want)
			}
		}
	}
	durable, plain := New(t, Durable(), WithoutMigrations()), New(t, WithoutMigrations())
	if durable.Config(pg.RoleApp).Port == plain.Config(pg.RoleApp).Port {
		t.Error("durable and speed databases share one server")
	}
}

// TestRestartKeepsHostPort: a durable database survives a restart of its container on the same
// loopback host port with its data, and a database with Limits runs with those container limits.
func TestRestartKeepsHostPort(t *testing.T) {
	t.Parallel()
	db := New(t, WithoutMigrations(), Durable(), Limits(1, 512<<20))
	ctx := context.Background()
	before := db.Config(pg.RoleOwner).Port
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(ctx, "CREATE TABLE kept (n int); INSERT INTO kept VALUES (7)"); err != nil {
		t.Fatal(err)
	}
	info, err := db.c.ctr.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.HostConfig.NanoCPUs != 1e9 || info.HostConfig.Memory != 512<<20 {
		t.Errorf("limits: %d nano CPUs, %d bytes; want 1e9 and %d", info.HostConfig.NanoCPUs, info.HostConfig.Memory, 512<<20)
	}
	if err := db.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	if after := db.Config(pg.RoleOwner).Port; after != before {
		t.Fatalf("host port %d after the restart, want %d", after, before)
	}
	conn := db.SuperConn(t)
	var n int
	if err := conn.QueryRow(ctx, "SELECT n FROM kept").Scan(&n); err != nil || n != 7 {
		t.Fatalf("data after the restart: %d, %v", n, err)
	}
	info, err = db.c.ctr.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range info.NetworkSettings.Ports[network.MustParsePort("5432/tcp")] {
		if b.HostIP.String() != "127.0.0.1" || b.HostPort != strconv.Itoa(before) {
			t.Errorf("published %s:%s after the restart, want 127.0.0.1:%d", b.HostIP, b.HostPort, before)
		}
	}
}
