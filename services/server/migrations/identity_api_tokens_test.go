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

// TestAPITokensMigration_Schema holds api_tokens against docs/spec/03-datenmodell.md section 4.2
// column by column, with the foreign keys, the unique token_hash and the partial index of the
// token list (M1-44).
func TestAPITokensMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"principal_id", "uuid", false, ""},
		{"name", "text", false, ""},
		{"token_prefix", "text", false, ""},
		{"token_hash", "bytea", false, ""},
		{"scopes", "ARRAY", false, "'{}'::text[]"},
		{"dpop_jkt", "text", true, ""},
		{"expires_at", ts, false, ""},
		{"created_by", "uuid", false, ""},
		{"created_at", ts, false, "now()"},
		{"last_used_at", ts, true, ""},
		{"revoked_at", ts, true, ""},
		{"revoked_by", "uuid", true, ""},
	}
	constraints := []string{
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (revoked_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
		"UNIQUE (token_hash)",
	}
	idx := []string{
		"CREATE INDEX api_tokens_principal_active_idx ON public.api_tokens USING btree (principal_id) WHERE (revoked_at IS NULL)",
		"CREATE UNIQUE INDEX api_tokens_pkey ON public.api_tokens USING btree (id)",
		"CREATE UNIQUE INDEX api_tokens_token_hash_key ON public.api_tokens USING btree (token_hash)",
	}
	if got := principalColumns(t, conn, "api_tokens"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "api_tokens")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "api_tokens")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestAPITokensMigration_Constraints exercises the rules of 03 as bothy_app: expires_at is
// required, token_hash unique, the principals exist, scopes default to the empty list, and
// bothy_app may update (revocation, last_used_at) but not delete.
func TestAPITokensMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const agent = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a01"
	const owner = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a02"
	const tok = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b01"
	end := time.Now().Add(time.Hour)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, agent)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, owner)
	ins := `INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, expires_at, created_by)
		VALUES ($1, $2, 'Adapter', 'bothy_agt_EXAMPL', $3, $4, $5)`
	var scopes []string
	if err := app.QueryRow(context.Background(), ins+` RETURNING scopes`, tok, agent, []byte{1}, end, owner).Scan(&scopes); err != nil || scopes == nil || len(scopes) != 0 {
		t.Fatalf("insert: scopes %v, %v (want the empty list)", scopes, err)
	}
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b02", agent, []byte{2}, nil, owner), "23502", "expires_at missing")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b03", agent, []byte{1}, end, owner), "23505", "token_hash taken")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b04", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8aff", []byte{4}, end, owner),
		"23503", "unknown principal")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b05", agent, []byte{5}, end, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8aff"),
		"23503", "unknown creator")
	mustExec(t, app, `UPDATE api_tokens SET revoked_at = now(), revoked_by = $2, last_used_at = now() WHERE id = $1`, tok, owner)
	wantState(t, exec(app, `DELETE FROM api_tokens WHERE id = $1`, tok), "42501", "bothy_app deletes")
}

// TestAPITokensMigration_Down holds the Down section: down to version 12 removes api_tokens, up
// applies again.
func TestAPITokensMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 12); err != nil {
		t.Fatalf("down to 12: %v", err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('api_tokens') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("api_tokens survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('api_tokens') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("api_tokens after up again: %v, %v", exists, err)
	}
}
