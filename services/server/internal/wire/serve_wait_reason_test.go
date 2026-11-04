// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// TestWaitReasonOfAQueuedRun (F-WARTESCHLANGE-GRUND, 09 4.8 and 5.2.1): the runner reports a
// delivered run as waiting for a slot; the run document then carries state_reason slots_full,
// the agent card the slot use 2 of 2, and every change appends run.state_changed. A heartbeat
// that no longer lists the run clears the reason, and the start leaves none.
func TestWaitReasonOfAQueuedRun(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)
	ch, a, handle := w.operated("queue", false)
	c, _ := w.ready(t, w.agentParams(a, ""))
	readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
	m, _ := w.say(w.owner, ch.thread, "@"+handle+" please check the export")
	f := w.nextInvocation(c, true)
	sendAck(t, c, f.Seq)
	r, ok := w.runOf(m)
	if !ok || r.state != "queued" || r.reason != "" {
		t.Fatalf("run after delivery %+v %v", r, ok)
	}
	changes := func() int {
		return w.count(`SELECT count(*) FROM event_log WHERE type = 'run.state_changed' AND resource_id = $1`, r.id)
	}
	before := changes()

	beat := func(body string) {
		t.Helper()
		code, b := w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/heartbeat", json.RawMessage(body))
		if code != http.StatusOK {
			t.Fatalf("sendHeartbeat = %d %s", code, b)
		}
	}
	getRun := func() rest.Run {
		t.Helper()
		code, b := w.rest(w.base, w.owner, http.MethodGet, "/api/v1/runs/"+r.id, nil)
		if code != http.StatusOK {
			t.Fatalf("getRun = %d %s", code, b)
		}
		var doc rest.Run
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}

	beat(`{"state":"online","run_slots":{"used":2,"total":2},"waiting_runs":[{"run_id":"` + r.id + `","reason":"slots_full"}]}`)
	doc := getRun()
	if doc.State != rest.RunStateQueued || doc.StateReason == nil || *doc.StateReason != "slots_full" {
		t.Fatalf("run document state %s reason %v, want queued with slots_full", doc.State, doc.StateReason)
	}
	if got := changes(); got != before+1 {
		t.Fatalf("run.state_changed events %d, want %d", got, before+1)
	}
	code, b := w.rest(w.base, w.owner, http.MethodGet, "/api/v1/agents/"+a.id.String(), nil)
	if code != http.StatusOK {
		t.Fatalf("getAgent = %d %s", code, b)
	}
	var card rest.Agent
	if err := json.Unmarshal(b, &card); err != nil {
		t.Fatal(err)
	}
	if s := card.Presence.RunSlots; s == nil || s.Used != 2 || s.Total != 2 {
		t.Fatalf("agent card run_slots %+v, want 2 of 2: %s", s, b)
	}

	// The same report again changes nothing.
	beat(`{"state":"online","run_slots":{"used":2,"total":2},"waiting_runs":[{"run_id":"` + r.id + `","reason":"slots_full"}]}`)
	if got := changes(); got != before+1 {
		t.Fatalf("an unchanged report appended run.state_changed: %d", got)
	}

	// The run is no longer listed: the runner reason goes.
	beat(`{"state":"online","run_slots":{"used":1,"total":2}}`)
	if doc := getRun(); doc.StateReason != nil {
		t.Fatalf("state_reason %q after the run left waiting_runs", *doc.StateReason)
	}
	if got := changes(); got != before+2 {
		t.Fatalf("run.state_changed events %d after the clear, want %d", got, before+2)
	}

	// The start clears a reason the runner reported last.
	beat(`{"state":"online","run_slots":{"used":2,"total":2},"waiting_runs":[{"run_id":"` + r.id + `","reason":"thread_busy"}]}`)
	if doc := getRun(); doc.StateReason == nil || *doc.StateReason != "thread_busy" {
		t.Fatalf("state_reason %v, want thread_busy", doc.StateReason)
	}
	code, b = w.call(w.base, a, http.MethodPost, "/api/v1/runs/"+r.id+"/start",
		map[string]any{"invocation_token": f.InvocationToken, "billing_mode_used": "organization", "tool_class_used": "chat"})
	if code != http.StatusOK {
		t.Fatalf("startRun = %d %s", code, b)
	}
	if doc := getRun(); doc.State != rest.RunStateRunning || doc.StateReason != nil {
		t.Fatalf("after the start state %s reason %v", doc.State, doc.StateReason)
	}
	// A running run listed by mistake gets no reason.
	beat(`{"state":"online","run_slots":{"used":2,"total":2},"waiting_runs":[{"run_id":"` + r.id + `","reason":"slots_full"}]}`)
	if doc := getRun(); doc.StateReason != nil {
		t.Fatalf("a running run got the wait reason %q", *doc.StateReason)
	}
}
