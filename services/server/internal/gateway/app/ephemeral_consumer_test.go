// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
)

// TestEphemeral_ConsumerNotBeforeCatchUpComplete: an agent consumer connection (M2-07) gets no
// ephemeral frame (M1-66) before its runner queued catchup.complete, even though the hub marks it
// live at Attach; once catchup.complete is queued, the next one reaches it (INT-W41/1).
func TestEphemeral_ConsumerNotBeforeCatchUpComplete(t *testing.T) {
	t.Parallel()
	s := newEphemeralScene(t)
	agent := s.e.newAgent()
	th := s.e.w.thread(s.e.support)
	s.e.w.grant(agent.ID, th.ChannelID, levelRead)
	clog := &consumerLog{hidden: map[int64]bool{}}
	cursors := newCursorStore()
	cursors.acked[agents.ConsumerKey{AgentID: agent.ID, Consumer: "default"}] = 0
	eng, err := app.NewConsumerEngine(app.ConsumerConfig{MaxInflight: 10, AckTimeout: time.Hour, PersistInterval: time.Hour,
		IdleWindow: time.Minute, PageSize: 4, CursorEvery: 100, RetryBase: time.Millisecond, RetryCap: 10 * time.Millisecond},
		app.ConsumerDeps{Consumers: cursors, CatchUp: clog, Position: s.e.hub, Hook: dropHook{}, Epoch: staticEpoch{}, Clock: s.e.clk})
	if err != nil {
		t.Fatal(err)
	}
	s.e.hub.BindConsumers(eng)
	qc := newQConn(64)
	c, sess := app.NewTestConnection(qc, agent)
	sess.Consumer = "default"
	s.e.hub.Register(context.Background(), c, sess)
	s.e.hub.ForceFocus(c, th.ID)
	t.Cleanup(func() { s.e.hub.Detach(c) })
	if err := s.e.hub.Attach(context.Background(), c, 0, 41); err != nil {
		t.Fatal(err)
	}
	// The hub position is 0, so the runner cannot read up to head 41 and stays in its backlog.
	if err := s.e.hub.PublishEphemeral(context.Background(), typingOf(s.alex, th.ID)); err != nil {
		t.Fatal(err)
	}
	if n := qc.dispatches(); n != 0 {
		t.Fatalf("%d ephemeral frames before catchup.complete", n)
	}
	clog.mu.Lock()
	clog.head = 41
	clog.mu.Unlock()
	s.e.hub.Seed(41)
	eng.WakeAll()
	deadline := time.Now().Add(5 * time.Second)
	for !queuedOp(qc, "catchup.complete") {
		if time.Now().After(deadline) {
			t.Fatal("no catchup.complete")
		}
		time.Sleep(time.Millisecond)
	}
	if err := s.e.hub.PublishEphemeral(context.Background(), typingOf(s.alex, th.ID)); err != nil {
		t.Fatal(err)
	}
	if n := qc.dispatches(); n != 1 {
		t.Fatalf("%d ephemeral frames after catchup.complete, want 1", n)
	}
}

// queuedOp drains q and reports whether a frame with op was written so far.
func queuedOp(q *qconn, op string) bool {
	for _, e := range q.drain() {
		if strings.Contains(string(e.Frame), `"op":"`+op+`"`) {
			return true
		}
	}
	return false
}
