// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// The migrations of team channels (FEAT-TEAM-CHANNEL-SERVER, R-DM-62, docs/spec/09-agents.md 3.9):
// 00106 creates team_channels, team_members, team_inputs and team_mirrors and widens runs with the
// trigger kind team NOT VALID, 00107 validates the two checks.

// beforeTeams is the last version before the team channels.
const beforeTeams = 105

// TestMigration_TeamTables holds the four tables against docs/spec/03-data-model.md section 4.8:
// columns, checks, keys and indexes, and the validated checks of runs.
func TestMigration_TeamTables(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	type table struct {
		name string
		cols []principalColumn
		cons []string
	}
	for _, tb := range []table{
		{"team_channels", []principalColumn{
			{"channel_id", "uuid", false, ""}, {"sponsor_id", "uuid", false, ""}, {"member_turns", "integer", false, "0"},
			{"pingpong_from", "uuid", true, ""}, {"pingpong_to", "uuid", true, ""}, {"pingpong_count", "integer", false, "0"},
			{"pause_reason", "text", true, ""}, {"created_at", tz, false, "now()"}, {"updated_at", tz, false, "now()"},
			{"version", "bigint", false, "1"},
		}, []string{
			"CHECK (((member_turns >= 0) AND (pingpong_count >= 0)))",
			"CHECK ((pause_reason = ANY (ARRAY['turn_budget'::text, 'pingpong'::text, 'stopped'::text])))",
			"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
			"FOREIGN KEY (sponsor_id) REFERENCES principals(id) ON DELETE RESTRICT",
			"PRIMARY KEY (channel_id)",
		}},
		{"team_members", []principalColumn{
			{"id", "uuid", false, ""}, {"channel_id", "uuid", false, ""}, {"agent_id", "uuid", false, ""},
			{"home_channel_id", "uuid", false, ""}, {"work_thread_id", "uuid", false, ""}, {"handle", "text", false, ""},
			{"created_by", "uuid", false, ""}, {"created_at", tz, false, "now()"},
		}, []string{
			"CHECK ((handle ~ '^[a-z0-9][a-z0-9_-]*$'::text))",
			"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE CASCADE",
			"FOREIGN KEY (channel_id) REFERENCES team_channels(channel_id) ON DELETE CASCADE",
			"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (home_channel_id) REFERENCES channels(id) ON DELETE CASCADE",
			"FOREIGN KEY (work_thread_id) REFERENCES threads(id) ON DELETE CASCADE",
			"PRIMARY KEY (id)",
			"UNIQUE (channel_id, agent_id, home_channel_id)",
			"UNIQUE (channel_id, handle)",
			"UNIQUE (work_thread_id)",
		}},
		{"team_inputs", []principalColumn{
			{"message_id", "uuid", false, ""}, {"member_id", "uuid", false, ""}, {"source_message_id", "uuid", false, ""},
			{"source", "text", false, ""}, {"source_member_id", "uuid", true, ""}, {"authority_id", "uuid", false, ""},
			{"created_at", tz, false, "now()"},
		}, []string{
			"CHECK ((source = ANY (ARRAY['owner'::text, 'member'::text])))",
			"FOREIGN KEY (authority_id) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (member_id) REFERENCES team_members(id) ON DELETE CASCADE",
			"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
			"FOREIGN KEY (source_member_id) REFERENCES team_members(id) ON DELETE SET NULL",
			"FOREIGN KEY (source_message_id) REFERENCES messages(id) ON DELETE CASCADE",
			"PRIMARY KEY (message_id)",
			"UNIQUE (source_message_id, member_id)",
		}},
		{"team_mirrors", []principalColumn{
			{"source_message_id", "uuid", false, ""}, {"mirror_message_id", "uuid", false, ""}, {"member_id", "uuid", true, ""},
			{"created_at", tz, false, "now()"},
		}, []string{
			"FOREIGN KEY (member_id) REFERENCES team_members(id) ON DELETE SET NULL",
			"FOREIGN KEY (mirror_message_id) REFERENCES messages(id) ON DELETE CASCADE",
			"FOREIGN KEY (source_message_id) REFERENCES messages(id) ON DELETE CASCADE",
			"PRIMARY KEY (source_message_id)",
			"UNIQUE (mirror_message_id)",
		}},
	} {
		if got := principalColumns(t, conn, tb.name); !reflect.DeepEqual(got, tb.cols) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tb.name, got, tb.cols)
		}
		cons := principalConstraints(t, conn, tb.name)
		slices.Sort(cons)
		want := slices.Clone(tb.cons)
		slices.Sort(want)
		if !reflect.DeepEqual(cons, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tb.name, cons, want)
		}
	}
	var validated int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM pg_constraint WHERE conrelid = 'runs'::regclass
		AND conname IN ('runs_triggering_check', 'runs_trigger_kind_check') AND convalidated`).Scan(&validated); err != nil || validated != 2 {
		t.Errorf("validated team constraints of runs: %d %v, want 2", validated, err)
	}
	var idx []string
	for _, tb := range []string{"team_members", "team_inputs", "team_mirrors"} {
		for _, i := range indexes(t, conn, tb) {
			if strings.Contains(i, "_idx ") {
				idx = append(idx, i)
			}
		}
	}
	if len(idx) != 5 {
		t.Errorf("indexes of the referencing columns: %q, want 5", idx)
	}
}

// TestMigration_TeamRules shows the rules as bothy_app: a team on a channel, a member with a valid
// handle and its unique keys, an input once per source and member, and a team run that carries its
// input as triggering message, which a wake run cannot.
func TestMigration_TeamRules(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	w := seedWakeWorld(t, app)
	const (
		msg    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7701"
		input  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7702"
		member = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7711"
		other  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7712"
		run    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7721"
	)
	mustExec(t, app, `INSERT INTO team_channels (channel_id, sponsor_id) VALUES ($1, $2)`, w.channel, w.person)
	wantCheckViolation(t, exec(app, `UPDATE team_channels SET pause_reason = 'bored' WHERE channel_id = $1`, w.channel), "an unknown pause reason")
	addMember := `INSERT INTO team_members (id, channel_id, agent_id, home_channel_id, work_thread_id, handle, created_by)
		VALUES ($1, $2, $3, $2, $4, $5, $6)`
	wantCheckViolation(t, exec(app, addMember, member, w.channel, w.agent, w.thread, "Coding", w.person), "an upper-case handle")
	mustExec(t, app, addMember, member, w.channel, w.agent, w.thread, "coding", w.person)
	wantState(t, exec(app, addMember, other, w.channel, w.agent, w.thread, "review", w.person), "23505", "the same work thread twice")

	addMsg := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, $4, $5, $1, '\x00', $6, 'x')`
	mustExec(t, app, addMsg, msg, w.thread, w.channel, 1, w.person, "human")
	mustExec(t, app, addMsg, input, w.thread, w.channel, 2, w.agent, "agent")
	addInput := `INSERT INTO team_inputs (message_id, member_id, source_message_id, source, authority_id) VALUES ($1, $2, $3, $4, $5)`
	wantCheckViolation(t, exec(app, addInput, input, member, msg, "robot", w.person), "an unknown source")
	mustExec(t, app, addInput, input, member, msg, "owner", w.person)
	wantState(t, exec(app, addInput, msg, member, msg, "owner", w.person), "23505", "a source twice for one member")

	runIns := `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, state,
		billing_mode_expected, invocation_jti) VALUES ($1, $2, $3, $4, $5, $6, 'queued', 'organization', $1)`
	wantCheckViolation(t, exec(app, runIns, run, w.thread, w.agent, "wake", input, w.person), "a wake run with a triggering message")
	mustExec(t, app, runIns, run, w.thread, w.agent, "team", input, w.person)
	mustExec(t, app, `INSERT INTO team_mirrors (source_message_id, mirror_message_id, member_id) VALUES ($1, $2, $3)`, input, msg, member)
	mustExec(t, app, `DELETE FROM team_channels WHERE channel_id = $1`, w.channel)
	var left int
	if err := app.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM team_members) + (SELECT count(*) FROM team_inputs)`).
		Scan(&left); err != nil || left != 0 {
		t.Fatalf("members and inputs after switching team mode off: %d %v", left, err)
	}
}

// TestMigration_TeamDown: the down step of 00106 refuses while a team run exists and restores the
// old checks once none does.
func TestMigration_TeamDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, sqlDB, app := wakeProvider(t)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	w := seedWakeWorld(t, app)
	const run = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7731"
	mustExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, billing_mode_expected,
		invocation_jti, finished_at) VALUES ($1, $2, $3, 'team', $4, 'completed', 'organization', $1, now())`, run, w.thread, w.agent, w.person)
	if _, err := provider.DownTo(ctx, beforeTeams); err == nil || !strings.Contains(err.Error(), "trigger_kind team") {
		t.Fatalf("down with a team run: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM runs WHERE id = $1`, run); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, beforeTeams); err != nil {
		t.Fatalf("down: %v", err)
	}
	var tables int
	if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM pg_tables WHERE tablename LIKE 'team_%'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("team tables after down: %d %v", tables, err)
	}
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state,
		billing_mode_expected, invocation_jti) VALUES ($1, $2, $3, 'team', $4, 'queued', 'organization', $1)`, run, w.thread, w.agent, w.person); err == nil {
		t.Fatal("the old check accepted trigger_kind team")
	}
}
