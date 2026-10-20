// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// revExpiry is the lifetime of the expiring grants and memberships of the expiry rows (a test
// value below terminal.idle_timeout, so no relay deadline fires on the way).
const revExpiry = 5 * time.Minute

// revOptions are the options of every world of these tests: auth.dpop_proof_max_age as wide as
// auth.clock_leeway, because the clients sign their DPoP proofs with the real clock while the fake
// clock of the server runs ahead after a window passed (the proof age, not the token leeway, bounds
// a request after such a jump).
func revOptions() worldOptions {
	return worldOptions{cfg: map[string]string{"auth.dpop_proof_max_age": "48h"}}
}

// revReason is the reason text of every owner operation of these tests.
const revReason = "terminal test"

// revBody is a body with the reason text and the members kv.
func revBody(kv ...any) map[string]any {
	b := map[string]any{"reason": revReason}
	for i := 0; i+1 < len(kv); i += 2 {
		b[kv[i].(string)] = kv[i+1]
	}
	return b
}

// revIfMatch sends r as Robin with the ETag of the host (GET first) and a step-up when asked; it
// fails unless the status is 200.
func (w *world) revIfMatch(method string, host ids.ID, path string, body any) {
	w.t.Helper()
	get := w.must(w.robin, http.MethodGet, "/hosts/"+host.String(), nil, http.StatusOK)
	w.syncClock()
	harness.MustDo(w.t, w.robin, harness.Request{Method: method, Path: path, Body: body,
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}}}, http.StatusOK)
}

// revSetHostState sets the state of host through setHostState.
func (w *world) revSetHostState(host ids.ID, state string) {
	w.t.Helper()
	w.revIfMatch(http.MethodPost, host, "/hosts/"+host.String()+"/state", revBody("state", state))
}

// revAddOwner makes c a second owner through addOwner.
func (w *world) revAddOwner(c *harness.Client) {
	w.t.Helper()
	w.must(w.robin, http.MethodPost, "/owners", revBody("user_id", c.PrincipalID), http.StatusCreated)
}

// revUntouched checks that l was not concerned by a cause: the row is still active, the host got no
// session.close of it, and the device stream still relays (one message echoed by the Fake-Host).
func (w *world) revUntouched(l live) {
	w.t.Helper()
	if got := w.state(l.id.String()); got != "active" {
		w.t.Fatalf("the unconcerned session %s is %s, want active", l.id, got)
	}
	for _, c := range l.host.Closes() {
		if ids.ID(c.SessionId) == l.id {
			w.t.Fatalf("the unconcerned session %s got session.close %s", l.id, c.CloseReason)
		}
	}
	ctx := w.ctx()
	msg := []byte("still-there-" + l.id.String()[:8])
	if err := l.dev.Send(ctx, msg); err != nil {
		w.t.Fatalf("the unconcerned session %s: send: %v", l.id, err)
	}
	if m, err := l.dev.Recv(ctx); err != nil || string(m) != string(msg) {
		w.t.Fatalf("the unconcerned session %s: echo %q %v", l.id, m, err)
	}
}

// revSameCommit fails unless the terminal.terminate entry of the session carries the correlation
// of the cause's audit entry action.
func (w *world) revSameCommit(id ids.ID, action string) {
	w.t.Helper()
	if !w.sameCommit(id, action) {
		w.t.Fatalf("the end of %s did not commit with %s (correlation of terminal.terminate: %s; causes: %s)", id, action,
			w.text(`SELECT coalesce(string_agg(coalesce(correlation_id::text, '-'), ','), '') FROM audit_events
				WHERE action = 'terminal.terminate' AND resource_id = $1`, id),
			w.text(`SELECT coalesce(string_agg(coalesce(correlation_id::text, '-'), ','), '') FROM audit_events WHERE action = $1`, action))
	}
}

