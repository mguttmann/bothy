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

// The migrations of wake-ups (AR-04, R-DM-60, docs/spec/09-agents.md 5.8): 00097 creates
// run_wakes, 00098 run_autonomy, 00099 widens runs.trigger_kind with wake and adds
// runs.triggering_wake_id NOT VALID, 00100 validates them, 00101 adds
// channels.allow_autonomous_runs.

// beforeWakes is the last version before stage AR.
const beforeWakes = 96

// TestMigration_RunWakes holds run_wakes against docs/spec/03-data-model.md section 4.8 column by
// column, with every check, key and index.
func TestMigration_RunWakes(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	want := []principalColumn{
		{"id", "uuid", false, ""},
		{"agent_id", "uuid", false, ""},
		{"thread_id", "uuid", false, ""},
		{"source_run_id", "uuid", false, ""},
		{"triggered_by", "uuid", false, ""},
		{"kind", "text", false, ""},
		{"condition", "jsonb", false, ""},
		{"label", "text", false, "''::text"},
		{"reason", "text", false, ""},
		{"outage", "integer", true, ""},
		{"not_before", tz, false, ""},
		{"deadline", tz, false, ""},
		{"state", "text", false, "'pending'::text"},
		{"state_reason", "text", true, ""},
		{"result_text", "text", false, "''::text"},
		{"fired_run_id", "uuid", true, ""},
		{"fired_into_run_id", "uuid", true, ""},
		{"created_at", tz, false, "now()"},
		{"ended_at", tz, true, ""},
		{"version", "bigint", false, "1"},
	}
	if got := principalColumns(t, conn, "run_wakes"); !reflect.DeepEqual(got, want) {
		t.Errorf("run_wakes columns:\n got %+v\nwant %+v", got, want)
	}
	cons := principalConstraints(t, conn, "run_wakes")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK (((reason = 'usage_limit'::text) OR (outage IS NULL)))",
		"CHECK (((state = 'pending'::text) = (ended_at IS NULL)))",
		"CHECK ((kind = ANY (ARRAY['at'::text, 'pr_checks'::text, 'command'::text])))",
		"CHECK ((not_before <= deadline))",
		"CHECK ((reason = ANY (ARRAY['agent'::text, 'recovery'::text, 'usage_limit'::text])))",
		"CHECK ((state = ANY (ARRAY['pending'::text, 'fired'::text, 'cancelled'::text, 'expired'::text, 'refused'::text, 'failed'::text])))",
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (fired_into_run_id) REFERENCES runs(id) ON DELETE SET NULL",
		"FOREIGN KEY (fired_run_id) REFERENCES runs(id) ON DELETE SET NULL",
		"FOREIGN KEY (source_run_id) REFERENCES runs(id) ON DELETE CASCADE",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
		"FOREIGN KEY (triggered_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("run_wakes constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX run_wakes_agent_pending_idx ON public.run_wakes USING btree (agent_id) WHERE (state = 'pending'::text)",
		"CREATE INDEX run_wakes_deadline_idx ON public.run_wakes USING btree (deadline) WHERE (state = 'pending'::text)",
		"CREATE INDEX run_wakes_due_idx ON public.run_wakes USING btree (not_before) WHERE ((state = 'pending'::text) AND (kind = 'at'::text))",
		"CREATE INDEX run_wakes_thread_created_idx ON public.run_wakes USING btree (thread_id, created_at DESC)",
		"CREATE UNIQUE INDEX run_wakes_outage_key ON public.run_wakes USING btree (source_run_id, outage) WHERE (outage IS NOT NULL)",
		"CREATE UNIQUE INDEX run_wakes_pkey ON public.run_wakes USING btree (id)",
		"CREATE UNIQUE INDEX run_wakes_recover_key ON public.run_wakes USING btree (source_run_id, reason) " +
			"WHERE ((reason = 'recovery'::text) OR ((reason = 'usage_limit'::text) AND (outage IS NULL)))",
	}
	if got := indexes(t, conn, "run_wakes"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("run_wakes indexes:\n got %q\nwant %q", got, wantIdx)
	}

	wantA := []principalColumn{
		{"thread_id", "uuid", false, ""},
		{"person_id", "uuid", false, ""},
		{"chain_count", "integer", false, "0"},
		{"idle_count", "integer", false, "0"},
		{"ended_reason", "text", true, ""},
		{"updated_at", tz, false, "now()"},
	}
	if got := principalColumns(t, conn, "run_autonomy"); !reflect.DeepEqual(got, wantA) {
		t.Errorf("run_autonomy columns:\n got %+v\nwant %+v", got, wantA)
	}
	acons := principalConstraints(t, conn, "run_autonomy")
	slices.Sort(acons)
	wantACons := []string{
		"CHECK (((chain_count >= 0) AND (idle_count >= 0)))",
		"FOREIGN KEY (person_id) REFERENCES principals(id) ON DELETE CASCADE",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
		"PRIMARY KEY (thread_id, person_id)",
	}
	if !reflect.DeepEqual(acons, wantACons) {
		t.Errorf("run_autonomy constraints:\n got %q\nwant %q", acons, wantACons)
	}

	var validated int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM pg_constraint WHERE conrelid = 'runs'::regclass
		AND conname IN ('runs_triggering_wake_id_fkey', 'runs_triggering_wake_check', 'runs_trigger_kind_check') AND convalidated`).
		Scan(&validated); err != nil || validated != 3 {
		t.Errorf("validated wake constraints of runs: %d %v, want 3", validated, err)
	}
}

// wakeWorld are the rows one thread with a finished run needs, written as bothy_app.
type wakeWorld struct {
	person, agent, thread, channel, run string
}

// seedWakeWorld inserts a person, an agent, a space, a channel, its main thread and a finished
// message run of the person as bothy_app (the ids of the white-label people Alex and Robin).
func seedWakeWorld(t *testing.T, app *pg.Pool) wakeWorld {
	t.Helper()
	w := wakeWorld{
		person: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7501", agent: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7502",
		channel: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7521", thread: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7531",
		run: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7561",
	}
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7511"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, w.person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, w.person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'robin', 'Robin')`, w.agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, w.agent)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, w.person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, w.channel, space, w.person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, w.thread, w.channel, w.person)
	mustExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, billing_mode_expected,
		invocation_jti, finished_at) VALUES ($1, $2, $3, 'message', $4, 'completed', 'organization', $1, now())`,
		w.run, w.thread, w.agent, w.person)
	return w
}

// TestMigration_RunWakesRules shows the rules of 03 run_wakes, run_autonomy and runs as
// bothy_app: the closed values, the window, the end time of a state, the idempotency keys of
// run.wake.recover and of an outage, a wake run with its wake-up, and a wake-up on a message run.
func TestMigration_RunWakesRules(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	w := seedWakeWorld(t, app)
	ins := `INSERT INTO run_wakes (id, agent_id, thread_id, source_run_id, triggered_by, kind, condition, reason, outage,
		not_before, deadline, state, ended_at) VALUES ($1, $2, $3, $4, $5, $6, '{}', $7, $8, now(), now() + $9::interval, $10, $11)`
	add := func(id, kind, reason string, outage any, window, state string, ended any) error {
		return exec(app, ins, id, w.agent, w.thread, w.run, w.person, kind, reason, outage, window, state, ended)
	}
	const (
		wake1 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7601"
		wake2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7602"
		wake3 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7603"
		wake4 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7604"
	)
	if err := add(wake1, "at", "recovery", nil, "1 hour", "pending", nil); err != nil {
		t.Fatalf("a recovery wake-up: %v", err)
	}
	wantState(t, add(wake2, "at", "recovery", nil, "1 hour", "pending", nil), "23505", "a second recovery of one run")
	wantCheckViolation(t, add(wake2, "timer", "agent", nil, "1 hour", "pending", nil), "an unknown kind")
	wantCheckViolation(t, add(wake2, "at", "boredom", nil, "1 hour", "pending", nil), "an unknown reason")
	wantCheckViolation(t, add(wake2, "at", "agent", 1, "1 hour", "pending", nil), "an outage without usage_limit")
	wantCheckViolation(t, add(wake2, "at", "agent", nil, "-1 hour", "pending", nil), "a deadline before not_before")
	wantCheckViolation(t, add(wake2, "at", "agent", nil, "1 hour", "fired", nil), "an end without ended_at")
	wantCheckViolation(t, add(wake2, "at", "agent", nil, "1 hour", "pending", time.Now()), "pending with ended_at")
	wantCheckViolation(t, add(wake2, "at", "agent", nil, "1 hour", "asleep", time.Now()), "an unknown state")
	if err := add(wake2, "at", "usage_limit", 1, "1 hour", "pending", nil); err != nil {
		t.Fatalf("outage 1: %v", err)
	}
	wantState(t, add(wake3, "at", "usage_limit", 1, "1 hour", "pending", nil), "23505", "outage 1 twice")
	if err := add(wake3, "at", "usage_limit", 2, "1 hour", "pending", nil); err != nil {
		t.Fatalf("outage 2: %v", err)
	}
	if err := add(wake4, "pr_checks", "usage_limit", nil, "1 hour", "pending", nil); err != nil {
		t.Fatalf("a usage-limit recovery next to outages: %v", err)
	}

	runIns := `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggering_wake_id, triggered_by,
		state, billing_mode_expected, invocation_jti) VALUES ($1, $2, $3, $4, NULL, $5, $6, 'queued', 'organization', $1)`
	const wakeRun = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7562"
	wantCheckViolation(t, exec(app, runIns, wakeRun, w.thread, w.agent, "message", wake1, w.person), "a wake-up on a message run")
	wantCheckViolation(t, exec(app, runIns, wakeRun, w.thread, w.agent, "alarm", nil, w.person), "an unknown trigger kind")
	wantState(t, exec(app, runIns, wakeRun, w.thread, w.agent, "wake", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f76ff", w.person),
		"23503", "an unknown wake-up")
	mustExec(t, app, runIns, wakeRun, w.thread, w.agent, "wake", wake1, w.person)
	mustExec(t, app, `UPDATE run_wakes SET state = 'fired', ended_at = now(), fired_run_id = $2, version = version + 1 WHERE id = $1`,
		wake1, wakeRun)

	auto := `INSERT INTO run_autonomy (thread_id, person_id, chain_count, idle_count, ended_reason) VALUES ($1, $2, $3, $4, $5)`
	wantCheckViolation(t, exec(app, auto, w.thread, w.person, -1, 0, nil), "a negative chain count")
	mustExec(t, app, auto, w.thread, w.person, 3, 1, "autonomous_chain_max")
	wantState(t, exec(app, auto, w.thread, w.person, 0, 0, nil), "23505", "a second count row of one thread and person")
}

// TestMigration_WakesKeepExisting: runs, channels and agents with a stored heartbeat written
// before stage AR keep every value through 00097 to 00101; the new columns read as "as before"
// (NULL and false). Acceptance criterion 3 of AR-04.
func TestMigration_WakesKeepExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, sqlDB, app := wakeProvider(t)
	w := seedWakeWorld(t, app)
	mustExec(t, app, `UPDATE agents SET reported_details = '{"capabilities": {"followups": "steer"}}', last_heartbeat_at = now()
		WHERE principal_id = $1`, w.agent)
	snapshots := map[string]string{
		"runs":     `SELECT string_agg(row_to_json(r)::text, '|' ORDER BY id) FROM runs r`,
		"channels": `SELECT string_agg(row_to_json(c)::text, '|' ORDER BY id) FROM channels c`,
		"agents":   `SELECT string_agg(row_to_json(a)::text, '|' ORDER BY principal_id) FROM agents a`,
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
		"runs":     `SELECT string_agg((row_to_json(r)::jsonb - 'triggering_wake_id')::text, '|' ORDER BY id) FROM runs r`,
		"channels": `SELECT string_agg((row_to_json(c)::jsonb - 'allow_autonomous_runs')::text, '|' ORDER BY id) FROM channels c`,
		"agents":   snapshots["agents"],
	}
	for k, q := range after {
		var s string
		if err := sqlDB.QueryRowContext(ctx, q).Scan(&s); err != nil {
			t.Fatal(err)
		}
		// row_to_json and jsonb order members differently; compare as jsonb on both sides.
		var same bool
		if err := sqlDB.QueryRowContext(ctx, `SELECT (SELECT array_agg(x::jsonb) FROM unnest(string_to_array($1, '|')) x)
			= (SELECT array_agg(x::jsonb) FROM unnest(string_to_array($2, '|')) x)`, before[k], s).Scan(&same); err != nil {
			t.Fatal(err)
		}
		if !same {
			t.Errorf("%s changed by the migration:\n%s\n-> %s", k, before[k], s)
		}
	}
	var wakeIDs, allowed int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM runs WHERE triggering_wake_id IS NOT NULL),
		(SELECT count(*) FROM channels WHERE allow_autonomous_runs)`).Scan(&wakeIDs, &allowed); err != nil || wakeIDs != 0 || allowed != 0 {
		t.Fatalf("new columns after the migration: %d %d %v", wakeIDs, allowed, err)
	}
}

