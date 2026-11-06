// SPDX-License-Identifier: Apache-2.0

package appsworld

// The scenarios of the webhook transport for agents (V1-01; 09 section 4.2, 04 section 15.6): an
// owner subscribes an agent, and the delivery job sends its events strictly in seq order, every
// invocation-bearing event with a freshly issued invocation token.

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// Invocations is the fake of the port runs.Delivery in the world: it records every call and
// answers with a fresh token, a denial or a hold.
type Invocations struct {
	mu    stdsync.Mutex
	calls []runs.DeliveryInput
	deny  bool
	hold  bool
}

// Prepare implements runs.Delivery.
func (f *Invocations) Prepare(_ context.Context, in runs.DeliveryInput) (runs.DeliveryOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	switch {
	case f.hold:
		return runs.DeliveryOutcome{Hold: true}, nil
	case f.deny:
		return runs.DeliveryOutcome{}, nil
	}
	return runs.DeliveryOutcome{Send: true, InvocationToken: "inv-" + strconv.Itoa(len(f.calls))}, nil
}

// Calls returns the recorded calls.
func (f *Invocations) Calls() []runs.DeliveryInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// Set changes the answer: deny refuses, hold holds; both false sends.
func (f *Invocations) Set(deny, hold bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deny, f.hold = deny, hold
}

// Invocations is the fake runs.Delivery of the world.
func (w *World) Invocations() *Invocations { return w.web.Invocations }

// Agent inserts an active agent principal with its agents row and returns its id.
func (w *World) Agent(handle string) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, id.String(), handle)
	w.SQL(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, id.String())
	return id
}

// GrantAgent grants agent level on the channel target as Robin.
func (w *World) GrantAgent(agent, target ids.ID, level string) ids.ID {
	w.t.Helper()
	return w.grantSubject("agent", agent, target, level)
}

// agentWebhooksPath is the collection of the subscriptions of agent.
func agentWebhooksPath(agent ids.ID) string { return "/api/v1/agents/" + agent.String() + "/webhooks" }

// SubscribeAgent creates a subscription of agent on types at the hooks receiver as o and returns
// the created body.
func (w *World) SubscribeAgent(o *Client, agent ids.ID, types ...string) map[string]any {
	w.t.Helper()
	return w.Want(o.Call(http.MethodPost, agentWebhooksPath(agent), subscribeBody(w.web.Hooks.URL("/agent"), types, false), Idem()...),
		http.StatusCreated, "")
}

