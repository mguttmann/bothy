// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"net/http"
	"testing"
)

// TestNewSession_ReferenceAtStart (runner path, 09 9.1): the first trigger delivers session_mode
// new without agent_session; startRun with a reference creates a current row with started false
// and sets runs.agent_session_id; a report with started true and If-Match "1" gives version 2;
// the next trigger delivers resume with that row.
func TestNewSession_ReferenceAtStart(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.trigger(th)
	p := w.invocation(run)
	if p["session_mode"] != "new" || sessionOf(p) != nil {
		t.Fatalf("first payload %v", p)
	}
	body := w.wantOK(w.start("lotse", run, w.deliver(run), `"external_session_ref":"sess-7f3a"`, `"model":"model-large"`))
	rows := w.sessionRows(th)
	if len(rows) != 1 || !rows[0].Current || rows[0].Started || rows[0].Ref != "sess-7f3a" || rows[0].Version != 1 {
		t.Fatalf("rows %+v", rows)
	}
	if w.runSession(run) != rows[0].ID || body["agent_session_id"] != rows[0].ID {
		t.Fatalf("run session %s, answer %v, want %s", w.runSession(run), body["agent_session_id"], rows[0].ID)
	}
	rep := w.want(w.report("lotse", th, `{"external_session_ref":"sess-7f3a","started":true}`, append(match(1), key()...)...),
		http.StatusOK, "")
	if version(rep) != 2 || rep["started"] != true {
		t.Fatalf("report %v", rep)
	}
	w.finishOK(run)
	next := w.trigger(th)
	p = w.invocation(next)
	if s := sessionOf(p); p["session_mode"] != "resume" || s == nil || s["id"] != rows[0].ID || version(s) != 2 {
		t.Fatalf("next payload %v", p)
	}
}
