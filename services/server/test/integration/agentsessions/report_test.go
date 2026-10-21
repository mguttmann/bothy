// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestReportConcurrent (plan acceptance "two concurrent reports yield exactly one success"; M2-12
// design note 11 e): two reports with the same If-Match give one 200 and one 412 version_conflict
// with current; two first reports without If-Match give one 200 and one 412 version_conflict
// whose current is the winner's row; exactly one row and one event per success.
func TestReportConcurrent(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)

	// Same If-Match on an existing row.
	th := w.newThread(w.allgemein)
	w.b6Started(th, `"external_session_ref":"sess-a"`)
	row, _ := w.current(th)
	events := len(w.sessionEvents(th))
	rs := b6Concurrent(
		func() *httpservertest.Response {
			return w.report("lotse", th, `{"external_session_ref":"sess-a","started":true}`, append(match(row.Version), key()...)...)
		},
		func() *httpservertest.Response {
			return w.report("lotse", th, `{"external_session_ref":"sess-a","started":true,"model":"model-large"}`,
				append(match(row.Version), key()...)...)
		},
	)
	ok, lost := b6Split(t, rs)
	win := w.b6Decode(ok)
	if version(win) != row.Version+1 {
		t.Fatalf("winner %v", win)
	}
	p := w.want(lost, http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur == nil || cur["id"] != row.ID || version(cur) != row.Version+1 {
		t.Fatalf("412 current %v, want the row %s at version %d", p["current"], row.ID, row.Version+1)
	}
	if got := len(w.sessionEvents(th)); got != events+1 {
		t.Fatalf("%d agent_session.changed, want %d", got, events+1)
	}
	if n := w.b6Count(th); n != 1 {
		t.Fatalf("%d rows", n)
	}

	// Two first reports of a run without a row.
	th2 := w.newThread(w.allgemein)
	run := w.b6Started(th2)
	if n := w.b6Count(th2); n != 0 {
		t.Fatalf("startRun without reference created %d rows", n)
	}
	rs = b6Concurrent(
		func() *httpservertest.Response {
			return w.report("lotse", th2, `{"external_session_ref":"first-a","started":true}`, key()...)
		},
		func() *httpservertest.Response {
			return w.report("lotse", th2, `{"external_session_ref":"first-b","started":true}`, key()...)
		},
	)
	ok, lost = b6Split(t, rs)
	win = w.b6Decode(ok)
	p = w.want(lost, http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur == nil || cur["id"] != win["id"] {
		t.Fatalf("412 current %v, want the winner %v", p["current"], win["id"])
	}
	rows := w.sessionRows(th2)
	if len(rows) != 1 || rows[0].ID != win["id"] || !rows[0].Current {
		t.Fatalf("rows %+v, winner %v", rows, win["id"])
	}
	if got := len(w.sessionEvents(th2)); got != 1 {
		t.Fatalf("%d agent_session.changed, want 1", got)
	}
	if w.runSession(run) != rows[0].ID {
		t.Fatalf("runs.agent_session_id %q, want %s", w.runSession(run), rows[0].ID)
	}
}

// b6Split checks that exactly one of two responses is 200 and the other 412 and returns them in
// that order.
func b6Split(t *testing.T, rs []*httpservertest.Response) (ok, lost *httpservertest.Response) {
	t.Helper()
	st := b6Statuses(rs)
	slices.Sort(st)
	if !slices.Equal(st, []int{http.StatusOK, http.StatusPreconditionFailed}) {
		t.Fatalf("statuses %v, want one 200 and one 412: %s | %s", b6Statuses(rs), rs[0].Body, rs[1].Body)
	}
	if rs[0].StatusCode == http.StatusOK {
		return rs[0], rs[1]
	}
	return rs[1], rs[0]
}

// TestReportRules (M2-12 design notes 11 and 13): no active run, another agent's run, a finished
// run and a guest run each give 404; a wrong If-Match gives 412; If-Match on a run without row
// gives 412; started false keeps started; an unchanged report gives 200 without version change
// and without event; an invalid reference gives 400 validation_failed at /external_session_ref;
// an unknown member gives 400 validation_failed; a replay with the same Idempotency-Key returns
// the stored answer with Idempotent-Replay true, a different body with it gives 422
// idempotency_key_reuse.
func TestReportRules(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	body := `{"external_session_ref":"sess-r","started":true}`

	// No active run: the not_found of an unknown thread.
	idle := w.newThread(w.allgemein)
	w.want(w.report("lotse", idle, body, key()...), http.StatusNotFound, "not_found")

	// If-Match on a run without row.
	th := w.newThread(w.allgemein)
	run := w.b6Started(th)
	p := w.want(w.report("lotse", th, body, append(match(1), key()...)...), http.StatusPreconditionFailed, "version_conflict")
	if _, has := p["current"]; has {
		t.Fatalf("412 without a row has current: %v", p)
	}
	// Invalid reference and unknown member: 400 validation_failed, nothing stored.
	p = w.want(w.report("lotse", th, `{"external_session_ref":"   ","started":true}`, key()...), http.StatusBadRequest, "validation_failed")
	w.b6Pointer(p, "/external_session_ref")
	p = w.want(w.report("lotse", th, `{"external_session_ref":"`+strings.Repeat("x", sessionRefMaxBytes+1)+`"}`, key()...),
		http.StatusBadRequest, "validation_failed")
	w.b6Pointer(p, "/external_session_ref")
	w.want(w.report("lotse", th, `{"external_session_ref":"sess-r","color":"blue"}`, key()...), http.StatusBadRequest, "validation_failed")
	if n := w.b6Count(th); n != 0 {
		t.Fatalf("a refused report stored %d rows", n)
	}

	// The first report creates the row; a replay returns the stored answer.
	k := ids.New().String()
	first := w.report("lotse", th, body, "Idempotency-Key", k)
	created := w.want(first, http.StatusOK, "")
	replay := w.report("lotse", th, body, "Idempotency-Key", k)
	if replay.StatusCode != http.StatusOK || replay.Header.Get("Idempotent-Replay") != "true" || !reflect.DeepEqual(w.b6Decode(replay), created) {
		t.Fatalf("replay %d %q: %s", replay.StatusCode, replay.Header.Get("Idempotent-Replay"), replay.Body)
	}
	w.want(w.report("lotse", th, `{"external_session_ref":"sess-other"}`, "Idempotency-Key", k), http.StatusUnprocessableEntity,
		"idempotency_key_reuse")
	if n := w.b6Count(th); n != 1 || len(w.sessionEvents(th)) != 1 {
		t.Fatalf("after the replay %d rows, %d events", n, len(w.sessionEvents(th)))
	}
	if w.runSession(run) != created["id"] {
		t.Fatalf("runs.agent_session_id %q, want %v", w.runSession(run), created["id"])
	}

	// A wrong If-Match and a missing one give 412 with current.
	v := version(created)
	for _, h := range [][]string{append(match(v+5), key()...), key()} {
		p = w.want(w.report("lotse", th, body, h...), http.StatusPreconditionFailed, "version_conflict")
		if cur, _ := p["current"].(map[string]any); cur == nil || cur["id"] != created["id"] || version(cur) != v {
			t.Fatalf("412 current %v", p["current"])
		}
	}

	// started false keeps started; an unchanged report changes nothing.
	events := len(w.sessionEvents(th))
	for _, b := range []string{`{"external_session_ref":"sess-r","started":false}`, body, `{"external_session_ref":"sess-r"}`} {
		got := w.want(w.report("lotse", th, b, append(match(v), key()...)...), http.StatusOK, "")
		if version(got) != v || got["started"] != true {
			t.Fatalf("report %s gave %v", b, got)
		}
	}
	if len(w.sessionEvents(th)) != events {
		t.Fatalf("an unchanged report appended an event")
	}
	if r, _ := w.current(th); r.Version != v || !r.Started {
		t.Fatalf("row %+v", r)
	}

	// Another agent's run: Texter can read Support/Eskalation, the run there is Lotse's.
	esk := w.newThread(w.eskalation)
	_ = w.b6Started(esk)
	w.want(w.report("texter", esk, body, key()...), http.StatusNotFound, "not_found")

	// A guest run: Texter, mentioned by the owner in Support/Eskalation (agent.read lets Alex mention
	// only operators of channels Alex reads, D-M2-05-22), has no session.
	gth := w.newThread(w.eskalation)
	guest := w.b6Post("owner", gth, "@texter please check the copy")
	if guest == "" || w.b6Run(guest).Agent != w.texter.String() {
		t.Fatalf("no guest run of Texter: %q", guest)
	}
	w.wantOK(w.start("texter", guest, w.deliver(guest)))
	w.want(w.report("texter", gth, body, key()...), http.StatusNotFound, "not_found")

	// A finished run.
	w.finishOK(run)
	w.want(w.report("lotse", th, body, append(match(v), key()...)...), http.StatusNotFound, "not_found")
}
