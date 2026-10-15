// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestSigningKeysMigration_Schema holds server_signing_keys against docs/spec/03-data-model.md
// section 4.1 column by column, with every check and index (M1-32).
func TestSigningKeysMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	wantCols := []principalColumn{
		{"kid", "text", false, ""},
		{"purpose", "text", false, ""},
		{"alg", "text", false, ""},
		{"public_jwk", "jsonb", false, ""},
		{"private_key_ref", "text", false, ""},
		{"state", "text", false, "'active'::text"},
		{"created_at", ts, false, "now()"},
		{"not_after", ts, true, ""},
	}
	if got := principalColumns(t, conn, "server_signing_keys"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, wantCols)
	}
	wantCons := []string{
		"CHECK ((alg = ANY (ARRAY['EdDSA'::text, 'ES256'::text])))",
		"CHECK (((purpose <> ALL (ARRAY['invocation'::text, 'terminal'::text, 'webhook_out'::text])) OR (alg = 'EdDSA'::text)))",
		"CHECK ((purpose = ANY (ARRAY['access_token'::text, 'invocation'::text, 'terminal'::text, 'webhook_out'::text, 'audit_checkpoint'::text])))",
		"CHECK ((state = ANY (ARRAY['next'::text, 'active'::text, 'retiring'::text, 'retired'::text])))",
		"PRIMARY KEY (kid)",
	}
	slices.Sort(wantCons)
	gotCons := principalConstraints(t, conn, "server_signing_keys")
	slices.Sort(gotCons)
	if !reflect.DeepEqual(gotCons, wantCons) {
		t.Errorf("constraints:\n got %q\nwant %q", gotCons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX server_signing_keys_purpose_state_idx ON public.server_signing_keys USING btree (purpose, state)",
		"CREATE UNIQUE INDEX server_signing_keys_active_purpose_key ON public.server_signing_keys USING btree (purpose) WHERE (state = 'active'::text)",
		"CREATE UNIQUE INDEX server_signing_keys_pkey ON public.server_signing_keys USING btree (kid)",
	}
	slices.Sort(wantIdx)
	gotIdx := indexes(t, conn, "server_signing_keys")
	slices.Sort(gotIdx)
	if !reflect.DeepEqual(gotIdx, wantIdx) {
		t.Errorf("indexes:\n got %q\nwant %q", gotIdx, wantIdx)
	}
}

// TestSigningKeysMigration_Constraints exercises the rules of 03 as bothy_app: purpose, alg and
// state checks, EdDSA only for invocation, terminal and webhook_out, one active key per purpose,
// no delete for bothy_app.
func TestSigningKeysMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	ins := `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state) VALUES ($1, $2, $3, '{}', 'file:/k', $4)`
	wantCheckViolation(t, exec(app, ins, "k1", "session", "EdDSA", "active"), "purpose session")
	wantCheckViolation(t, exec(app, ins, "k2", "access_token", "RS256", "active"), "alg RS256")
	wantCheckViolation(t, exec(app, ins, "k3", "access_token", "EdDSA", "expired"), "state expired")
	for _, p := range []string{"invocation", "terminal", "webhook_out"} {
		wantCheckViolation(t, exec(app, ins, "k4-"+p, p, "ES256", "active"), "ES256 for "+p)
	}
	mustExec(t, app, ins, "k5", "access_token", "ES256", "retired")
	mustExec(t, app, ins, "k6", "access_token", "EdDSA", "active")
	wantState(t, exec(app, ins, "k7", "access_token", "EdDSA", "active"), "23505", "second active access_token key")
	mustExec(t, app, ins, "k8", "access_token", "EdDSA", "next")
	mustExec(t, app, ins, "k9", "invocation", "EdDSA", "active")
	mustExec(t, app, `UPDATE server_signing_keys SET state = 'retiring', not_after = now() WHERE kid = 'k6'`)
	mustExec(t, app, `UPDATE server_signing_keys SET state = 'active' WHERE kid = 'k8'`)
	var state string
	if err := app.QueryRow(context.Background(), `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref)
		VALUES ('k10', 'webhook_out', 'EdDSA', '{}', 'file:/k') RETURNING state`).Scan(&state); err != nil || state != "active" {
		t.Fatalf("default state %q, %v", state, err)
	}
	wantState(t, exec(app, `DELETE FROM server_signing_keys WHERE kid = 'k5'`), "42501", "bothy_app deletes a key")
}

// TestSigningKeysMigration_Down: down to version 9 removes the table, up applies again.
func TestSigningKeysMigration_Down(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 9); err != nil {
		t.Fatalf("down to 9: %v", err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('server_signing_keys') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("server_signing_keys survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
