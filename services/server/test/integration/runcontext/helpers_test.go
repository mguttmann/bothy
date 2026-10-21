// SPDX-License-Identifier: Apache-2.0

package runcontext_test

import (
	"context"
	"net/http"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

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
