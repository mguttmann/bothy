// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The wake-ups of stage AR (AR-04, docs/spec/09-agents.md 5.8): the server parts of AR-T08,
// AR-T09, AR-T13 and AR-T14, each operation with its refusals and each end state with its note,
// audit entry and notification event.

// TestWake_CreateAt: an owner's run asks for an at wake-up with a time in the past; it is stored
// pending with not_before at least run.wake_min_delay after now and the deadline run.wake_max_wait
// after now, the job runs.wake_due is scheduled at both, run.wake_changed goes out, the call is
// audited, and the agent and the readers of the thread list it (09 5.8.1, 5.8.2, 5.8.6).
func TestWake_CreateAt(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	now := w.clk.Now()
	m := w.want(w.createWake("lotse", r.ID, `{"kind":"at","reason":"agent","label":"Check the build","not_before":"`+
		now.Add(-time.Hour).Format(time.RFC3339)+`"}`), http.StatusCreated, "")
	id := m["id"].(string)
	got := w.wake(id)
	if got.State != "pending" || got.Kind != "at" || got.WakeReason != "agent" || got.Label != "Check the build" || got.Person != w.owner.String() ||
		!got.NotBefore.Equal(now.Add(wakeCfg.MinDelay)) || !got.Deadline.Equal(now.Add(wakeCfg.MaxWait)) || got.Ended {
		t.Fatalf("stored wake-up %+v", got)
	}
	if jobs := w.jobs.of(id); len(jobs) != 2 || !jobs[0].Equal(got.NotBefore) || !jobs[1].Equal(got.Deadline) {
		t.Fatalf("jobs %v", jobs)
	}
	ev := w.wakeEvents(r.Thread)
	if len(ev) != 1 || ev[0]["wake_id"] != id || ev[0]["state"] != "pending" || ev[0]["agent_id"] != w.lotse.String() ||
		ev[0]["triggered_by"] != w.owner.String() {
		t.Fatalf("run.wake_changed %v", ev)
	}
	if n := w.audits("run.wake.create", "allow", ""); n != 1 {
		t.Fatalf("%d allowed audit entries", n)
	}
	pending := w.wantOK(w.do("lotse", http.MethodGet, "/api/v1/agents/me/wakes", ""))
	if items := pending["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != id {
		t.Fatalf("listPendingRunWakes %v", pending)
	}
	for _, reader := range []string{"owner", "alex", "sam"} {
		page := w.wantOK(w.do(reader, http.MethodGet, "/api/v1/threads/"+r.Thread+"/wakes", ""))
		if items := page["items"].([]any); len(items) != 1 {
			t.Fatalf("%s: listThreadWakes %v", reader, page)
		}
	}
	// Another agent sees none of it; a thread nobody may read is 404.
	if items := w.wantOK(w.do("texter", http.MethodGet, "/api/v1/agents/me/wakes", ""))["items"].([]any); len(items) != 0 {
		t.Fatalf("another agent lists %v", items)
	}
	w.want(w.do("robin", http.MethodGet, "/api/v1/threads/"+r.Thread+"/wakes", ""), http.StatusNotFound, "not_found")
}

