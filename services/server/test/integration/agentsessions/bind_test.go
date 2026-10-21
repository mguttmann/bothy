// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestAG17_StaleExpectedSession (AG-17, 09 9.4 point 3): a bind with an outdated
// expected_current_session_id gives 409 conflict with reason session_changed; rows, events and
// audit entries are unchanged.
func TestAG17_StaleExpectedSession(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	first := w.a6Bind("alex", th, "sess-ag17-1", "", false)
	w.a6Bind("alex", th, "sess-ag17-2", first["id"].(string), false)
	before := w.a6State()
	w.wantReason(w.bind("alex", th, a6BindBody("sess-ag17-3", first["id"].(string), false)), http.StatusConflict, "conflict", "session_changed")
	// Absent expected_current_session_id means "the thread has none": stale as well.
	w.wantReason(w.bind("alex", th, a6BindBody("sess-ag17-3", "", false)), http.StatusConflict, "conflict", "session_changed")
	w.a6Unchanged(before)
}

// TestAG17_TakeOverWithoutFlag (AG-17, 09 9.4 point 1): the reference is current in another
// thread of Support; a bind without take_over gives 409 conflict with reason
// session_bound_elsewhere; nothing changed.
func TestAG17_TakeOverWithoutFlag(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	elsewhere, th := w.newThread(w.eskalation), w.newThread(w.allgemein)
	w.a6Bind("alex", elsewhere, "sess-ag17-shared", "", false)
	own := w.a6Bind("alex", th, "sess-ag17-own", "", false)
	before := w.a6State()
	w.wantReason(w.bind("alex", th, a6BindBody("sess-ag17-shared", own["id"].(string), false)), http.StatusConflict, "conflict",
		"session_bound_elsewhere")
	w.a6Unchanged(before)
	if r, _ := w.current(elsewhere); r.Ref != "sess-ag17-shared" {
		t.Fatalf("the other thread lost its session: %+v", r)
	}
}

// TestBindTakeOver (09 9.4, design notes 8, 14 and 15): with take_over true both old rows end with
// rebind, the new row is current with started true and the computed cost baseline; events in the
// order of design note 15; one allow audit entry with the IDs of design note 15 and without the
// reference; the next trigger in this thread delivers bind, in the other thread new.
func TestBindTakeOver(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	other, th := w.newThread(w.eskalation), w.newThread(w.allgemein)
	taken := w.a6Bind("alex", other, "sess-take-1", "", false)
	takenID := taken["id"].(string)
	// The amounts M2-22 books; fixture of the most recent row of the reference (design note 14).
	w.sql(`UPDATE agent_sessions SET cost_baseline_micros = 500, cumulative_cost_micros = 1500, currency = 'USD' WHERE id = $1`, takenID)
	own := w.a6Bind("alex", th, "sess-take-own", "", false)
	ownID := own["id"].(string)
	auditsBefore := len(w.a6Allows("agent_session.bind"))
	seq := w.maxSeq()
	body := w.a6Bind("alex", th, "sess-take-1", ownID, true)
	newID := body["id"].(string)
	if body["is_current"] != true || body["started"] != true || body["external_session_ref"] != "sess-take-1" || version(body) != 1 {
		t.Fatalf("answer %v", body)
	}
	for _, id := range []struct {
		thread ids.ID
		row    string
	}{{other, takenID}, {th, ownID}} {
		if r := w.a6RowByID(id.thread, id.row); r.Current || r.EndReason != "rebind" {
			t.Fatalf("row %+v", r)
		}
	}
	n := w.a6RowByID(th, newID)
	if !n.Current || !n.Started || n.Baseline != 2000 || n.Currency != "USD" || n.Cumulative != 0 {
		t.Fatalf("new row %+v, want baseline 2000 USD", n)
	}
	if _, ok := w.current(other); ok {
		t.Fatal("the other thread kept a current row")
	}
	// Ended rows first in ascending id, then the new row; each in the scope of its thread.
	ended := []a6Change{{Thread: other.String(), Session: takenID, EndReason: "rebind"}, {Thread: th.String(), Session: ownID, EndReason: "rebind"}}
	slices.SortFunc(ended, func(x, y a6Change) int { return strings.Compare(x.Session, y.Session) })
	want := append(slices.Clone(ended), a6Change{Thread: th.String(), Session: newID, Current: true})
	if got := w.a6ChangesAfter(seq); !slices.Equal(got, want) {
		t.Fatalf("events\n%+v\nwant\n%+v", got, want)
	}
	allows := w.a6Allows("agent_session.bind")
	if len(allows) != auditsBefore+1 {
		t.Fatalf("%d allow entries, want %d", len(allows), auditsBefore+1)
	}
	a := allows[len(allows)-1]
	wantDetails := map[string]any{
		"session_id": newID, "ended_session_id": ownID, "taken_over_session_id": takenID, "taken_over_thread_id": other.String(),
		"take_over": true, "cause": "bind",
	}
	for k, v := range wantDetails {
		if a.Details[k] != v {
			t.Fatalf("audit %s = %v, want %v: %v", k, a.Details[k], v, a.Details)
		}
	}
	if a.ResourceID != th.String() {
		t.Fatalf("audit resource %s, want %s", a.ResourceID, th)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent_session.bind' AND details::text LIKE '%sess-take%'`); n != 0 {
		t.Fatalf("%d audit entries carry the reference", n)
	}
	next := w.trigger(th)
	w.a6WantMode(next, "bind", newID)
	elsewhere := w.trigger(other)
	w.a6WantMode(elsewhere, "new", "")
}

