// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestAcceptance_ThreadMessageOpensRun: Alex writes in a thread of Support/Allgemein without
// mention; one queued run of Lotse, the anchor in the same thread, trigger_state received with
// version 2 in the 201, and the events in the order of brief note 6 step 8.
func TestAcceptance_ThreadMessageOpensRun(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.allgemein)
	before := w.maxSeq()
	body := w.post("alex", th, "The export fails since this morning")
	msg := body["id"].(string)
	if body["trigger_state"] != "received" || body["version"].(float64) != 2 || body["trigger_state_at"] == nil {
		t.Fatalf("201 body %v", body)
	}
	r, ok := w.runOf(msg)
	if !ok || r.State != "queued" || r.Agent != w.lotse.String() || r.TriggeredBy != w.alex.String() || r.Thread != th.String() || r.Version != 1 {
		t.Fatalf("run %+v %v", r, ok)
	}
	var kind, author, origin, content, anchor string
	var silent bool
	var anchorSeq, msgSeq int64
	must(t, w.db.QueryRow(t.Context(), `SELECT id::text, kind, author_principal_id::text, origin, content_md, silent, thread_seq FROM messages
		WHERE run_id = $1 AND thread_id = $2`, r.ID, th.String()).Scan(&anchor, &kind, &author, &origin, &content, &silent, &anchorSeq))
	if kind != "run" || author != w.lotse.String() || origin != "agent" || content != "" || !silent {
		t.Fatalf("anchor %s %s %s %q %v", kind, author, origin, content, silent)
	}
	must(t, w.db.QueryRow(t.Context(), `SELECT thread_seq FROM messages WHERE id = $1`, msg).Scan(&msgSeq))
	if anchorSeq != msgSeq+1 {
		t.Fatalf("thread_seq of the message %d, of the anchor %d", msgSeq, anchorSeq)
	}
	if st, reason, v := w.triggerOf(msg); st != "received" || reason != "" || v != 2 {
		t.Fatalf("trigger %s %s %d", st, reason, v)
	}
	evs := w.eventsAfter(before)
	// The acceptance ends with agent.invocation to the agent, without a token (M2-08).
	want := []string{"message.created", "message.created", "run.state_changed", "message.updated", "agent.invocation"}
	if !slices.Equal(types(evs), want) {
		t.Fatalf("events %v, want %v", types(evs), want)
	}
	if evs[0].Resource != msg || evs[1].Resource != anchor || evs[2].Resource != r.ID || evs[2].Version != 1 || evs[3].Resource != msg || evs[3].Version != 2 {
		t.Fatalf("event resources %+v", evs)
	}
	for _, e := range evs {
		if e.Thread != th.String() {
			t.Fatalf("event outside the thread %+v", e)
		}
	}
	// The anchor is in the history of the thread.
	page := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/threads/"+th.String()+"/messages", ""))
	found := false
	for _, it := range page["items"].([]any) {
		m := it.(map[string]any)
		if m["id"] == anchor {
			found = m["kind"] == "run" && m["run_id"] == r.ID
		}
	}
	if !found {
		t.Fatalf("anchor missing in the history: %v", page)
	}
}

// TestAcceptance_MainThreadAutoThread (server part of AG-05): Alex mentions Lotse in the main
// thread of Support/Allgemein; a new thread with parent_message_id M, title_source auto and
// operator Lotse; run and anchor live in it, trigger_state stays on M in the main thread.
func TestAcceptance_MainThreadAutoThread(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	main := w.mainThread(w.allgemein)
	before := w.maxSeq()
	body := w.post("alex", main, "@lotse the export fails")
	msg := body["id"].(string)
	if body["trigger_state"] != "received" || body["thread_id"] != main.String() {
		t.Fatalf("201 body %v", body)
	}
	var th, source, op, title string
	must(t, w.db.QueryRow(t.Context(), `SELECT id::text, title_source, coalesce(operator_agent_id::text, ''), title FROM threads WHERE parent_message_id = $1`,
		msg).Scan(&th, &source, &op, &title))
	if source != "auto" || op != w.lotse.String() || title != "" {
		t.Fatalf("thread %s %s %s %q", th, source, op, title)
	}
	r, ok := w.runOf(msg)
	if !ok || r.Thread != th {
		t.Fatalf("run %+v", r)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE run_id = $1 AND thread_id = $2`, r.ID, th); n != 1 {
		t.Fatalf("%d anchors in the new thread", n)
	}
	want := []string{"message.created", "thread.created", "message.created", "run.state_changed", "message.updated", "agent.invocation"}
	if got := types(w.eventsAfter(before)); !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
}

// noRun fails when message has a run, or when any thread or anchor appeared since the counts.
func (w *world) noRun(message string, threads, anchors int64) {
	w.t.Helper()
	if _, ok := w.runOf(message); ok {
		w.t.Fatalf("message %s opened a run", message)
	}
	if n := w.count(`SELECT count(*) FROM threads`); n != threads {
		w.t.Fatalf("threads %d, want %d", n, threads)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE kind = 'run'`); n != anchors {
		w.t.Fatalf("anchors %d, want %d", n, anchors)
	}
}

