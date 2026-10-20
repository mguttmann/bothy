// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigration_RunsAndRunSteps holds runs and run_steps against docs/spec/03-data-model.md
// section 4.8 column by column, with every check, key and index, and with exactly the deviations
// 00051 names: no foreign keys of agent_session_id, triggering_interaction_id and
// dev_environment_id yet (triggering_command_id references commands since 00054, M2-16).
// messages.run_id references runs, deferred and validated (00053). As bothy_app it then shows the rules: a second active run in
// one thread fails on the partial unique index, an end state needs finished_at, two triggering_*
// columns fail, and the anchor key is checked at the commit.
func TestMigration_RunsAndRunSteps(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	wantRuns := []principalColumn{
		{"id", "uuid", false, ""},
		{"thread_id", "uuid", false, ""},
		{"agent_id", "uuid", false, ""},
		{"agent_session_id", "uuid", true, ""},
		{"trigger_kind", "text", false, ""},
		{"triggering_message_id", "uuid", true, ""},
		{"triggering_command_id", "uuid", true, ""},
		{"triggering_interaction_id", "uuid", true, ""},
		{"triggered_by", "uuid", false, ""},
		{"state", "text", false, "'queued'::text"},
		{"state_reason", "text", true, ""},
		{"version", "bigint", false, "1"},
		{"billing_mode_expected", "text", false, ""},
		{"billing_mode_used", "text", true, ""},
		{"billing_evidence", "jsonb", false, "'{}'::jsonb"},
		{"invocation_jti", "uuid", false, ""},
		{"started_jti", "uuid", true, ""},
		{"renewed_from_jti", "uuid", true, ""},
		{"dev_environment_id", "uuid", true, ""},
		{"model", "text", true, ""},
		{"status_line", "text", true, ""},
		{"last_text_excerpt", "text", true, ""},
		{"plan", "jsonb", false, "'{}'::jsonb"},
		{"metrics", "jsonb", false, "'{}'::jsonb"},
		{"cost_micros", "bigint", false, "0"},
		{"currency", "text", false, "'USD'::text"},
		{"cost_is_estimate", "boolean", false, "true"},
		{"summary", "text", true, ""},
		{"stall_warned_at", tz, true, ""},
		{"queued_at", tz, false, "now()"},
		{"invocation_held_until", tz, true, ""},
		{"started_at", tz, true, ""},
		{"finished_at", tz, true, ""},
		{"duration_ms", "bigint", true, ""},
	}
	if got := principalColumns(t, conn, "runs"); !reflect.DeepEqual(got, wantRuns) {
		t.Errorf("runs columns:\n got %+v\nwant %+v", got, wantRuns)
	}
	cons := principalConstraints(t, conn, "runs")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK (((state <> ALL (ARRAY['completed'::text, 'failed'::text, 'cancelled'::text])) OR (finished_at IS NOT NULL)))",
		"CHECK ((((triggering_message_id IS NULL) OR (trigger_kind = 'message'::text)) AND ((triggering_command_id IS NULL) OR (trigger_kind = 'command'::text)) AND ((triggering_interaction_id IS NULL) OR (trigger_kind = 'interaction'::text))))",
		"CHECK ((billing_mode_expected = ANY (ARRAY['personal'::text, 'organization'::text])))",
		"CHECK ((state = ANY (ARRAY['queued'::text, 'running'::text, 'waiting_input'::text, 'completed'::text, 'failed'::text, 'cancelled'::text])))",
		"CHECK ((trigger_kind = ANY (ARRAY['message'::text, 'command'::text, 'interaction'::text])))",
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE RESTRICT",
		"FOREIGN KEY (triggered_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (triggering_command_id) REFERENCES commands(id) ON DELETE SET NULL",
		"FOREIGN KEY (triggering_message_id) REFERENCES messages(id) ON DELETE SET NULL",
		"PRIMARY KEY (id)",
		"UNIQUE (invocation_jti)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("runs constraints:\n got %q\nwant %q", cons, wantCons)
	}
	active := "WHERE (state = ANY (ARRAY['queued'::text, 'running'::text, 'waiting_input'::text]))"
	wantIdx := []string{
		"CREATE INDEX runs_agent_finished_idx ON public.runs USING btree (agent_id, finished_at DESC)",
		"CREATE INDEX runs_state_agent_active_idx ON public.runs USING btree (state, agent_id) " + active,
		"CREATE INDEX runs_thread_queued_idx ON public.runs USING btree (thread_id, queued_at DESC)",
		"CREATE UNIQUE INDEX runs_invocation_jti_key ON public.runs USING btree (invocation_jti)",
		"CREATE UNIQUE INDEX runs_pkey ON public.runs USING btree (id)",
		"CREATE UNIQUE INDEX runs_thread_active_key ON public.runs USING btree (thread_id) " + active,
	}
	if got := indexes(t, conn, "runs"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("runs indexes:\n got %q\nwant %q", got, wantIdx)
	}

	wantSteps := []principalColumn{
		{"id", "uuid", false, ""},
		{"run_id", "uuid", false, ""},
		{"ordinal", "integer", false, ""},
		{"parent_step_id", "uuid", true, ""},
		{"kind", "text", false, ""},
		{"name", "text", false, ""},
		{"title", "text", false, "''::text"},
		{"status", "text", false, "'running'::text"},
		{"detail", "jsonb", false, "'{}'::jsonb"},
		{"started_at", tz, false, "now()"},
		{"finished_at", tz, true, ""},
	}
	if got := principalColumns(t, conn, "run_steps"); !reflect.DeepEqual(got, wantSteps) {
		t.Errorf("run_steps columns:\n got %+v\nwant %+v", got, wantSteps)
	}
	scons := principalConstraints(t, conn, "run_steps")
	slices.Sort(scons)
	wantSCons := []string{
		"CHECK ((kind = ANY (ARRAY['tool_call'::text, 'status'::text, 'subagent'::text, 'workflow'::text, 'background_job'::text])))",
		"CHECK ((status = ANY (ARRAY['pending'::text, 'running'::text, 'completed'::text, 'failed'::text, 'cancelled'::text])))",
		"FOREIGN KEY (parent_step_id) REFERENCES run_steps(id) ON DELETE CASCADE",
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
		"UNIQUE (run_id, ordinal)",
	}
	slices.Sort(wantSCons)
	if !reflect.DeepEqual(scons, wantSCons) {
		t.Errorf("run_steps constraints:\n got %q\nwant %q", scons, wantSCons)
	}
	wantSIdx := []string{
		"CREATE INDEX run_steps_running_idx ON public.run_steps USING btree (run_id) WHERE (status = 'running'::text)",
		"CREATE UNIQUE INDEX run_steps_pkey ON public.run_steps USING btree (id)",
		"CREATE UNIQUE INDEX run_steps_run_id_ordinal_key ON public.run_steps USING btree (run_id, ordinal)",
	}
	if got := indexes(t, conn, "run_steps"); !reflect.DeepEqual(got, wantSIdx) {
		t.Errorf("run_steps indexes:\n got %q\nwant %q", got, wantSIdx)
	}

	var def string
	var validated bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conname = 'messages_run_id_fkey'`).Scan(&def, &validated); err != nil {
		t.Fatalf("messages_run_id_fkey: %v", err)
	}
	if def != "FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED" || !validated {
		t.Errorf("messages run foreign key %q, validated %v", def, validated)
	}
	// The deviation list of 00051: these keys come with the tickets that create their tables
	// (triggering_command_id came with M2-16, 00054 and 00055).
	var deviating int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
		WHERE c.conrelid = 'runs'::regclass AND c.contype = 'f'
		AND a.attname IN ('agent_session_id', 'triggering_interaction_id', 'dev_environment_id')`).Scan(&deviating); err != nil {
		t.Fatal(err)
	}
	if deviating != 0 {
		t.Errorf("runs has %d foreign keys of the deviation list, want none", deviating)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		person  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6501"
		agent   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6502"
		space   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6511"
		channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6521"
		main    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6531"
		msg     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6541"
		run1    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6561"
		run2    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6562"
		run3    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6563"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'robin', 'Robin')`, agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, main, channel, person)
	mustExec(t, app, `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, 1, $4, $1, '\x00', 'human', 'hello')`, msg, main, channel, person)

	ins := `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggering_command_id,
		triggered_by, state, billing_mode_expected, invocation_jti, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'organization', $1, $9)`
	mustExec(t, app, ins, run1, main, agent, "message", msg, nil, person, "queued", nil)
	wantState(t, exec(app, ins, run2, main, agent, "message", msg, nil, person, "running", nil),
		"23505", "a second active run in one thread")
	wantCheckViolation(t, exec(app, ins, run2, main, agent, "message", msg, nil, person, "completed", nil),
		"an end state without finished_at")
	wantCheckViolation(t, exec(app, ins, run2, main, agent, "message", msg, run3, person, "completed", time.Now()),
		"two triggering_* columns")
	wantCheckViolation(t, exec(app, ins, run2, main, agent, "command", msg, nil, person, "completed", time.Now()),
		"a triggering message on a command run")
	wantCheckViolation(t, exec(app, ins, run2, main, agent, "message", msg, nil, person, "done", time.Now()),
		"an unknown state")
	// An ended run frees the thread for the next active run.
	mustExec(t, app, ins, run2, main, agent, "message", msg, nil, person, "completed", time.Now())
	wantState(t, exec(app, `UPDATE runs SET state = 'cancelled' WHERE id = $1`, run1), "23514",
		"cancelled without finished_at")

	step := `INSERT INTO run_steps (id, run_id, ordinal, kind, name, status) VALUES ($1, $2, $3, $4, 'search', $5)`
	mustExec(t, app, step, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6571", run1, 1, "tool_call", "running")
	wantState(t, exec(app, step, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6572", run1, 1, "status", "failed"),
		"23505", "the same ordinal twice in one run")
	wantCheckViolation(t, exec(app, step, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6573", run1, 2, "thought", "running"),
		"an unknown step kind")
	wantCheckViolation(t, exec(app, step, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6574", run1, 2, "status", "done"),
		"an unknown step status")
	mustExec(t, app, `DELETE FROM run_steps WHERE run_id = $1 AND ordinal = 1`, run1)

	// The anchor key is deferred: the anchor may name its run before the run row exists, the commit
	// checks it.
	anchor := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, kind, silent, run_id) VALUES ($1, $2, $3, $4, $5, $1, '\x00', 'agent', 'run', true, $6)`
	wantState(t, exec(app, anchor, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6542", main, channel, 2, agent, run3),
		"23503", "an anchor of an unknown run")
	tx, err := app.Begin(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), anchor, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6543", main, channel, 2, agent, run3); err != nil {
		t.Fatalf("anchor before its run: %v", err)
	}
	if _, err := tx.Exec(context.Background(), `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by,
		state, billing_mode_expected, invocation_jti, finished_at) VALUES ($1, $2, $3, 'message', $4, 'failed', 'personal', $1, now())`,
		run3, main, agent, person); err != nil {
		t.Fatalf("run after its anchor: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit of anchor and run: %v", err)
	}
}

// TestMigration_RunsDown: down to version 50 removes runs, run_steps and the anchor key of
// messages; up applies again.
func TestMigration_RunsDown(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 50); err != nil {
		t.Fatalf("down to 50: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('runs') IS NOT NULL)::int + (to_regclass('run_steps') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM pg_constraint WHERE conname = 'messages_run_id_fkey')`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d of runs, run_steps and messages_run_id_fkey left after down", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
