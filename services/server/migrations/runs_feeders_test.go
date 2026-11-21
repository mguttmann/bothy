// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// The migration of the feeder record (FEAT-TEAM-CHANNEL-SERVER, R-DM-63, docs/spec/09-agents.md
// 3.9.4): 00108 creates run_feeders and the triggers that record every stored input of a run.

// TestMigration_RunFeedersTable holds run_feeders against docs/spec/03-data-model.md section 4.8
// and the three triggers that write it.
func TestMigration_RunFeedersTable(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	cols := []principalColumn{
		{"run_id", "uuid", false, ""}, {"principal_id", "uuid", true, ""}, {"source", "text", false, ""},
		{"created_at", "timestamp with time zone", false, "now()"},
	}
	if got := principalColumns(t, conn, "run_feeders"); !reflect.DeepEqual(got, cols) {
		t.Errorf("run_feeders columns:\n got %+v\nwant %+v", got, cols)
	}
	cons := principalConstraints(t, conn, "run_feeders")
	slices.Sort(cons)
	want := []string{
		"CHECK ((source = ANY (ARRAY['trigger'::text, 'followup'::text, 'interaction'::text, 'command'::text, 'wake_input'::text, 'context'::text])))",
		"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE",
		"UNIQUE NULLS NOT DISTINCT (run_id, principal_id, source)",
	}
	slices.Sort(want)
	if !reflect.DeepEqual(cons, want) {
		t.Errorf("run_feeders constraints:\n got %q\nwant %q", cons, want)
	}
	if got := feederTriggers(t, conn); !reflect.DeepEqual(got, inputTriggers) {
		t.Errorf("feeder triggers %v, want %v", got, inputTriggers)
	}
}

// inputTriggers are the tables whose rows are inputs of a run, each with the trigger that records
// its feeder in run_feeders.
var inputTriggers = map[string]string{
	"interactions":  "interactions_record_feeder",
	"run_followups": "run_followups_record_feeder",
	"runs":          "runs_record_feeder",
}

// feederTriggers returns the triggers that write run_feeders, by table.
func feederTriggers(t *testing.T, conn *pgx.Conn) map[string]string {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT c.relname, tg.tgname FROM pg_trigger tg JOIN pg_class c ON c.oid = tg.tgrelid
		WHERE NOT tg.tgisinternal AND tg.tgname LIKE '%_record_feeder'`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			t.Fatal(err)
		}
		out[table] = name
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestMigration_EveryRunReferenceIsClassified enumerates every column that references runs: each
// is an input of the run whose table records its feeder (inputTriggers), or it is named here as
// no input with the reason. A new table that points at a run fails until it is classified, so a
// new input path cannot store inputs without being recorded.
func TestMigration_EveryRunReferenceIsClassified(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	classified := map[string]string{
		"run_followups.run_id":        "input: run_followups_record_feeder records the follow-up's person",
		"message_components.run_id":   "input: interactions_record_feeder records the principal of every interaction with the component",
		"run_feeders.run_id":          "the record itself",
		"run_steps.run_id":            "no input: the steps the agent reports",
		"messages.run_id":             "no input: the anchor of the run's live box",
		"cost_entries.run_id":         "no input: the cost booked for the run",
		"run_wakes.source_run_id":     "no input: the wake-up the run's agent created; it reaches a run as agent.wake_input, recorded by the delivery",
		"run_wakes.fired_run_id":      "no input: the wake run it fired, whose trigger runs_record_feeder records",
		"run_wakes.fired_into_run_id": "no input: the run it fired into; the delivery of agent.wake_input records it",
	}
	rows, err := conn.Query(context.Background(), `SELECT c.conrelid::regclass::text || '.' || a.attname
		FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'f' AND c.confrelid = 'runs'::regclass ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		if _, ok := classified[ref]; !ok {
			t.Errorf("%s references runs but is not classified: record its inputs in run_feeders (a trigger in inputTriggers) or name why it is no input", ref)
		}
	}
	for ref := range classified {
		if !slices.Contains(refs, ref) {
			t.Errorf("%s is classified but no longer references runs", ref)
		}
	}
}