// TestAcceptance_Rejections: the rejections of 09 3.5 write failed with the reason and no run, no
// new thread and no anchor; a person without trigger and without mention gets nothing.
func TestAcceptance_Rejections(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	alg, esk := w.newThread(w.allgemein), w.newThread(w.eskalation)
	counts := func() (int64, int64) {
		return w.count(`SELECT count(*) FROM threads`), w.count(`SELECT count(*) FROM messages WHERE kind = 'run'`)
	}
	expect := func(name string, thread ids.ID, text, state, reason string) {
		t.Helper()
		threads, anchors := counts()
		m := w.post(name, thread, text)["id"].(string)
		if st, r, _ := w.triggerOf(m); st != state || r != reason {
			t.Fatalf("%q by %s: trigger %q %q, want %q %q", text, name, st, r, state, reason)
		}
		w.noRun(m, threads, anchors)
	}
	deniesBefore := len(w.denies("agent.invoke", "level_insufficient"))
	allInvoke := len(w.audits("agent.invoke"))
	// agent.read lets a person mention only agents that operate a channel the person reads, so the
	// mentions of Texter (operator of Marketing) come from the owner (D-M2-05-22).
	expect("owner", esk, "@lotse @texter both of you", "failed", "multiple_agents_mentioned")
	if got := len(w.audits("agent.invoke")); got != allInvoke {
		t.Fatalf("multiple_agents_mentioned wrote %d audit entries", got-allInvoke)
	}
	expect("owner", alg, "@texter please", "failed", "agent_not_allowed_here")
	expect("robin", esk, "@lotse please", "failed", "no_trigger_right")
	if got := len(w.denies("agent.invoke", "level_insufficient")); got != deniesBefore+1 {
		t.Fatalf("deny audit entries %d, want %d", got, deniesBefore+1)
	}
	expect("robin", esk, "just a note", "", "")
	if got := len(w.denies("agent.invoke", "level_insufficient")); got != deniesBefore+1 {
		t.Fatalf("a message without mention audited a denial")
	}
	// An active Lotse run in a thread of Support/Eskalation; Texter is mentioned there.
	busy := w.newThread(w.eskalation)
	first := w.post("alex", busy, "Lotse, please start")["id"].(string)
	if _, ok := w.runOf(first); !ok {
		t.Fatal("no run of Lotse")
	}
	expect("owner", busy, "@texter can you help", "failed", "run_active_other_agent")
	// A disabled Lotse.
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/agents/"+w.lotse.String()+"/disable", `{"reason":"Maintenance"}`, key()...))
	expect("alex", w.newThread(w.allgemein), "Anyone there", "failed", "agent_disabled")
}

// TestAcceptance_AgentMessageNeverTriggers: Lotse writes a message mentioning Texter in a channel
// where guests answer; no run, no trigger_state (mirrors AZ-M30, owned by M2-08).
func TestAcceptance_AgentMessageNeverTriggers(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.eskalation)
	threads, anchors := w.count(`SELECT count(*) FROM threads`), w.count(`SELECT count(*) FROM messages WHERE kind = 'run'`)
	m := w.post("lotse", th, "@texter please take over")["id"].(string)
	if st, _, _ := w.triggerOf(m); st != "" {
		t.Fatalf("trigger %q", st)
	}
	w.noRun(m, threads, anchors)
}

// TestAcceptance_ConcurrentMessagesOneRun: Alex and the owner write in the same thread at the same
// time; one run exists, and the other message has no trigger_state (follow-up, M2-11).
func TestAcceptance_ConcurrentMessagesOneRun(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.allgemein)
	var wg sync.WaitGroup
	got := make([]string, 2)
	for i, name := range []string{"alex", "owner"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := w.do(name, http.MethodPost, "/api/v1/threads/"+th.String()+"/messages",
				`{"client_msg_id":"`+ids.New().String()+`","content_md":"At the same time"}`, key()...)
			if r.StatusCode != http.StatusCreated {
				t.Errorf("%s: %d %s", name, r.StatusCode, r.Body)
				return
			}
			var m struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(r.Body, &m); err != nil {
				t.Error(err)
				return
			}
			got[i] = m.ID
		}()
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, th.String()); n != 1 {
		t.Fatalf("%d runs", n)
	}
	states := []string{}
	for _, m := range got {
		st, _, _ := w.triggerOf(m)
		states = append(states, st)
	}
	slices.Sort(states)
	if !slices.Equal(states, []string{"", "received"}) {
		t.Fatalf("trigger states %v", states)
	}
}