// TestWake_CreateRefusals: every refusal of run.wake.create is its own answer (06 R-API-30): a
// wrong field 400, the reasons of 09 5.8 409; a refused request stores nothing.
func TestWake_CreateRefusals(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	own := w.runOfPerson("owner")
	w.want(w.createWake("lotse", own.ID, `{"kind":"at","reason":"agent","label":"`+strings.Repeat("x", 121)+`"}`), http.StatusBadRequest, "validation_failed")
	w.want(w.createWake("lotse", own.ID, `{"kind":"at","reason":"recovery"}`), http.StatusBadRequest, "validation_failed")
	w.want(w.createWake("lotse", own.ID, `{"kind":"pr_checks","reason":"agent","condition":{"repository":"app","number":1}}`), http.StatusBadRequest,
		"validation_failed")
	// tool_class_mismatch: a command needs an agent of tool class host; Lotse is chat.
	w.wantReason(w.createWake("lotse", own.ID, `{"kind":"command","reason":"agent","condition":{"command_line":"test -f done"}}`), http.StatusConflict,
		"conflict", "tool_class_mismatch")
	// Another agent's run is not found.
	w.want(w.createWake("texter", own.ID, `{"kind":"at","reason":"agent"}`), http.StatusNotFound, "not_found")
	// autonomous_not_allowed: Alex is no owner and Support/Allgemein has no allow_autonomous_runs.
	alex := w.runOfPerson("alex")
	w.wantReason(w.createWake("lotse", alex.ID, `{"kind":"at","reason":"agent"}`), http.StatusConflict, "conflict", "autonomous_not_allowed")
	if n := w.audits("run.wake.create", "deny", "autonomous_not_allowed"); n != 1 {
		t.Fatalf("%d audit entries of the refusal", n)
	}
	// With the switch on Alex's run may ask, but pr_checks stays owner only.
	if ch := w.setAutonomous(w.allgemein, true); ch["allow_autonomous_runs"] != true {
		t.Fatalf("channel %v", ch)
	}
	w.atWake(alex.ID, "Alex's check")
	w.wantReason(w.createWake("lotse", alex.ID, `{"kind":"pr_checks","reason":"agent","condition":{"repository":"example/app","number":3}}`),
		http.StatusConflict, "conflict", "pr_checks_owner_only")
	// wake_max_pending: the thread holds at most run.wake_max_pending pending wake-ups.
	for range wakeCfg.MaxPending {
		w.atWake(own.ID, "")
	}
	w.wantReason(w.createWake("lotse", own.ID, `{"kind":"at","reason":"agent"}`), http.StatusConflict, "conflict", "wake_max_pending")
	if n := w.count(`SELECT count(*) FROM run_wakes WHERE thread_id = $1`, own.Thread); n != int64(wakeCfg.MaxPending) {
		t.Fatalf("%d wake-ups after the refusals", n)
	}
	// run_not_active: an ended run asks nothing; a deferred request is stored refused with its
	// note and run.wake_changed, so the person sees it (09 18.19.8).
	w.wantOK(w.finish("lotse", alex.ID, `"state":"completed"`))
	w.wantReason(w.createWake("lotse", alex.ID, `{"kind":"at","reason":"agent"}`), http.StatusConflict, "conflict", "run_not_active")
	before := w.count(`SELECT count(*) FROM run_wakes WHERE thread_id = $1`, alex.Thread)
	w.wantReason(w.createWake("lotse", alex.ID, `{"kind":"at","reason":"agent","label":"Late request","deferred":true}`), http.StatusConflict, "conflict",
		"run_not_active")
	if n := w.count(`SELECT count(*) FROM run_wakes WHERE thread_id = $1 AND state = 'refused' AND state_reason = 'run_not_active'`, alex.Thread); n != 1 ||
		w.count(`SELECT count(*) FROM run_wakes WHERE thread_id = $1`, alex.Thread) != before+1 {
		t.Fatalf("deferred refusal stored %d", n)
	}
	notes := w.notes(alex.Thread)
	if len(notes) != 1 || !strings.Contains(notes[0], "Late request") || !strings.Contains(notes[0], "run_not_active") {
		t.Fatalf("notes %v", notes)
	}
	ev := w.wakeEvents(alex.Thread)
	last := ev[len(ev)-1]
	if last["state"] != "refused" || last["note_message_id"] == nil || last["triggered_by"] != w.alex.String() {
		t.Fatalf("run.wake_changed %v", last)
	}
}

// TestWake_UsageLimitOutage (18.19.10): a usage_limit request carries the outage; the repetition
// of a pending outage moves not_before and the deadline counts from it (D-AR-01-12); a new outage
// gets its own wake-up; above run.usage_wake_max the request is stored refused with its note.
func TestWake_UsageLimitOutage(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	now := w.clk.Now()
	reset := now.Add(5 * 24 * time.Hour)
	body := func(outage int, at time.Time) string {
		return `{"kind":"at","reason":"usage_limit","outage":` + strconv.Itoa(outage) + `,"not_before":"` + at.Format(time.RFC3339) + `"}`
	}
	first := w.want(w.createWake("lotse", r.ID, body(1, reset)), http.StatusCreated, "")["id"].(string)
	if got := w.wake(first); !got.NotBefore.Equal(reset) || !got.Deadline.Equal(reset.Add(wakeCfg.MaxWait)) {
		t.Fatalf("window %+v", got)
	}
	later := reset.Add(2 * time.Hour)
	again := w.want(w.createWake("lotse", r.ID, body(1, later)), http.StatusOK, "")
	if again["id"] != first || !w.wake(first).NotBefore.Equal(later) || !w.wake(first).Deadline.Equal(later.Add(wakeCfg.MaxWait)) {
		t.Fatalf("repeated outage %v %+v", again, w.wake(first))
	}
	w.want(w.createWake("lotse", r.ID, body(1, reset)), http.StatusOK, "")
	if !w.wake(first).NotBefore.Equal(later) {
		t.Fatal("an earlier reset moved the wake-up back")
	}
	// A reset past the stored deadline moves the deadline in the same update, so not_before <=
	// deadline holds (issue 567 item 14, R-API-30: no 500 from run_wakes_window_check).
	beyond := w.wake(first).Deadline.Add(time.Hour)
	w.want(w.createWake("lotse", r.ID, body(1, beyond)), http.StatusOK, "")
	if got := w.wake(first); !got.NotBefore.Equal(beyond) || !got.Deadline.Equal(beyond.Add(wakeCfg.MaxWait)) || got.State != "pending" {
		t.Fatalf("a reset past the deadline %+v", got)
	}
	if jobs := w.jobs.of(first); len(jobs) == 0 || !jobs[len(jobs)-1].Equal(beyond.Add(wakeCfg.MaxWait)) {
		t.Fatalf("jobs after the move %v", jobs)
	}
	w.want(w.createWake("lotse", r.ID, body(2, reset)), http.StatusCreated, "")
	w.want(w.createWake("lotse", r.ID, body(3, reset)), http.StatusCreated, "")
	w.wantReason(w.createWake("lotse", r.ID, body(4, reset)), http.StatusConflict, "conflict", "usage_wake_max")
	if n := w.count(`SELECT count(*) FROM run_wakes WHERE source_run_id = $1 AND outage = 4 AND state = 'refused'`, r.ID); n != 1 {
		t.Fatalf("usage_wake_max stored %d", n)
	}
	if notes := w.notes(r.Thread); len(notes) != 1 || !strings.Contains(notes[0], "usage_wake_max") {
		t.Fatalf("notes %v", notes)
	}
	// The same refused outage again answers its refusal without a second row or note.
	w.wantReason(w.createWake("lotse", r.ID, body(4, reset)), http.StatusConflict, "conflict", "usage_wake_max")
	if len(w.notes(r.Thread)) != 1 {
		t.Fatal("a second note for the same outage")
	}
}