// ScenarioAgentWebhookSubscriptions (V1-01, 05 webhook_out.manage and webhook_out.read on an
// agent): an owner with a fresh step-up subscribes an agent and lists its subscriptions; the
// answer names agent_id and no app_id; every key under webhooks: is allowed, a personal event of
// people is not; an unknown agent, a person and a member without the right get the same 404; a
// stale step-up is 401; the audit entry names the agent; the subscription operations of 04 15.6
// (update, delete, deliveries) work on it as on an app subscription.
func ScenarioAgentWebhookSubscriptions(t *testing.T) {
	t.Parallel()
	w := NewWorld(t)
	agent := w.Agent("lotse")
	o := w.RobinPhone.StepUp()
	url := w.Hooks().URL("/agent")
	types := []string{"agent.invocation", "agent.followup", "run.cancel_requested", "message.created"}

	m := w.Want(o.Call(http.MethodPost, agentWebhooksPath(agent), subscribeBody(url, types, true), Idem()...), http.StatusCreated, "")
	sub := m["subscription"].(map[string]any)
	if sub["agent_id"] != agent.String() || sub["app_id"] != nil || sub["state"] != "active" || !EqualList(sub["event_types"], types...) {
		t.Fatalf("created %v", m)
	}
	if secret, _ := m["hmac_secret"].(string); !strings.HasPrefix(secret, "whsec_") || !EqualList(sub["signature_schemes"], "v1a", "v1") {
		t.Fatalf("created with hmac: %v", m)
	}
	id := SubscriptionID(m)

	list := w.Want(o.Call(http.MethodGet, agentWebhooksPath(agent), ""), http.StatusOK, "")
	items := list["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != id.String() || items[0].(map[string]any)["agent_id"] != agent.String() {
		t.Fatalf("list %v", list)
	}

	w.WantField(o.Call(http.MethodPost, agentWebhooksPath(agent), subscribeBody(url, []string{"draft.updated"}, false), Idem()...),
		"validation_failed", "/event_types/0", "not_subscribable")
	w.WantField(o.Call(http.MethodPost, agentWebhooksPath(agent), subscribeBody("https://other.example.com/agent", types, false), Idem()...),
		"webhook_host_not_allowed", "/url", "webhook_host_not_allowed")

	alex := w.Phone(w.Alex)
	for name, r := range map[string]struct {
		c    *Client
		path string
	}{
		"unknown agent":   {o, agentWebhooksPath(ids.New())},
		"a person":        {o, agentWebhooksPath(w.Sam)},
		"member, create":  {alex, agentWebhooksPath(agent)},
		"member, unknown": {alex, agentWebhooksPath(ids.New())},
	} {
		resp := r.c.Call(http.MethodPost, r.path, subscribeBody(url, types, false), Idem()...)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: create %d %s", name, resp.StatusCode, resp.Body)
		}
		if resp := r.c.Call(http.MethodGet, r.path, ""); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: list %d %s", name, resp.StatusCode, resp.Body)
		}
	}
	w.Want(w.Phone(w.Robin).Call(http.MethodPost, agentWebhooksPath(agent), subscribeBody(url, types, false), Idem()...),
		http.StatusUnauthorized, "step_up_required")

	audits := w.Allows("webhook_out.manage")
	if len(audits) != 1 || audits[0].ResourceID != agent.String() || audits[0].Details["agent_id"] != agent.String() ||
		audits[0].Details["change"] != "created" || strings.Contains(audits[0].Raw, "/agent") || strings.Contains(audits[0].Raw, "whsec_") {
		t.Fatalf("manage entries %+v", audits)
	}
	var kind string
	w.Scalar(&kind, `SELECT resource_kind FROM audit_events WHERE action = 'webhook_out.manage'`)
	if kind != "agent" {
		t.Fatalf("resource_kind %q, want agent", kind)
	}

	// The subscription operations by id work on an agent subscription.
	patch := `{"event_types":["agent.invocation","agent.config_changed"],"state":"paused"}`
	u := w.Want(o.Call(http.MethodPatch, "/api/v1/webhooks/"+id.String(), patch, Idem()...), http.StatusOK, "")
	if s := u["subscription"].(map[string]any); s["state"] != "paused" || !EqualList(s["event_types"], "agent.invocation", "agent.config_changed") ||
		s["agent_id"] != agent.String() {
		t.Fatalf("updated %v", u)
	}
	w.WantField(o.Call(http.MethodPatch, "/api/v1/webhooks/"+id.String(), `{"event_types":["settings.updated"]}`, Idem()...),
		"validation_failed", "/event_types/0", "not_subscribable")
	w.Want(alex.Call(http.MethodPatch, "/api/v1/webhooks/"+id.String(), `{"state":"active"}`, Idem()...), http.StatusNotFound, "not_found")
	w.Want(o.Call(http.MethodGet, "/api/v1/webhooks/"+id.String()+"/deliveries", ""), http.StatusOK, "")
	w.Want(o.Call(http.MethodDelete, "/api/v1/webhooks/"+id.String(), "", Idem()...), http.StatusNoContent, "")
	if list := w.Want(o.Call(http.MethodGet, agentWebhooksPath(agent), ""), http.StatusOK, ""); len(list["items"].([]any)) != 0 {
		t.Fatalf("after delete %v", list)
	}
}

// agentEvent appends a personal event of type for agent (audience self) on a run and returns its
// seq.
func agentEvent(w *World, eventType string, agent ids.ID) int64 {
	return w.AppendEvent(eventlog.Event{Type: eventType, SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: w.Alex,
		SpaceID: w.Support, ChannelID: w.Eskalation, ThreadID: w.EskalationThread, ResourceKind: "run", ResourceID: ids.New(),
		ReceiveAction: "self", AudiencePrincipalID: agent, Payload: []byte(`{"run":{"id":"x"}}`)})
}

// received is the decoded body of one request at the receiver.
type received struct {
	Seq             int64  `json:"seq"`
	Type            string `json:"type"`
	InvocationToken string `json:"invocation_token"`
	Redelivered     bool   `json:"redelivered"`
	SyncEpoch       string `json:"sync_epoch"`
}

// bodies decodes every request the hooks receiver got, in order.
func bodies(t *testing.T, w *World) []received {
	t.Helper()
	var out []received
	for _, r := range w.Hooks().Got() {
		var b received
		if err := json.Unmarshal(r.Body, &b); err != nil {
			t.Fatal(err)
		}
		if r.Header.Get("webhook-signature") == "" || r.Header.Get("webhook-id") == "" {
			t.Fatalf("request without the Standard Webhooks headers: %v", r.Header)
		}
		out = append(out, b)
	}
	return out
}