// revSameEventCommit fails unless the terminal.terminate entry of the session carries the
// correlation of an event typ of resource (a cause without its own audit entry: the report of a
// host writes host.updated in the unit that ends the sessions).
func (w *world) revSameEventCommit(id ids.ID, typ string, resource ids.ID) {
	w.t.Helper()
	n := w.count(`SELECT count(*) FROM audit_events t JOIN event_log e ON e.correlation_id = t.correlation_id
		WHERE t.action = 'terminal.terminate' AND t.resource_id = $1 AND e.type = $2 AND e.resource_id = $3`, id, typ, resource)
	if n == 0 {
		w.t.Fatalf("the end of %s did not commit with %s of %s", id, typ, resource)
	}
}

// revNoReattach checks that attachTerminalSession for the ended session l, as c with or without a
// fresh step-up, answers a problem with a code and issues no session_token, and that the row stays
// closed with reason.
func (w *world) revNoReattach(l live, c *harness.Client, stepUp bool, reason domain.CloseReason) {
	w.t.Helper()
	r := w.attach(c, l.id.String(), stepUp)
	var p struct {
		Code         string `json:"code"`
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(r.Body, &p)
	if r.Status < 400 || r.Status >= 500 || p.Code == "" || p.SessionToken != "" || strings.Contains(string(r.Body), "session_token") {
		w.t.Fatalf("attach of the ended session (step-up %v): %d with code %q and a token %v", stepUp, r.Status, p.Code, p.SessionToken != "")
	}
	if got := w.state(l.id.String()); got != "closed:"+string(reason) {
		w.t.Fatalf("after the attach the session is %s, want closed:%s", got, reason)
	}
}

// revValidateSchema validates raw against the schema name of the contract; it returns the problems.
func revValidateSchema(name string, raw []byte) ([]string, error) {
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		return nil, err
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, err
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		return []string{"no schema " + name}, nil
	}
	var out []string
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaBytes(proxy.Schema(), raw); !valid {
		for _, pr := range problems {
			out = append(out, pr.Message+" "+pr.Reason)
		}
	}
	return out, nil
}

// revHasNull reports whether the decoded JSON value v holds a null anywhere.
func revHasNull(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, e := range x {
			if revHasNull(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if revHasNull(e) {
				return true
			}
		}
	}
	return false
}

// revAssertEnded checks the end of the row c for l: assertEnded, or for a cause that changes the
// state of the host the same checks with the close code c.control on the host's control
// connection in place of session.close (a session.close that still arrives must carry the reason).
func (w *world) revAssertEnded(l live, c revCase) {
	w.t.Helper()
	if c.control == 0 {
		w.assertEnded(l, c.reason)
		return
	}
	w.waitState(l.id.String(), "closed:"+string(c.reason))
	w.closeCode("device stream", l.dev.CloseCode, 4100, string(c.reason))
	w.closeCode("data connection", l.data.CloseCode, 4100, string(c.reason))
	if code, err := l.host.ControlClose(w.ctx()); err != nil || code != c.control {
		w.t.Fatalf("control connection: close %d %v, want %d", code, err, c.control)
	}
	for _, cl := range l.host.Closes() {
		if ids.ID(cl.SessionId) == l.id && string(cl.CloseReason) != string(c.reason) {
			w.t.Fatalf("session.close with %s, want %s", cl.CloseReason, c.reason)
		}
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, l.id); n != 1 {
		w.t.Fatalf("%d entries terminal.terminate, want 1", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1
		AND payload->>'state' = 'closed'`, l.id); n != 1 {
		w.t.Fatalf("%d events closed, want 1", n)
	}
	w.eventually("bothy.terminal.sessions.closed "+string(c.reason), func() bool { return w.metrics.closedOf(c.reason) > 0 })
}

// revSameTx fails unless the terminal.terminate entry of the session was written in the
// transaction of an audit entry action (a job commits without a request, so no correlation links
// them; the inserting transaction xmin of both rows does).
func (w *world) revSameTx(id ids.ID, action string) {
	w.t.Helper()
	n := w.count(`SELECT count(*) FROM audit_events t JOIN audit_events c ON c.xmin::text = t.xmin::text
		WHERE t.action = 'terminal.terminate' AND t.resource_id = $1 AND c.action = $2`, id, action)
	if n == 0 {
		w.t.Fatalf("the end of %s was not written in the transaction of %s", id, action)
	}
}
