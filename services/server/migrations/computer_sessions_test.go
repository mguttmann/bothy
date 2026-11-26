// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"database/sql"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// The migrations of computer use (CU-04, R-DM-65, docs/spec/27-computer-use.md): 00124 adds the
// enablement mirror of hosts, 00125 creates computer_sessions, 00126 widens
// agent_sessions.end_reason with computer_use NOT VALID and 00127 validates it.

// beforeComputer is the last version before stage CU.
const beforeComputer = 123

// TestMigration_ComputerSessions holds computer_sessions against docs/spec/03-data-model.md
// section 4.14 column by column, with every check, key and index, and the widened end reason of
// agent_sessions.
func TestMigration_ComputerSessions(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	want := []principalColumn{
		{"id", "uuid", false, ""},
		{"host_id", "uuid", false, ""},
		{"user_id", "uuid", false, ""},
		{"agent_id", "uuid", false, ""},
		{"run_id", "uuid", false, ""},
		{"state", "text", false, "'opening'::text"},
		{"pause_reason", "text", true, ""},
		{"token_jti", "uuid", false, ""},
		{"generation", "integer", false, ""},
		{"key_generation", "integer", false, ""},
		{"attached_at", tz, true, ""},
		{"opened_at", tz, false, "now()"},
		{"started_at", tz, true, ""},
		{"paused_at", tz, true, ""},
		{"paused_ms", "bigint", false, "0"},
		{"closed_at", tz, true, ""},
		{"stopped_by", "uuid", true, ""},
		{"close_reason", "text", true, ""},
		{"close_detail", "text", true, ""},
		{"action_count", "integer", false, "0"},
		{"front_app", "text", true, ""},
		{"version", "bigint", false, "1"},
	}
	if got := principalColumns(t, conn, "computer_sessions"); !reflect.DeepEqual(got, want) {
		t.Errorf("computer_sessions columns:\n got %+v\nwant %+v", got, want)
	}
	cons := principalConstraints(t, conn, "computer_sessions")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK (((state = 'closed'::text) = (close_reason IS NOT NULL)))",
		"CHECK (((state = 'closed'::text) = (closed_at IS NOT NULL)))",
		"CHECK (((state = 'paused'::text) = (pause_reason IS NOT NULL)))",
		"CHECK (((state = 'paused'::text) = (paused_at IS NOT NULL)))",
		"CHECK (((close_detail IS NULL) OR (state = 'closed'::text)))",
		"CHECK ((close_reason = ANY (ARRAY['done'::text, 'stopped_by_user'::text, 'device_disabled'::text, 'run_ended'::text, " +
			"'access_revoked'::text, 'host_disabled'::text, 'host_retired'::text, 'host_rekeyed'::text, 'host_restarted'::text, " +
			"'connection_lost'::text, 'open_failed'::text, 'error'::text])))",
		"CHECK ((pause_reason = 'locked'::text))",
		"CHECK ((state = ANY (ARRAY['opening'::text, 'active'::text, 'paused'::text, 'closed'::text])))",
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE RESTRICT",
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE RESTRICT",
		"FOREIGN KEY (stopped_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
		"UNIQUE (token_jti)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("computer_sessions constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX computer_sessions_closed_at_idx ON public.computer_sessions USING btree (closed_at, id) WHERE (closed_at IS NOT NULL)",
		"CREATE INDEX computer_sessions_opened_at_brin ON public.computer_sessions USING brin (opened_at)",
		"CREATE INDEX computer_sessions_user_open_idx ON public.computer_sessions USING btree (user_id) WHERE (state <> 'closed'::text)",
		"CREATE UNIQUE INDEX computer_sessions_host_open_key ON public.computer_sessions USING btree (host_id) WHERE (state <> 'closed'::text)",
		"CREATE UNIQUE INDEX computer_sessions_pkey ON public.computer_sessions USING btree (id)",
		"CREATE UNIQUE INDEX computer_sessions_run_open_key ON public.computer_sessions USING btree (run_id) WHERE (state <> 'closed'::text)",
		"CREATE UNIQUE INDEX computer_sessions_token_jti_key ON public.computer_sessions USING btree (token_jti)",
	}
	if got := indexes(t, conn, "computer_sessions"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("computer_sessions indexes:\n got %q\nwant %q", got, wantIdx)
	}
	var def string
	var valid bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conrelid = 'agent_sessions'::regclass AND conname = 'agent_sessions_end_reason_check'`).Scan(&def, &valid); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(def, "'computer_use'::text") || !valid {
		t.Errorf("agent_sessions end reason check %q validated %v", def, valid)
	}
}

// computerWorld are the rows a computer session needs, written as bothy_app.
type computerWorld struct {
	wakeWorld
	host, host2, run2 string
}

// seedComputerWorld inserts the world of the wake tests (Alex, the agent Robin, a thread and a
// finished run), a second run and two active hosts.
func seedComputerWorld(t *testing.T, app *pg.Pool) computerWorld {
	t.Helper()
	w := computerWorld{wakeWorld: seedWakeWorld(t, app), host: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c11",
		host2: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c12", run2: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c21"}
	for i, h := range []string{w.host, w.host2} {
		mustExec(t, app, `INSERT INTO hosts (id, name, state, noise_public_key, auth_public_jwk, auth_jkt, key_fingerprint,
			confirmed_by, confirmed_at, created_by) VALUES ($1, $2, 'active', $3, '{"kty":"OKP","crv":"Ed25519","x":"AA"}', $4, 'FP', $5, now(), $5)`,
			h, []string{"desk-a", "desk-b"}[i], make([]byte, 32), "jkt-"+h, w.person)
	}
	mustExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, billing_mode_expected,
		invocation_jti, finished_at) VALUES ($1, $2, $3, 'message', $4, 'completed', 'organization', $1, now())`,
		w.run2, w.thread, w.agent, w.person)
	return w
}

