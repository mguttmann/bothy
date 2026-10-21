// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// resumeFailed is the system_code of the fail-open line (03 messages.system_code, R-DM-42).
const resumeFailed = "agent_session.resume_failed"

// b6ImportBound triggers Lotse for Alex in thread, which holds an import row of Lotse with ref,
// and checks the import binding of 09 9.5 and 22 section 9: a current row with the same
// reference, started true and the copied baseline, one allow audit entry (cause import, actor
// Alex), one agent_session.changed, the payload with session_mode bind and that row. It returns
// the run and the bound row.
func (w *world) b6ImportBound(thread ids.ID, ref string, baseline int64) (string, sessionRow) {
	w.t.Helper()
	imported := w.b6Import(thread, w.lotse, ref, baseline)
	run := w.trigger(thread)
	bound, ok := w.current(thread)
	if !ok || bound.ID == imported || bound.Ref != ref || !bound.Started || bound.Baseline != baseline || bound.Agent != w.lotse.String() {
		w.t.Fatalf("bound row %+v (import %s)", bound, imported)
	}
	if n := w.b6Count(thread); n != 2 {
		w.t.Fatalf("%d rows after the import binding", n)
	}
	audits := w.b6BindAudits(thread)
	if len(audits) != 1 || audits[0].Actor != w.alex.String() || audits[0].Details["cause"] != "import" ||
		audits[0].Details["session_id"] != bound.ID || audits[0].Details["take_over"] != false {
		w.t.Fatalf("audit %+v", audits)
	}
	if ev := w.sessionEvents(thread); len(ev) != 1 || ev[0]["agent_session"].(map[string]any)["id"] != bound.ID {
		w.t.Fatalf("events %v", ev)
	}
	p := w.invocation(run)
	if s := sessionOf(p); p["session_mode"] != "bind" || s == nil || s["id"] != bound.ID {
		w.t.Fatalf("payload %v", p)
	}
	return run, bound
}

// TestAG18_ImportBindingSessionMissing (AG-18; 09 9.5 point 4, 22 section 9 step 5, M2-12 design
// notes 9 and 12): the import row binds at the first trigger; a report with bind_failed
// session_missing ends the bound row with rebind, makes the new row current, sets
// runs.agent_session_id to it and writes exactly one system line (author Lotse, origin system, no
// run); the run stays running and finishes; a replay writes no second line; the next trigger
// delivers resume with the new row.
func TestAG18_ImportBindingSessionMissing(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run, bound := w.b6ImportBound(th, "imported-7", 1500)
	w.wantOK(w.start("lotse", run, w.deliver(run), `"external_session_ref":"imported-7"`))
	if w.runSession(run) != bound.ID {
		t.Fatalf("runs.agent_session_id %q, want %s", w.runSession(run), bound.ID)
	}
	runs := len(w.runsWhere(`thread_id = $1`, th.String()))
	k := ids.New().String()
	body := `{"external_session_ref":"fresh-8","started":true,"bind_failed":"session_missing"}`
	first := w.report("lotse", th, body, append(match(bound.Version), "Idempotency-Key", k)...)
	n := w.want(first, http.StatusOK, "")
	b6CheckFailOpen(w, th, run, bound, n)
	if got := len(w.runsWhere(`thread_id = $1`, th.String())); got != runs {
		t.Fatalf("the system line triggered: %d runs, want %d", got, runs)
	}
	if st := w.b6Run(run).State; st != "running" {
		t.Fatalf("run state %s", st)
	}
	replay := w.report("lotse", th, body, append(match(bound.Version), "Idempotency-Key", k)...)
	if replay.StatusCode != http.StatusOK || replay.Header.Get("Idempotent-Replay") != "true" || !reflect.DeepEqual(w.b6Decode(replay), n) {
		t.Fatalf("replay %d %q: %s", replay.StatusCode, replay.Header.Get("Idempotent-Replay"), replay.Body)
	}
	// A duplicate under a new key fails the If-Match compare-and-swap.
	w.want(w.report("lotse", th, body, append(match(bound.Version), key()...)...), http.StatusPreconditionFailed, "version_conflict")
	if lines := w.b6SystemLines(th); len(lines) != 1 {
		t.Fatalf("%d system lines after the replay", len(lines))
	}
	w.finishOK(run)
	next := w.trigger(th)
	p := w.invocation(next)
	if s := sessionOf(p); p["session_mode"] != "resume" || s == nil || s["id"] != n["id"] {
		t.Fatalf("next payload %v", p)
	}
}

