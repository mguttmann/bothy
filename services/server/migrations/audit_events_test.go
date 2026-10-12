// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// auditRowInsert inserts one audit row with seq n through q (a pool or a connection).
const auditRowInsert = `INSERT INTO audit_events (seq, id, action, decision, prev_hash, hash)
	VALUES ($1::bigint, gen_random_uuid(), 'grant.create', 'allow', '\x00'::bytea, decode(md5($1::text), 'hex'))`

// TestMigration_AuditTablesMatchSpec compares audit_events and audit_ip_keys with
// docs/spec/03-data-model.md 4.12: columns, types, nullability, defaults, checks, keys and
// indexes.
func TestMigration_AuditTablesMatchSpec(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)

	wantEvents := []column{
		{"seq", "bigint", "NO", ""},
		{"id", "uuid", "NO", ""},
		{"occurred_at", "timestamp with time zone", "NO", "now()"},
		{"actor_principal_id", "uuid", "YES", ""},
		{"actor_device_id", "uuid", "YES", ""},
		{"actor_session_id", "uuid", "YES", ""},
		{"action", "text", "NO", ""},
		{"decision", "text", "NO", ""},
		{"deny_reason", "text", "YES", ""},
		{"resource_kind", "text", "YES", ""},
		{"resource_id", "uuid", "YES", ""},
		{"details", "jsonb", "NO", "'{}'::jsonb"},
		{"correlation_id", "uuid", "YES", ""},
		{"prev_hash", "bytea", "NO", ""},
		{"hash", "bytea", "NO", ""},
	}
	if got := columns(t, super, "audit_events"); !slices.Equal(got, wantEvents) {
		t.Errorf("audit_events columns\n got %v\nwant %v", got, wantEvents)
	}
	wantEventCons := []string{
		"c CHECK ((decision = ANY (ARRAY['allow'::text, 'deny'::text, 'error'::text])))",
		"p PRIMARY KEY (seq)", "u UNIQUE (hash)", "u UNIQUE (id)",
	}
	if got := constraints(t, super, "audit_events"); !slices.Equal(got, wantEventCons) {
		t.Errorf("audit_events constraints\n got %v\nwant %v", got, wantEventCons)
	}
	wantIndexes := []string{
		"CREATE INDEX audit_events_action_idx ON public.audit_events USING btree (action, occurred_at DESC)",
		"CREATE INDEX audit_events_actor_idx ON public.audit_events USING btree (actor_principal_id, occurred_at DESC)",
		"CREATE INDEX audit_events_occurred_at_brin ON public.audit_events USING brin (occurred_at)",
		"CREATE INDEX audit_events_resource_idx ON public.audit_events USING btree (resource_kind, resource_id)",
	}
	rows, err := super.Query(context.Background(), `SELECT indexdef FROM pg_indexes
		WHERE tablename = 'audit_events' AND indexdef NOT LIKE 'CREATE UNIQUE%' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	indexes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(indexes, wantIndexes) {
		t.Errorf("audit_events indexes\n got %v\nwant %v", indexes, wantIndexes)
	}

	wantKeys := []column{
		{"id", "uuid", "NO", ""},
		{"valid_from", "timestamp with time zone", "NO", ""},
		{"valid_until", "timestamp with time zone", "NO", ""},
		{"key_ciphertext", "bytea", "YES", ""},
		{"key_ref", "text", "NO", ""},
		{"created_at", "timestamp with time zone", "NO", "now()"},
		{"destroyed_at", "timestamp with time zone", "YES", ""},
	}
	if got := columns(t, super, "audit_ip_keys"); !slices.Equal(got, wantKeys) {
		t.Errorf("audit_ip_keys columns\n got %v\nwant %v", got, wantKeys)
	}
	wantKeyCons := []string{
		"c CHECK (((key_ciphertext IS NULL) = (destroyed_at IS NOT NULL)))",
		"c CHECK ((valid_until > valid_from))",
		"p PRIMARY KEY (id)", "u UNIQUE (valid_from)",
	}
	if got := constraints(t, super, "audit_ip_keys"); !slices.Equal(got, wantKeyCons) {
		t.Errorf("audit_ip_keys constraints\n got %v\nwant %v", got, wantKeyCons)
	}
}

// sqlstate returns the SQLSTATE of err, or "".
func sqlstate(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// TestSECT16_AppRoleCannotUpdateOrDelete is the application-role part of SEC-T16 (SEC-AUD-1):
// bothy_app may insert and read audit rows, and UPDATE, DELETE and TRUNCATE fail with 42501
// because the rights are revoked, before any trigger runs.
func TestSECT16_AppRoleCannotUpdateOrDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	app := db.Pool(t, pg.RoleApp)
	if _, err := app.Exec(ctx, auditRowInsert, 1); err != nil {
		t.Fatalf("bothy_app cannot insert an audit row: %v", err)
	}
	var n int
	if err := app.QueryRow(ctx, "SELECT count(*) FROM audit_events").Scan(&n); err != nil || n != 1 {
		t.Fatalf("bothy_app reads %d rows: %v", n, err)
	}
	for _, q := range []string{
		"UPDATE audit_events SET action = 'x'",
		"DELETE FROM audit_events",
		"TRUNCATE audit_events",
	} {
		_, err := app.Exec(ctx, q)
		if sqlstate(err) != "42501" || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s as bothy_app: %v, want permission denied (42501)", q, err)
		}
	}
	// The IP keys: bothy_app may change only key_ciphertext and destroyed_at.
	if _, err := app.Exec(ctx, "UPDATE audit_ip_keys SET key_ref = 'x'"); sqlstate(err) != "42501" {
		t.Errorf("UPDATE audit_ip_keys.key_ref as bothy_app: %v, want 42501", err)
	}
	if _, err := app.Exec(ctx, "UPDATE audit_ip_keys SET key_ciphertext = NULL, destroyed_at = now()"); err != nil {
		t.Errorf("UPDATE of the destruction columns as bothy_app: %v", err)
	}
	if _, err := app.Exec(ctx, "DELETE FROM audit_ip_keys"); sqlstate(err) != "42501" {
		t.Errorf("DELETE FROM audit_ip_keys as bothy_app: %v, want 42501", err)
	}
}

// TestAudit_TriggerBlocksOwnerUpdate: the table owner bothy_owner holds every right, and the
// triggers still refuse UPDATE, DELETE and TRUNCATE; only a DELETE by bothy_maint passes the
// trigger (the retention trim of M7, which grants bothy_maint the right itself).
func TestAudit_TriggerBlocksOwnerUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(ctx, auditRowInsert, 1); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"UPDATE audit_events SET action = 'x'",
		"DELETE FROM audit_events",
		"TRUNCATE audit_events",
	} {
		_, err := owner.Exec(ctx, q)
		if err == nil || !strings.Contains(err.Error(), "audit_events is append-only") {
			t.Errorf("%s as bothy_owner: %v, want the trigger to refuse", q, err)
		}
	}
	maint := db.Pool(t, pg.RoleMaint)
	if _, err := maint.Exec(ctx, "DELETE FROM audit_events"); sqlstate(err) != "42501" || strings.Contains(err.Error(), "append-only") {
		t.Errorf("DELETE as bothy_maint without grant: %v, want permission denied before the trigger", err)
	}
	super := db.SuperConn(t)
	if _, err := super.Exec(ctx, "GRANT SELECT, DELETE ON audit_events TO bothy_maint"); err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Exec(ctx, "DELETE FROM audit_events WHERE seq = 1"); err != nil {
		t.Errorf("DELETE as bothy_maint with grant: %v, want the trigger to let it pass", err)
	}
	if _, err := maint.Exec(ctx, "TRUNCATE audit_events"); err == nil {
		t.Error("TRUNCATE as bothy_maint passed")
	}
}
