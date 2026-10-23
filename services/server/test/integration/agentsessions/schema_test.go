// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	gosync "github.com/OWNER/bothy/services/server/internal/sync"
)

// TestSessionEventsAndPayloadSchemas (M2-12 design notes 9 and 15): after starts, reports,
// resets, binds, a take-over, an import binding with bind_failed and a guest run, every stored
// agent_session.changed validates against AgentSessionChangedEvent (resource thread without
// version, receive action thread.read, no null), and every agent.invocation, enveloped as the
// gateway delivers it with its token (this stands in for pollAgentEvents), validates against the
// changed AgentInvocationPayload. A payload with an unknown session_mode fails, so the check is
// not vacuous.
func TestSessionEventsAndPayloadSchemas(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	tokens := map[string]string{}
	startRun := func(run string, extra ...string) {
		tok := w.deliver(run)
		tokens[run] = tok
		w.wantOK(w.start(w.b6Handle(run), run, tok, extra...))
	}

	// Start with a reference, report, resume, reset, bind.
	one := w.newThread(w.allgemein)
	r1 := w.trigger(one)
	startRun(r1, `"external_session_ref":"schema-1"`, `"model":"model-large"`)
	w.want(w.report("lotse", one, `{"external_session_ref":"schema-1","started":true}`, append(match(1), key()...)...), http.StatusOK, "")
	w.finishOK(r1)
	r2 := w.trigger(one)
	startRun(r2)
	w.finishOK(r2)
	w.want(w.reset("alex", one), http.StatusCreated, "")
	bound := w.want(w.bind("alex", one, `{"external_session_ref":"schema-2"}`), http.StatusCreated, "")

	// A take-over into a second thread.
	two := w.newThread(w.allgemein)
	w.want(w.bind("alex", two, `{"external_session_ref":"schema-2","take_over":true}`), http.StatusCreated, "")
	_ = bound

	// An import binding, then bind_failed.
	three := w.newThread(w.allgemein)
	w.b6Import(three, w.lotse, "schema-3", 700)
	r3 := w.trigger(three)
	startRun(r3, `"external_session_ref":"schema-3"`)
	cur, _ := w.current(three)
	w.want(w.report("lotse", three, `{"external_session_ref":"schema-4","started":true,"bind_failed":"session_missing"}`,
		append(match(cur.Version), key()...)...), http.StatusOK, "")

	// A first report, and a guest run.
	four := w.newThread(w.allgemein)
	r4 := w.trigger(four)
	startRun(r4)
	w.want(w.report("lotse", four, `{"external_session_ref":"schema-5","model":"model-small"}`, key()...), http.StatusOK, "")
	esk := w.newThread(w.eskalation)
	guest := w.b6Post("owner", esk, "@texter please check the copy")
	startRun(guest)

	c := b6Contract(t)
	var changed, invocations int
	modes := map[string]bool{}
	for _, ev := range w.b6StoredEvents() {
		switch ev.Type {
		case "agent_session.changed":
			changed++
			if ev.ResourceKind != "thread" || ev.ResourceID != ev.ThreadID || ev.ResourceVersion != 0 || ev.ReceiveAction != "thread.read" {
				t.Fatalf("envelope of agent_session.changed: resource %s %s version %d, receive %s", ev.ResourceKind, ev.ResourceID,
					ev.ResourceVersion, ev.ReceiveAction)
			}
			if ev.SpaceID != w.support || ev.ChannelID.IsZero() || ev.ActorKind == "" {
				t.Fatalf("scope or actor of agent_session.changed: %+v", ev)
			}
			b6ValidFrame(t, c, ev.Envelope(gosync.DeliveryFields{}))
		case "agent.invocation":
			invocations++
			env := ev.Envelope(gosync.DeliveryFields{InvocationToken: tokens[ev.ResourceID.String()]})
			b6ValidFrame(t, c, env)
			var p map[string]any
			must(t, json.Unmarshal(ev.Payload, &p))
			mode, _ := p["session_mode"].(string)
			modes[mode] = true
			if mode == "resume" || mode == "bind" {
				// The negative control: an unknown mode must fail the schema.
				p["session_mode"] = "restart"
				bad, err := json.Marshal(p)
				must(t, err)
				env.Payload = bad
				raw, err := json.Marshal(map[string]any{"op": "dispatch", "d": env})
				must(t, err)
				if c.ValidateServerFrame(raw) == nil {
					t.Fatalf("a payload with session_mode restart passed the schema")
				}
			}
		}
	}
	// One: insert, report, reset, bind (ended none: the row was reset). Two: take-over ends the
	// row of one and inserts. Three: import binding, bind_failed (two). Four: first report.
	if changed < 9 || invocations != 5 {
		t.Fatalf("%d agent_session.changed, %d agent.invocation", changed, invocations)
	}
	for _, m := range []string{"new", "resume", "bind", ""} {
		if !modes[m] {
			t.Fatalf("no invocation with session_mode %q among %v", m, modes)
		}
	}
}

