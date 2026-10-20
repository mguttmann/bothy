// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// runsWorld is the world of 07 11.1 with the extensions of the brief M2-05 ("Tests"): Robin with
// write (no trigger) on Support/Eskalation, Texter with write on Support/Eskalation and
// allow_mentioned_agents on Support/Eskalation.
func runsWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	// The instance row of bothy migrate up (snapshot and catch-up read head and epoch).
	w.sql(`INSERT INTO instance (id, instance_id, schema_version, sync_epoch) VALUES (1, $1, 1, $2) ON CONFLICT (id) DO NOTHING`,
		ids.New().String(), ids.New().String())
	w.grant(`"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"channel","target_id":"` + w.eskalation.String() +
		`","level":"write","reason":"Escalation writer"`)
	w.grant(`"subject_kind":"agent","subject_id":"` + w.texter.String() + `","target_kind":"channel","target_id":"` + w.eskalation.String() +
		`","level":"write","reason":"Guest in escalation"`)
	w.behavior(w.eskalation, `"allow_mentioned_agents":true`)
	return w
}

// behavior replaces the behaviour of channel with the defaults of 09 3.3 and the extra members
// (putChannelBehavior as the owner, with step-up and If-Match).
func (w *world) behavior(channel ids.ID, extra string) {
	w.t.Helper()
	cur := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+channel.String()+"/behavior", ""))
	version := int64(cur["settings_version"].(float64))
	body := map[string]string{
		"require_mention": "true", "thread_require_mention": "false", "free_response": "false", "auto_thread": "true",
		"allow_mentioned_agents": "false", "history_backfill_limit": "20",
	}
	for _, kv := range strings.Split(extra, ",") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, ":")
		body[strings.Trim(k, `"`)] = v
	}
	var parts []string
	for k, v := range body {
		parts = append(parts, `"`+k+`":`+v)
	}
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/channels/"+channel.String()+"/behavior", "{"+strings.Join(parts, ",")+`,"prompt":"","skills":[]}`,
		ifMatch(version)...))
}

// post sends a text message as name into thread and returns the 201 body.
func (w *world) post(name string, thread ids.ID, text string) map[string]any {
	w.t.Helper()
	return w.want(w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages",
		`{"client_msg_id":"`+ids.New().String()+`","content_md":"`+text+`"}`, key()...), http.StatusCreated, "")
}

// newThread creates a thread of kind thread in the text channel as the owner without a first
// message: a message of the main thread (no mention: no trigger), then createThread on it.
func (w *world) newThread(channel ids.ID) ids.ID {
	w.t.Helper()
	parent := w.post("owner", w.mainThread(channel), "Question")
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/channels/"+channel.String()+"/threads",
		`{"parent_message_id":"`+parent["id"].(string)+`"}`, key()...), http.StatusCreated, "")
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string))
}

// runRow is the part of a runs row the tests read.
type runRow struct {
	ID, Thread, Agent, TriggeredBy, Triggering, State, Reason string
	Version                                                   int64
}

// runOf reads the run whose triggering message is message; ok is false without one.
func (w *world) runOf(message string) (runRow, bool) {
	w.t.Helper()
	rows := w.runsWhere(`triggering_message_id = $1`, message)
	if len(rows) == 0 {
		return runRow{}, false
	}
	if len(rows) > 1 {
		w.t.Fatalf("%d runs of one message", len(rows))
	}
	return rows[0], true
}

// runsWhere reads the runs that match the SQL condition cond.
func (w *world) runsWhere(cond string, args ...any) []runRow {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id::text, thread_id::text, agent_id::text, triggered_by::text,
		coalesce(triggering_message_id::text, ''), state, coalesce(state_reason, ''), version FROM runs WHERE `+cond+` ORDER BY queued_at, id`, args...)
	must(w.t, err)
	defer rs.Close()
	var out []runRow
	for rs.Next() {
		var r runRow
		must(w.t, rs.Scan(&r.ID, &r.Thread, &r.Agent, &r.TriggeredBy, &r.Triggering, &r.State, &r.Reason, &r.Version))
		out = append(out, r)
	}
	return out
}

// triggerOf reads trigger_state, trigger_state_reason and version of a message.
func (w *world) triggerOf(message string) (state, reason string, version int64) {
	w.t.Helper()
	must(w.t, w.db.QueryRow(context.Background(), `SELECT coalesce(trigger_state, ''), coalesce(trigger_state_reason, ''), version
		FROM messages WHERE id = $1`, message).Scan(&state, &reason, &version))
	return state, reason, version
}

// maxSeq is the highest seq of the event log.
func (w *world) maxSeq() int64 {
	return w.count(`SELECT coalesce(max(seq), 0) FROM event_log`)
}

// logEvent is one row of event_log after a position.
type logEvent struct {
	Type, Resource, Thread string
	Version                int64
}

// eventsAfter returns the events after seq in commit order.
func (w *world) eventsAfter(seq int64) []logEvent {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT type, coalesce(resource_id::text, ''), coalesce(thread_id::text, ''), coalesce(resource_version, 0)
		FROM event_log WHERE seq > $1 ORDER BY seq`, seq)
	must(w.t, err)
	defer rs.Close()
	var out []logEvent
	for rs.Next() {
		var e logEvent
		must(w.t, rs.Scan(&e.Type, &e.Resource, &e.Thread, &e.Version))
		out = append(out, e)
	}
	return out
}

// types returns the event types of list.
func types(list []logEvent) []string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.Type)
	}
	return out
}

// inUnit runs fn in a unit of work as Alex.
func (w *world) inUnit(fn func(ctx context.Context) error) error {
	ctx := actor.WithActor(context.Background(), w.actor("alex"))
	return w.uow.Do(ctx, fn)
}

// transition runs Transition of runs in a unit of work.
func (w *world) transition(cmd app.TransitionCmd) error {
	return w.inUnit(func(ctx context.Context) error {
		_, err := w.runs.Transition(ctx, cmd)
		return err
	})
}

// mustTransition is transition that fails the test on an error.
func (w *world) mustTransition(id string, from, to domain.State, version int64, reason string, by domain.Setter) {
	w.t.Helper()
	if err := w.transition(app.TransitionCmd{RunID: ids.MustParse(id), From: from, Version: version, To: to, Reason: reason, SetBy: by}); err != nil {
		w.t.Fatal(err)
	}
}

// openRun opens a run of Lotse for Alex in a new thread of Support/Allgemein through a message
// and returns the run and the message.
func (w *world) openRun() (runRow, string) {
	w.t.Helper()
	th := w.newThread(w.allgemein)
	m := w.post("alex", th, "Please look at the export")
	r, ok := w.runOf(m["id"].(string))
	if !ok {
		w.t.Fatal("the message opened no run")
	}
	return r, m["id"].(string)
}
