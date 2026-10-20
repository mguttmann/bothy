// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// The presence windows of the process world (M2-20): short, and agent.presence_timeout above
// sync.heartbeat_interval (cross-check presence_timeout_above_heartbeat_interval).
const (
	presenceTimeout = 3 * time.Second
	presencePersist = 500 * time.Millisecond
)

// presenceWorld is the delivery world with the presence windows, the agent support-agent created
// by the Owner Alex through the API, and Sam with read on a channel of Support only.
type presenceWorld struct {
	*consumerWorld
	agent   consumerAgent
	sam     gwPerson
	channel channel
}

func newPresenceWorld(t *testing.T) *presenceWorld {
	t.Helper()
	cw := &consumerWorld{dlWorld: newDeliveryWorld(t, "BOTHY_AGENT__PRESENCE_TIMEOUT="+presenceTimeout.String(),
		"BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL="+presencePersist.String(), "BOTHY_BILLING__MISMATCH_ALERT_AFTER=2s",
		"BOTHY_SYNC__HEARTBEAT_INTERVAL=1s", "BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST=1000", "BOTHY_RATELIMIT__RL__ADMIN__BURST=1000",
		"BOTHY_RATELIMIT__RL__WRITE__BURST=1000", "BOTHY_RATELIMIT__RL__MESSAGE__BURST=1000")}
	cw.space = cw.newSpace("Support")
	w := &presenceWorld{consumerWorld: cw}
	w.channel = w.newChannel(w.space, "Allgemein")
	id := ids.MustParse(w.must(w.owner, http.MethodPost, "/api/v1/agents", map[string]any{
		"handle": "support-agent", "display_name": "Support agent", "adapter_kind": "custom"}))
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	w.agent = consumerAgent{id: id, tok: w.apiToken(apitoken.KindAgent, id, key, time.Now().Add(time.Hour)), key: key}
	w.sam = w.newPerson("sam", identity.DeviceStateActive)
	w.grant(w.sam.user, "user", "channel", w.channel.id)
	return w
}

// heartbeat sends sendHeartbeat as the agent and requires 200 with a HeartbeatAck.
func (w *presenceWorld) heartbeat(body string) rest.HeartbeatAck {
	w.t.Helper()
	code, b := w.call(w.base, w.agent, http.MethodPost, "/api/v1/agents/me/heartbeat", json.RawMessage(body))
	if code != http.StatusOK {
		w.t.Fatalf("sendHeartbeat = %d %s", code, b)
	}
	var ack rest.HeartbeatAck
	if err := wstest.Strict(b, &ack); err != nil {
		w.t.Fatalf("HeartbeatAck %s: %v", b, err)
	}
	return ack
}

// card reads getAgent of the agent as the Owner.
func (w *presenceWorld) card() rest.AgentRuntimeStatus {
	w.t.Helper()
	code, b := w.rest(w.base, w.owner, http.MethodGet, "/api/v1/agents/"+w.agent.id.String(), nil)
	if code != http.StatusOK {
		w.t.Fatalf("getAgent = %d %s", code, b)
	}
	var a rest.Agent
	if err := wstest.Strict(b, &a); err != nil {
		w.t.Fatalf("Agent %s: %v", b, err)
	}
	return a.Presence
}

// presenceEvent reads the frames of c until agent.presence_changed of the agent arrives and
// returns its raw envelope.
func (w *presenceWorld) presenceEvent(c *wstest.Client) json.RawMessage {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			w.t.Fatalf("no agent.presence_changed: %v", err)
		}
		if op != "dispatch" {
			continue
		}
		var ev struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(d, &ev)
		if ev.Type == "agent.presence_changed" {
			return d
		}
	}
}