// TestMigration_WakesDown: down to 96 removes the tables and columns of stage AR while no wake run
// exists; with a wake run the down step of 00099 refuses and names the count, so a rollback of the
// schema never breaks the old check (the rollback note of the runs README).
func TestMigration_WakesDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, sqlDB, app := wakeProvider(t)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	w := seedWakeWorld(t, app)
	const wake = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7611"
	mustExec(t, app, `INSERT INTO run_wakes (id, agent_id, thread_id, source_run_id, triggered_by, kind, condition, reason,
		not_before, deadline, state, ended_at) VALUES ($1, $2, $3, $4, $5, 'at', '{}', 'agent', now(), now(), 'fired', now())`,
		wake, w.agent, w.thread, w.run, w.person)
	mustExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_wake_id, triggered_by, state,
		billing_mode_expected, invocation_jti, finished_at) VALUES ($1, $2, $3, 'wake', $4, $5, 'completed', 'organization', $1, now())`,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7563", w.thread, w.agent, wake, w.person)
	_, err := provider.DownTo(ctx, beforeWakes)
	if err == nil || !strings.Contains(err.Error(), "trigger_kind wake") {
		t.Fatalf("down with a wake run: %v, want the refusal", err)
	}
	mustExec(t, app, `UPDATE runs SET trigger_kind = 'message', triggering_wake_id = NULL WHERE trigger_kind = 'wake'`)
	if _, err := provider.DownTo(ctx, beforeWakes); err != nil {
		t.Fatalf("down without wake runs: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('run_wakes') IS NOT NULL)::int + (to_regclass('run_autonomy') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM information_schema.columns WHERE (table_name, column_name) IN
		(('runs', 'triggering_wake_id'), ('channels', 'allow_autonomous_runs')))`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d tables or columns of stage AR left after down", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

// wakeProvider returns a migration provider on a fresh database migrated down to beforeWakes, its
// SQL handle and a pool of bothy_app.
func wakeProvider(t *testing.T) (*goose.Provider, *sql.DB, *pg.Pool) {
	t.Helper()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(context.Background(), beforeWakes); err != nil {
		t.Fatalf("down to %d: %v", beforeWakes, err)
	}
	return provider, sqlDB, db.Pool(t, pg.RoleApp)
}
