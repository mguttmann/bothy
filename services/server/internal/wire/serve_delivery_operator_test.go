// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// restIfMatch is dlWorld.rest with an If-Match header; it returns status, body and ETag.
func (w *dlWorld) restIfMatch(p gwPerson, method, path, ifMatch string, body any) (int, []byte, string) {
	w.t.Helper()
	w.sql(`UPDATE auth_sessions SET step_up_at = now(), step_up_method = 'passkey', step_up_consumed_at = NULL WHERE id = $1`, p.set.Session.ID.String())
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, method, w.base+path, rd)
	req.Header.Set("Authorization", "DPoP "+p.access)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{HTM: method, HTU: gwPublicURL + path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(p.access)}))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Idempotency-Key", ids.New().String())
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, resp.Header.Get("ETag")
}

// TestGateway_SpaceDefaultOperatorGetsNewChannelLive: an agent that is the default operator of a space
// gets write (and so read) on every inheriting channel of the space through the operator rule.
// A channel created in the space later is no rights change (no access.changed), so the hub's
// indices of the agent's connection never learn it: the agent is no candidate of the channel's
// events and the live delivery drops them, while catch-up (fresh Scope) admits them.
func TestGateway_SpaceDefaultOperatorGetsNewChannelLive(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t, "BOTHY_SYNC__CURSOR_ADVANCE_EVERY=1")
	space := w.newSpace("Support")
	agent, _, params := w.agentWithToken()
	code, b, etag := w.restIfMatch(w.owner, http.MethodGet, "/api/v1/spaces/"+space, "", nil)
	if code != 200 {
		t.Fatalf("GET space = %d %s", code, b)
	}
	code, b, _ = w.restIfMatch(w.owner, http.MethodPut, "/api/v1/spaces/"+space+"/operator", etag,
		map[string]any{"mode": "agent", "agent_id": agent.String(), "reason": "test"})
	if code != 200 {
		t.Fatalf("PUT operator = %d %s", code, b)
	}
	c, r := w.ready(t, params)
	ch := w.newChannel(space, "tickets")
	cm := w.post(ch.thread, "for the operator")
	// A second, later event the agent sees anyway is not needed: with cursor_advance_every=1 the
	// hub sends cursor frames to the agent's live connection for every processed event.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var live []frame
	gotMsg := false
	var maxCursor int64
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			break
		}
		f := frame{op: op}
		if op == "dispatch" {
			_ = json.Unmarshal(d, &f.ev)
			f.seq = f.ev.Seq
			if f.ev.ClientMsgID == cm {
				gotMsg = true
			}
		}
		if op == "cursor" {
			var v struct {
				Seq int64 `json:"seq"`
			}
			_ = json.Unmarshal(d, &v)
			f.seq = v.Seq
			maxCursor = max(maxCursor, v.Seq)
		}
		live = append(live, f)
	}
	t.Logf("live frames of the operator agent: %v", ops(live))
	c.Close()
	// Control: the agent may read the channel; catch-up from the head of ready admits the message.
	c2 := w.dial()
	p := params
	p.Cursor, p.SyncEpoch = &r.HeadSeq, r.SyncEpoch.String()
	ictx, icancel := context.WithTimeout(context.Background(), 15*time.Second)
	if _, err := c2.Identify(ictx, p); err != nil {
		icancel()
		t.Fatalf("identify with cursor: %v", err)
	}
	icancel()
	fs := readUntil(t, c2, func(f frame) bool { return f.op == "catchup.complete" })
	t.Logf("catch-up frames from cursor %d: %v", r.HeadSeq, ops(fs))
	caught := false
	var msgSeq int64
	for _, f := range fs {
		if f.op == "dispatch" && f.ev.ClientMsgID == cm {
			caught, msgSeq = true, f.seq
		}
	}
	if !caught {
		t.Fatalf("catch-up did not admit the message either; the agent may not read the channel")
	}
	if !gotMsg {
		t.Fatalf("the default operator agent did not receive message.created@%d of the new channel live (catch-up admits it); highest live cursor %d", msgSeq, maxCursor)
	}
}