// presencePayload decodes the envelope of agent.presence_changed strictly.
func presencePayload(t *testing.T, raw json.RawMessage) rest.AgentPresencePayload {
	t.Helper()
	var ev rest.AgentPresenceChangedEvent
	if err := wstest.Strict(raw, &ev); err != nil {
		t.Fatalf("AgentPresenceChangedEvent %s: %v", raw, err)
	}
	var env map[string]json.RawMessage
	_ = json.Unmarshal(raw, &env)
	if _, ok := env["seq"]; ok || bytes.Contains(raw, []byte("permission_version")) || bytes.Contains(raw, []byte("null")) {
		t.Fatalf("ephemeral envelope with seq, permission_version or null: %s", raw)
	}
	var p rest.AgentPresencePayload
	b, _ := json.Marshal(ev.Payload)
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPresenceGateway is the process part of M2-20 (PostgreSQL 18 via pgtest, the api process,
// the wstest client): delivery of agent.presence_changed to exactly the readers of the agent, the
// snapshot and ready, the gateway member agent equal to REST, and offline after
// agent.presence_timeout.
func TestPresenceGateway(t *testing.T) {
	w := newPresenceWorld(t)
	outer := t
	alex, _ := w.ready(t, w.params(w.owner))
	sam, samReady := w.ready(t, w.params(w.sam))
	self, _ := w.ready(t, w.agentParams(w.agent, ""))
	if len(samReady.Snapshot.Agents) != 0 {
		t.Fatalf("Sam's snapshot lists agents: %+v", samReady.Snapshot.Agents)
	}

	t.Run("TestPresenceChangedDelivery", func(t *testing.T) {
		ack := w.heartbeat(`{"state":"online","reported_billing_mode":"organization","billing_evidence":{"key_type":"api_key"}}`)
		if ack.Presence.State != rest.AgentRuntimeStateOnline || ack.IntervalMs != time.Second.Milliseconds() {
			t.Fatalf("ack %+v", ack)
		}
		for name, c := range map[string]*wstest.Client{"Alex": alex, "the agent": self} {
			p := presencePayload(t, w.presenceEvent(c))
			if p.AgentId != rest.Uuid(w.agent.id) || p.State != rest.AgentRuntimeStateOnline || p.ReportedBillingMode == nil ||
				*p.ReportedBillingMode != rest.BillingModeOrganization {
				t.Fatalf("%s: payload %+v", name, p)
			}
		}
		readUntil(t, sam, message(w.post(w.channel.thread, "sentinel")))
		if n := sam.Observed()["dispatch agent.presence_changed"]; n != 0 {
			t.Fatalf("Sam received %d agent.presence_changed without agent.read", n)
		}
	})

	t.Run("TestPresenceInSnapshot", func(t *testing.T) {
		code, b := w.rest(w.base, w.owner, http.MethodGet, "/api/v1/sync/snapshot", nil)
		if code != http.StatusOK {
			t.Fatalf("getSnapshot = %d %s", code, b)
		}
		var snap rest.Snapshot
		if err := wstest.Strict(b, &snap); err != nil {
			t.Fatal(err)
		}
		if len(snap.Agents) != 1 || snap.Agents[0].AgentId != rest.Uuid(w.agent.id) || snap.Agents[0].State != rest.AgentRuntimeStateOnline ||
			snap.Agents[0].ReportedBillingMode == nil {
			t.Fatalf("Alex's snapshot agents %+v", snap.Agents)
		}
		// A new connection replaces the earlier one of the same device or token (4017); the later
		// subtests use the new ones, which live as long as the whole test.
		var again, own rest.GatewayReady
		alex, again = w.ready(outer, w.params(w.owner))
		if len(again.Snapshot.Agents) != 1 || again.Snapshot.Agents[0].State != rest.AgentRuntimeStateOnline {
			t.Fatalf("Alex's new ready %+v", again.Snapshot.Agents)
		}
		self, own = w.ready(outer, w.agentParams(w.agent, ""))
		if len(own.Snapshot.Agents) != 1 || own.Snapshot.Agents[0].AgentId != rest.Uuid(w.agent.id) {
			t.Fatalf("the agent's own ready %+v", own.Snapshot.Agents)
		}
		code, b = w.rest(w.base, w.sam, http.MethodGet, "/api/v1/sync/snapshot", nil)
		if code != http.StatusOK || bytes.Contains(b, []byte(w.agent.id.String())) {
			t.Fatalf("Sam's snapshot = %d %s", code, b)
		}
	})

	t.Run("TestGatewayHeartbeatEqualsRest", func(t *testing.T) {
		acks := self.Observed()["heartbeat_ack"]
		if err := self.Send("heartbeat", map[string]any{"last_seq": 0, "agent": map[string]any{"state": "degraded",
			"reported_billing_mode": "organization", "details": map[string]any{"degraded_reasons": []string{"slow"}}}}); err != nil {
			t.Fatal(err)
		}
		if p := presencePayload(t, w.presenceEvent(alex)); p.State != rest.AgentRuntimeStateDegraded {
			t.Fatalf("payload %+v", p)
		}
		if st := w.card(); st.State != rest.AgentRuntimeStateDegraded {
			t.Fatalf("card after the gateway heartbeat %+v", st)
		}
		if err := self.Send("heartbeat", map[string]any{"last_seq": 0, "agent": map[string]any{"state": "online",
			"details": map[string]any{"teleport": true}}}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for {
			op, d, err := self.Next(ctx)
			if err != nil {
				t.Fatalf("no error frame: %v", err)
			}
			if op != "error" {
				continue
			}
			var e rest.GatewayError
			if err := wstest.Strict(d, &e); err != nil || e.Op != "heartbeat" || e.Code != "validation_failed" {
				t.Fatalf("error frame %s %v", d, err)
			}
			break
		}
		if self.Observed()["heartbeat_ack"] < acks+2 {
			t.Fatal("no heartbeat_ack for the agent heartbeats")
		}
		if st := w.card(); st.State != rest.AgentRuntimeStateDegraded {
			t.Fatalf("an invalid heartbeat changed the presence: %+v", st)
		}
		// Sam does not see the agent: the agent part answers error not_found (D-M2-20-0) and keeps
		// the connection.
		if err := sam.Send("heartbeat", map[string]any{"last_seq": 0, "agent": map[string]any{"state": "online"}}); err != nil {
			t.Fatal(err)
		}
		for {
			op, d, err := sam.Next(ctx)
			if err != nil {
				t.Fatalf("no error frame for Sam: %v", err)
			}
			if op == "error" {
				if !bytes.Contains(d, []byte(`"not_found"`)) {
					t.Fatalf("Sam's error %s", d)
				}
				break
			}
		}
		readUntil(t, sam, message(w.post(w.channel.thread, "still open")))
	})

	t.Run("TestPresenceOfflineAfterTimeout", func(t *testing.T) {
		w.heartbeat(`{"state":"online","reported_billing_mode":"organization"}`)
		w.presenceEvent(alex)
		last := time.Now()
		raw := w.presenceEvent(alex)
		if elapsed := time.Since(last); elapsed < presenceTimeout-200*time.Millisecond {
			t.Fatalf("offline after %s, before agent.presence_timeout", elapsed)
		}
		p := presencePayload(t, raw)
		if p.State != rest.AgentRuntimeStateOffline || p.ReportedBillingMode == nil || *p.ReportedBillingMode != rest.BillingModeOrganization {
			t.Fatalf("payload %+v", p)
		}
		if st := w.card(); st.State != rest.AgentRuntimeStateOffline {
			t.Fatalf("card %+v", st)
		}
		eventually(t, 10*time.Second, "the row stored offline", func() bool {
			var state string
			if err := w.queryRow(context.Background(), `SELECT reported_runtime_state FROM agents WHERE principal_id = $1`, w.agent.id.String()).
				Scan(&state); err != nil {
				return false
			}
			return state == "offline"
		})
	})
}
