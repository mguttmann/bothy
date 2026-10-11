// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The password files pgtest writes, by role.
var secretFiles = map[pg.Role]string{
	pg.RoleOwner: "db_owner_password", pg.RoleApp: "db_app_password",
	pg.RoleMaint: "db_maint_password", pg.RoleMonitor: "db_monitor_password",
}

// pgtestPasswords reads the role passwords pgtest bootstrapped.
func pgtestPasswords(t *testing.T, db *pgtest.DB) map[pg.Role]secrets.Value {
	t.Helper()
	out := map[pg.Role]secrets.Value{}
	for r, f := range secretFiles {
		b, err := os.ReadFile(filepath.Join(db.SecretsDir(), f)) // #nosec G304 -- the files pgtest wrote
		if err != nil {
			t.Fatal(err)
		}
		out[r] = secrets.NewValue([]byte(strings.TrimSuffix(string(b), "\n")))
	}
	return out
}

// restorePasswords sets the pgtest passwords again when the test ends; tests that change a
// role password run alone (not parallel) and call it first.
func restorePasswords(t *testing.T, db *pgtest.DB) {
	t.Helper()
	original := pgtestPasswords(t, db)
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pg.Connect(ctx, db.SuperConfig(), providerFor(t, db.SecretsDir()), 10*time.Second, nil)
		if err != nil {
			t.Errorf("restore the role passwords: %v", err)
			return
		}
		defer func() { _ = conn.Close(ctx) }()
		if err := pg.BootstrapRoles(ctx, conn, original); err != nil {
			t.Errorf("restore the role passwords: %v", err)
		}
	})
}