// TestMigration_ComputerSessionsRules shows the rules of 03 computer_sessions as bothy_app: one
// session that is not closed per host and per run, the closed lists, the pairs of state and end,
// pause and its reason, the single token, and the widened agent session end.
func TestMigration_ComputerSessionsRules(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	w := seedComputerWorld(t, app)
	ins := `INSERT INTO computer_sessions (id, host_id, user_id, agent_id, run_id, state, pause_reason, paused_at, token_jti,
		generation, key_generation, closed_at, close_reason, close_detail) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, 1, $10, $11, $12)`
	add := func(id, host, run, state string, pause, pausedAt any, jti string, closed, reason, detail any) error {
		return exec(app, ins, id, host, w.person, w.agent, run, state, pause, pausedAt, jti, closed, reason, detail)
	}
	const (
		s1, s2, s3 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d01", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d02", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d03"
		j1, j2, j3 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7e01", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7e02", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7e03"
	)
	now := time.Now()
	if err := add(s1, w.host, w.run, "opening", nil, nil, j1, nil, nil, nil); err != nil {
		t.Fatalf("an opening session: %v", err)
	}
	wantState(t, add(s2, w.host, w.run2, "opening", nil, nil, j2, nil, nil, nil), "23505", "a second controller on one host")
	wantState(t, add(s2, w.host2, w.run, "opening", nil, nil, j2, nil, nil, nil), "23505", "a second session of one run")
	wantState(t, add(s2, w.host2, w.run2, "opening", nil, nil, j1, nil, nil, nil), "23505", "a token used twice")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "watching", nil, nil, j2, nil, nil, nil), "an unknown state")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "paused", nil, now, j2, nil, nil, nil), "paused without a reason")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "paused", "locked", nil, j2, nil, nil, nil), "paused without paused_at")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "paused", "sleeping", now, j2, nil, nil, nil), "an unknown pause reason")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "active", "locked", now, j2, nil, nil, nil), "a pause reason while active")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "closed", nil, nil, j2, nil, "done", nil), "closed without closed_at")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "closed", nil, nil, j2, now, nil, nil), "closed without a reason")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "closed", nil, nil, j2, now, "bored", nil), "an unknown reason")
	wantCheckViolation(t, add(s2, w.host2, w.run2, "active", nil, nil, j2, nil, nil, "open_timeout"), "a detail while open")
	if err := add(s2, w.host2, w.run2, "paused", "locked", now, j2, nil, nil, nil); err != nil {
		t.Fatalf("a paused session: %v", err)
	}
	mustExec(t, app, `UPDATE computer_sessions SET state = 'closed', closed_at = now(), close_reason = 'run_ended',
		pause_reason = NULL, paused_at = NULL, version = version + 1 WHERE id = $1`, s1)
	if err := add(s3, w.host, w.run2, "closed", nil, nil, j3, now, "open_failed", "open_timeout"); err != nil {
		t.Fatalf("a closed session next to a paused one of another host: %v", err)
	}
	mustExec(t, app, `DELETE FROM computer_sessions WHERE id = $1`, s3)

	mustExec(t, app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at, end_reason)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7f01', $1, $2, 'ref-1', false, now(), 'computer_use')`, w.thread, w.agent)
	wantCheckViolation(t, exec(app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at,
		end_reason) VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7f02', $1, $2, 'ref-2', false, now(), 'screenshot')`, w.thread, w.agent),
		"an unknown end reason")
}

