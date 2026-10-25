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

// TestDevicePairingsMigration_Schema holds device_pairings against docs/spec/03-data-model.md
// section 4.2 column by column, with every check, foreign key and index (M1-38).
func TestDevicePairingsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"pairing_code_hash", "bytea", false, ""},
		{"device_registration", "jsonb", false, ""},
		{"status", "text", false, "'requested'::text"},
		{"user_id", "uuid", true, ""},
		{"approved_by_device_id", "uuid", true, ""},
		{"device_id", "uuid", true, ""},
		{"created_at", ts, false, "now()"},
		{"expires_at", ts, false, ""},
		{"completed_at", ts, true, ""},
	}
	constraints := []string{
		"CHECK ((status = ANY (ARRAY['requested'::text, 'approved'::text, 'completed'::text, 'rejected'::text, 'expired'::text])))",
		"CHECK (((status <> ALL (ARRAY['approved'::text, 'completed'::text])) OR (user_id IS NOT NULL)))",
		"CHECK (((status <> 'completed'::text) OR (device_id IS NOT NULL)))",
		"FOREIGN KEY (approved_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
		"FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL",
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
	}
	idx := []string{
		"CREATE INDEX device_pairings_expires_at_idx ON public.device_pairings USING btree (expires_at)",
		"CREATE UNIQUE INDEX device_pairings_code_hash_requested_key ON public.device_pairings USING btree (pairing_code_hash) WHERE (status = 'requested'::text)",
		"CREATE UNIQUE INDEX device_pairings_pkey ON public.device_pairings USING btree (id)",
	}
	if got := principalColumns(t, conn, "device_pairings"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "device_pairings")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "device_pairings")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestDevicePairingsMigration_Constraints exercises the rules of 03 as bothy_app: status, the
// unique code hash among requested pairings only, the checks of approved and completed rows, the
// foreign keys with their delete rules, and that bothy_app may not delete pairings.
func TestDevicePairingsMigration_Constraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a01"
	const dev = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a02"
	const pairing = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a03"
	end := time.Now().Add(time.Hour)
	reg := `{"v":1}`
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, dev, person)
	ins := `INSERT INTO device_pairings (id, pairing_code_hash, device_registration, expires_at) VALUES ($1, $2, $3, $4)`
	mustExec(t, app, ins, pairing, []byte{1}, reg, end)
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a04", []byte{1}, reg, end), "23505", "code hash of an open pairing taken")
	wantCheckViolation(t, exec(app, `UPDATE device_pairings SET status = 'open' WHERE id = $1`, pairing), "status open")
	wantCheckViolation(t, exec(app, `UPDATE device_pairings SET status = 'approved' WHERE id = $1`, pairing), "approved without person")
	mustExec(t, app, `UPDATE device_pairings SET status = 'approved', user_id = $2, approved_by_device_id = $3 WHERE id = $1`, pairing, person, dev)
	wantCheckViolation(t, exec(app, `UPDATE device_pairings SET status = 'completed' WHERE id = $1`, pairing), "completed without device")
	mustExec(t, app, `UPDATE device_pairings SET status = 'completed', device_id = $2, completed_at = now() WHERE id = $1`, pairing, dev)
	// The hash is unique only among requested pairings: a closed one frees it.
	mustExec(t, app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a05", []byte{1}, reg, end)
	wantState(t, exec(app, `UPDATE device_pairings SET user_id = '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9aff' WHERE id = $1`, pairing),
		"23503", "unknown person")
	wantDeleteGranted(t, app, "bothy_app deletes a pairing", `DELETE FROM device_pairings WHERE id = $1`, pairing)
	// The device goes: the device references become NULL. The person goes: its pairings go.
	mustExec(t, app, `UPDATE device_pairings SET status = 'expired' WHERE id = $1`, pairing)
	mustExec(t, owner, `DELETE FROM devices WHERE id = $1`, dev)
	var refs int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM device_pairings WHERE device_id IS NOT NULL OR approved_by_device_id IS NOT NULL`).Scan(&refs); err != nil || refs != 0 {
		t.Fatalf("device references after the device went: %d, %v", refs, err)
	}
	mustExec(t, owner, `DELETE FROM users WHERE principal_id = $1`, person)
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM device_pairings WHERE user_id IS NOT NULL`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("pairings of a deleted person: %d, %v", n, err)
	}
}
