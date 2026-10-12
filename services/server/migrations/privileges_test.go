// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// appPrivileges is the expectation list of docs/spec/16-operations.md 7.1 rule 3: per table in
// schema public exactly the rights of bothy_app that 03 and 16 provide. A migration that creates
// a table adds its row here.
var appPrivileges = map[string][]string{
	"goose_db_version": {"SELECT"},
	// instance: bothy migrate writes it as bothy_owner; the server only reads it (M1-15).
	"instance": {"SELECT"},
	// sequence_counters: plus the column right UPDATE (value), which has_table_privilege does
	// not show; TestCounters_ValueNeverDecreases holds it.
	"sequence_counters": {"SELECT"},
	// 03 "Principles" item 7: principals are never hard deleted, owners are revoked, a users row lives
	// with its principal (M1-26).
	"principals": {"SELECT", "INSERT", "UPDATE"},
	"users":      {"SELECT", "INSERT", "UPDATE"},
	"owners":     {"SELECT", "INSERT", "UPDATE"},
	// 19 SEC-AUD-1: the audit log is append-only for bothy_app (M1-24).
	"audit_events": {"SELECT", "INSERT"},
	// plus the column right UPDATE (key_ciphertext, destroyed_at) for the destruction of a key,
	// which has_table_privilege does not show; TestSECT16_AppRoleCannotUpdateOrDelete holds it.
	"audit_ip_keys": {"SELECT", "INSERT"},
	// 03 section 4.3, R-DM-09: groups are soft deleted, memberships removed by removed_at and
	// grants revoked by revoked_at; the rows are history (M1-27).
	"groups":            {"SELECT", "INSERT", "UPDATE"},
	"group_memberships": {"SELECT", "INSERT", "UPDATE"},
	"grants":            {"SELECT", "INSERT", "UPDATE"},
}

// allPrivileges are the table rights PostgreSQL 18 knows (MAINTAIN since 17).
var allPrivileges = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"}

// TestPrivilegesAfterAllMigrations holds the grant rule of README.md for every migration, also
// the ones later tickets add: after all migrations every table in schema public belongs to
// bothy_owner, bothy_app may read it but holds none of TRUNCATE, REFERENCES and TRIGGER, PUBLIC
// holds nothing, and the version table of goose is read-only for bothy_app. bothy_app cannot
// create tables. A table whose migration forgot its grant fails here.
func TestPrivilegesAfterAllMigrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)

	rows, err := super.Query(ctx, `SELECT c.relname, pg_get_userbyid(c.relowner), c.relkind::text
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f') ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	type relation struct{ name, owner, kind string }
	var relations []relation
	for rows.Next() {
		var r relation
		if err := rows.Scan(&r.name, &r.owner, &r.kind); err != nil {
			t.Fatal(err)
		}
		relations = append(relations, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	has := func(role, table, privilege string) bool {
		t.Helper()
		var ok bool
		if err := super.QueryRow(ctx, "SELECT has_table_privilege($1, quote_ident($2), $3)", role, table, privilege).Scan(&ok); err != nil {
			t.Fatalf("has_table_privilege(%s, %s, %s): %v", role, table, privilege, err)
		}
		return ok
	}
	sawVersionTable := false
	for _, r := range relations {
		if r.owner != string(pg.RoleOwner) {
			t.Errorf("%s belongs to %s, want %s", r.name, r.owner, pg.RoleOwner)
		}
		var public bool
		if err := super.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_class c, aclexplode(c.relacl) a
			WHERE c.oid = to_regclass(quote_ident($1)) AND a.grantee = 0)`, r.name).Scan(&public); err != nil {
			t.Fatal(err)
		}
		if public {
			t.Errorf("PUBLIC holds a right on %s", r.name)
		}
		if r.kind == "S" {
			continue // sequences: the rule covers tables
		}
		sawVersionTable = sawVersionTable || r.name == "goose_db_version"
		want, listed := appPrivileges[r.name]
		if !listed {
			t.Errorf("table %s has no row in appPrivileges; add the rights of bothy_app that 03 and 16 provide", r.name)
			continue
		}
		if !has("bothy_app", r.name, "SELECT") {
			t.Errorf("bothy_app cannot read %s: its migration lacks the grant", r.name)
		}
		for _, p := range allPrivileges {
			if got, expected := has("bothy_app", r.name, p), slices.Contains(want, p); got != expected {
				t.Errorf("bothy_app on %s: %s is %v, want %v", r.name, p, got, expected)
			}
		}
		for _, p := range []string{"TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"} {
			if slices.Contains(want, p) {
				t.Errorf("appPrivileges grants %s on %s; bothy_app never holds it", p, r.name)
			}
		}
	}
	if !sawVersionTable {
		t.Fatal("goose_db_version is missing from schema public; the test did not see the migrated schema")
	}

	app := db.Pool(t, pg.RoleApp)
	c, err := app.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	_, err = c.Exec(ctx, "CREATE TABLE app_may_not_create (id int)")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("CREATE TABLE as bothy_app: %v, want insufficient_privilege (42501)", err)
	}
}

func TestEmbeddedMigrationsValidate(t *testing.T) {
	t.Parallel()
	if err := pg.Validate(migrations.FS); err != nil {
		t.Fatal(err)
	}
}