// TestMigration_ComputerKeepExisting: hosts, terminal sessions and agent sessions written before
// stage CU keep every value through 00124 to 00127; the new host columns read as "no desktop"
// (NULL). Acceptance criterion 3 of CU-04.
func TestMigration_ComputerKeepExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, sqlDB, app := computerProvider(t)
	w := seedWakeWorld(t, app)
	tmPeopleFor(t, app)
	tmActiveHost(t, app, tmR1, "desk-r1")
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, tmDev, tmRobin)
	mustExec(t, app, `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', $4)`,
		tmSess, tmRobin, tmDev, time.Now().Add(time.Hour))
	mustExec(t, app, `INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by) VALUES
		('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c31', $1, $2, 'bothy-s-sam', $2)`, tmR1, tmRobin)
	mustExec(t, app, `INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id,
		auth_session_id, profile, state, token_jti, key_generation) VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c41', $1, $2,
		'0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c31', $3, $3, $4, 'shell', 'active', '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c51', 1)`,
		tmR1, tmRobin, tmDev, tmSess)
	mustExec(t, app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at, end_reason)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7f11', $1, $2, 'ref-old', false, now(), 'reset')`, w.thread, w.agent)
	snapshots := map[string]string{
		"hosts":             `SELECT string_agg(row_to_json(h)::text, '|' ORDER BY id) FROM hosts h`,
		"terminal_sessions": `SELECT string_agg(row_to_json(s)::text, '|' ORDER BY id) FROM terminal_sessions s`,
		"agent_sessions":    `SELECT string_agg(row_to_json(a)::text, '|' ORDER BY id) FROM agent_sessions a`,
	}
	before := map[string]string{}
	for k, q := range snapshots {
		var s string
		if err := sqlDB.QueryRowContext(ctx, q).Scan(&s); err != nil {
			t.Fatal(err)
		}
		before[k] = s
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	after := map[string]string{
		"hosts": `SELECT string_agg((row_to_json(h)::jsonb - 'reported_computer' - 'computer_disabled_generation')::text, '|'
			ORDER BY id) FROM hosts h`,
		"terminal_sessions": snapshots["terminal_sessions"],
		"agent_sessions":    snapshots["agent_sessions"],
	}
	for k, q := range after {
		var s string
		if err := sqlDB.QueryRowContext(ctx, q).Scan(&s); err != nil {
			t.Fatal(err)
		}
		var same bool
		if err := sqlDB.QueryRowContext(ctx, `SELECT (SELECT array_agg(x::jsonb) FROM unnest(string_to_array($1, '|')) x)
			= (SELECT array_agg(x::jsonb) FROM unnest(string_to_array($2, '|')) x)`, before[k], s).Scan(&same); err != nil {
			t.Fatal(err)
		}
		if !same {
			t.Errorf("%s changed by the migration:\n%s\n-> %s", k, before[k], s)
		}
	}
	var mirrored int
	if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM hosts WHERE reported_computer IS NOT NULL
		OR computer_disabled_generation IS NOT NULL`).Scan(&mirrored); err != nil || mirrored != 0 {
		t.Fatalf("hosts with a computer mirror after the migration: %d %v", mirrored, err)
	}
}

// TestMigration_ComputerDown: down to 123 removes the table and columns of stage CU while no agent
// session ended with computer_use; with one the down step of 00126 refuses and names the count.
func TestMigration_ComputerDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, sqlDB, app := computerProvider(t)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	w := seedWakeWorld(t, app)
	mustExec(t, app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at, end_reason)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7f21', $1, $2, 'ref-cu', false, now(), 'computer_use')`, w.thread, w.agent)
	_, err := provider.DownTo(ctx, beforeComputer)
	if err == nil || !strings.Contains(err.Error(), "computer_use") {
		t.Fatalf("down with a computer_use end: %v, want the refusal", err)
	}
	mustExec(t, app, `UPDATE agent_sessions SET end_reason = 'reset' WHERE end_reason = 'computer_use'`)
	if _, err := provider.DownTo(ctx, beforeComputer); err != nil {
		t.Fatalf("down without computer_use ends: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('computer_sessions') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM information_schema.columns WHERE (table_name, column_name) IN
		(('hosts', 'reported_computer'), ('hosts', 'computer_disabled_generation')))`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d tables or columns of stage CU left after down", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

// computerProvider returns a migration provider on a fresh database migrated down to
// beforeComputer, its SQL handle and a pool of bothy_app.
func computerProvider(t *testing.T) (*goose.Provider, *sql.DB, *pg.Pool) {
	t.Helper()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(context.Background(), beforeComputer); err != nil {
		t.Fatalf("down to %d: %v", beforeComputer, err)
	}
	return provider, sqlDB, db.Pool(t, pg.RoleApp)
}

// tmPeopleFor inserts the person tmRobin of the terminal migration tests under the handle sam
// (the world of the wake tests holds Alex and the agent Robin).
func tmPeopleFor(t *testing.T, app *pg.Pool) {
	t.Helper()
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'sam', 'Sam')`, tmRobin)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, tmRobin)
}
