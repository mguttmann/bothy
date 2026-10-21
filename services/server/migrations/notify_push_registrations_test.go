// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigrationPushRegistrations holds push_registrations against docs/spec/03-datenmodell.md
// section 4.10 column by column, with every key, check and index (M3-19 design note 2): the
// three foreign keys cascade, (provider, token) is unique, preview_token_hash is a unique partial
// index, a second up changes nothing, and deleting a device or a session removes its rows.
func TestMigrationPushRegistrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"

	cols := []principalColumn{
		{"id", "uuid", false, ""}, {"user_id", "uuid", false, ""}, {"device_id", "uuid", false, ""},
		{"auth_session_id", "uuid", false, ""}, {"provider", "text", false, ""}, {"token", "text", false, ""},
		{"environment", "text", false, "'production'::text"}, {"app_id", "text", false, ""},
		{"preview_token_hash", "bytea", true, ""}, {"created_at", tz, false, "now()"}, {"last_success_at", tz, true, ""},
		{"failure_count", "integer", false, "0"}, {"disabled_at", tz, true, ""}, {"disabled_reason", "text", true, ""},
	}
	if got := principalColumns(t, conn, "push_registrations"); !reflect.DeepEqual(got, cols) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, cols)
	}
	cons := []string{
		"CHECK (((disabled_at IS NULL) = (disabled_reason IS NULL)))",
		"CHECK ((disabled_reason = ANY (ARRAY['token_invalid'::text, 'device_revoked'::text, 'logout'::text, 'topic_mismatch'::text, 'sender_mismatch'::text, 'repeated_failures'::text])))",
		"CHECK ((environment = ANY (ARRAY['production'::text, 'sandbox'::text])))",
		"CHECK ((provider = ANY (ARRAY['apns'::text, 'fcm'::text, 'unifiedpush'::text, 'wns'::text])))",
		"FOREIGN KEY (auth_session_id) REFERENCES auth_sessions(id) ON DELETE CASCADE",
		"FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE",
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
		"UNIQUE (provider, token)",
	}
	got := principalConstraints(t, conn, "push_registrations")
	slices.Sort(got)
	slices.Sort(cons)
	if !reflect.DeepEqual(got, cons) {
		t.Errorf("constraints:\n got %q\nwant %q", got, cons)
	}
	idx := []string{
		"CREATE INDEX push_registrations_auth_session_id_idx ON public.push_registrations USING btree (auth_session_id)",
		"CREATE INDEX push_registrations_user_id_active_idx ON public.push_registrations USING btree (user_id) WHERE (disabled_at IS NULL)",
		"CREATE UNIQUE INDEX push_registrations_pkey ON public.push_registrations USING btree (id)",
		"CREATE UNIQUE INDEX push_registrations_preview_token_hash_key ON public.push_registrations USING btree (preview_token_hash) WHERE (preview_token_hash IS NOT NULL)",
		"CREATE UNIQUE INDEX push_registrations_provider_token_key ON public.push_registrations USING btree (provider, token)",
	}
	if got := indexes(t, conn, "push_registrations"); !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}

	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}

	// Alex has a phone with a session and a tablet that registered through the phone's session:
	// deleting the tablet removes its registration, deleting the session removes the phone's.
	person, phone, tablet, session := ids.New().String(), ids.New().String(), ids.New().String(), ids.New().String()
	regPhone, regTablet := ids.New().String(), ids.New().String()
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`, person)
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, person)
	exec(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $3, 'phone', 'ios', 'active'), ($2, $3, 'tablet', 'android', 'active')`,
		phone, tablet, person)
	exec(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', now() + interval '1 day')`,
		session, person, phone)
	ins := `INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, preview_token_hash)
		VALUES ($1, $2, $3, $4, $5, $6, 'com.example.bothy', $7)`
	exec(ins, regPhone, person, phone, session, "apns", "token-phone", []byte{1})
	exec(ins, regTablet, person, tablet, session, "fcm", "token-tablet", nil)

	wantViolation := func(what, code, stmt string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, stmt, args...); sqlstate(err) != code {
			t.Errorf("%s: %v, want SQLSTATE %s", what, err, code)
		}
	}
	wantViolation("same provider and token", "23505", ins, ids.New().String(), person, tablet, session, "apns", "token-phone", nil)
	wantViolation("same preview token hash", "23505", ins, ids.New().String(), person, tablet, session, "wns", "token-other", []byte{1})
	wantViolation("unknown provider", "23514", ins, ids.New().String(), person, tablet, session, "smoke", "token-smoke", nil)
	wantViolation("disabled without reason", "23514", `UPDATE push_registrations SET disabled_at = now() WHERE id = $1`, regPhone)
	wantViolation("reason without disabled_at", "23514", `UPDATE push_registrations SET disabled_reason = 'logout' WHERE id = $1`, regPhone)
	wantViolation("unknown reason", "23514", `UPDATE push_registrations SET disabled_at = now(), disabled_reason = 'stolen' WHERE id = $1`, regPhone)
	wantViolation("unknown environment", "23514", `UPDATE push_registrations SET environment = 'staging' WHERE id = $1`, regPhone)

	var env string
	var failures int
	if err := conn.QueryRow(ctx, `SELECT environment, failure_count FROM push_registrations WHERE id = $1`, regPhone).Scan(&env, &failures); err != nil {
		t.Fatal(err)
	}
	if env != "production" || failures != 0 {
		t.Errorf("defaults environment %q failure_count %d, want production and 0", env, failures)
	}

	count := func(id string) int {
		t.Helper()
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM push_registrations WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	exec(`DELETE FROM devices WHERE id = $1`, tablet)
	if count(regTablet) != 0 || count(regPhone) != 1 {
		t.Errorf("after deleting the tablet: tablet rows %d, phone rows %d; want 0 and 1", count(regTablet), count(regPhone))
	}
	exec(`DELETE FROM auth_sessions WHERE id = $1`, session)
	if count(regPhone) != 0 {
		t.Errorf("after deleting the session: %d phone rows survive", count(regPhone))
	}
}
