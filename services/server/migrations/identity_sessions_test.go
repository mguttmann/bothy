// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestSessionsMigration_Schema holds auth_sessions, refresh_token_families and refresh_tokens
// against docs/spec/03-data-model.md section 4.2 column by column, with every check, foreign key
// and index (M1-34), including the foreign key auth_sessions.identity_id that the migration of
// user_identities adds (M1-36).
func TestSessionsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	tests := []struct {
		table       string
		columns     []principalColumn
		constraints []string
		indexes     []string
	}{
		{
			table: "auth_sessions",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"user_id", "uuid", false, ""},
				{"device_id", "uuid", false, ""},
				{"client_component", "text", false, "'app'::text"},
				{"parent_session_id", "uuid", true, ""},
				{"identity_id", "uuid", true, ""},
				{"auth_method", "text", false, ""},
				{"cookie_hash", "bytea", true, ""},
				{"created_at", ts, false, "now()"},
				{"expires_at", ts, false, ""},
				{"step_up_at", ts, true, ""},
				{"step_up_method", "text", true, ""},
				{"step_up_consumed_at", ts, true, ""},
				{"last_seen_at", ts, true, ""},
				{"last_ip", "inet", true, ""},
				{"revoked_at", ts, true, ""},
				{"revoke_reason", "text", true, ""},
			},
			constraints: []string{
				"CHECK (((client_component = 'console'::text) = (cookie_hash IS NOT NULL)))",
				"CHECK (((parent_session_id IS NULL) OR (client_component = 'share_ext'::text)))",
				"CHECK (((step_up_at IS NULL) = (step_up_method IS NULL)))",
				"CHECK (((step_up_consumed_at IS NULL) OR (step_up_at IS NOT NULL)))",
				"CHECK ((auth_method = ANY (ARRAY['oidc'::text, 'trusted_header'::text, 'passkey'::text, 'password_totp'::text, 'enrollment_code'::text, 'device_pairing'::text])))",
				"CHECK ((client_component = ANY (ARRAY['app'::text, 'share_ext'::text, 'console'::text])))",
				"CHECK ((revoke_reason = ANY (ARRAY['logout'::text, 'admin'::text, 'device_revoked'::text, 'refresh_reuse'::text, 'user_disabled'::text, 'expired'::text, 'inactive'::text, 'step_up_failed'::text, 'trusted_header_mismatch'::text])))",
				"CHECK ((step_up_method = ANY (ARRAY['passkey'::text, 'device_key'::text, 'password_totp'::text, 'oidc_reauth'::text])))",
				"FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE RESTRICT",
				"FOREIGN KEY (identity_id) REFERENCES user_identities(id) ON DELETE SET NULL",
				"FOREIGN KEY (parent_session_id) REFERENCES auth_sessions(id) ON DELETE CASCADE",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE INDEX auth_sessions_device_id_idx ON public.auth_sessions USING btree (device_id)",
				"CREATE INDEX auth_sessions_expires_at_idx ON public.auth_sessions USING btree (expires_at)",
				"CREATE INDEX auth_sessions_parent_session_id_idx ON public.auth_sessions USING btree (parent_session_id) WHERE (parent_session_id IS NOT NULL)",
				"CREATE INDEX auth_sessions_user_id_active_idx ON public.auth_sessions USING btree (user_id) WHERE (revoked_at IS NULL)",
				"CREATE UNIQUE INDEX auth_sessions_cookie_hash_key ON public.auth_sessions USING btree (cookie_hash) WHERE (cookie_hash IS NOT NULL)",
				"CREATE UNIQUE INDEX auth_sessions_pkey ON public.auth_sessions USING btree (id)",
			},
		},
		{
			table: "refresh_token_families",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"session_id", "uuid", false, ""},
				{"dpop_jkt", "text", false, ""},
				{"current_generation", "integer", false, "0"},
				{"rotated_at", ts, true, ""},
				{"created_at", ts, false, "now()"},
				{"revoked_at", ts, true, ""},
				{"revoke_reason", "text", true, ""},
			},
			constraints: []string{
				"CHECK ((revoke_reason = ANY (ARRAY['reuse_detected'::text, 'logout'::text, 'session_revoked'::text, 'expired'::text])))",
				"FOREIGN KEY (session_id) REFERENCES auth_sessions(id) ON DELETE CASCADE",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX refresh_token_families_active_session_key ON public.refresh_token_families USING btree (session_id) WHERE (revoked_at IS NULL)",
				"CREATE UNIQUE INDEX refresh_token_families_pkey ON public.refresh_token_families USING btree (id)",
			},
		},
		{
			table: "refresh_tokens",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"family_id", "uuid", false, ""},
				{"generation", "integer", false, ""},
				{"token_hash", "bytea", false, ""},
				{"issued_at", ts, false, "now()"},
				{"expires_at", ts, false, ""},
				{"used_at", ts, true, ""},
			},
			constraints: []string{
				"FOREIGN KEY (family_id) REFERENCES refresh_token_families(id) ON DELETE CASCADE",
				"PRIMARY KEY (id)",
				"UNIQUE (family_id, generation)",
				"UNIQUE (token_hash)",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX refresh_tokens_family_id_generation_key ON public.refresh_tokens USING btree (family_id, generation)",
				"CREATE UNIQUE INDEX refresh_tokens_pkey ON public.refresh_tokens USING btree (id)",
				"CREATE UNIQUE INDEX refresh_tokens_token_hash_key ON public.refresh_tokens USING btree (token_hash)",
			},
		},
	}
	for _, tt := range tests {
		if got := principalColumns(t, conn, tt.table); !reflect.DeepEqual(got, tt.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tt.table, got, tt.columns)
		}
		want := slices.Clone(tt.constraints)
		slices.Sort(want)
		got := principalConstraints(t, conn, tt.table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tt.table, got, want)
		}
		want = slices.Clone(tt.indexes)
		slices.Sort(want)
		got = indexes(t, conn, tt.table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", tt.table, got, want)
		}
	}
}