// TestMigration_RunFeedersRecordEveryStoredInput shows the record as bothy_app: a run records its
// trigger; a follow-up records the author of a person's message, the authority of a team input
// and NULL for any other message, also when it moves to the follow-on run; an interaction records
// its principal on the component's run. Deleting the card and its interaction leaves the rows, the
// server can neither change nor delete them, and they go with their run.
func TestMigration_RunFeedersRecordEveryStoredInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	w := seedWakeWorld(t, app)
	const (
		sam      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7801"
		run      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7811"
		next     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7812"
		human    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7821"
		agentMsg = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7822"
		teamMsg  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7823"
		card     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7824"
		moved    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7825"
		member   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7831"
		comp     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7841"
		ix       = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7851"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'sam', 'Sam')`, sam)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, sam)
	runIns := `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, billing_mode_expected, invocation_jti)
		VALUES ($1, $2, $3, 'message', $4, $5, 'organization', $1)`
	mustExec(t, app, runIns, run, w.thread, w.agent, w.person, "running")
	addMsg := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, $4, $5, $1, '\x00', $6, 'x')`
	mustExec(t, app, addMsg, human, w.thread, w.channel, 1, sam, "human")
	mustExec(t, app, addMsg, agentMsg, w.thread, w.channel, 2, w.agent, "agent")
	mustExec(t, app, addMsg, teamMsg, w.thread, w.channel, 3, w.agent, "agent")
	mustExec(t, app, addMsg, card, w.thread, w.channel, 4, w.agent, "agent")
	mustExec(t, app, addMsg, moved, w.thread, w.channel, 5, sam, "human")
	mustExec(t, app, `INSERT INTO team_channels (channel_id, sponsor_id) VALUES ($1, $2)`, w.channel, w.person)
	mustExec(t, app, `INSERT INTO team_members (id, channel_id, agent_id, home_channel_id, work_thread_id, handle, created_by)
		VALUES ($1, $2, $3, $2, $4, 'coding', $5)`, member, w.channel, w.agent, w.thread, w.person)
	mustExec(t, app, `INSERT INTO team_inputs (message_id, member_id, source_message_id, source, authority_id) VALUES ($1, $2, $3, 'owner', $4)`,
		teamMsg, member, human, w.person)
	for _, m := range []string{human, agentMsg, teamMsg, moved} {
		mustExec(t, app, `INSERT INTO run_followups (run_id, message_id) VALUES ($1, $2)`, run, m)
	}
	mustExec(t, app, `INSERT INTO message_components (id, message_id, custom_id, type, definition, position, run_id)
		VALUES ($1, $2, 'ask', 'button', '{}', 0, $3)`, comp, card, run)
	mustExec(t, app, `INSERT INTO interactions (id, component_id, message_id, principal_id, client_interaction_id)
		VALUES ($1, $2, $3, $4, $1)`, ix, comp, card, sam)
	mustExec(t, app, `DELETE FROM message_components WHERE id = $1`, comp)
	mustExec(t, app, `UPDATE runs SET state = 'completed', finished_at = now() WHERE id = $1`, run)
	mustExec(t, app, runIns, next, w.thread, w.agent, w.person, "queued")
	mustExec(t, app, `UPDATE run_followups SET run_id = $1 WHERE message_id = $2`, next, moved)

	feeders := func(r string) []string {
		t.Helper()
		rows, err := app.Query(ctx, `SELECT coalesce(principal_id::text, '-') || '/' || source FROM run_feeders WHERE run_id = $1`, r)
		if err != nil {
			t.Fatal(err)
		}
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(out)
		return out
	}
	want := []string{"-/followup", w.person + "/followup", w.person + "/trigger", sam + "/followup", sam + "/interaction"}
	slices.Sort(want)
	if got := feeders(run); !slices.Equal(got, want) {
		t.Errorf("feeders of the run:\n got %v\nwant %v", got, want)
	}
	if got, want := feeders(next), []string{w.person + "/trigger", sam + "/followup"}; !slices.Equal(got, want) {
		t.Errorf("feeders of the follow-on run %v, want %v", got, want)
	}
	wantState(t, exec(app, `UPDATE run_feeders SET principal_id = $1`, w.person), "42501", "bothy_app changing the record")
	wantState(t, exec(app, `DELETE FROM run_feeders WHERE run_id = $1`, run), "42501", "bothy_app deleting the record")
	if _, err := db.SuperConn(t).Exec(ctx, `DELETE FROM runs WHERE id = $1`, next); err != nil {
		t.Fatal(err)
	}
	if got := feeders(next); len(got) != 0 {
		t.Errorf("feeders of a deleted run %v", got)
	}
}
