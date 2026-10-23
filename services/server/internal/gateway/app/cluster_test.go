// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// bus is the channel ephemeral in memory: every Send reaches every listening cluster, the sender
// included, in the order of the sends, as PostgreSQL delivers NOTIFY (03 6.6).
type bus struct {
	mu        gosync.Mutex
	listeners []*app.Cluster
	sent      [][]byte
	fail      bool
}

// Send implements app.SignalSender.
func (b *bus) Send(ctx context.Context, payload []byte) error {
	b.mu.Lock()
	if b.fail {
		b.mu.Unlock()
		return errors.New("notify failed")
	}
	b.sent = append(b.sent, slices.Clone(payload))
	listeners := slices.Clone(b.listeners)
	b.mu.Unlock()
	for _, l := range listeners {
		l.HandleEphemeral(ctx, payload)
	}
	return nil
}

// signals returns the decoded gateway signals sent so far.
func (b *bus) signals(t *testing.T) []domain.Signal {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []domain.Signal
	for _, p := range b.sent {
		s, ours, err := domain.DecodeSignal(p)
		if err != nil || !ours {
			t.Fatalf("sent %s: ours %v, %v", p, ours, err)
		}
		out = append(out, s)
	}
	return out
}

// newCluster returns a cluster of a new node on b that listens on b.
func newCluster(t *testing.T, b *bus, w *focusWorld) *app.Cluster {
	t.Helper()
	c, err := app.NewCluster(app.ClusterConfig{Node: ids.New(), TypingTTL: typingTTL}, app.ClusterDeps{Sender: b, Threads: w, Clock: clock.NewFake(t0)})
	if err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	b.listeners = append(b.listeners, c)
	b.mu.Unlock()
	return c
}

// twoNodes is two api nodes over one world and one channel.
type twoNodes struct {
	w    *focusWorld
	bus  *bus
	a, b *fenv
}

func newTwoNodes(t *testing.T) *twoNodes {
	t.Helper()
	n := &twoNodes{w: newFocusWorld(), bus: &bus{}}
	n.a = newFocusEnvOn(t, n.w, newCluster(t, n.bus, n.w))
	n.b = newFocusEnvOn(t, n.w, newCluster(t, n.bus, n.w))
	n.b.support = n.a.support
	return n
}

// typingEvents returns the typing.started dispatches queued on cl.
func typingEvents(t *testing.T, cl *client) []wireEvent {
	t.Helper()
	var out []wireEvent
	for _, ev := range cl.events(t) {
		if ev.D.Type == "typing.started" {
			out = append(out, ev)
		}
	}
	return out
}