// b6CheckFailOpen checks the state after an accepted bind_failed report whose answer is n: the
// bound row ended with rebind, n current with the reported reference, runs.agent_session_id n,
// events bound then n, exactly one system line agent_session.resume_failed of Lotse.
func b6CheckFailOpen(w *world, th ids.ID, run string, bound sessionRow, n map[string]any) {
	w.t.Helper()
	rows := w.sessionRows(th)
	var old, fresh sessionRow
	for _, r := range rows {
		switch r.ID {
		case bound.ID:
			old = r
		case n["id"]:
			fresh = r
		}
	}
	if old.Current || old.EndReason != "rebind" || !fresh.Current || fresh.Ref != "fresh-8" || !fresh.Started || fresh.Baseline != 0 {
		w.t.Fatalf("rows %+v", rows)
	}
	if w.runSession(run) != fresh.ID {
		w.t.Fatalf("runs.agent_session_id %q, want %s", w.runSession(run), fresh.ID)
	}
	ev := w.sessionEvents(th)
	if len(ev) < 2 || ev[len(ev)-2]["agent_session"].(map[string]any)["id"] != bound.ID || ev[len(ev)-1]["agent_session"].(map[string]any)["id"] != fresh.ID {
		w.t.Fatalf("events %v", ev)
	}
	lines := w.b6SystemLines(th)
	if len(lines) != 1 {
		w.t.Fatalf("%d system lines, want 1: %+v", len(lines), lines)
	}
	l := lines[0]
	if l.Code != resumeFailed || l.Author != w.lotse.String() || l.Origin != "system" || l.Content != "" || !l.Silent || l.TriggerState != "" {
		w.t.Fatalf("system line %+v", l)
	}
	if _, triggered := w.runOf(l.ID); triggered {
		w.t.Fatal("the system line opened a run")
	}
}