// TestWake_Recover (09 5.8.2, 18.19.9): only an ended failed run gets a recovery wake-up, one per
// run and reason; without the autonomy rule it is stored refused with its note.
func TestWake_Recover(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	recover := func(run, body string) *httpservertest.Response {
		return w.do("lotse", http.MethodPost, "/api/v1/runs/"+run+"/wakes/recover", body, key()...)
	}
	w.wantReason(recover(r.ID, `{"reason":"recovery"}`), http.StatusConflict, "conflict", "run_not_ended")
	w.want(recover(r.ID, `{"reason":"agent"}`), http.StatusBadRequest, "validation_failed")
	w.wantOK(w.finish("lotse", r.ID, `"state":"failed","reason":"runtime_error"`))
	first := w.want(recover(r.ID, `{"reason":"recovery","label":"Lost background work"}`), http.StatusCreated, "")
	if first["kind"] != "at" || first["reason"] != "recovery" || first["state"] != "pending" {
		t.Fatalf("recovery %v", first)
	}
	again := w.want(recover(r.ID, `{"reason":"recovery"}`), http.StatusOK, "")
	if again["id"] != first["id"] {
		t.Fatalf("repetition %v", again)
	}
	w.want(recover(r.ID, `{"reason":"usage_limit"}`), http.StatusCreated, "")
	// Alex is no owner and the channel has no switch: the recovery is stored refused with its note.
	alex := w.runOfPerson("alex")
	w.wantOK(w.finish("lotse", alex.ID, `"state":"failed","reason":"runtime_error"`))
	w.wantReason(recover(alex.ID, `{"reason":"recovery","label":"Lost work"}`), http.StatusConflict, "conflict", "autonomous_not_allowed")
	if n := w.count(`SELECT count(*) FROM run_wakes WHERE source_run_id = $1 AND state = 'refused'`, alex.ID); n != 1 {
		t.Fatalf("refused recovery stored %d", n)
	}
	if notes := w.notes(alex.Thread); len(notes) != 1 || !strings.Contains(notes[0], "Lost work") {
		t.Fatalf("notes %v", notes)
	}
}

// TestWake_RecoverUsageLimitWhileOutagePending (issue 567 item 4, 09 18.19.10): a usage_limit
// recovery after a restart changes nothing while the outage's wake-up of run.wake.create is
// pending, so one outage never gets two wake-ups; it answers that wake-up.
func TestWake_RecoverUsageLimitWhileOutagePending(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	reset := w.clk.Now().Add(3 * time.Hour)
	outage := w.want(w.createWake("lotse", r.ID, `{"kind":"at","reason":"usage_limit","outage":1,"not_before":"`+reset.Format(time.RFC3339)+`"}`),
		http.StatusCreated, "")["id"].(string)
	w.wantOK(w.finish("lotse", r.ID, `"state":"failed","reason":"usage_limited"`))
	got := w.want(w.do("lotse", http.MethodPost, "/api/v1/runs/"+r.ID+"/wakes/recover", `{"reason":"usage_limit"}`, key()...), http.StatusOK, "")
	if got["id"] != outage || got["state"] != "pending" {
		t.Fatalf("recover answered %v", got)
	}
	if n := w.count(`SELECT count(*) FROM run_wakes WHERE source_run_id = $1 AND reason = 'usage_limit'`, r.ID); n != 1 {
		t.Fatalf("%d usage_limit wake-ups of one outage", n)
	}
	if !w.wake(outage).NotBefore.Equal(reset) {
		t.Fatalf("the pending wake-up changed %+v", w.wake(outage))
	}
	// Once that wake-up is no longer pending, a recovery after a restart gets its own.
	w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+outage+"/cancel", `{"reason":"usage_cleared"}`, key()...))
	w.want(w.do("lotse", http.MethodPost, "/api/v1/runs/"+r.ID+"/wakes/recover", `{"reason":"usage_limit"}`, key()...), http.StatusCreated, "")
}