// TestSessionsMigration_Constraints exercises the rules of 03 as bothy_app: the table checks of
// auth_sessions, one active family per session, the unique token_hash and (family_id,
// generation), the cascades from a session to its children, families and tokens, and that
// bothy_app cannot delete.
func TestSessionsMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6a01"
	const dev = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6a02"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, dev, person)
	id := func(n byte) string {
		return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6b" + string("0123456789abcdef"[n>>4]) + string("0123456789abcdef"[n&15])
	}
	end := time.Now().Add(time.Hour)
	ins := `INSERT INTO auth_sessions (id, user_id, device_id, client_component, parent_session_id, auth_method, cookie_hash,
		expires_at, step_up_at, step_up_method, step_up_consumed_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	sess := func(n byte, comp string, parent any, method string, cookie any, upAt, upMethod, consumed any) error {
		return exec(app, ins, id(n), person, dev, comp, parent, method, cookie, end, upAt, upMethod, consumed)
	}
	mustExec(t, app, ins, id(1), person, dev, "app", nil, "oidc", nil, end, nil, nil, nil)
	wantCheckViolation(t, sess(2, "console", nil, "oidc", nil, nil, nil, nil), "console without cookie_hash")
	wantCheckViolation(t, sess(3, "app", nil, "oidc", []byte{1}, nil, nil, nil), "cookie_hash on an app session")
	wantCheckViolation(t, sess(4, "app", id(1), "oidc", nil, nil, nil, nil), "parent on an app session")
	wantCheckViolation(t, sess(5, "app", nil, "oidc", nil, time.Now(), nil, nil), "step_up_at without method")
	wantCheckViolation(t, sess(6, "app", nil, "oidc", nil, nil, "passkey", nil), "step_up_method without time")
	wantCheckViolation(t, sess(7, "app", nil, "oidc", nil, nil, nil, time.Now()), "consumed without step-up")
	wantCheckViolation(t, sess(8, "app", nil, "magic", nil, nil, nil, nil), "auth_method magic")
	wantCheckViolation(t, sess(9, "widget", nil, "oidc", nil, nil, nil, nil), "client_component widget")
	wantCheckViolation(t, exec(app, `UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'stolen' WHERE id = $1`, id(1)), "revoke_reason stolen")
	wantState(t, exec(app, `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', $4)`,
		id(10), person, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6aff", end), "23503", "unknown device")
	mustExec(t, app, ins, id(11), person, dev, "share_ext", id(1), "oidc", nil, end, time.Now(), "device_key", time.Now())
	mustExec(t, app, ins, id(12), person, dev, "console", nil, "passkey", []byte{1}, end, nil, nil, nil)
	wantState(t, sess(13, "console", nil, "passkey", []byte{1}, nil, nil, nil), "23505", "cookie_hash taken")
	var comp string
	if err := app.QueryRow(context.Background(), `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at)
		VALUES ($1, $2, $3, 'oidc', $4) RETURNING client_component`, id(14), person, dev, end).Scan(&comp); err != nil || comp != "app" {
		t.Fatalf("default client_component %q, %v", comp, err)
	}

	fam := `INSERT INTO refresh_token_families (id, session_id, dpop_jkt) VALUES ($1, $2, 'jkt')`
	mustExec(t, app, fam, id(20), id(1))
	wantState(t, exec(app, fam, id(21), id(1)), "23505", "second active family of a session")
	wantCheckViolation(t, exec(app, `UPDATE refresh_token_families SET revoked_at = now(), revoke_reason = 'stolen' WHERE id = $1`, id(20)), "family reason stolen")
	mustExec(t, app, `UPDATE refresh_token_families SET revoked_at = now(), revoke_reason = 'logout' WHERE id = $1`, id(20))
	mustExec(t, app, fam, id(22), id(1))
	mustExec(t, app, fam, id(23), id(11))
	var gen int
	if err := app.QueryRow(context.Background(), `SELECT current_generation FROM refresh_token_families WHERE id = $1`, id(22)).Scan(&gen); err != nil || gen != 0 {
		t.Fatalf("default current_generation %d, %v", gen, err)
	}

	tok := `INSERT INTO refresh_tokens (id, family_id, generation, token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`
	mustExec(t, app, tok, id(30), id(22), 0, []byte("h0"), end)
	wantState(t, exec(app, tok, id(31), id(22), 1, []byte("h0"), end), "23505", "token_hash taken")
	wantState(t, exec(app, tok, id(32), id(22), 0, []byte("h1"), end), "23505", "generation taken in the family")
	mustExec(t, app, tok, id(33), id(23), 0, []byte("h2"), end)

	wantState(t, exec(app, `DELETE FROM refresh_tokens WHERE id = $1`, id(30)), "42501", "bothy_app deletes a token")
	wantState(t, exec(app, `DELETE FROM auth_sessions WHERE id = $1`, id(1)), "42501", "bothy_app deletes a session")
	wantState(t, exec(owner, `DELETE FROM devices WHERE id = $1`, dev), "23001", "device with sessions deleted (ON DELETE RESTRICT)")
	mustExec(t, owner, `DELETE FROM auth_sessions WHERE id = $1`, id(1))
	for _, q := range []string{
		`SELECT count(*) FROM auth_sessions WHERE id = '` + id(11) + `'`,
		`SELECT count(*) FROM refresh_token_families WHERE session_id IN ('` + id(1) + `', '` + id(11) + `')`,
		`SELECT count(*) FROM refresh_tokens`,
	} {
		var n int
		if err := owner.QueryRow(context.Background(), q).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s = %d, %v (want 0 through ON DELETE CASCADE)", q, n, err)
		}
	}
}

// TestSessionsMigration_Down holds the Down section: down to version 10 removes the three tables,
// up applies again.
func TestSessionsMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 10); err != nil {
		t.Fatalf("down to 10: %v", err)
	}
	for _, rel := range []string{"auth_sessions", "refresh_token_families", "refresh_tokens"} {
		var exists bool
		if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", rel).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Errorf("table %s survives the down migration", rel)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT 1 FROM refresh_tokens LIMIT 1").Scan(new(int)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("refresh_tokens after up again: %v", err)
	}
}
