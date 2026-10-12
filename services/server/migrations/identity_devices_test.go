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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestDevicesMigration_Schema holds devices and device_keys against docs/spec/03-datenmodell.md
// section 4.2 column by column, with every check, foreign key and index (M1-33).
func TestDevicesMigration_Schema(t *testing.T) {
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
			table: "devices",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"user_id", "uuid", false, ""},
				{"name", "text", false, ""},
				{"platform", "text", false, ""},
				{"state", "text", false, "'pending'::text"},
				{"app_version", "text", true, ""},
				{"approved_by", "uuid", true, ""},
				{"approved_at", ts, true, ""},
				{"revoked_by", "uuid", true, ""},
				{"revoked_at", ts, true, ""},
				{"revoke_reason", "text", true, ""},
				{"created_at", ts, false, "now()"},
				{"last_seen_at", ts, true, ""},
			},
			constraints: []string{
				"CHECK (((revoke_reason IS NULL) OR (state = 'revoked'::text)))",
				"CHECK (((state = 'revoked'::text) = (revoked_at IS NOT NULL)))",
				"CHECK ((platform = ANY (ARRAY['ios'::text, 'android'::text, 'macos'::text, 'windows'::text, 'web'::text, 'cli'::text])))",
				"CHECK ((revoke_reason = ANY (ARRAY['revoked_by_self'::text, 'revoked_by_owner'::text, 'user_disabled'::text, 'inactive'::text, 'verification_failed'::text, 'pending_expired'::text, 'pairing_expired'::text])))",
				"CHECK ((state = ANY (ARRAY['pending'::text, 'active'::text, 'revoked'::text])))",
				"FOREIGN KEY (approved_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"FOREIGN KEY (revoked_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE INDEX devices_pending_idx ON public.devices USING btree (state) WHERE (state = 'pending'::text)",
				"CREATE INDEX devices_user_id_state_idx ON public.devices USING btree (user_id, state)",
				"CREATE UNIQUE INDEX devices_pkey ON public.devices USING btree (id)",
			},
		},
		{
			table: "device_keys",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"device_id", "uuid", false, ""},
				{"purpose", "text", false, ""},
				{"jkt", "text", false, ""},
				{"public_jwk", "jsonb", false, ""},
				{"alg", "text", false, ""},
				{"attestation", "jsonb", true, ""},
				{"created_at", ts, false, "now()"},
				{"revoked_at", ts, true, ""},
			},
			constraints: []string{
				"CHECK ((alg = ANY (ARRAY['ES256'::text, 'EdDSA'::text])))",
				"CHECK ((purpose = ANY (ARRAY['dpop'::text, 'step_up'::text])))",
				"FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX device_keys_active_purpose_key ON public.device_keys USING btree (device_id, purpose) WHERE (revoked_at IS NULL)",
				"CREATE UNIQUE INDEX device_keys_jkt_key ON public.device_keys USING btree (jkt)",
				"CREATE UNIQUE INDEX device_keys_pkey ON public.device_keys USING btree (id)",
			},
		},
	}
	for _, tt := range tests {
		if got := principalColumns(t, conn, tt.table); !reflect.DeepEqual(got, tt.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tt.table, got, tt.columns)
		}
		want := slices.Clone(tt.constraints)
		slices.Sort(want)
		// The database sorts with its collation; compare in byte order on both sides.
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

// wantState fails unless err carries SQLSTATE code.
func wantState(t *testing.T, err error, code, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: %v, want SQLSTATE %s", what, err, code)
	}
}

