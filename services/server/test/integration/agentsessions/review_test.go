// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestBindReferenceWithNUL: a reference with U+0000 never reaches the database before the
// permission and the validation are decided (R-AZ-01, design note 13): Robin, who cannot read the
// thread, gets the 404 of an unknown thread; Alex gets 400 validation_failed on
// /external_session_ref, never a 500.
func TestBindReferenceWithNUL(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	body := `{"external_session_ref":"sess-\u0000-x"}`
	invisible := w.bind("robin", th, body)
	unknown := w.bind("robin", ids.New(), body)
	if invisible.StatusCode != http.StatusNotFound {
		t.Fatalf("robin: %d %s, want the 404 of an unknown thread", invisible.StatusCode, invisible.Body)
	}
	a6SameNotFound(t, invisible, unknown)
	p := w.want(w.bind("alex", th, body), http.StatusBadRequest, "validation_failed")
	raw, err := json.Marshal(p)
	must(t, err)
	if !strings.Contains(string(raw), "/external_session_ref") {
		t.Fatalf("problem %v", p)
	}
}

// TestBindWhileRunQueued: a /resume while the operator's run is still queued leaves that run on
// the row it was invoked with (09 9.4, design note 8 h): the start attaches the invoked row, and
// the next run gets session_mode bind with the bound row.
func TestBindWhileRunQueued(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	first := w.a6StartedRun(th, `"external_session_ref":"sess-one"`)
	s, _ := w.current(th)
	w.wantOK(w.report("lotse", th, `{"external_session_ref":"sess-one","started":true}`, append(match(s.Version), key()...)...))
	w.finishOK(first)
	queued := w.trigger(th)
	if p := w.invocation(queued); p["session_mode"] != "resume" || sessionOf(p)["id"] != s.ID {
		t.Fatalf("queued payload %v", p)
	}
	bound := w.want(w.bind("alex", th, `{"external_session_ref":"sess-two","expected_current_session_id":"`+s.ID+`"}`), http.StatusCreated, "")
	w.wantOK(w.start("lotse", queued, w.deliver(queued)))
	if got := w.runSession(queued); got != s.ID {
		t.Fatalf("the queued run attached %q at its start, want the invoked row %s", got, s.ID)
	}
	w.finishOK(queued)
	next := w.trigger(th)
	if p := w.invocation(next); p["session_mode"] != "bind" || sessionOf(p)["id"] != bound["id"] {
		t.Fatalf("next payload %v, want bind with %v", p, bound["id"])
	}
}