// TestCluster_TypingCrossNode: typing.start of Alex on node A reaches Sam on node B, who focuses
// the thread, as typing.started with the place of the thread and expires_at = now +
// sync.typing_ttl on node B; the signal carries only the thread and the principal; the sender's
// own node, which receives its signal too, delivers the event once.
func TestCluster_TypingCrossNode(t *testing.T) {
	t.Parallel()
	n := newTwoNodes(t)
	th := n.w.thread(n.a.support)
	alex, sam, robin := n.a.newPerson(), n.a.newPerson(), n.a.newPerson()
	for _, p := range []ids.ID{alex.ID, sam.ID, robin.ID} {
		n.w.grant(p, th.ChannelID, levelWrite)
	}
	alexA, robinA, samB := n.a.join(alex), n.a.join(robin), n.b.join(sam)
	n.a.focus(robinA, false, th.ID)
	n.b.focus(samB, false, th.ID)
	n.a.typing(alexA, "typing.start", th.ID)

	got := typingEvents(t, samB)
	if len(got) != 1 {
		t.Fatalf("Sam on node B got %d typing.started, want 1", len(got))
	}
	ev := got[0]
	if ev.D.Actor == nil || ev.D.Actor.ID != alex.ID.String() || ev.D.Scope["thread_id"] != th.ID.String() ||
		ev.D.Scope["channel_id"] != th.ChannelID.String() || ev.D.Scope["space_id"] != th.SpaceID.String() {
		t.Fatalf("typing.started on node B %+v", ev.D)
	}
	var body struct {
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(ev.D.Payload, &body); err != nil || body.ExpiresAt != t0.UTC().Add(typingTTL).Format("2006-01-02T15:04:05.000Z") {
		t.Fatalf("expires_at %q, %v", body.ExpiresAt, err)
	}
	if got := typingEvents(t, robinA); len(got) != 1 {
		t.Fatalf("Robin on node A got %d typing.started, want 1 (its own signal is ignored)", len(got))
	}
	var sigs []domain.Signal
	for _, s := range n.bus.signals(t) {
		if s.Kind == domain.SignalTyping {
			sigs = append(sigs, s)
		}
	}
	if len(sigs) != 1 || sigs[0].ThreadID != th.ID || sigs[0].PrincipalID != alex.ID {
		t.Fatalf("signals %+v", sigs)
	}
}

// TestCluster_DeviceReplacedAcrossNodes: a second identify of Alex's device on node B closes the
// older connection on node A with 4017 (04 6.1) and leaves the new one open; a signal with an
// older binding than the one a node holds changes nothing.
func TestCluster_DeviceReplacedAcrossNodes(t *testing.T) {
	t.Parallel()
	n := newTwoNodes(t)
	alex := n.a.newPerson()
	older := ids.New()
	onA := n.a.join(alex)
	onB := n.b.join(alex)
	wantClosed(t, onA.fc, domain.CloseDeviceReplaced)
	wantOpen(t, onB.fc)

	n.b.gw.DeviceTaken(alex.DeviceID, ids.New(), older)
	wantOpen(t, onB.fc)
	sigs := n.bus.signals(t)
	if len(sigs) != 2 || sigs[0].Kind != domain.SignalDevice || sigs[1].DeviceID != alex.DeviceID || sigs[0].BindID == sigs[1].BindID {
		t.Fatalf("signals %+v", sigs)
	}
}

// TestCluster_HintFromWorker: a worker's cluster, which has no hub, publishes
// notification.created as a signal; the node that holds the recipient's connection delivers it to
// that connection only, and the other node delivers nothing.
func TestCluster_HintFromWorker(t *testing.T) {
	t.Parallel()
	n := newTwoNodes(t)
	worker := newCluster(t, n.bus, n.w)
	if err := worker.Bind(nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	alex, sam := n.a.newPerson(), n.a.newPerson()
	samA, alexB := n.a.join(sam), n.b.join(alex)
	nid := ids.New()
	err := worker.Publisher(nil).PublishEphemeral(context.Background(), eventlog.EphemeralEvent{
		Type: "notification.created", SchemaVersion: 1, Audience: alex.ID,
		Resource: eventlog.ResourceRef{Kind: "user", ID: alex.ID},
		Payload:  map[string]any{"nid": nid, "kind": "mention", "quiet": false, "count": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	var hints []wireEvent
	for _, ev := range alexB.events(t) {
		if ev.D.Type == "notification.created" {
			hints = append(hints, ev)
		}
	}
	if len(hints) != 1 {
		t.Fatalf("Alex on node B got %d hints, want 1", len(hints))
	}
	var body struct {
		NID   string `json:"nid"`
		Kind  string `json:"kind"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(hints[0].D.Payload, &body); err != nil || body.NID != nid.String() || body.Kind != "mention" || body.Count != 2 {
		t.Fatalf("hint %s %v", hints[0].D.Payload, err)
	}
	for _, ev := range samA.events(t) {
		if ev.D.Type == "notification.created" {
			t.Fatal("Sam on node A got Alex's hint")
		}
	}
}

// TestCluster_PublisherSignals: the publisher sends a signal for run.updated (run, thread,
// version) and agent.presence_changed (agent, state, billing mode, actor) with IDs and codes only,
// none for a type that stays on its node, and a failed send is dropped without failing the local
// publish (at most once).
func TestCluster_PublisherSignals(t *testing.T) {
	t.Parallel()
	b := &bus{}
	c, err := app.NewCluster(app.ClusterConfig{Node: ids.New(), TypingTTL: time.Second}, app.ClusterDeps{Sender: b})
	if err != nil {
		t.Fatal(err)
	}
	pub := c.Publisher(nil)
	run, thread, agent, actorID := ids.New(), ids.New(), ids.New(), ids.New()
	ctx := context.Background()
	evs := []eventlog.EphemeralEvent{
		{Type: "run.updated", SchemaVersion: 1, Payload: runs.RunUpdated{RunID: run, ThreadID: thread, BaseVersion: 6, Version: 7}},
		{Type: "agent.presence_changed", SchemaVersion: 1, Resource: eventlog.ResourceRef{Kind: "agent", ID: agent},
			Actor:   &eventlog.ActorRef{Kind: eventlog.ActorAgent, PrincipalID: actorID},
			Payload: map[string]any{"agent_id": agent.String(), "state": "online", "reported_billing_mode": "subscription", "text": "never sent"}},
		{Type: "host.presence_changed", SchemaVersion: 1},
	}
	for _, ev := range evs {
		if err := pub.PublishEphemeral(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	sigs := b.signals(t)
	if len(sigs) != 2 {
		t.Fatalf("signals %+v", sigs)
	}
	if s := sigs[0]; s.Kind != domain.SignalRun || s.RunID != run || s.ThreadID != thread || s.Version != 7 {
		t.Errorf("run signal %+v", s)
	}
	if s := sigs[1]; s.Kind != domain.SignalAgentPresence || s.AgentID != agent || s.State != "online" || s.BillingMode != "subscription" || s.ActorID != actorID {
		t.Errorf("agent signal %+v", s)
	}
	b.mu.Lock()
	b.fail = true
	b.mu.Unlock()
	if err := pub.PublishEphemeral(ctx, evs[0]); err != nil {
		t.Fatalf("a failed send failed the publish: %v", err)
	}
}