// TestDevicesMigration_Constraints exercises the rules of 03 as bothy_app: the checks on
// platform, state, purpose and alg, the revocation checks, the unique jkt, one active key per
// device and purpose (a revoked one plus a new one passes) and ON DELETE CASCADE to device_keys.
func TestDevicesMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a01"
	const dev = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a02"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	ins := `INSERT INTO devices (id, user_id, name, platform, state, revoked_at, revoke_reason) VALUES ($1, $2, 'Phone', $3, $4, $5, $6)`
	id := func(n byte) string {
		return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5b" + string("0123456789abcdef"[n>>4]) + string("0123456789abcdef"[n&15])
	}
	wantCheckViolation(t, exec(app, ins, id(1), person, "tv", "pending", nil, nil), "platform tv")
	wantCheckViolation(t, exec(app, ins, id(2), person, "ios", "lost", nil, nil), "state lost")
	wantCheckViolation(t, exec(app, ins, id(3), person, "ios", "revoked", nil, nil), "revoked without revoked_at")
	wantCheckViolation(t, exec(app, ins, id(4), person, "ios", "active", time.Now(), nil), "revoked_at on an active device")
	wantCheckViolation(t, exec(app, ins, id(5), person, "ios", "active", nil, "inactive"), "reason on an active device")
	wantCheckViolation(t, exec(app, ins, id(6), person, "ios", "revoked", time.Now(), "lost"), "unknown reason")
	wantState(t, exec(app, ins, id(7), "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5aff", "ios", "pending", nil, nil), "23503", "unknown person")
	mustExec(t, app, ins, id(8), person, "cli", "revoked", time.Now(), "pending_expired")
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, dev, person)
	var state string
	if err := app.QueryRow(context.Background(), `SELECT state FROM devices WHERE id = $1`, dev).Scan(&state); err != nil || state != "pending" {
		t.Fatalf("default state %q, %v", state, err)
	}

	key := `INSERT INTO device_keys (id, device_id, purpose, jkt, public_jwk, alg, revoked_at) VALUES ($1, $2, $3, $4, '{}', $5, $6)`
	wantCheckViolation(t, exec(app, key, id(20), dev, "other", "j0", "ES256", nil), "purpose other")
	wantCheckViolation(t, exec(app, key, id(21), dev, "dpop", "j0", "RS256", nil), "alg RS256")
	mustExec(t, app, key, id(22), dev, "dpop", "j1", "ES256", nil)
	wantState(t, exec(app, key, id(23), dev, "dpop", "j2", "ES256", nil), "23505", "second active dpop key")
	wantState(t, exec(app, key, id(24), dev, "step_up", "j1", "EdDSA", nil), "23505", "jkt taken")
	mustExec(t, app, key, id(25), dev, "step_up", "j3", "EdDSA", nil)
	mustExec(t, app, `UPDATE device_keys SET revoked_at = now() WHERE id = $1`, id(22))
	mustExec(t, app, key, id(26), dev, "dpop", "j4", "ES256", nil)
	wantState(t, exec(app, key, id(27), dev, "dpop", "j1", "ES256", time.Now()), "23505", "jkt of a revoked key reused")

	wantState(t, exec(app, `DELETE FROM devices WHERE id = $1`, dev), "42501", "bothy_app deletes a device")
	mustExec(t, owner, `DELETE FROM devices WHERE id = $1`, dev)
	var keys int
	if err := owner.QueryRow(context.Background(), `SELECT count(*) FROM device_keys WHERE device_id = $1`, dev).Scan(&keys); err != nil || keys != 0 {
		t.Fatalf("keys after deleting the device: %d, %v (want ON DELETE CASCADE)", keys, err)
	}
	wantState(t, exec(owner, `DELETE FROM users WHERE principal_id = $1`, person), "23001", "person with a device deleted (ON DELETE RESTRICT)")
}

// TestDevicesMigration_Down holds the Down section: down to version 6 removes both tables, up
// applies again.
func TestDevicesMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 6); err != nil {
		t.Fatalf("down to 6: %v", err)
	}
	for _, rel := range []string{"devices", "device_keys"} {
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
	if err := sqlDB.QueryRowContext(ctx, "SELECT 1 FROM device_keys LIMIT 1").Scan(new(int)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("device_keys after up again: %v", err)
	}
}
