// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestLocalCredentialsMigration_Schema holds local_credentials against
// docs/spec/03-datenmodell.md section 4.2 column by column, with both checks, the foreign key and
// the primary key (M1-39).
func TestLocalCredentialsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"identity_id", "uuid", false, ""},
		{"password_hash", "text", false, ""},
		{"password_changed_at", ts, false, "now()"},
		{"totp_secret_ciphertext", "bytea", true, ""},
		{"totp_key_ref", "text", true, ""},
		{"totp_last_step", "bigint", true, ""},
		{"totp_enabled_at", ts, true, ""},
		{"failed_count", "integer", false, "0"},
		{"fail_window_started_at", ts, true, ""},
		{"locked_until", ts, true, ""},
	}
	constraints := []string{
		"CHECK (((failed_count = 0) = (fail_window_started_at IS NULL)))",
		"CHECK (((totp_secret_ciphertext IS NULL) = (totp_key_ref IS NULL)))",
		"FOREIGN KEY (identity_id) REFERENCES user_identities(id) ON DELETE CASCADE",
		"PRIMARY KEY (identity_id)",
	}
	if got := principalColumns(t, conn, "local_credentials"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "local_credentials")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	want := []string{"CREATE UNIQUE INDEX local_credentials_pkey ON public.local_credentials USING btree (identity_id)"}
	if got := indexes(t, conn, "local_credentials"); !reflect.DeepEqual(got, want) {
		t.Errorf("indexes:\n got %q\nwant %q", got, want)
	}
}

// TestLocalCredentialsMigration_Constraints exercises the rules of 03 as bothy_app: both checks,
// the foreign key with its delete rule and that bothy_app may not delete a row.
func TestLocalCredentialsMigration_Constraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a01"
	const provider = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a02"
	const ident = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a03"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'local', 'Local', '{}')`, provider)
	mustExec(t, app, `INSERT INTO user_identities (id, user_id, provider_id, subject) VALUES ($1, $2, $3, 'alex')`, ident, person, provider)
	ins := `INSERT INTO local_credentials (identity_id, password_hash, totp_secret_ciphertext, totp_key_ref, failed_count, fail_window_started_at)
		VALUES ($1, '$argon2id$v=19$m=19456,t=2,p=1$x$y', $2, $3, $4, $5)`
	wantCheckViolation(t, exec(app, ins, ident, []byte{1}, nil, 0, nil), "ciphertext without key ref")
	wantCheckViolation(t, exec(app, ins, ident, nil, "file:/k", 0, nil), "key ref without ciphertext")
	wantCheckViolation(t, exec(app, ins, ident, nil, nil, 1, nil), "failures without a window")
	wantCheckViolation(t, exec(app, ins, ident, nil, nil, 0, "2026-10-14T09:00:00Z"), "a window without failures")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9aff", nil, nil, 0, nil), "23503", "unknown identity")
	mustExec(t, app, ins, ident, []byte{1}, "file:/k", 2, "2026-10-14T09:00:00Z")
	wantState(t, exec(app, `INSERT INTO local_credentials (identity_id, password_hash) VALUES ($1, 'x')`, ident), "23505", "second row of an identity")
	wantState(t, exec(app, `DELETE FROM local_credentials WHERE identity_id = $1`, ident), "42501", "bothy_app deletes a row")
	// The identity goes: its credentials go.
	mustExec(t, owner, `DELETE FROM user_identities WHERE id = $1`, ident)
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM local_credentials`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("credentials of a deleted identity: %d, %v", n, err)
	}
}
