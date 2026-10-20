// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
)

// invocationFrame is the part of a dispatch of agent.invocation the delivery tests read.
type invocationFrame struct {
	Seq             int64  `json:"seq"`
	Type            string `json:"type"`
	InvocationToken string `json:"invocation_token"`
	Redelivered     bool   `json:"redelivered"`
	Audience        string `json:"audience"`
	Resource        struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		Version int64  `json:"version"`
	} `json:"resource"`
	Payload struct {
		RunID     string `json:"run_id"`
		ThreadID  string `json:"thread_id"`
		MessageID string `json:"message_id"`
	} `json:"payload"`
}

// invocationWorld is the consumer world with one person Sam without owner role.
type invocationWorld struct {
	*consumerWorld
	sam gwPerson
}

// newInvocationWorld starts the world of the delivery tests (M2-08).
func newInvocationWorld(t *testing.T) *invocationWorld {
	t.Helper()
	w := newConsumerWorld(t)
	return &invocationWorld{consumerWorld: w, sam: w.newPerson("sam", identity.DeviceStateActive)}
}

// operated creates a text channel operated by a new agent with auto_thread as given; it returns
// the channel, the agent and its handle.
func (w *invocationWorld) operated(name string, autoThread bool) (channel, consumerAgent, string) {
	w.t.Helper()
	ch := w.newChannel(w.space, name)
	a := w.agent()
	w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1, auto_thread = $2 WHERE id = $3`, a.id.String(), autoThread, ch.id)
	var handle string
	if err := w.queryRow(context.Background(), `SELECT handle FROM principals WHERE id = $1`, a.id.String()).Scan(&handle); err != nil {
		w.t.Fatal(err)
	}
	return ch, a, handle
}

// triggerGrant gives the person p the level trigger on the channel and returns the grant id.
func (w *invocationWorld) triggerGrant(p gwPerson, ch channel) string {
	return w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": p.user.String(), "target_kind": "channel", "target_id": ch.id, "level": "trigger", "reason": "test"})
}

// say posts text as p in thread and returns the message id and its trigger_state in the answer.
func (w *invocationWorld) say(p gwPerson, thread, text string) (string, string) {
	w.t.Helper()
	code, b := w.rest(w.base, p, http.MethodPost, "/api/v1/threads/"+thread+"/messages", map[string]any{"client_msg_id": ids.New().String(), "content_md": text})
	if code != http.StatusCreated && code != http.StatusOK {
		w.t.Fatalf("createMessage = %d %s", code, b)
	}
	var m struct {
		ID           string `json:"id"`
		TriggerState string `json:"trigger_state"`
	}
	_ = json.Unmarshal(b, &m)
	return m.ID, m.TriggerState
}

// runRow is the run of a triggering message as stored.
type runRow struct {
	id, thread, state, reason, jti, renewed, billing string
	version                                          int64
}

// runOf reads the run of message; ok false without one.
func (w *invocationWorld) runOf(message string) (runRow, bool) {
	w.t.Helper()
	var r runRow
	err := w.queryRow(context.Background(), `SELECT id::text, thread_id::text, state, coalesce(state_reason, ''), invocation_jti::text,
		coalesce(renewed_from_jti::text, ''), billing_mode_expected, version FROM runs WHERE triggering_message_id = $1`, message).
		Scan(&r.id, &r.thread, &r.state, &r.reason, &r.jti, &r.renewed, &r.billing, &r.version)
	return r, err == nil
}

// triggerOf reads trigger_state and trigger_state_reason of message.
func (w *invocationWorld) triggerOf(message string) (string, string) {
	w.t.Helper()
	var st, reason string
	if err := w.queryRow(context.Background(), `SELECT coalesce(trigger_state, ''), coalesce(trigger_state_reason, '') FROM messages WHERE id = $1`, message).
		Scan(&st, &reason); err != nil {
		w.t.Fatal(err)
	}
	return st, reason
}

// count runs a count query.
func (w *invocationWorld) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.queryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// nextInvocation reads frames of c until a dispatch of agent.invocation arrives or d passed; ok
// false when none came. ack acknowledges the other dispatches on the way.
func (w *invocationWorld) nextInvocation(c *wstest.Client, d time.Duration, ack bool) (invocationFrame, bool) {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return invocationFrame{}, false
		}
		if err != nil {
			w.t.Fatal(err)
		}
		if op != "dispatch" {
			continue
		}
		var f invocationFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			w.t.Fatal(err)
		}
		if f.Type != "agent.invocation" {
			// With ack, every other event is acknowledged at once, so agent.max_inflight_events
			// never holds the invocation back; the invocation itself stays unacknowledged.
			if ack {
				sendAck(w.t, c, f.Seq)
			}
			continue
		}
		{
			if f.InvocationToken != "" {
				w.remember(f.InvocationToken)
			}
			return f, true
		}
	}
}

// pollInvocations polls consumer of a until its cursor reaches upTo, acknowledging every page;
// it returns the agent.invocation envelopes it got and the last next_cursor.
func (w *invocationWorld) pollInvocations(a consumerAgent, consumer string, upTo int64) ([]invocationFrame, int64) {
	w.t.Helper()
	var out []invocationFrame
	deadline := time.Now().Add(15 * time.Second)
	for {
		p, _ := w.poll(w.base, a, consumer, "?limit=50")
		for _, raw := range p.Events {
			var f invocationFrame
			if err := json.Unmarshal(raw, &f); err != nil {
				w.t.Fatal(err)
			}
			if f.Type == "agent.invocation" {
				if f.InvocationToken != "" {
					w.remember(f.InvocationToken)
				}
				out = append(out, f)
			}
		}
		if code, b := w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/consumers/"+consumer+"/ack", map[string]any{"seq": p.NextCursor}); code != http.StatusOK {
			w.t.Fatalf("ack = %d %s", code, b)
		}
		if p.NextCursor >= upTo {
			return out, p.NextCursor
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("the cursor stayed at %d below %d", p.NextCursor, upTo)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// invocationSeq reads the seq of the agent.invocation of run.
func (w *invocationWorld) invocationSeq(run string) int64 {
	w.t.Helper()
	var seq int64
	if err := w.queryRow(context.Background(), `SELECT seq FROM event_log WHERE type = 'agent.invocation' AND resource_id = $1`, run).Scan(&seq); err != nil {
		w.t.Fatal(err)
	}
	return seq
}

// verifier is the reference verifier of the adapter check for agent a against getJwks.
func (w *invocationWorld) verifier(a consumerAgent) *runstest.ReferenceVerifier {
	w.t.Helper()
	var iss string
	if err := w.queryRow(context.Background(), `SELECT instance_id::text FROM instance`).Scan(&iss); err != nil {
		w.t.Fatal(err)
	}
	v, err := runstest.NewReferenceVerifier(runstest.ReferenceOptions{
		JWKSURL: w.base + "/api/v1/instance/jwks", Client: http.DefaultClient, Clock: clock.System(), Issuer: ids.MustParse(iss),
		AgentID: a.id, Leeway: 30 * time.Second, RefreshMinInterval: time.Second,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return v
}

// TestInvocationDelivery is the integration suite of M2-08 against a started api process: the
// acceptance writes the run and agent.invocation without a token, every delivery over the gateway
// and the REST poll passes runs.Delivery with a fresh token, and a denial before the delivery
// fails the run without sending anything.
func TestInvocationDelivery(t *testing.T) {
	t.Parallel()
	w := newInvocationWorld(t)

	t.Run("TestDelivery_HappyPathCarriesFreshToken", func(t *testing.T) {
		ch, a, handle := w.operated("happy", true)
		w.triggerGrant(w.sam, ch)
		c, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		m, state := w.say(w.sam, ch.thread, "@"+handle+" the export fails")
		if state != "received" {
			t.Fatalf("201 trigger_state %q, want received", state)
		}
		r, ok := w.runOf(m)
		if !ok || r.thread == ch.thread || r.state != "queued" {
			t.Fatalf("run %+v in the main thread", r)
		}
		if n := w.count(`SELECT count(*) FROM threads WHERE id = $1 AND parent_message_id = $2`, r.thread, m); n != 1 {
			t.Fatal("the run's thread is not the auto thread of the message")
		}
		if n := w.count(`SELECT count(*) FROM messages WHERE run_id = $1 AND thread_id = $2 AND kind = 'run' AND origin = 'agent' AND silent`, r.id, r.thread); n != 1 {
			t.Fatal("no anchor in the auto thread")
		}
		f, ok := w.nextInvocation(c, 15*time.Second, true)
		if !ok || f.InvocationToken == "" || f.Redelivered || f.Audience != "self" || f.Resource.Kind != "run" || f.Resource.ID != r.id ||
			f.Resource.Version != 0 || f.Payload.MessageID != m {
			t.Fatalf("invocation %+v", f)
		}
		claims, err := w.verifier(a).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{
			RunID: ids.MustParse(r.id), ThreadID: ids.MustParse(r.thread), MessageID: ids.MustParse(m), ActualBillingMode: "organization",
			ActualToolClass: "chat"})
		if err != nil {
			t.Fatalf("reference check: %v", err)
		}
		r, _ = w.runOf(m)
		if claims.Sub != w.sam.user.String() || claims.Jti != r.jti || claims.Level != "trigger" || claims.ToolClass != "chat" ||
			claims.BillingMode != "organization" || claims.TriggerKind != "message" || claims.ChannelID != ch.id || claims.SpaceID != w.space ||
			r.renewed != "" {
			t.Fatalf("claims %+v run %+v", claims, r)
		}
		if st, _ := w.triggerOf(m); st != "queued" {
			t.Fatalf("trigger_state %s, want queued", st)
		}
		updates := func() int {
			return w.count(`SELECT count(*) FROM event_log WHERE type = 'message.updated' AND resource_id = $1`, m)
		}
		// received at the acceptance, queued at the first delivery.
		if n := updates(); n != 2 {
			t.Fatalf("message.updated of M: %d", n)
		}
		// TestDelivery_TokenNeverStored.
		if n := w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%' OR payload::text LIKE '%invocation_token%'`, f.InvocationToken); n != 0 {
			t.Fatal("the token is in event_log")
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, f.InvocationToken); n != 0 {
			t.Fatal("the token is in audit_events")
		}
		// TestDelivery_EveryDeliveryFreshToken: without an ack the redelivery carries a new token.
		g, ok := w.nextInvocation(c, 3*consumerAckTimeout, false)
		if !ok || !g.Redelivered || g.InvocationToken == "" || g.InvocationToken == f.InvocationToken {
			t.Fatalf("redelivery %+v", g)
		}
		c2, err := w.verifier(a).Check(context.Background(), g.InvocationToken, runstest.ReferenceExpectation{RunID: ids.MustParse(r.id),
			ActualBillingMode: "organization", ActualToolClass: "chat"})
		r2, _ := w.runOf(m)
		if err != nil || c2.Jti == claims.Jti || c2.Jti != r2.jti || r2.renewed != "" || r2.version != r.version+1 {
			t.Fatalf("redelivered claims %+v %v, run %+v", c2, err, r2)
		}
		if n := updates(); n != 2 {
			t.Fatalf("a redelivery changed the trigger state again: %d updates", n)
		}
		sendAck(t, c, g.Seq)
	})

	t.Run("TestDelivery_AG07_AZM51_RevokedBeforeDelivery", func(t *testing.T) {
		ch, a, handle := w.operated("revoked", false)
		grant := w.triggerGrant(w.sam, ch)
		c, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		c.Close()
		m, _ := w.say(w.sam, ch.thread, "@"+handle+" please look")
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+grant+"/revoke", map[string]any{"reason": "left the team"})
		c, _ = w.ready(t, w.agentParams(a, ""))
		if f, ok := w.nextInvocation(c, 3*time.Second, true); ok {
			t.Fatalf("a revoked trigger was delivered: %+v", f)
		}
		eventually(t, 10*time.Second, "the failed run", func() bool { r, _ := w.runOf(m); return r.state == "failed" })
		r, _ := w.runOf(m)
		if r.reason != "access_revoked" {
			t.Fatalf("run %+v", r)
		}
		if st, reason := w.triggerOf(m); st != "failed" || reason != "access_revoked" {
			t.Fatalf("trigger_state %s %s", st, reason)
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'run.state_changed' AND resource_id = $1`, r.id); n != 2 {
			t.Fatalf("run.state_changed of the run: %d", n)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.invoke' AND decision = 'deny' AND actor_principal_id = $1
			AND resource_kind = 'thread' AND resource_id = $2 AND details->>'run_id' = $3 AND details->>'agent_id' = $4
			AND details->>'stage' = 'delivery' AND actor_session_id IS NULL AND actor_device_id IS NULL`, w.sam.user.String(), ch.thread, r.id, a.id.String()); n != 1 {
			t.Fatalf("audit entries of the denial: %d", n)
		}
	})

	t.Run("TestDelivery_RESTPoll", func(t *testing.T) {
		ch, a, handle := w.operated("poll", false)
		w.triggerGrant(w.sam, ch)
		w.poll(w.base, a, "rest", "")
		m, _ := w.say(w.sam, ch.thread, "@"+handle+" over rest")
		r, _ := w.runOf(m)
		got, _ := w.pollInvocations(a, "rest", w.invocationSeq(r.id))
		if len(got) != 1 || got[0].InvocationToken == "" || got[0].Resource.ID != r.id {
			t.Fatalf("polled invocations %+v", got)
		}
		if _, err := w.verifier(a).Check(context.Background(), got[0].InvocationToken, runstest.ReferenceExpectation{RunID: ids.MustParse(r.id),
			ActualBillingMode: "organization", ActualToolClass: "chat"}); err != nil {
			t.Fatalf("reference check: %v", err)
		}
		if r2, _ := w.runOf(m); r2.jti == r.jti {
			t.Fatal("the poll did not record the jti of its token")
		}
		// A second channel of the same agent, so the first run does not hold the thread.
		ch2 := w.newChannel(w.space, "poll-two")
		w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1, auto_thread = false WHERE id = $2`, a.id.String(), ch2.id)
		w.triggerGrant(w.sam, ch2)
		m2, _ := w.say(w.sam, ch2.thread, "@"+handle+" again")
		r2, _ := w.runOf(m2)
		// BR-04 (b), I2 without I1: the agent becomes personal of the owner behind the back of the
		// rights hook while Sam keeps the grant; the delivery refuses.
		w.sql(`UPDATE agents SET billing_mode = 'personal', credential_holder_id = $1 WHERE principal_id = $2`, w.owner.user.String(), a.id.String())
		seq := w.invocationSeq(r2.id)
		refused, cursor := w.pollInvocations(a, "rest", seq)
		if len(refused) != 0 || cursor < seq {
			t.Fatalf("a refused invocation was polled %+v, cursor %d below %d", refused, cursor, seq)
		}
		if r2, _ = w.runOf(m2); r2.state != "failed" || r2.reason != "billing_mode_personal_not_holder" {
			t.Fatalf("run %+v", r2)
		}
		if st, reason := w.triggerOf(m2); st != "failed" || reason != "billing_mode_personal_not_holder" {
			t.Fatalf("trigger_state %s %s", st, reason)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.invoke' AND decision = 'deny' AND deny_reason = 'billing_mode_personal_not_holder'
			AND details->>'run_id' = $1`, r2.id); n != 1 {
			t.Fatalf("audit entries: %d", n)
		}
		// Back to organization, so I1 does not refuse the grants of the later subtests.
		w.sql(`UPDATE agents SET billing_mode = 'organization', credential_holder_id = NULL WHERE principal_id = $1`, a.id.String())
	})

	t.Run("TestDelivery_AZM19_AZM20_ToolClassHost", func(t *testing.T) {
		ch, a, handle := w.operated("werkzeuge", false)
		w.sql(`UPDATE agents SET tool_class = 'host' WHERE principal_id = $1`, a.id.String())
		c, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		// AZ-M19: the owner triggers the host agent; the delivery has no request of the owner.
		m, _ := w.say(w.owner, ch.thread, "@"+handle+" build it")
		f, ok := w.nextInvocation(c, 15*time.Second, true)
		if !ok || f.InvocationToken == "" {
			t.Fatal("the owner's trigger of a host agent was not delivered")
		}
		r, _ := w.runOf(m)
		cl, err := w.verifier(a).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{RunID: ids.MustParse(r.id),
			ActualBillingMode: "organization", ActualToolClass: "host"})
		if err != nil || cl.ToolClass != "host" || cl.Sub != w.owner.user.String() {
			t.Fatalf("claims %+v %v", cl, err)
		}
		sendAck(t, c, f.Seq)
		// AZ-M20 at delivery: Sam triggers in a second channel of the agent while it is scoped, and
		// it reports host before the delivery.
		w.sql(`UPDATE agents SET tool_class = 'scoped' WHERE principal_id = $1`, a.id.String())
		ch2 := w.newChannel(w.space, "werkzeuge-two")
		w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1, auto_thread = false WHERE id = $2`, a.id.String(), ch2.id)
		w.triggerGrant(w.sam, ch2)
		c.Close()
		m2, _ := w.say(w.sam, ch2.thread, "@"+handle+" build it for me")
		if st, _ := w.triggerOf(m2); st != "received" {
			t.Fatalf("acceptance of a scoped agent: %s", st)
		}
		w.sql(`UPDATE agents SET reported_tool_class = 'host' WHERE principal_id = $1`, a.id.String())
		c, _ = w.ready(t, w.agentParams(a, ""))
		if f, ok := w.nextInvocation(c, 3*time.Second, true); ok {
			t.Fatalf("delivered %+v", f)
		}
		eventually(t, 10*time.Second, "the refused run", func() bool { r, _ := w.runOf(m2); return r.state == "failed" })
		if r, _ := w.runOf(m2); r.reason != "tool_class_owner_only" {
			t.Fatalf("run %+v", r)
		}
	})

	t.Run("TestDelivery_QueuedTooOldWithoutJob", func(t *testing.T) {
		ch, a, handle := w.operated("expired", false)
		w.triggerGrant(w.sam, ch)
		// The first connection starts at the head; the consumer exists before the trigger.
		first, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, first, func(f frame) bool { return f.op == "catchup.complete" })
		first.Close()
		m, _ := w.say(w.sam, ch.thread, "@"+handle+" anyone there")
		r, _ := w.runOf(m)
		// AG-27: the job is scheduled at queued_at + run.queued_max_age in the creating transaction.
		if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.queued_expiry' AND args->>'run_id' = $1::text
			AND scheduled_at = (SELECT queued_at + interval '48 hours' FROM runs WHERE id = $1::uuid)`, r.id); n != 1 {
			t.Fatalf("expiry jobs of the run: %d", n)
		}
		w.sql(`UPDATE runs SET queued_at = queued_at - interval '49 hours' WHERE id = $1`, r.id)
		c, _ := w.ready(t, w.agentParams(a, ""))
		if f, ok := w.nextInvocation(c, 3*time.Second, true); ok {
			t.Fatalf("an expired run was delivered: %+v", f)
		}
		eventually(t, 10*time.Second, "the expired run", func() bool { r, _ := w.runOf(m); return r.state == "failed" })
		if r, _ := w.runOf(m); r.reason != "agent_unavailable" {
			t.Fatalf("run %+v", r)
		}
		if st, reason := w.triggerOf(m); st != "failed" || reason != "agent_unavailable" {
			t.Fatalf("trigger_state %s %s", st, reason)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.invoke' AND details->>'run_id' = $1`, r.id); n != 0 {
			t.Fatal("an expiry was audited")
		}
	})

	t.Run("TestAcceptance_AZM30_AgentMessageNeverTriggers", func(t *testing.T) {
		ch, a, _ := w.operated("agents", false)
		w.sql(`UPDATE channels SET free_response = true WHERE id = $1`, ch.id)
		code, b := w.call(w.base, a, http.MethodPost, "/api/v1/threads/"+ch.thread+"/messages",
			map[string]any{"client_msg_id": ids.New().String(), "content_md": "a note from the agent"})
		if code != http.StatusCreated {
			t.Fatalf("agent message = %d %s", code, b)
		}
		var msg struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(b, &msg)
		if _, ok := w.runOf(msg.ID); ok {
			t.Fatal("an agent message opened a run")
		}
		if st, _ := w.triggerOf(msg.ID); st != "" {
			t.Fatalf("trigger_state %s", st)
		}
		if strings.Contains(string(b), "trigger_state") {
			t.Fatalf("the answer carries a trigger state: %s", b)
		}
	})
}
