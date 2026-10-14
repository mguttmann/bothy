// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestEnrollmentCodesMigration_Schema holds enrollment_codes against docs/spec/03-data-model.md
// section 4.2 column by column, with every check, foreign key, unique and index (M1-37), and the
// foreign key auth_challenges.enrollment_code_id that M1-35 deferred.
func TestEnrollmentCodesMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"user_id", "uuid", false, ""},
		{"purpose", "text", false, ""},
		{"code_hash", "bytea", false, ""},
		{"expires_at", ts, false, ""},
		{"attempts", "integer", false, "0"},
		{"created_by", "uuid", true, ""},
		{"created_at", ts, false, "now()"},
		{"used_at", ts, true, ""},
		{"used_by_device_id", "uuid", true, ""},
	}
	constraints := []string{
		"CHECK ((purpose = ANY (ARRAY['first_login'::text, 'identity_link'::text, 'recovery'::text])))",
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (used_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
		"UNIQUE (code_hash)",
	}
	idx := []string{
		"CREATE INDEX enrollment_codes_expires_at_idx ON public.enrollment_codes USING btree (expires_at)",
		"CREATE INDEX enrollment_codes_user_id_unused_idx ON public.enrollment_codes USING btree (user_id) WHERE (used_at IS NULL)",
		"CREATE UNIQUE INDEX enrollment_codes_code_hash_key ON public.enrollment_codes USING btree (code_hash)",
		"CREATE UNIQUE INDEX enrollment_codes_pkey ON public.enrollment_codes USING btree (id)",
	}
	if got := principalColumns(t, conn, "enrollment_codes"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "enrollment_codes")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "enrollment_codes")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
	if !slices.Contains(principalConstraints(t, conn, "auth_challenges"),
		"FOREIGN KEY (enrollment_code_id) REFERENCES enrollment_codes(id) ON DELETE CASCADE") {
		t.Error("auth_challenges lacks the foreign key on enrollment_code_id")
	}
}

// TestEnrollmentCodesMigration_Constraints exercises the rules of 03 as bothy_app: purpose, the
// unique hash, the foreign keys with their delete rules, that bothy_app may not delete codes, and
// that bothy_app may lock and complete the instance row for bothy setup.
func TestEnrollmentCodesMigration_Constraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a01"
	const dev = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a02"
	const code = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a03"
	end := time.Now().Add(time.Hour)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, dev, person)
	ins := `INSERT INTO enrollment_codes (id, user_id, purpose, code_hash, expires_at, created_by) VALUES ($1, $2, $3, $4, $5, $6)`
	mustExec(t, app, ins, code, person, "first_login", []byte{1}, end, nil)
	wantCheckViolation(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a04", person, "login", []byte{2}, end, nil), "purpose login")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a05", person, "recovery", []byte{1}, end, nil), "23505", "code_hash taken")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a06", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8aff", "recovery", []byte{3}, end, nil),
		"23503", "unknown person")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a07", person, "recovery", []byte{4}, end, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8aff"),
		"23503", "unknown creator")
	mustExec(t, app, `UPDATE enrollment_codes SET used_at = now(), used_by_device_id = $2 WHERE id = $1`, code, dev)
	wantState(t, exec(app, `DELETE FROM enrollment_codes WHERE id = $1`, code), "42501", "bothy_app deletes a code")
	mustExec(t, app, `INSERT INTO auth_challenges (id, purpose, enrollment_code_id, challenge_hash, expires_at)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a10', 'passkey_enroll', $1, $2, $3)`, code, []byte{9}, end)
	wantState(t, exec(app, `INSERT INTO auth_challenges (id, purpose, enrollment_code_id, challenge_hash, expires_at)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8a11', 'passkey_enroll', '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8aff', $1, $2)`, []byte{10}, end),
		"23503", "challenge with unknown code")
	// The device goes: used_by_device_id becomes NULL. The person goes: its codes and their
	// challenges go.
	mustExec(t, owner, `DELETE FROM devices WHERE id = $1`, dev)
	var used *string
	if err := owner.QueryRow(ctx, `SELECT used_by_device_id::text FROM enrollment_codes WHERE id = $1`, code).Scan(&used); err != nil || used != nil {
		t.Fatalf("used_by_device_id after the device went: %v, %v", used, err)
	}
	mustExec(t, owner, `DELETE FROM users WHERE principal_id = $1`, person)
	var n int
	if err := owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM enrollment_codes) + (SELECT count(*) FROM auth_challenges)`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("codes and challenges of a deleted person: %d, %v", n, err)
	}
}

// TestEnrollmentCodesMigration_InstanceSetupGrant: bothy_app may lock the instance row and set
// setup_completed_at (bothy setup bootstrap-owner, 16 section 7.1), but no other column.
func TestEnrollmentCodesMigration_InstanceSetupGrant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	owner := db.Pool(t, pg.RoleOwner)
	app := db.Pool(t, pg.RoleApp)
	mustExec(t, owner, `INSERT INTO instance (id, instance_id, schema_version, sync_epoch)
		VALUES (1, '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b01', 1, '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b02') ON CONFLICT (id) DO NOTHING`)
	tx, err := app.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM instance WHERE id = 1 FOR UPDATE`); err != nil {
		t.Fatalf("bothy_app locks the instance row: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE instance SET setup_completed_at = now() WHERE id = 1`); err != nil {
		t.Fatalf("bothy_app sets setup_completed_at: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wantState(t, exec(app, `UPDATE instance SET schema_version = 99 WHERE id = 1`), "42501", "bothy_app changes schema_version")
	wantState(t, exec(app, `UPDATE instance SET instance_id = '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8b03' WHERE id = 1`), "42501", "bothy_app changes instance_id")
}