// TestBindTakeOverTargetRights (07 4.2, design note 5): a target thread the actor cannot read
// gives 404 with the same status, body and headers as an unknown thread ID; a target with only
// read gives 403 forbidden.
func TestBindTakeOverTargetRights(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	// Lotse operates Marketing/Kampagnen too, so one reference of Lotse can be current there and in
	// Support; Alex cannot read Marketing, Sam triggers in Marketing (G-2) and only reads
	// Support/Allgemein (G-4).
	w.grant(`"subject_kind":"agent","subject_id":"` + w.lotse.String() + `","target_kind":"channel","target_id":"` + w.kampagnen.String() +
		`","level":"write","reason":"Campaign agent writes"`)
	w.wantOK(w.setChannelOperator(w.kampagnen, `{"mode":"agent","agent_id":"`+w.lotse.String()+`","reason":"Campaign agent"}`))
	hidden := w.newThread(w.kampagnen)
	w.a6Bind("owner", hidden, "sess-hidden", "", false)
	support := w.newThread(w.allgemein)
	w.a6Bind("alex", support, "sess-readonly", "", false)
	before := w.a6State()

	body := a6BindBody("sess-hidden", "", true)
	a6SameNotFound(t, w.bind("alex", support, body), w.bind("alex", ids.New(), body))
	campaign := w.newThread(w.kampagnen)
	w.want(w.bind("sam", campaign, a6BindBody("sess-readonly", "", true)), http.StatusForbidden, "forbidden")
	w.a6Unchanged(before)
	if d := w.a6Denials("agent_session.bind", w.alex); len(d) != 1 {
		t.Fatalf("bind denials of Alex %+v", d)
	}
	if d := w.a6Denials("agent_session.bind", w.sam); len(d) != 1 || d[0].Reason != "level_insufficient" {
		t.Fatalf("bind denials of Sam %+v", d)
	}
	// Both paths fine: Sam binds a reference current nowhere in the campaign thread.
	w.a6Bind("sam", campaign, "sess-campaign", "", false)
}