func providerFor(t *testing.T, dir string) *secrets.Provider {
	t.Helper()
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

// login connects as role with password and reports the error.
func login(t *testing.T, db *pgtest.DB, role pg.Role, password string) error {
	t.Helper()
	dir := t.TempDir()
	writeSecret(t, filepath.Join(dir, "pw"), password)
	cfg := db.Config(role)
	cfg.PasswordRef = "file:" + filepath.Join(dir, "pw")
	conn, err := pg.Connect(context.Background(), cfg, providerFor(t, dir), 0, nil)
	if err == nil {
		_ = conn.Close(context.Background())
	}
	return err
}

// TestBootstrapIdempotent is not parallel: it rotates a shared role password.
func TestBootstrapIdempotent(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t)
	restorePasswords(t, db)
	super := db.SuperConn(t)
	passwords := pgtestPasswords(t, db)

	// An operator widened a role by hand; the bootstrap narrows it again.
	if _, err := super.Exec(ctx, "ALTER ROLE bothy_maint CREATEDB CREATEROLE"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := pg.Bootstrap(ctx, super, db.Name(), passwords); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := super.Query(ctx, `SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication,
		rolbypassrls, rolpassword LIKE 'SCRAM-SHA-256$4096:%' FROM pg_authid WHERE rolname LIKE 'bothy\_%' ORDER BY rolname`)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for rows.Next() {
		var name string
		var login, super, createdb, createrole, replication, bypass, scram bool
		if err := rows.Scan(&name, &login, &super, &createdb, &createrole, &replication, &bypass, &scram); err != nil {
			t.Fatal(err)
		}
		seen++
		if !login || super || createdb || createrole || replication || bypass || !scram {
			t.Errorf("role %s: login %v superuser %v createdb %v createrole %v replication %v bypassrls %v scram %v",
				name, login, super, createdb, createrole, replication, bypass, scram)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen != 4 {
		t.Errorf("%d bothy roles, want 4", seen)
	}
	var monitor bool
	if err := super.QueryRow(ctx, "SELECT pg_has_role('bothy_monitor', 'pg_monitor', 'MEMBER')").Scan(&monitor); err != nil || !monitor {
		t.Errorf("bothy_monitor is not a member of pg_monitor (%v)", err)
	}
	for r, pw := range passwords {
		if err := login(t, db, r, string(pw.Bytes())); err != nil {
			t.Errorf("role %s cannot log in with its password: %v", r, err)
		}
	}

	// A changed secret and another run rotate the password.
	rotated := pgtestPasswords(t, db)
	rotated[pg.RoleMaint] = secrets.NewValue([]byte("rotated-EXAMPLE-maint"))
	if err := pg.Bootstrap(ctx, super, db.Name(), rotated); err != nil {
		t.Fatal(err)
	}
	if err := login(t, db, pg.RoleMaint, string(passwords[pg.RoleMaint].Bytes())); err == nil {
		t.Error("the old password still works after the rotation")
	}
	if err := login(t, db, pg.RoleMaint, "rotated-EXAMPLE-maint"); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}
}

// TestBootstrapNeverSendsPlaintext is not parallel: it sets canary passwords on the shared roles.
func TestBootstrapNeverSendsPlaintext(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	restorePasswords(t, db)
	super := db.SuperConn(t)
	if _, err := super.Exec(ctx, "SET log_statement = 'all'"); err != nil {
		t.Fatal(err)
	}
	canaries := map[pg.Role]secrets.Value{}
	for _, r := range pg.Roles() {
		canaries[r] = secrets.NewValue([]byte("canary-EXAMPLE-" + strings.TrimPrefix(string(r), "bothy_") + "-" + db.Name()))
	}
	if err := pg.Bootstrap(ctx, super, db.Name(), canaries); err != nil {
		t.Fatal(err)
	}
	// The bootstrap took effect, so the statements that set the passwords did run.
	if err := login(t, db, pg.RoleMaint, string(canaries[pg.RoleMaint].Bytes())); err != nil {
		t.Fatalf("the canary password does not work: %v", err)
	}
	logs := ""
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if logs = db.ContainerLogs(t); strings.Contains(logs, `ROLE "bothy_monitor" WITH`) {
			break
		}
	}
	if !strings.Contains(logs, `ROLE "bothy_monitor" WITH`) {
		t.Fatal("the server log does not show the logged role statements; the test would prove nothing")
	}
	for r, v := range canaries {
		if strings.Contains(logs, string(v.Bytes())) {
			t.Errorf("the server log contains the plaintext password of %s", r)
		}
	}
	if !strings.Contains(logs, "PASSWORD 'SCRAM-SHA-256$4096:") {
		t.Error("the logged role statements carry no SCRAM-SHA-256 verifier")
	}
}

func TestBootstrapDatabaseGrants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	super := db.SuperConn(t)
	var dbOwner, schemaOwner string
	if err := super.QueryRow(ctx, "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = current_database()").Scan(&dbOwner); err != nil {
		t.Fatal(err)
	}
	if err := super.QueryRow(ctx, "SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'public'").Scan(&schemaOwner); err != nil {
		t.Fatal(err)
	}
	if dbOwner != "bothy_owner" || schemaOwner != "bothy_owner" {
		t.Errorf("database owner %s, schema owner %s; want bothy_owner for both", dbOwner, schemaOwner)
	}
	checks := []struct {
		query string
		want  bool
	}{
		{"SELECT has_database_privilege('bothy_app', current_database(), 'CONNECT')", true},
		{"SELECT has_database_privilege('bothy_maint', current_database(), 'CONNECT')", true},
		{"SELECT has_database_privilege('bothy_monitor', current_database(), 'CONNECT')", true},
		{"SELECT has_database_privilege('bothy_app', current_database(), 'CREATE')", false},
		{"SELECT has_database_privilege('bothy_app', current_database(), 'TEMPORARY')", false},
		{"SELECT has_schema_privilege('bothy_app', 'public', 'USAGE')", true},
		{"SELECT has_schema_privilege('bothy_maint', 'public', 'USAGE')", true},
		{"SELECT has_schema_privilege('bothy_monitor', 'public', 'USAGE')", false},
		{"SELECT has_schema_privilege('bothy_app', 'public', 'CREATE')", false},
		// PUBLIC (grantee 0) keeps no right on the database or the schema.
		{"SELECT EXISTS (SELECT 1 FROM pg_database, aclexplode(datacl) a WHERE datname = current_database() AND a.grantee = 0)", false},
		{"SELECT EXISTS (SELECT 1 FROM pg_namespace, aclexplode(nspacl) a WHERE nspname = 'public' AND a.grantee = 0)", false},
		// No default privileges: each migration grants on its own tables.
		{"SELECT EXISTS (SELECT 1 FROM pg_default_acl)", false},
	}
	for _, c := range checks {
		var got bool
		if err := super.QueryRow(ctx, c.query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.query, err)
		}
		if got != c.want {
			t.Errorf("%s = %v, want %v", c.query, got, c.want)
		}
	}
}

func TestBootstrapWrongDatabase(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	err := pg.BootstrapDatabase(context.Background(), db.SuperConn(t), "other_database")
	if err == nil || !strings.Contains(err.Error(), db.Name()) || !strings.Contains(err.Error(), "other_database") {
		t.Errorf("BootstrapDatabase on the wrong connection: %v, want an error naming both databases", err)
	}
}

func TestBootstrapRejectsBadInput(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	passwords := pgtestPasswords(t, db)
	delete(passwords, pg.RoleMonitor)
	err := pg.Bootstrap(context.Background(), db.SuperConn(t), db.Name(), passwords)
	if err == nil || !strings.Contains(err.Error(), "bothy_monitor") {
		t.Errorf("Bootstrap without the monitor password: %v, want an error naming the role", err)
	}
	// A quote in a database name is quoted as an identifier, not executed.
	if err := pg.BootstrapDatabase(context.Background(), db.SuperConn(t), `x"; DROP DATABASE postgres; --`); err == nil {
		t.Error("BootstrapDatabase accepted a database it is not connected to")
	}
	var n int
	if err := db.SuperConn(t).QueryRow(context.Background(), "SELECT count(*) FROM pg_database WHERE datname = 'postgres'").Scan(&n); err != nil || n != 1 {
		t.Errorf("database postgres: %d, %v", n, err)
	}
}
