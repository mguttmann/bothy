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

// appPrivileges is the expectation list of docs/spec/16-betrieb.md 7.1 rule 3: per table in
// schema public exactly the rights of bothy_app that 03 and 16 provide. A migration that creates
// a table adds its row here.
var appPrivileges = map[string][]string{
	"goose_db_version": {"SELECT"},
	// instance: bothy migrate writes it as bothy_owner; the server reads it (M1-15), plus the
	// column right UPDATE (setup_completed_at) of bothy setup (M1-37), which has_table_privilege
	// does not show; TestEnrollmentCodesMigration_InstanceSetupGrant holds it.
	"instance": {"SELECT"},
	// sequence_counters: plus the column right UPDATE (value), which has_table_privilege does
	// not show; TestCounters_ValueNeverDecreases holds it.
	"sequence_counters": {"SELECT"},
	// 03 Grundsatz 7: principals are never hard deleted, owners are revoked, a users row lives
	// with its principal (M1-26).
	"principals": {"SELECT", "INSERT", "UPDATE"},
	"users":      {"SELECT", "INSERT", "UPDATE"},
	"owners":     {"SELECT", "INSERT", "UPDATE"},
	// 19 SEC-AUD-1: the audit log is append-only for bothy_app (M1-24).
	"audit_events": {"SELECT", "INSERT"},
	// plus the column right UPDATE (key_ciphertext, destroyed_at) for the destruction of a key,
	// which has_table_privilege does not show; TestSECT16_AppRoleCannotUpdateOrDelete holds it.
	"audit_ip_keys": {"SELECT", "INSERT"},
	// 03 audit_checkpoints (M1-50): rights as audit_events, plus the column right UPDATE
	// (exported_at) of the export, which has_table_privilege does not show;
	// TestSECT16_CheckpointsAppendOnly holds it.
	"audit_checkpoints": {"SELECT", "INSERT"},
	// 03 section 4.3, R-DM-09: groups are soft deleted, memberships removed by removed_at and
	// grants revoked by revoked_at; the rows are history (M1-27).
	"groups":            {"SELECT", "INSERT", "UPDATE"},
	"group_memberships": {"SELECT", "INSERT", "UPDATE"},
	"grants":            {"SELECT", "INSERT", "UPDATE"},
	// 03 event_log, R-DM-18: the envelope is append-only for bothy_app; redaction adds its column
	// grant with M1-58, and partitions change only through the SECURITY DEFINER functions (M1-16).
	"event_log": {"SELECT", "INSERT"},
	// 03 idempotency_keys (M1-20): the stage inserts, completes and replaces expired rows, the
	// cleanup job deletes them.
	"idempotency_keys": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 section 4.2: devices are revoked by state, keys by revoked_at; the rows are security
	// history, deleted only by the retention job of class security, which grants it (M1-33).
	"devices":     {"SELECT", "INSERT", "UPDATE"},
	"device_keys": {"SELECT", "INSERT", "UPDATE"},
	// 03 event_consumer_cursors: a cursor row is never deleted in operation; restore finalize
	// moves it (M2-01, M7-04).
	"event_consumer_cursors": {"SELECT", "INSERT", "UPDATE"},
	// 03 server_signing_keys (M1-32): keys change by state and not_after; a retired row stays as
	// the record of a key that signed, deleted only by the retention job of class security.
	"server_signing_keys": {"SELECT", "INSERT", "UPDATE"},
	// 03 section 4.2: sessions and families are revoked by revoked_at, tokens marked by used_at;
	// the rows are security history, deleted only by the cleanup and retention jobs, which grant
	// their delete (M1-34).
	"auth_sessions":          {"SELECT", "INSERT", "UPDATE"},
	"refresh_token_families": {"SELECT", "INSERT", "UPDATE"},
	"refresh_tokens":         {"SELECT", "INSERT", "UPDATE"},
	// 03 auth_challenges (M1-35): transient single-use challenges; the job identity.auth_cleanup
	// deletes the expired rows.
	"auth_challenges": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 section 4.2 (M1-36): a provider without identities may be deleted (08 section 3.3); an
	// identity is unlinked by unlinked_at and stays as history.
	"identity_providers": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"user_identities":    {"SELECT", "INSERT", "UPDATE"},
	// 03 api_tokens (M1-44): revocation and last_used_at are updates, the rows are security
	// history.
	"api_tokens": {"SELECT", "INSERT", "UPDATE"},
	// 03 enrollment_codes (M1-37): a code ends by used_at, expires_at or attempts; the rows are
	// security history, deleted only by the retention job, which grants its delete.
	"enrollment_codes": {"SELECT", "INSERT", "UPDATE"},
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