// TestWake_RecoverUsageLimitWhileOutagePendingAfterAnyEnd (issue 567 item 4, 09 5.8.2): the
// pending outage wake-up is answered for every ended source run, also one that ended cancelled
// or completed, instead of a second, refused wake-up with run_not_failed.
func TestWake_RecoverUsageLimitWhileOutagePendingAfterAnyEnd(t *testing.T) {
	for _, end := range []string{`"state":"cancelled","reason":"user_cancelled"`, `"state":"completed"`} {
		w := newWakeWorld(t, wakeCfg)
		r := w.runOfPerson("owner")
		reset := w.clk.Now().Add(3 * time.Hour)
		outage := w.want(w.createWake("lotse", r.ID, `{"kind":"at","reason":"usage_limit","outage":1,"not_before":"`+reset.Format(time.RFC3339)+`"}`),
			http.StatusCreated, "")["id"].(string)
		w.wantOK(w.finish("lotse", r.ID, end))
		if st := w.wake(outage).State; st != "pending" {
			t.Fatalf("%s: the end of the run left the outage wake-up %s", end, st)
		}
		got := w.want(w.do("lotse", http.MethodPost, "/api/v1/runs/"+r.ID+"/wakes/recover", `{"reason":"usage_limit"}`, key()...), http.StatusOK, "")
		if got["id"] != outage || got["state"] != "pending" {
			t.Fatalf("%s: recover answered %v", end, got)
		}
		if n := w.count(`SELECT count(*) FROM run_wakes WHERE source_run_id = $1`, r.ID); n != 1 {
			t.Fatalf("%s: %d wake-ups of one outage", end, n)
		}
	}
}

// TestWake_RecoverOfARunThatDidNotFail (issue 567 item 7, 09 18.19.9): lost work of a run that
// ended cancelled or completed gets no recovery wake-up; the wake-up is stored refused with the
// reason run_not_failed, so the thread gets the note and the person the notification; a run that
// has not ended stores nothing.
func TestWake_RecoverOfARunThatDidNotFail(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	recover := func(run, body string) *httpservertest.Response {
		return w.do("lotse", http.MethodPost, "/api/v1/runs/"+run+"/wakes/recover", body, key()...)
	}
	for _, end := range []string{`"state":"cancelled","reason":"user_cancelled"`, `"state":"completed"`} {
		r := w.runOfPerson("owner")
		w.wantReason(recover(r.ID, `{"reason":"recovery","label":"Lost background work"}`), http.StatusConflict, "conflict", "run_not_ended")
		if n := w.count(`SELECT count(*) FROM run_wakes WHERE source_run_id = $1`, r.ID); n != 0 {
			t.Fatalf("a running run stored %d", n)
		}
		w.wantOK(w.finish("lotse", r.ID, end))
		w.wantReason(recover(r.ID, `{"reason":"recovery","label":"Lost background work"}`), http.StatusConflict, "conflict", "run_not_failed")
		if n := w.count(`SELECT count(*) FROM run_wakes WHERE source_run_id = $1 AND state = 'refused' AND state_reason = 'run_not_failed'`,
			r.ID); n != 1 {
			t.Fatalf("%s: refused wake-ups %d", end, n)
		}
		notes := w.notes(r.Thread)
		if len(notes) != 1 || !strings.Contains(notes[0], "Lost background work") || !strings.Contains(notes[0], "run_not_failed") {
			t.Fatalf("%s: notes %v", end, notes)
		}
		ev := w.wakeEvents(r.Thread)
		if last := ev[len(ev)-1]; last["state"] != "refused" || last["note_message_id"] == nil {
			t.Fatalf("%s: run.wake_changed %v", end, last)
		}
		// A repetition (the next runner after a crash) adds no second note.
		recover(r.ID, `{"reason":"recovery","label":"Lost background work"}`)
		if len(w.notes(r.Thread)) != 1 {
			t.Fatalf("%s: a second note", end)
		}
		if n := w.audits("run.wake.recover", "deny", "run_not_failed"); n < 1 {
			t.Fatalf("%s: %d audit entries", end, n)
		}
	}
}

