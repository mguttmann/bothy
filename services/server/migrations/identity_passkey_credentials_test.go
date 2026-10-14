// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPasskeyCredentialsMigration_Schema holds passkey_credentials against
// docs/spec/03-datenmodell.md section 4.2 column by column, with its foreign key, unique and index
// (M1-40).
func TestPasskeyCredentialsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"user_id", "uuid", false, ""},
		{"credential_id", "bytea", false, ""},
		{"public_key_cose", "bytea", false, ""},
		{"sign_count", "bigint", false, "0"},
		{"aaguid", "uuid", true, ""},
		{"transports", "ARRAY", false, "'{}'::text[]"},
		{"backup_eligible", "boolean", false, "false"},
		{"backup_state", "boolean", false, "false"},
		{"name", "text", false, ""},
		{"created_at", ts, false, "now()"},
		{"last_used_at", ts, true, ""},
		{"revoked_at", ts, true, ""},
	}
	constraints := []string{
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
		"UNIQUE (credential_id)",
	}
	idx := []string{
		"CREATE INDEX passkey_credentials_user_id_active_idx ON public.passkey_credentials USING btree (user_id) WHERE (revoked_at IS NULL)",
		"CREATE UNIQUE INDEX passkey_credentials_credential_id_key ON public.passkey_credentials USING btree (credential_id)",
		"CREATE UNIQUE INDEX passkey_credentials_pkey ON public.passkey_credentials USING btree (id)",
	}
	if got := principalColumns(t, conn, "passkey_credentials"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "passkey_credentials")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "passkey_credentials")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestPasskeyCredentialsMigration_Constraints exercises the rules of 03 as bothy_app: the unique
// credential ID, the person must exist, a person with passkeys cannot be deleted, and bothy_app
// may revoke but not delete a passkey.
func TestPasskeyCredentialsMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a01"
	const pk = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a02"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	ins := `INSERT INTO passkey_credentials (id, user_id, credential_id, public_key_cose, name) VALUES ($1, $2, $3, $4, 'Laptop')`
	mustExec(t, app, ins, pk, person, []byte{1}, []byte{2})
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a03", person, []byte{1}, []byte{3}), "23505", "credential_id taken")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a04", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9aff", []byte{4}, []byte{3}),
		"23503", "unknown person")
	mustExec(t, app, `UPDATE passkey_credentials SET revoked_at = now(), sign_count = 3, last_used_at = now() WHERE id = $1`, pk)
	wantState(t, exec(app, `DELETE FROM passkey_credentials WHERE id = $1`, pk), "42501", "bothy_app deletes a passkey")
	wantState(t, exec(owner, `DELETE FROM users WHERE principal_id = $1`, person), "23001", "a person with passkeys is deleted")
}
