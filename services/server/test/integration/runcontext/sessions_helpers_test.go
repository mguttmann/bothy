// SPDX-License-Identifier: Apache-2.0

package runcontext_test

import (
	"net/http"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// invocation returns the payload of the agent.invocation event of run.
func (w *world) invocation(run string) map[string]any {
	w.t.Helper()
	list := w.eventsOf("agent.invocation", run)
	if len(list) != 1 {
		w.t.Fatalf("run %s has %d agent.invocation events, want 1", run, len(list))
	}
	return list[0]
}

// sessionPath is the path of the session operations of thread.
func sessionPath(thread ids.ID, op string) string {
	p := "/api/v1/threads/" + thread.String() + "/agent-sessions"
	if op != "" {
		p += "/" + op
	}
	return p
}

// reset sends resetAgentSession as name with a fresh Idempotency-Key.
func (w *world) reset(name string, thread ids.ID) *httpservertest.Response {
	return w.do(name, http.MethodPost, sessionPath(thread, "reset"), "", key()...)
}

// bind sends bindAgentSession as name with the JSON body and a fresh Idempotency-Key.
func (w *world) bind(name string, thread ids.ID, body string) *httpservertest.Response {
	return w.do(name, http.MethodPost, sessionPath(thread, "bind"), body, key()...)
}

// report sends reportAgentSession as the agent handle with the JSON body and the headers.
func (w *world) report(handle string, thread ids.ID, body string, headers ...string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, sessionPath(thread, "report"), body, headers...)
}

// match returns the If-Match header of version.
func match(version int64) []string {
	return []string{"If-Match", `"` + strconv.FormatInt(version, 10) + `"`}
}

// finishOK finishes run of Lotse as completed.
func (w *world) finishOK(run string) {
	w.t.Helper()
	w.wantOK(w.finish("lotse", run, `"state":"completed"`))
}