// TestWake_CheckFireAndWakeInput (AR-T09, AR-T14 server part): run.wake.check refuses before
// not_before and allows after it with the configured values; a fire into the running run of the
// same person is one wake input with its own token (trigger_kind wake, wake_id, run, thread, sub);
// a skipped input returns the wake-up to pending; it fires after that run as a wake run whose
// agent.invocation names the wake-up and whose token starts the run.
func TestWake_CheckFireAndWakeInput(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	id := w.prWake(r.ID)
	check := func() *httpservertest.Response {
		return w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/check", "")
	}
	w.wantReason(check(), http.StatusConflict, "conflict", "wake_not_due")
	w.want(w.do("texter", http.MethodPost, "/api/v1/wakes/"+id+"/check", ""), http.StatusNotFound, "not_found")
	w.clk.Advance(wakeCfg.MinDelay)
	got := w.wantOK(check())
	if got["tool_class"] != "chat" || got["billing_mode"] != "organization" {
		t.Fatalf("check %v", got)
	}
	fired := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/fire", `{"outcome":"fired","result_text":"head abc1234: build success"}`))
	if fired["state"] != "fired" || fired["fired_into_run_id"] != r.ID {
		t.Fatalf("fire %v", fired)
	}
	inputs := w.payloads(`SELECT payload::text FROM event_log WHERE type = 'agent.wake_input' AND resource_id = $1`, r.ID)
	if len(inputs) != 1 || inputs[0]["wake_id"] != id || inputs[0]["wake"].(map[string]any)["result_text"] != "head abc1234: build success" {
		t.Fatalf("agent.wake_input %v", inputs)
	}
	out := w.deliverWakeInput(r.ID)
	if !out.Send {
		t.Fatal("the wake input was not sent")
	}
	c := claimsOf(t, out.InvocationToken)
	if c["trigger_kind"] != "wake" || c["wake_id"] != id || c["run_id"] != r.ID || c["thread_id"] != r.Thread || c["sub"] != w.owner.String() {
		t.Fatalf("token claims %v", c)
	}
	if n := w.count(`SELECT chain_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2`, r.Thread, w.owner.String()); n != 1 {
		t.Fatalf("chain count %d", n)
	}
	// The runner skipped the input: the wake-up returns to pending once and waits for the run.
	skip := `{"outcome":"input_skipped","run_id":"` + r.ID + `"}`
	if back := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/fire", skip)); back["state"] != "pending" || back["state_reason"] != "wake_input_skipped" {
		t.Fatalf("skip %v", back)
	}
	w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/fire", skip))
	if v := w.wake(id); v.State != "pending" || v.FiredInto != "" {
		t.Fatalf("after a repeated skip %+v", v)
	}
	w.due(id)
	if v := w.wake(id); v.State != "pending" {
		t.Fatalf("the skipped input fired into the same run: %+v", v)
	}
	// The runner keeps checking the pending wake-up (09 18.19.8) and reports the condition again:
	// the skip is kept, nothing fires into the same run and the chain count stays.
	w.wantOK(check())
	again := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/fire", `{"outcome":"fired","result_text":"head abc1234: build success"}`))
	if again["state"] != "pending" || again["state_reason"] != "wake_input_skipped" {
		t.Fatalf("a fire after the skip %v", again)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.wake_input' AND resource_id = $1`, r.ID); n != 1 {
		t.Fatalf("wake inputs into the skipped run %d", n)
	}
	if n := w.count(`SELECT chain_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2`, r.Thread, w.owner.String()); n != 1 {
		t.Fatalf("chain count after a fire of the skipped wake-up %d", n)
	}
	// The run ends; the job fires the wake-up as a wake run.
	jobsBefore := len(w.jobs.of(id))
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	if len(w.jobs.of(id)) != jobsBefore+1 {
		t.Fatal("the end of the run scheduled no runs.wake_due")
	}
	w.due(id)
	v := w.wake(id)
	if v.State != "fired" || v.FiredRun == "" {
		t.Fatalf("after the run %+v", v)
	}
	wr := w.runsWhere(`id = $1`, v.FiredRun)[0]
	if wr.TriggeredBy != w.owner.String() || wr.State != "queued" || w.text(`SELECT trigger_kind FROM runs WHERE id = $1`, wr.ID) != "wake" ||
		w.text(`SELECT triggering_wake_id::text FROM runs WHERE id = $1`, wr.ID) != id {
		t.Fatalf("wake run %+v", wr)
	}
	inv := w.payloads(`SELECT payload::text FROM event_log WHERE type = 'agent.invocation' AND resource_id = $1`, wr.ID)
	if len(inv) != 1 || inv[0]["trigger_kind"] != "wake" || inv[0]["wake_id"] != id || inv[0]["message_id"] != nil ||
		inv[0]["wake"].(map[string]any)["label"] != "CI of #12" {
		t.Fatalf("agent.invocation %v", inv)
	}
	tok := w.deliver(wr.ID)
	if c := claimsOf(t, tok); c["trigger_kind"] != "wake" || c["wake_id"] != id {
		t.Fatalf("wake run token %v", c)
	}
	run := w.wantOK(w.start("lotse", wr.ID, tok))
	if run["trigger_kind"] != "wake" || run["triggering_wake_id"] != id {
		t.Fatalf("started wake run %v", run)
	}
	if n := w.count(`SELECT chain_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2`, r.Thread, w.owner.String()); n != 2 {
		t.Fatalf("chain count after the wake run %d", n)
	}
}

