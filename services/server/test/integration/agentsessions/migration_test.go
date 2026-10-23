// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// Fixture IDs of the migration tests: Alex, the agent Lotse, the space Support with the channels
// general and billing and their main threads, one run and the session rows.
const (
	migAlex     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7101"
	migLotse    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7102"
	migSpace    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7111"
	migGeneral  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7121"
	migBilling  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7122"
	migThreadA  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7131"
	migThreadB  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7132"
	migRun      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7161"
	migSession1 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7171"
	migSession2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7172"
	migSession3 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7173"
	migSession4 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7174"
)

// migColumn is one column of information_schema.columns: name, type, nullability and default.
type migColumn struct {
	Name, Type string
	Nullable   bool
	Default    string
}

// TestMigration_AgentSessions holds agent_sessions against 03 agent_sessions column by column and
// then shows its rules as bothy_app: a second current row of a thread fails; the same
// (agent_id, external_session_ref) current in two threads fails; the same reference ended in one
// thread and current in another passes; is_current contradicting ended_at fails; an unknown
// end_reason fails; deleting a session row sets runs.agent_session_id to NULL.
func TestMigration_AgentSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	want := []migColumn{
		{"id", "uuid", false, ""},
		{"thread_id", "uuid", false, ""},
		{"agent_id", "uuid", false, ""},
		{"external_session_ref", "text", false, ""},
		{"is_current", "boolean", false, "true"},
		{"started", "boolean", false, "false"},
		{"model", "text", true, ""},
		{"cumulative_cost_micros", "bigint", false, "0"},
		{"cost_baseline_micros", "bigint", false, "0"},
		{"currency", "text", false, "'USD'::text"},
		{"cost_is_estimate", "boolean", false, "true"},
		{"dev_environment_id", "uuid", true, ""},
		{"created_at", tz, false, "now()"},
		{"ended_at", tz, true, ""},
		{"end_reason", "text", true, ""},
		{"version", "bigint", false, "1"},
	}
	rows, err := conn.Query(ctx, `SELECT column_name, data_type, is_nullable = 'YES', coalesce(column_default, '')
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'agent_sessions' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	var got []migColumn
	for rows.Next() {
		var c migColumn
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("agent_sessions columns:\n got %+v\nwant %+v", got, want)
	}

	app := db.Pool(t, pg.RoleApp)
	migFixtures(t, app)
	migExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, billing_mode_expected, invocation_jti)
		VALUES ($1, $2, $3, 'message', $4, 'organization', $1)`, migRun, migThreadA, migLotse, migAlex)
	ins := `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at, end_reason)
		VALUES ($1, $2, $3, $4, $5, CASE WHEN $5 THEN NULL ELSE now() END, $6)`
	migExec(t, app, ins, migSession1, migThreadA, migLotse, "ref-a", true, nil)
	migWantState(t, migTry(app, ins, migSession2, migThreadA, migLotse, "ref-b", true, nil), "23505",
		"a second current row of a thread")
	migWantState(t, migTry(app, ins, migSession2, migThreadB, migLotse, "ref-a", true, nil), "23505",
		"the same reference current in two threads")
	// The same reference ended in one thread and current in another passes.
	migExec(t, app, ins, migSession2, migThreadB, migLotse, "ref-a", false, "import")
	migExec(t, app, ins, migSession3, migThreadB, migLotse, "ref-b", true, nil)
	migWantState(t, migTry(app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at)
		VALUES ($1, $2, $3, 'ref-c', false, NULL)`, migSession4, migThreadB, migLotse), "23514",
		"is_current false without ended_at")
	migWantState(t, migTry(app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, ended_at)
		VALUES ($1, $2, $3, 'ref-c', true, now())`, migSession4, migThreadB, migLotse), "23514",
		"is_current true with ended_at")
	migWantState(t, migTry(app, ins, migSession4, migThreadB, migLotse, "ref-c", false, "expired"), "23514",
		"an unknown end_reason")

	// runs.agent_session_id references agent_sessions: an unknown row fails, deleting the row sets
	// NULL. bothy_app never deletes a session row, so the owner deletes it.
	migWantState(t, migTry(app, `UPDATE runs SET agent_session_id = $1 WHERE id = $2`, migSession4, migRun), "23503",
		"runs.agent_session_id of an unknown session")
	migExec(t, app, `UPDATE runs SET agent_session_id = $1 WHERE id = $2`, migSession1, migRun)
	if _, err := conn.Exec(ctx, `DELETE FROM agent_sessions WHERE id = $1`, migSession1); err != nil {
		t.Fatalf("deleting a session row: %v", err)
	}
	var session *string
	if err := app.QueryRow(ctx, `SELECT agent_session_id::text FROM runs WHERE id = $1`, migRun).Scan(&session); err != nil {
		t.Fatal(err)
	}
	if session != nil {
		t.Errorf("runs.agent_session_id after the delete = %q, want NULL", *session)
	}
	var validated bool
	if err := conn.QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname = 'runs_agent_session_id_fkey'`).
		Scan(&validated); err != nil || !validated {
		t.Errorf("runs_agent_session_id_fkey validated = %v, %v", validated, err)
	}
}

// TestMigration_SystemCodeResumeFailed: on the migrated schema a system line (kind system, origin
// system, silent, empty content_md) with system_code agent_session.resume_failed passes, one with
// thread.reopened still passes, an unknown code fails, and the CHECK on system_code is validated.
func TestMigration_SystemCodeResumeFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	migFixtures(t, app)
	line := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, kind, silent, content_md, system_code)
		VALUES ($1, $2, $3, $4, $5, $1, '\x00', 'system', 'system', true, '', $6)`
	migExec(t, app, line, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7141", migThreadA, migGeneral, 1, migLotse, "agent_session.resume_failed")
	migExec(t, app, line, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7142", migThreadA, migGeneral, 2, migLotse, "thread.reopened")
	migWantState(t, migTry(app, line, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7143", migThreadA, migGeneral, 3, migLotse, "agent_session.lost"),
		"23514", "an unknown system_code")

	var def string
	var validated bool
	if err := db.SuperConn(t).QueryRow(ctx, `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conrelid = 'messages'::regclass AND conname = 'messages_system_code_check'`).Scan(&def, &validated); err != nil {
		t.Fatalf("messages_system_code_check: %v", err)
	}
	const wantDef = "CHECK ((system_code = ANY (ARRAY['thread.reopened'::text, 'agent_session.resume_failed'::text])))"
	if def != wantDef || !validated {
		t.Errorf("messages_system_code_check %q, validated %v; want %q, validated", def, validated, wantDef)
	}
}

// migFixtures inserts Alex, the agent Lotse, the space Support with the channels general and
// billing and their main threads A and B.
func migFixtures(t *testing.T, db *pg.Pool) {
	t.Helper()
	migExec(t, db, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, migAlex)
	migExec(t, db, `INSERT INTO users (principal_id) VALUES ($1)`, migAlex)
	migExec(t, db, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, migLotse)
	migExec(t, db, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, migLotse)
	migExec(t, db, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, migSpace, migAlex)
	channel := `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', $3, $4, 20, $5)`
	migExec(t, db, channel, migGeneral, migSpace, "general", "a0", migAlex)
	migExec(t, db, channel, migBilling, migSpace, "billing", "a1", migAlex)
	thread := `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`
	migExec(t, db, thread, migThreadA, migGeneral, migAlex)
	migExec(t, db, thread, migThreadB, migBilling, migAlex)
}

// migTry runs one statement and returns its error.
func migTry(db *pg.Pool, sql string, args ...any) error {
	_, err := db.Exec(context.Background(), sql, args...)
	return err
}

// migExec runs one statement and fails t when it fails.
func migExec(t *testing.T, db *pg.Pool, sql string, args ...any) {
	t.Helper()
	if err := migTry(db, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// migWantState fails t unless err is a PostgreSQL error with SQLSTATE state.
func migWantState(t *testing.T, err error, state, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != state {
		t.Errorf("%s: got %v, want SQLSTATE %s", what, err, state)
	}
}
