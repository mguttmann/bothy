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

// TestStepUpMigration_Schema holds auth_challenges against docs/spec/03-datenmodell.md section 4.2
// column by column, with every check, foreign key, unique and index (M1-35), including the foreign
// key enrollment_code_id -> enrollment_codes(id) that came with enrollment_codes (M1-37).
func TestStepUpMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"purpose", "text", false, ""},
		{"session_id", "uuid", true, ""},
		{"enrollment_code_id", "uuid", true, ""},
		{"challenge_hash", "bytea", false, ""},
		{"webauthn_session", "jsonb", true, ""},
		{"status", "text", false, "'issued'::text"},
		{"created_at", ts, false, "now()"},
		{"expires_at", ts, false, ""},
		{"finished_at", ts, true, ""},
	}
	constraints := []string{
		"CHECK (((enrollment_code_id IS NOT NULL) = (purpose = 'passkey_enroll'::text)))",
		"CHECK (((session_id IS NOT NULL) = (purpose = ANY (ARRAY['step_up'::text, 'passkey_register'::text]))))",
		"CHECK (((status = 'issued'::text) = (finished_at IS NULL)))",
		"CHECK ((purpose = ANY (ARRAY['step_up'::text, 'passkey_login'::text, 'passkey_enroll'::text, 'passkey_register'::text])))",
		"CHECK ((status = ANY (ARRAY['issued'::text, 'used'::text, 'failed'::text])))",
		"FOREIGN KEY (enrollment_code_id) REFERENCES enrollment_codes(id) ON DELETE CASCADE",
		"FOREIGN KEY (session_id) REFERENCES auth_sessions(id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
		"UNIQUE (challenge_hash)",
	}
	idx := []string{
		"CREATE INDEX auth_challenges_expires_at_idx ON public.auth_challenges USING btree (expires_at)",
		"CREATE INDEX auth_challenges_session_failed_idx ON public.auth_challenges USING btree (session_id, finished_at) WHERE (status = 'failed'::text)",
		"CREATE UNIQUE INDEX auth_challenges_challenge_hash_key ON public.auth_challenges USING btree (challenge_hash)",
		"CREATE UNIQUE INDEX auth_challenges_pkey ON public.auth_challenges USING btree (id)",
	}
	if got := principalColumns(t, conn, "auth_challenges"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "auth_challenges")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "auth_challenges")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestStepUpMigration_Constraints exercises the rules of 03 as bothy_app: purpose, status and the
// three pairing checks, the unique challenge_hash, the cascade from a session, and that bothy_app
// may delete (the job identity.auth_cleanup).
func TestStepUpMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a01"
	const dev = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a02"
	const sess = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a03"
	const code = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a04"
	end := time.Now().Add(time.Hour)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, dev, person)
	mustExec(t, app, `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', $4)`, sess, person, dev, end)
	mustExec(t, app, `INSERT INTO enrollment_codes (id, user_id, purpose, code_hash, expires_at) VALUES ($1, $2, 'first_login', $3, $4)`, code, person, []byte{0xec}, end)
	id := func(n byte) string {
		return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b" + string("0123456789abcdef"[n>>4]) + string("0123456789abcdef"[n&15])
	}
	ins := `INSERT INTO auth_challenges (id, purpose, session_id, enrollment_code_id, challenge_hash, status, expires_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	ch := func(n byte, purpose string, session, enroll any, status string, finished any) error {
		return exec(app, ins, id(n), purpose, session, enroll, []byte{n}, status, end, finished)
	}
	if err := ch(1, "step_up", sess, nil, "issued", nil); err != nil {
		t.Fatal(err)
	}
	if err := ch(2, "passkey_register", sess, nil, "used", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ch(3, "passkey_login", nil, nil, "failed", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ch(4, "passkey_enroll", nil, code, "issued", nil); err != nil {
		t.Fatal(err)
	}
	wantCheckViolation(t, ch(10, "login", nil, nil, "issued", nil), "purpose login")
	wantCheckViolation(t, ch(11, "step_up", sess, nil, "expired", time.Now()), "status expired")
	wantCheckViolation(t, ch(12, "step_up", nil, nil, "issued", nil), "step_up without session")
	wantCheckViolation(t, ch(13, "passkey_login", sess, nil, "issued", nil), "passkey_login with session")
	wantCheckViolation(t, ch(14, "passkey_enroll", nil, nil, "issued", nil), "passkey_enroll without code")
	wantCheckViolation(t, ch(15, "step_up", sess, code, "issued", nil), "step_up with code")
	wantCheckViolation(t, ch(16, "step_up", sess, nil, "used", nil), "used without finished_at")
	wantCheckViolation(t, ch(17, "step_up", sess, nil, "issued", time.Now()), "issued with finished_at")
	wantState(t, exec(app, ins, id(18), "step_up", sess, nil, []byte{1}, "issued", end, nil), "23505", "challenge_hash taken")
	wantState(t, ch(19, "step_up", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7aff", nil, "issued", nil), "23503", "unknown session")
	var status string
	if err := app.QueryRow(context.Background(), `INSERT INTO auth_challenges (id, purpose, session_id, challenge_hash, expires_at)
		VALUES ($1, 'step_up', $2, $3, $4) RETURNING status`, id(20), sess, []byte{20}, end).Scan(&status); err != nil || status != "issued" {
		t.Fatalf("default status %q, %v", status, err)
	}
	mustExec(t, app, `DELETE FROM auth_challenges WHERE id = $1`, id(3))
	mustExec(t, owner, `DELETE FROM auth_sessions WHERE id = $1`, sess)
	var n int
	if err := owner.QueryRow(context.Background(), `SELECT count(*) FROM auth_challenges WHERE session_id IS NOT NULL`).Scan(&n); err != nil || n != 0 {
		t.Errorf("challenges of a deleted session: %d, %v (want 0 through ON DELETE CASCADE)", n, err)
	}
}

// TestStepUpMigration_Down holds the Down section: down to version 11 removes auth_challenges, up
// applies again.
func TestStepUpMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 11); err != nil {
		t.Fatalf("down to 11: %v", err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('auth_challenges') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("auth_challenges survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('auth_challenges') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("auth_challenges after up again: %v, %v", exists, err)
	}
}