// TestWake_OtherPersonsRunKeepsPending (AR-T09): a wake-up that fires while another person's run
// is active stays pending and fires after it.
func TestWake_OtherPersonsRunKeepsPending(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	id := w.atWake(r.ID, "Later")
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	// Alex writes in the same thread: a run of Alex is active when the wake-up is due.
	m := w.post("alex", ids.MustParse(r.Thread), "One more thing")
	alexRun, ok := w.runOf(m["id"].(string))
	if !ok {
		t.Fatal("Alex's message opened no run")
	}
	w.clk.Advance(wakeCfg.MinDelay)
	w.due(id)
	if v := w.wake(id); v.State != "pending" {
		t.Fatalf("fired into another person's run: %+v", v)
	}
	w.wantOK(w.start("lotse", alexRun.ID, w.deliver(alexRun.ID)))
	w.wantOK(w.finish("lotse", alexRun.ID, `"state":"completed"`))
	w.due(id)
	if v := w.wake(id); v.State != "fired" || v.FiredRun == "" {
		t.Fatalf("after the other run %+v", v)
	}
}

// TestWake_EndStates (09 5.8.4): every end other than fired posts the note that names the label
// and the reason; refused, expired and failed carry it in run.wake_changed for the notification;
// a cancel posts the note only; each end is counted.
func TestWake_EndStates(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	expiring := w.atWake(r.ID, "Nightly report")
	// failed: the runner gives up after its failures.
	failing := w.prWake(r.ID)
	if v := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+failing+"/fire", `{"outcome":"failed","reason":"gh_missing"}`)); v["state"] != "failed" ||
		v["state_reason"] != "gh_missing" {
		t.Fatalf("failed %v", v)
	}
	w.wantReason(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+failing+"/fire", `{"outcome":"failed"}`), http.StatusConflict, "conflict", "wake_not_pending")
	// cancelled by the agent and by the person; another person without manage may not.
	byAgent := w.atWake(r.ID, "Agent cancels")
	if v := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+byAgent+"/cancel", `{"reason":"usage_cleared"}`, key()...)); v["state"] != "cancelled" ||
		v["state_reason"] != "usage_cleared" {
		t.Fatalf("cancel by the agent %v", v)
	}
	byPerson := w.atWake(r.ID, "Person cancels")
	w.want(w.do("alex", http.MethodPost, "/api/v1/wakes/"+byPerson+"/cancel", `{}`, key()...), http.StatusForbidden, "")
	if v := w.wantOK(w.do("owner", http.MethodPost, "/api/v1/wakes/"+byPerson+"/cancel", `{}`, key()...)); v["state_reason"] != "cancelled_by_person" {
		t.Fatalf("cancel by the person %v", v)
	}
	// A repeated cancel answers the ended wake-up unchanged.
	if v := w.wantOK(w.do("owner", http.MethodPost, "/api/v1/wakes/"+byPerson+"/cancel", `{}`, key()...)); v["state"] != "cancelled" {
		t.Fatalf("repeated cancel %v", v)
	}
	// expired: the job finds the deadline passed (last, because the agent's token expires too).
	w.clk.Advance(wakeCfg.MaxWait)
	w.due(expiring)
	if v := w.wake(expiring); v.State != "expired" || v.Reason != "deadline_passed" || !v.Ended {
		t.Fatalf("expired %+v", v)
	}
	notes := w.notes(r.Thread)
	for _, want := range []string{"Nightly report", "CI of #12", "Agent cancels", "Person cancels"} {
		if !slices.ContainsFunc(notes, func(n string) bool { return strings.Contains(n, want) }) {
			t.Errorf("no note names %q: %v", want, notes)
		}
	}
	notified := 0
	for _, ev := range w.wakeEvents(r.Thread) {
		switch ev["state"] {
		case "expired", "failed", "cancelled":
			if ev["note_message_id"] == nil {
				t.Errorf("an end without its note: %v", ev)
			}
			if ev["state"] != "cancelled" {
				notified++
			}
		}
	}
	if notified != 2 {
		t.Fatalf("%d ends for a notification", notified)
	}
	if len(w.counts.ended) != 4 {
		t.Fatalf("counted ends %v", w.counts.ended)
	}
}