// ScenarioAgentWebhookDelivery (V1-01; 09 section 4.2, 04 section 15.6, R-SY-07): the consumer
// round creates rows for the agent's own personal events and for the events of channels it may
// read, never for another agent's events; the job delivers them strictly in seq order; every
// attempt of an invocation-bearing event, a retry included, carries a fresh invocation token from
// runs.Delivery, a retry with redelivered; an event without an invocation carries none; a denial
// of runs.Delivery ends the row dead with not_authorized and sends nothing; a hold sends nothing
// and keeps the row and the order until it ends; an event of a channel the agent cannot read is
// dead with not_authorized; 2xx moves last_delivered_seq.
func ScenarioAgentWebhookDelivery(t *testing.T) {
	t.Parallel()
	w := NewWorld(t)
	agent, other := w.Agent("lotse"), w.Agent("other")
	o := w.RobinPhone.StepUp()
	sub := SubscriptionID(w.SubscribeAgent(o, agent, "agent.invocation", "agent.config_changed", "message.created"))
	w.GrantAgent(agent, w.Eskalation, "read")

	inv := agentEvent(w, "agent.invocation", agent)
	agentEvent(w, "agent.invocation", other)
	cfg := agentEvent(w, "agent.config_changed", agent)
	msg := esk(w, "Readable")
	hidden := w.Post(w.Alex, w.Allgemein, w.AllgemeinThread, "Not readable")
	var invID string
	w.Scalar(&invID, `SELECT id::text FROM event_log WHERE seq = $1`, inv)
	w.Hooks().Respond(failFirst(ids.MustParse(invID), 1))
	w.Round()
	var seqs []int64
	for _, d := range w.Deliveries(sub) {
		seqs = append(seqs, d.Seq)
	}
	if want := []int64{inv, cfg, msg.Seq, hidden.Seq}; !slices.Equal(seqs, want) {
		t.Fatalf("rows %v, want %v (none for the other agent)", seqs, want)
	}
	drain(w, 4)

	got := bodies(t, w)
	if len(got) != 4 {
		t.Fatalf("%d requests, want 4: %+v", len(got), got)
	}
	first, retry := got[0], got[1]
	if first.Seq != inv || retry.Seq != inv || first.InvocationToken == "" || retry.InvocationToken == "" ||
		first.InvocationToken == retry.InvocationToken || first.Redelivered || !retry.Redelivered || first.SyncEpoch == "" {
		t.Fatalf("invocation attempts %+v %+v: each needs its own fresh token, the retry redelivered", first, retry)
	}
	if got[2].Seq != cfg || got[2].InvocationToken != "" || got[3].Seq != msg.Seq || got[3].InvocationToken != "" {
		t.Fatalf("later requests %+v", got[2:])
	}
	calls := w.Invocations().Calls()
	if len(calls) != 2 || calls[0].AgentID != agent || calls[0].Seq != inv || calls[0].Type != "agent.invocation" ||
		calls[0].ResourceKind != "run" || calls[0].Redelivered || !calls[1].Redelivered {
		t.Fatalf("runs.Delivery calls %+v", calls)
	}
	rows := w.Deliveries(sub)
	if rows[0].Status != "succeeded" || rows[0].Attempt != 2 || rows[3].Status != "dead" || rows[3].LastError != "not_authorized" {
		t.Fatalf("rows %+v", rows)
	}
	if r := w.SubscriptionRow(sub); r.LastSeq != msg.Seq || r.State != "active" {
		t.Fatalf("subscription %+v", r)
	}

	// A denial of runs.Delivery sends nothing and ends the row.
	w.Invocations().Set(true, false)
	denied := agentEvent(w, "agent.invocation", agent)
	w.Round()
	drain(w, 2)
	if n := len(w.Hooks().Got()); n != 4 {
		t.Fatalf("a denied invocation was sent: %d requests", n)
	}
	if d := w.Deliveries(sub); d[len(d)-1].Seq != denied || d[len(d)-1].Status != "dead" || d[len(d)-1].LastError != "not_authorized" {
		t.Fatalf("denied row %+v", d[len(d)-1])
	}

	// A hold sends nothing, keeps the row pending and the next event behind it.
	w.Invocations().Set(false, true)
	held := agentEvent(w, "agent.invocation", agent)
	after := agentEvent(w, "agent.config_changed", agent)
	w.Round()
	w.Pump(5)
	if n := len(w.Hooks().Got()); n != 4 {
		t.Fatalf("a held invocation or the event behind it was sent: %d requests", n)
	}
	rows = w.Deliveries(sub)
	h := rows[len(rows)-2]
	if h.Seq != held || h.Status != "pending" || h.Attempt != 0 || h.NextAttemptAt == nil || !h.NextAttemptAt.After(w.Clk.Now()) {
		t.Fatalf("held row %+v", h)
	}
	w.Invocations().Set(false, false)
	drain(w, 3)
	got = bodies(t, w)
	if len(got) != 6 || got[4].Seq != held || got[4].InvocationToken == "" || got[5].Seq != after {
		t.Fatalf("after the hold %+v", got)
	}
	if r := w.SubscriptionRow(sub); r.LastSeq != after {
		t.Fatalf("subscription after the hold %+v", r)
	}
}
