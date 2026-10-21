// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMigration_RunFollowups holds run_followups against docs/spec/03-datenmodell.md section 4.8
// (M2-11): columns and defaults, the CHECK of the five states, the primary key, the unique message,
// both foreign keys with ON DELETE CASCADE and the index (run_id, state). As bothy_app it shows the
// rules: an unknown state fails, a second row of one message under another run fails, and
// deleting the run removes its rows. migrations.sha256 holds the line of the file.
func TestMigration_RunFollowups(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	want := []principalColumn{
		{"run_id", "uuid", false, ""},
		{"message_id", "uuid", false, ""},
		{"state", "text", false, "'queued'::text"},
		{"created_at", tz, false, "now()"},
		{"updated_at", tz, false, "now()"},
	}
	if got := principalColumns(t, conn, "run_followups"); !reflect.DeepEqual(got, want) {
		t.Errorf("run_followups columns:\n got %+v\nwant %+v", got, want)
	}
	cons := principalConstraints(t, conn, "run_followups")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK ((state = ANY (ARRAY['queued'::text, 'delivered'::text, 'absorbed'::text, 'deferred'::text, 'discarded'::text])))",
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE",
		"PRIMARY KEY (run_id, message_id)",
		"UNIQUE (message_id)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("run_followups constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX run_followups_run_id_state_idx ON public.run_followups USING btree (run_id, state)",
		"CREATE UNIQUE INDEX run_followups_message_id_key ON public.run_followups USING btree (message_id)",
		"CREATE UNIQUE INDEX run_followups_pkey ON public.run_followups USING btree (run_id, message_id)",
	}
	if got := indexes(t, conn, "run_followups"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("run_followups indexes:\n got %q\nwant %q", got, wantIdx)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		person  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7501"
		agent   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7502"
		space   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7511"
		channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7521"
		main    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7531"
		msg1    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7541"
		msg2    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7542"
		run1    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7561"
		run2    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7562"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, main, channel, person)
	msg := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, $4, $5, $1, '\x00', 'human', 'hello')`
	mustExec(t, app, msg, msg1, main, channel, 1, person)
	mustExec(t, app, msg, msg2, main, channel, 2, person)
	run := `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, state,
		billing_mode_expected, invocation_jti, finished_at) VALUES ($1, $2, $3, 'message', $4, $5, $6, 'organization', $1, $7)`
	mustExec(t, app, run, run1, main, agent, msg1, person, "completed", "2026-10-21T10:00:00Z")
	mustExec(t, app, run, run2, main, agent, msg1, person, "queued", nil)

	mustExec(t, app, `INSERT INTO run_followups (run_id, message_id) VALUES ($1, $2)`, run1, msg2)
	var state string
	if err := app.QueryRow(context.Background(), `SELECT state FROM run_followups WHERE message_id = $1`, msg2).Scan(&state); err != nil || state != "queued" {
		t.Fatalf("default state %q: %v", state, err)
	}
	wantCheckViolation(t, exec(app, `UPDATE run_followups SET state = 'failed' WHERE message_id = $1`, msg2), "an unknown state")
	wantState(t, exec(app, `INSERT INTO run_followups (run_id, message_id) VALUES ($1, $2)`, run2, msg2),
		"23505", "a second row of one message under another run")
	owner := db.Pool(t, pg.RoleOwner)
	mustExec(t, owner, `DELETE FROM runs WHERE id = $1`, run1)
	var left int
	if err := app.QueryRow(context.Background(), `SELECT count(*) FROM run_followups`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d rows after deleting the run: %v", left, err)
	}
	mustExec(t, app, `INSERT INTO run_followups (run_id, message_id) VALUES ($1, $2)`, run2, msg2)
	mustExec(t, owner, `DELETE FROM messages WHERE id = $1`, msg2)
	if err := app.QueryRow(context.Background(), `SELECT count(*) FROM run_followups`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d rows after deleting the message: %v", left, err)
	}

	sums, err := os.ReadFile("../migrations.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sums), "_runs_run_followups.sql\n") {
		t.Error("migrations.sha256 has no line of the run_followups migration")
	}
}