// TestWake_CheckRefused (R-AG-50, 09 5.8.3): before an execution the server checks the moment's
// state; a channel that became read-only refuses the check, the wake-up ends refused with the
// audit entry naming the check, the note and the notification, and the check does not run again.
func TestWake_CheckRefused(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	id := w.prWake(r.ID)
	w.clk.Advance(wakeCfg.MinDelay)
	v := w.count(`SELECT version FROM channels WHERE id = $1`, w.allgemein.String())
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/channels/"+w.allgemein.String()+"/read-only", `{"read_only":true,"reason":"Frozen"}`, ifMatch(v)...))
	w.wantReason(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/check", ""), http.StatusConflict, "conflict", "channel_read_only")
	if got := w.wake(id); got.State != "refused" || got.Reason != "channel_read_only" {
		t.Fatalf("refused check %+v", got)
	}
	if n := w.audits("run.wake.check", "deny", "channel_read_only"); n != 1 {
		t.Fatalf("%d audit entries of the refused check", n)
	}
	if notes := w.notes(r.Thread); len(notes) != 1 || !strings.Contains(notes[0], "channel_read_only") {
		t.Fatalf("notes %v", notes)
	}
	w.wantReason(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/check", ""), http.StatusConflict, "conflict", "wake_not_pending")
}

// TestWake_ChainCap (AR-T13 server part, 09 5.8.6): two wake runs reach run.autonomous_chain_max;
// the next fire is refused with autonomous_chain_max, a note that a reply continues and the
// counter; another person's run does not reset the owner's chain, a run of the owner does.
func TestWake_ChainCap(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	src := r.ID
	for i := range wakeCfg.ChainMax {
		id := w.atWake(src, "Step")
		w.wantOK(w.finish("lotse", src, `"state":"completed"`))
		w.clk.Advance(wakeCfg.MinDelay)
		w.due(id)
		v := w.wake(id)
		if v.State != "fired" {
			t.Fatalf("fire %d: %+v", i, v)
		}
		src = v.FiredRun
		w.wantOK(w.start("lotse", src, w.deliver(src)))
	}
	id := w.atWake(src, "One step too many")
	w.wantOK(w.finish("lotse", src, `"state":"completed"`))
	// Alex writes in the thread: a run of Alex, which resets only Alex's own counts.
	alexRun, _ := w.runOf(w.post("alex", ids.MustParse(r.Thread), "A question of mine")["id"].(string))
	w.wantOK(w.start("lotse", alexRun.ID, w.deliver(alexRun.ID)))
	w.wantOK(w.finish("lotse", alexRun.ID, `"state":"completed"`))
	if n := w.count(`SELECT chain_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2`, r.Thread, w.owner.String()); n != int64(wakeCfg.ChainMax) {
		t.Fatalf("the owner's chain after Alex's run: %d", n)
	}
	w.clk.Advance(wakeCfg.MinDelay)
	w.due(id)
	if v := w.wake(id); v.State != "refused" || v.Reason != "autonomous_chain_max" {
		t.Fatalf("over the cap %+v", v)
	}
	notes := w.notes(r.Thread)
	if len(notes) == 0 || !strings.Contains(notes[len(notes)-1], "reply") {
		t.Fatalf("notes %v", notes)
	}
	if !slices.Contains(w.counts.caps, "autonomous_chain_max") || w.audits("run.wake.fire", "deny", "autonomous_chain_max") != 1 {
		t.Fatalf("caps %v", w.counts.caps)
	}
	// A message of the owner starts a run that is no wake run and resets the chain.
	w.post("owner", ids.MustParse(r.Thread), "Continue")
	if n := w.count(`SELECT chain_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2`, r.Thread, w.owner.String()); n != 0 {
		t.Fatalf("the owner's chain after the owner's run: %d", n)
	}
}

// TestWake_IdleCap (09 5.8.6): the idle flag of /metrics/autonomy_last counts each distinct input
// once; at run.autonomous_idle_max the chain ends with a note, the patch answers
// autonomy_cap_reached, and a wake input into the held run is refused; a follow-up of the person
// resets the counts.
func TestWake_IdleCap(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	patch := func(input string, idle bool) map[string]any {
		t.Helper()
		flag := "false"
		if idle {
			flag = "true"
		}
		body := `{"base_version":` + strconv.FormatInt(w.runVersion(r.ID), 10) + `,"ops":[{"op":"add","path":"/metrics/autonomy_last","value":{"input":"` +
			input + `","idle":` + flag + `}}]}`
		return w.wantOK(w.do("lotse", http.MethodPatch, "/api/v1/runs/"+r.ID, body))
	}
	if res := patch("turn-1", true); res["autonomy_cap_reached"] != nil {
		t.Fatalf("first idle input %v", res)
	}
	patch("turn-1", true)
	if n := w.count(`SELECT idle_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2`, r.Thread, w.owner.String()); n != 1 {
		t.Fatalf("one input counted %d times", n)
	}
	if res := patch("turn-2", true); res["autonomy_cap_reached"] != true {
		t.Fatalf("idle cap %v", res)
	}
	if notes := w.notes(r.Thread); len(notes) != 1 || !strings.Contains(notes[0], "autonomous_idle_max") {
		t.Fatalf("notes %v", notes)
	}
	if !slices.Contains(w.counts.caps, "autonomous_idle_max") {
		t.Fatalf("caps %v", w.counts.caps)
	}
	id := w.prWake(r.ID)
	w.clk.Advance(wakeCfg.MinDelay)
	if v := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/wakes/"+id+"/fire", `{"outcome":"fired","result_text":"green"}`)); v["state"] != "refused" ||
		v["state_reason"] != "autonomous_idle_max" {
		t.Fatalf("wake input into the held run %v", v)
	}
	// A follow-up of the owner resets the counts.
	w.post("owner", ids.MustParse(r.Thread), "Go on")
	if n := w.count(`SELECT idle_count + chain_count FROM run_autonomy WHERE thread_id = $1 AND person_id = $2 AND ended_reason IS NULL`, r.Thread,
		w.owner.String()); n != 0 {
		t.Fatalf("counts after the follow-up %d", n)
	}
}

// TestWake_LaunchCheck (R-AG-48, 09 18.19.6): a running run passes with the configured values; a
// run that is not running is refused with run_not_running and an audit entry; another agent gets
// 404.
func TestWake_LaunchCheck(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("alex")
	got := w.wantOK(w.do("lotse", http.MethodPost, "/api/v1/runs/"+r.ID+"/launch-check", ""))
	if got["tool_class"] != "chat" || got["billing_mode"] != "organization" {
		t.Fatalf("launch check %v", got)
	}
	w.want(w.do("texter", http.MethodPost, "/api/v1/runs/"+r.ID+"/launch-check", ""), http.StatusNotFound, "not_found")
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	w.wantReason(w.do("lotse", http.MethodPost, "/api/v1/runs/"+r.ID+"/launch-check", ""), http.StatusConflict, "conflict", "run_not_running")
	if n := w.audits("run.launch.check", "deny", "run_not_running"); n != 1 {
		t.Fatalf("%d audit entries", n)
	}
}

// TestWake_UsageLimitedReason (09 5.2): an adapter finish with usage_limited is stored as
// usage_limited, no longer as adapter_error.
func TestWake_UsageLimitedReason(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	r := w.runOfPerson("owner")
	w.wantOK(w.finish("lotse", r.ID, `"state":"failed","reason":"usage_limited","summary":"Usage limit of the account reached; resets at 18:00"`))
	if got := w.lifecycle(r.ID); got.State != "failed" || got.Reason != "usage_limited" {
		t.Fatalf("stored %+v", got)
	}
}

// TestWake_ChannelSwitch (07 2.14 item 1): allow_autonomous_runs needs manage, If-Match, is
// audited, appears in the channel and lets a run of a person who is no owner ask.
func TestWake_ChannelSwitch(t *testing.T) {
	w := newWakeWorld(t, wakeCfg)
	path := "/api/v1/channels/" + w.allgemein.String() + "/autonomous-runs"
	v := w.count(`SELECT version FROM channels WHERE id = $1`, w.allgemein.String())
	w.want(w.do("alex", http.MethodPut, path, `{"allow_autonomous_runs":true}`, ifMatch(v)...), http.StatusForbidden, "")
	w.want(w.do("owner", http.MethodPut, path, `{"allow_autonomous_runs":true}`, key()...), http.StatusPreconditionRequired, "")
	ch := w.setAutonomous(w.allgemein, true)
	if ch["allow_autonomous_runs"] != true || w.count(`SELECT count(*) FROM channels WHERE id = $1 AND allow_autonomous_runs`, w.allgemein.String()) != 1 {
		t.Fatalf("switch on %v", ch)
	}
	if n := w.audits("channel.autonomous_runs.set", "allow", ""); n != 1 {
		t.Fatalf("%d audit entries", n)
	}
	r := w.runOfPerson("alex")
	w.atWake(r.ID, "Alex may")
	w.setAutonomous(w.allgemein, false)
	w.wantReason(w.createWake("lotse", r.ID, `{"kind":"at","reason":"agent"}`), http.StatusConflict, "conflict", "autonomous_not_allowed")
}
