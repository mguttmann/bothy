// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestLoginFlowsMigration_Schema holds auth_login_flows against docs/spec/03-data-model.md section
// 4.2 column by column, with every check, foreign key, unique and index (M1-41).
func TestLoginFlowsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"provider_id", "uuid", false, ""},
		{"mode", "text", false, ""},
		{"user_id", "uuid", true, ""},
		{"device_id", "uuid", true, ""},
		{"oidc_state_hash", "bytea", false, ""},
		{"oidc_nonce_hash", "bytea", false, ""},
		{"client_challenge", "text", false, ""},
		{"app_redirect_uri", "text", false, ""},
		{"login_code_hash", "bytea", true, ""},
		{"login_code_expires_at", ts, true, ""},
		{"enrollment_code_id", "uuid", true, ""},
		{"external_identity", "jsonb", true, ""},
		{"status", "text", false, "'started'::text"},
		{"error", "text", true, ""},
		{"created_at", ts, false, "now()"},
		{"expires_at", ts, false, ""},
	}
	constraints := []string{
		"CHECK ((mode = ANY (ARRAY['login'::text, 'link'::text])))",
		"CHECK ((status = ANY (ARRAY['started'::text, 'code_issued'::text, 'completed'::text, 'failed'::text])))",
		"CHECK (((status = 'code_issued'::text) = (external_identity IS NOT NULL)))",
		"CHECK (((mode <> 'link'::text) OR (enrollment_code_id IS NULL)))",
		"FOREIGN KEY (enrollment_code_id) REFERENCES enrollment_codes(id) ON DELETE SET NULL",
		"FOREIGN KEY (provider_id) REFERENCES identity_providers(id) ON DELETE CASCADE",
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
		"UNIQUE (oidc_state_hash)",
	}
	idx := []string{
		"CREATE INDEX auth_login_flows_expires_at_idx ON public.auth_login_flows USING btree (expires_at)",
		"CREATE UNIQUE INDEX auth_login_flows_login_code_hash_key ON public.auth_login_flows USING btree (login_code_hash) WHERE (login_code_hash IS NOT NULL)",
		"CREATE UNIQUE INDEX auth_login_flows_oidc_state_hash_key ON public.auth_login_flows USING btree (oidc_state_hash)",
		"CREATE UNIQUE INDEX auth_login_flows_pkey ON public.auth_login_flows USING btree (id)",
	}
	if got := principalColumns(t, conn, "auth_login_flows"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "auth_login_flows")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "auth_login_flows")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestLoginFlowsMigration_Constraints exercises the rules of 03 as bothy_app: the external
// identity exactly with code_issued, no enrollment code in link mode, the unique hashes, the
// delete rules of the foreign keys, and that bothy_app may delete expired flows (cleanup job).
func TestLoginFlowsMigration_Constraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a01"
	const prov = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a02"
	const code = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a03"
	end := time.Now().Add(time.Hour)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', 'Entra', '{}')`, prov)
	mustExec(t, app, `INSERT INTO enrollment_codes (id, user_id, purpose, code_hash, expires_at) VALUES ($1, $2, 'identity_link', $3, $4)`,
		code, person, []byte{1}, end)
	ins := `INSERT INTO auth_login_flows (id, provider_id, mode, user_id, oidc_state_hash, oidc_nonce_hash, client_challenge,
		app_redirect_uri, enrollment_code_id, expires_at) VALUES ($1, $2, $3, $4, $5, $6, 'c', 'com.example.app:/cb', $7, $8)`
	const f1, f2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a11", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a12"
	mustExec(t, app, ins, f1, prov, "login", nil, []byte{1}, []byte{1}, code, end)
	wantCheckViolation(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a13", prov, "link", person, []byte{2}, []byte{2}, code, end), "link with code")
	wantCheckViolation(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a14", prov, "relink", nil, []byte{3}, []byte{3}, nil, end), "mode relink")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a15", prov, "login", nil, []byte{1}, []byte{4}, nil, end), "23505", "state hash taken")
	mustExec(t, app, ins, f2, prov, "link", person, []byte{5}, []byte{5}, nil, end)
	// code_issued needs the external identity, every other status forbids it.
	wantCheckViolation(t, exec(app, `UPDATE auth_login_flows SET status = 'code_issued' WHERE id = $1`, f1), "code_issued without identity")
	mustExec(t, app, `UPDATE auth_login_flows SET status = 'code_issued', external_identity = '{"schema":"external_identity.v1"}',
		login_code_hash = $2, login_code_expires_at = $3 WHERE id = $1`, f1, []byte{7}, end)
	wantCheckViolation(t, exec(app, `UPDATE auth_login_flows SET status = 'completed' WHERE id = $1`, f1), "completed with identity")
	wantState(t, exec(app, `UPDATE auth_login_flows SET login_code_hash = $2 WHERE id = $1`, f2, []byte{7}), "23505", "login code hash taken")
	// The code goes: the reference becomes NULL. The person goes: its link flows go.
	mustExec(t, owner, `DELETE FROM enrollment_codes WHERE id = $1`, code)
	var ref *string
	if err := owner.QueryRow(ctx, `SELECT enrollment_code_id::text FROM auth_login_flows WHERE id = $1`, f1).Scan(&ref); err != nil || ref != nil {
		t.Fatalf("enrollment_code_id after the code went: %v, %v", ref, err)
	}
	mustExec(t, owner, `DELETE FROM users WHERE principal_id = $1`, person)
	mustExec(t, app, `DELETE FROM auth_login_flows WHERE expires_at > now()`)
	mustExec(t, owner, `DELETE FROM identity_providers WHERE id = $1`, prov)
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM auth_login_flows`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("flows left: %d, %v", n, err)
	}
}