// TestFailOpenLineKeepsThreadClosed (R-DM-42; 03 messages, 01 6.1): as AG-18, but Alex closes the
// thread while the run is running; the report with bind_failed writes exactly one line
// agent_session.resume_failed of Lotse, the thread stays closed, and neither thread.reopened nor
// a system line thread.reopened is written.
func TestFailOpenLineKeepsThreadClosed(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run, bound := w.b6ImportBound(th, "imported-9", 0)
	w.wantOK(w.start("lotse", run, w.deliver(run), `"external_session_ref":"imported-9"`))
	w.b6Close("alex", th)
	if st := w.b6ThreadState(th); st != "closed" {
		t.Fatalf("thread state %s after closeThread", st)
	}
	after := w.maxSeq()
	n := w.want(w.report("lotse", th, `{"external_session_ref":"fresh-8","started":true,"bind_failed":"session_missing"}`,
		append(match(bound.Version), key()...)...), http.StatusOK, "")
	b6CheckFailOpen(w, th, run, bound, n)
	if st := w.b6ThreadState(th); st != "closed" {
		t.Fatalf("the fail-open line reopened the thread: %s", st)
	}
	if slices.Contains(types(w.eventsAfter(after)), "thread.reopened") {
		t.Fatalf("events %v", types(w.eventsAfter(after)))
	}
	if c := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND system_code = 'thread.reopened'`, th.String()); c != 0 {
		t.Fatalf("%d lines thread.reopened", c)
	}
}

// TestImportBindingOnce (22 section 16 "Binding once", 22 section 9 step 3; M2-12 design note 9):
// two concurrent first triggers in one thread bind once; two threads with import rows of the same
// reference and concurrent first triggers bind once, the other run gets new; after a reset the
// import row never binds again; an import row of an agent that no longer operates the thread never
// binds; an import reference current in another thread does not bind and the run gets new.
func TestImportBindingOnce(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)

	// Two concurrent first triggers in the same thread, through ForInvocation in two units of
	// work behind a barrier (one thread holds one active run, so two acceptances would not both
	// reach the session decision through messages).
	same := w.newThread(w.allgemein)
	w.b6Import(same, w.lotse, "shared-once", 0)
	var mu sync.Mutex
	var got []string
	gate := make(chan struct{})
	var wg sync.WaitGroup
	errsOf := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			errsOf[i] = w.inUnit(func(ctx context.Context) error {
				s, err := w.sessions.ForInvocation(ctx, agents.InvocationSessionQuery{ThreadID: same, AgentID: w.lotse, TriggeredBy: w.alex})
				if err == nil && s.Row != nil {
					mu.Lock()
					got = append(got, s.Row.ID.String())
					mu.Unlock()
				}
				return err
			})
		}()
	}
	close(gate)
	wg.Wait()
	for _, err := range errsOf {
		must(t, err)
	}
	cur, ok := w.current(same)
	if !ok || len(got) != 2 || got[0] != cur.ID || got[1] != cur.ID {
		t.Fatalf("rows returned %v, current %+v", got, cur)
	}
	if n := w.b6Count(same); n != 2 || len(w.b6BindAudits(same)) != 1 || len(w.sessionEvents(same)) != 1 {
		t.Fatalf("%d rows, %d audits, %d events", n, len(w.b6BindAudits(same)), len(w.sessionEvents(same)))
	}

	// Two threads with import rows of the same reference, concurrent first triggers over HTTP.
	a, b := w.newThread(w.allgemein), w.newThread(w.allgemein)
	w.b6Import(a, w.lotse, "shared-two", 0)
	w.b6Import(b, w.lotse, "shared-two", 0)
	body := func() string { return `{"client_msg_id":"` + ids.New().String() + `","content_md":"Please continue"}` }
	rs := b6Concurrent(
		func() *httpservertest.Response {
			return w.do("alex", http.MethodPost, "/api/v1/threads/"+a.String()+"/messages", body(), key()...)
		},
		func() *httpservertest.Response {
			return w.do("alex", http.MethodPost, "/api/v1/threads/"+b.String()+"/messages", body(), key()...)
		},
	)
	modes := map[string]int{}
	for i, th := range []ids.ID{a, b} {
		if rs[i].StatusCode != http.StatusCreated {
			t.Fatalf("post %d: %d %s", i, rs[i].StatusCode, rs[i].Body)
		}
		r, ok := w.runOf(w.b6Decode(rs[i])["id"].(string))
		if !ok {
			t.Fatalf("post %d opened no run", i)
		}
		p := w.invocation(r.ID)
		mode, _ := p["session_mode"].(string)
		modes[mode]++
		cur, bound := w.current(th)
		switch mode {
		case "bind":
			if !bound || cur.Ref != "shared-two" || sessionOf(p)["id"] != cur.ID || len(w.b6BindAudits(th)) != 1 {
				t.Fatalf("bind in thread %d: %+v %v", i, cur, p)
			}
		case "new":
			if bound || sessionOf(p) != nil || len(w.b6BindAudits(th)) != 0 || w.b6Count(th) != 1 {
				t.Fatalf("new in thread %d: %+v %v", i, cur, p)
			}
		}
	}
	if modes["bind"] != 1 || modes["new"] != 1 {
		t.Fatalf("modes %v, want one bind and one new", modes)
	}
	if c := w.count(`SELECT count(*) FROM agent_sessions WHERE agent_id = $1 AND external_session_ref = 'shared-two' AND is_current`, w.lotse.String()); c != 1 {
		t.Fatalf("shared-two current %d times", c)
	}

	// After a reset the import row never binds again.
	re := w.newThread(w.allgemein)
	run, _ := w.b6ImportBound(re, "reset-once", 0)
	w.wantOK(w.start("lotse", run, w.deliver(run)))
	w.finishOK(run)
	w.want(w.reset("alex", re), http.StatusCreated, "")
	next := w.trigger(re)
	if p := w.invocation(next); p["session_mode"] != "new" || sessionOf(p) != nil {
		t.Fatalf("after the reset %v", p)
	}
	if n := w.b6Count(re); n != 2 || len(w.b6BindAudits(re)) != 1 {
		t.Fatalf("after the reset %d rows, %d audits", n, len(w.b6BindAudits(re)))
	}

	// An import row of an agent that does not operate the thread.
	foreign := w.newThread(w.allgemein)
	w.b6Import(foreign, w.texter, "texter-old", 0)
	fr := w.trigger(foreign)
	if p := w.invocation(fr); p["session_mode"] != "new" || sessionOf(p) != nil {
		t.Fatalf("foreign import %v", p)
	}
	if n := w.b6Count(foreign); n != 1 || len(w.b6BindAudits(foreign)) != 0 {
		t.Fatalf("foreign import: %d rows", n)
	}

	// An import reference current in another thread fails open.
	holder := w.newThread(w.allgemein)
	w.want(w.bind("alex", holder, `{"external_session_ref":"held-ref"}`), http.StatusCreated, "")
	held := w.newThread(w.allgemein)
	w.b6Import(held, w.lotse, "held-ref", 0)
	hr := w.trigger(held)
	if p := w.invocation(hr); p["session_mode"] != "new" || sessionOf(p) != nil {
		t.Fatalf("held import %v", p)
	}
	if n := w.b6Count(held); n != 1 || len(w.b6BindAudits(held)) != 0 {
		t.Fatalf("held import: %d rows", n)
	}
	if cur, ok := w.current(holder); !ok || cur.Ref != "held-ref" {
		t.Fatalf("holder lost its row: %+v", cur)
	}
}

// TestAgentChanged (03 end_reason agent_changed, R-AG-02; M2-12 design note 9 b): in a main thread
// with a session of Lotse, the owner sets Texter as operator of the channel (I1: Texter writes
// there); the next trigger ends Lotse's row with agent_changed, appends agent_session.changed and
// delivers new to Texter.
func TestAgentChanged(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	// Runs live in the main thread without auto_thread (09 3.3).
	w.behavior(w.allgemein, `"auto_thread":false`)
	main := w.mainThread(w.allgemein)
	run := w.b6Post("alex", main, "@lotse the export fails")
	if run == "" || w.b6Run(run).Thread != main.String() {
		t.Fatalf("no run of Lotse in the main thread: %q", run)
	}
	w.wantOK(w.start("lotse", run, w.deliver(run), `"external_session_ref":"main-lotse"`))
	lotseRow, ok := w.current(main)
	if !ok || lotseRow.Agent != w.lotse.String() {
		t.Fatalf("no session of Lotse: %+v", w.sessionRows(main))
	}
	w.finishOK(run)
	w.grant(`"subject_kind":"agent","subject_id":"` + w.texter.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"write","reason":"Texter operates Allgemein"`)
	w.wantOK(w.setChannelOperator(w.allgemein, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Texter takes over"}`))
	events := len(w.sessionEvents(main))
	next := w.b6Post("alex", main, "@texter please take a look")
	if next == "" || w.b6Run(next).Agent != w.texter.String() {
		t.Fatalf("no run of Texter: %q", next)
	}
	rows := w.sessionRows(main)
	if len(rows) != 1 || rows[0].ID != lotseRow.ID || rows[0].Current || rows[0].EndReason != "agent_changed" {
		t.Fatalf("rows %+v", rows)
	}
	ev := w.sessionEvents(main)
	if len(ev) != events+1 {
		t.Fatalf("%d agent_session.changed, want %d", len(ev), events+1)
	}
	last := ev[len(ev)-1]["agent_session"].(map[string]any)
	if last["id"] != lotseRow.ID || last["end_reason"] != "agent_changed" || last["is_current"] != false {
		t.Fatalf("event %v", last)
	}
	if p := w.invocation(next); p["session_mode"] != "new" || sessionOf(p) != nil {
		t.Fatalf("payload of Texter %v", p)
	}
}

// TestGuestRunHasNoSession (09 3.6, R-AG-04): a guest run's payload has neither session_mode nor
// agent_session; its startRun with a reference creates no row; its report gives 404.
func TestGuestRunHasNoSession(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.eskalation)
	run := w.b6Post("owner", th, "@texter please check the copy")
	if run == "" || w.b6Run(run).Agent != w.texter.String() {
		t.Fatalf("no guest run of Texter: %q", run)
	}
	p := w.invocation(run)
	if _, has := p["session_mode"]; has {
		t.Fatalf("guest payload has session_mode: %v", p)
	}
	if _, has := p["agent_session"]; has {
		t.Fatalf("guest payload has agent_session: %v", p)
	}
	w.wantOK(w.start("texter", run, w.deliver(run), `"external_session_ref":"guest-1"`, `"model":"model-large"`))
	if n := w.b6Count(th); n != 0 || w.runSession(run) != "" {
		t.Fatalf("the guest start stored %d rows, run session %q", n, w.runSession(run))
	}
	w.want(w.report("texter", th, `{"external_session_ref":"guest-1","started":true}`, key()...), http.StatusNotFound, "not_found")
	if n := w.b6Count(th); n != 0 || len(w.sessionEvents(th)) != 0 {
		t.Fatalf("the guest report stored %d rows", n)
	}
}