// TestBindConcurrent (design notes 8 f and 16): two binds with the same
// expected_current_session_id give exactly one 201 and one 409 conflict (session_changed); one
// current row.
func TestBindConcurrent(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	cur := w.a6Bind("alex", th, "sess-race-0", "", false)["id"].(string)
	gate := make(chan struct{})
	var wg sync.WaitGroup
	answers := make([]*httpservertest.Response, 2)
	for i, ref := range []string{"sess-race-1", "sess-race-2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			answers[i] = w.h.Do(w.h.NewRequest(http.MethodPost, sessionPath(th, "bind"), a6BindBody(ref, cur, false),
				"Authorization", "Bearer alex", "Idempotency-Key", ids.New().String()))
		}()
	}
	close(gate)
	wg.Wait()
	created, conflicts := 0, 0
	for _, r := range answers {
		switch r.StatusCode {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			if p := r.Problem(t); p["code"] != "conflict" || p["reason"] != "session_changed" {
				t.Fatalf("conflict %s", r.Body)
			}
			conflicts++
		default:
			t.Fatalf("status %d: %s", r.StatusCode, r.Body)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("%d created, %d conflicts", created, conflicts)
	}
	if n := w.count(`SELECT count(*) FROM agent_sessions WHERE thread_id = $1 AND is_current`, th.String()); n != 1 {
		t.Fatalf("%d current rows", n)
	}
	if n := len(w.sessionRows(th)); n != 2 {
		t.Fatalf("%d rows, want the ended one and the winner", n)
	}
	w.a6OneCurrentPerRef()
}

// TestExternalSessionInOneThread (plan acceptance, 03 agent_sessions): after any sequence of binds,
// take-overs and reports no (agent_id, external_session_ref) is current twice; a report that changes
// the reference to one current in another thread gives 409 conflict with reason
// session_bound_elsewhere.
func TestExternalSessionInOneThread(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	t1, t2, t3 := w.newThread(w.allgemein), w.newThread(w.eskalation), w.newThread(w.allgemein)
	w.a6Bind("alex", t1, "sess-one-a", "", false)
	w.a6OneCurrentPerRef()
	w.a6Bind("alex", t2, "sess-one-b", "", false)
	w.a6OneCurrentPerRef()
	w.wantReason(w.bind("alex", t3, a6BindBody("sess-one-a", "", false)), http.StatusConflict, "conflict", "session_bound_elsewhere")
	c := w.a6Bind("alex", t3, "sess-one-a", "", true)
	w.a6OneCurrentPerRef()
	w.a6Bind("alex", t1, "sess-one-b", "", true)
	w.a6OneCurrentPerRef()
	if r, _ := w.current(t1); r.Ref != "sess-one-b" {
		t.Fatalf("t1 %+v", r)
	}
	if _, ok := w.current(t2); ok {
		t.Fatal("t2 kept its current row")
	}
	// A run in t3 on the bound row; its report may not move the reference onto one current in t1.
	run := w.trigger(t3)
	w.a6WantMode(run, "bind", c["id"].(string))
	w.wantOK(w.start("lotse", run, w.deliver(run)))
	if w.runSession(run) != c["id"] {
		t.Fatalf("run session %q, want %v", w.runSession(run), c["id"])
	}
	before := w.a6State()
	w.wantReason(w.report("lotse", t3, `{"external_session_ref":"sess-one-b"}`, append(match(version(c)), key()...)...), http.StatusConflict,
		"conflict", "session_bound_elsewhere")
	w.a6Unchanged(before)
	w.a6OneCurrentPerRef()
	moved := w.wantOK(w.report("lotse", t3, `{"external_session_ref":"sess-one-c"}`, append(match(version(c)), key()...)...))
	if moved["id"] != c["id"] || moved["external_session_ref"] != "sess-one-c" {
		t.Fatalf("report %v", moved)
	}
	w.a6OneCurrentPerRef()
	w.finishOK(run)
	// The freed reference binds again elsewhere without a take-over.
	w.a6Bind("alex", t2, "sess-one-a", "", false)
	w.a6OneCurrentPerRef()
}