// TestGateway_OperatorNewChannelWhileCatchingUp (red team round 4): the default operator agent of
// an empty space reconnects with a gap; while its catch-up runs, a channel is created in the space
// and a message is posted there. The agent receives the message (M1-65/15), not only the later
// live message of its other channel. The backlog is written to the log directly and the client
// does not read meanwhile, so the catch-up is still running when the channel is created.
func TestGateway_OperatorNewChannelWhileCatchingUp(t *testing.T) {
	t.Parallel()
	const backlog = 20000
	w := newDeliveryWorld(t, "BOTHY_SYNC__CATCHUP_PAGE_SIZE=50", "BOTHY_SYNC__OUTBOUND_QUEUE_MAX=50",
		"BOTHY_SYNC__TAIL_BATCH_SIZE=50000", "BOTHY_SYNC__CURSOR_ADVANCE_EVERY=1")
	eng := w.newSpace("Engineering")
	work := w.newChannel(eng, "work")
	other := w.newChannel(eng, "other")
	robin := w.newPerson("robin", identity.DeviceStateActive)
	w.grant(robin.user, "user", "channel", other.id)
	watch, _ := w.ready(t, w.params(robin))
	sup := w.newSpace("Support")
	agent, _, params := w.agentWithToken()
	w.grant(agent, "agent", "channel", work.id)
	code, b, etag := w.restIfMatch(w.owner, http.MethodGet, "/api/v1/spaces/"+sup, "", nil)
	if code != 200 {
		t.Fatalf("GET space = %d %s", code, b)
	}
	code, b, _ = w.restIfMatch(w.owner, http.MethodPut, "/api/v1/spaces/"+sup+"/operator", etag,
		map[string]any{"mode": "agent", "agent_id": agent.String(), "reason": "test"})
	if code != 200 {
		t.Fatalf("PUT operator = %d %s", code, b)
	}
	c, r := w.ready(t, params)
	cm := w.post(work.thread, "before")
	readUntil(t, c, message(cm))
	c.Close()
	w.sql(`WITH h AS (UPDATE sequence_counters SET value = value + $2 WHERE name = 'event_seq' RETURNING value - $2 AS base)
		INSERT INTO event_log (seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id, channel_id,
			thread_id, resource_kind, resource_id, resource_version, receive_action, payload, client_msg_id, permission_version)
		SELECT h.base + g, gen_random_uuid(), e.type, e.schema_version, e.occurred_at, e.actor_kind, e.actor_principal_id, e.space_id,
			e.channel_id, e.thread_id, e.resource_kind, e.resource_id, e.resource_version, e.receive_action, e.payload, e.client_msg_id,
			e.permission_version
		FROM h, generate_series(1, $2::bigint) g, (SELECT * FROM event_log WHERE client_msg_id = $1 LIMIT 1) e`, cm, backlog)
	tail := w.post(other.thread, "tail passed")
	wctx, wcancel := context.WithTimeout(context.Background(), 3*time.Minute)
	for {
		op, d, err := watch.Next(wctx)
		if err != nil {
			wcancel()
			t.Fatalf("the tail did not pass the backlog: %v", err)
		}
		if op == "dispatch" && bytes.Contains(d, []byte(tail)) {
			break
		}
	}
	wcancel()
	c2 := w.dial(wstest.WithoutHeartbeat())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	p := params
	p.Cursor, p.SyncEpoch = &r.HeadSeq, r.SyncEpoch.String()
	if _, err := c2.Identify(ctx, p); err != nil {
		cancel()
		t.Fatalf("identify with cursor: %v", err)
	}
	cancel()
	// The client does not read: the catch-up runner waits for queue space meanwhile.
	tickets := w.newChannel(sup, "tickets")
	cm2 := w.post(tickets.thread, "for the operator")
	after := w.post(work.thread, "after")
	rctx, rcancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer rcancel()
	got, complete := false, false
	var maxCursor, msgSeq int64
	var afterSeq int64
	n := 0
	for {
		op, d, err := c2.Next(rctx)
		if err != nil {
			t.Fatalf("after %d dispatches: %v", n, err)
		}
		switch op {
		case "catchup.complete":
			complete = true
		case "cursor":
			var v struct {
				Seq int64 `json:"seq"`
			}
			_ = json.Unmarshal(d, &v)
			if complete {
				maxCursor = max(maxCursor, v.Seq)
			}
		case "dispatch":
			n++
			var ev event
			_ = json.Unmarshal(d, &ev)
			if ev.ClientMsgID == cm2 {
				got, msgSeq = true, ev.Seq
			}
			if ev.ClientMsgID == after {
				afterSeq = ev.Seq
			}
		}
		if complete && afterSeq > 0 {
			break
		}
	}
	if !got {
		t.Fatalf("the default operator agent never received the message of the channel created during its catch-up, "+
			"but received the later live message at seq %d (%d dispatches, cursor after complete up to %d)", afterSeq, n, maxCursor)
	}
	if msgSeq > afterSeq {
		t.Fatalf("message of the new channel at seq %d after the later message at seq %d", msgSeq, afterSeq)
	}
}