// b6Handle is the handle of the agent of run.
func (w *world) b6Handle(run string) string {
	if w.b6Run(run).Agent == w.texter.String() {
		return "texter"
	}
	return "lotse"
}

// TestAZP1_SessionRoutes (AZ-P1, 05 rule 3): the four session routes are enabled, each with its
// x-authz-action, its rate-limit class known to the catalogue, Idempotency-Key on the writes,
// If-Match optional on the report and ETag on the single-session answers; the server of the world
// starts with them registered (routes.Build verifies the mounted patterns) and each answers
// through its handler.
func TestAZP1_SessionRoutes(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		action, method, class string
		write                 bool
		ifMatch               routes.IfMatchMode
		etag                  []int
	}{
		"listAgentSessions":  {"agent_session.read", http.MethodGet, "rl.read", false, routes.IfMatchNone, nil},
		"resetAgentSession":  {"agent_session.reset", http.MethodPost, "rl.trigger", true, routes.IfMatchNone, []int{http.StatusCreated}},
		"bindAgentSession":   {"agent_session.bind", http.MethodPost, "rl.trigger", true, routes.IfMatchNone, []int{http.StatusCreated}},
		"reportAgentSession": {"agent_session.report", http.MethodPost, "rl.agent_runtime", true, routes.IfMatchOptional, []int{http.StatusOK}},
	}
	var found []string
	cat := routes.SpecCatalog()
	for _, op := range routes.Enabled() {
		if !strings.Contains(op.Path, "/agent-sessions") {
			continue
		}
		found = append(found, op.ID)
		x, ok := want[op.ID]
		if !ok {
			t.Fatalf("unexpected session route %s", op.ID)
		}
		if op.Action != x.action || op.Method != x.method || op.RateLimitClass != x.class || op.IdempotencyKey != x.write || op.IfMatch != x.ifMatch ||
			!slices.Equal(op.ETagStatuses, x.etag) {
			t.Fatalf("route %s: %+v", op.ID, op)
		}
		if class, ok := cat.Class(op.Action); !ok || class != op.RateLimitClass {
			t.Fatalf("catalogue class of %s: %q %v", op.Action, class, ok)
		}
		if got, ok := routes.ByPattern(op.Pattern); !ok || got.ID != op.ID {
			t.Fatalf("pattern %s not registered", op.Pattern)
		}
	}
	if len(found) != 4 {
		t.Fatalf("%d session routes enabled: %v", len(found), found)
	}

	// The world mounts the routes through routes.Build (which verifies them); each answers.
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	if l := w.wantOK(w.listSessions("alex", th)); len(l["items"].([]any)) != 0 {
		t.Fatalf("list %v", l)
	}
	w.wantReason(w.reset("alex", th), http.StatusConflict, "conflict", "no_current_session")
	w.b6Started(th)
	r := w.want(w.report("lotse", th, `{"external_session_ref":"route-1"}`, key()...), http.StatusOK, "")
	b := w.bind("alex", th, `{"external_session_ref":"route-2","expected_current_session_id":"`+r["id"].(string)+`"}`)
	if b.StatusCode != http.StatusCreated || b.Header.Get("ETag") == "" {
		t.Fatalf("bind %d %q: %s", b.StatusCode, b.Header.Get("ETag"), b.Body)
	}
}
