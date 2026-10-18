// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the hub is the EphemeralPublisher of the process.
var _ eventlog.EphemeralPublisher = (*app.Hub)(nil)

// typingOf returns a typing.started event of actor a on th.
func typingOf(a actor.Actor, th ids.ID) eventlog.EphemeralEvent {
	return eventlog.EphemeralEvent{Type: "typing.started", SchemaVersion: 1,
		Actor:    &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: a.ID},
		Scope:    eventlog.Scope{ThreadID: th},
		Resource: eventlog.ResourceRef{Kind: "thread", ID: th},
		Payload:  map[string]string{"thread_id": th.String()},
	}
}

// ephemeralScene is a hub over the focus world with one thread Sam reads.
type ephemeralScene struct {
	e    *fenv
	th   ids.ID
	sam  actor.Actor
	alex actor.Actor
}

func newEphemeralScene(t *testing.T) *ephemeralScene {
	t.Helper()
	e := newFocusEnv(t)
	th := e.w.thread(e.support)
	s := &ephemeralScene{e: e, th: th.ID, sam: e.newPerson(), alex: e.newPerson()}
	e.w.grant(s.sam.ID, th.ChannelID, levelRead)
	return s
}

// register registers a connection of a with conn on the hub (catching up) and focuses th.
func (s *ephemeralScene) register(conn app.Conn, a actor.Actor) *app.Connection {
	c, sess := app.NewTestConnection(conn, a)
	s.e.hub.Register(context.Background(), c, sess)
	s.e.hub.ForceFocus(c, s.th)
	return c
}

// TestEphemeral_NotDeliveredDuringCatchUp: before catchup.complete an ephemeral frame is not
// delivered and not buffered; once live, the next one is.
func TestEphemeral_NotDeliveredDuringCatchUp(t *testing.T) {
	t.Parallel()
	s := newEphemeralScene(t)
	qc := newQConn(10)
	c := s.register(qc, s.sam)
	if err := s.e.hub.PublishEphemeral(context.Background(), typingOf(s.alex, s.th)); err != nil {
		t.Fatal(err)
	}
	if n := qc.len(); n != 0 {
		t.Fatalf("%d frames during catch-up", n)
	}
	if err := s.e.hub.Attach(context.Background(), c, 41, 41); err != nil {
		t.Fatal(err)
	}
	if n := qc.dispatches(); n != 0 {
		t.Fatalf("%d dispatches after going live: the ephemeral frame was buffered", n)
	}
	if err := s.e.hub.PublishEphemeral(context.Background(), typingOf(s.alex, s.th)); err != nil {
		t.Fatal(err)
	}
	if n := qc.dispatches(); n != 1 {
		t.Fatalf("%d dispatches when live, want 1", n)
	}
}

// TestEphemeral_DroppedWhenQueueFullNoClose: a full outbound queue drops the ephemeral frame and
// never closes with 4014.
func TestEphemeral_DroppedWhenQueueFullNoClose(t *testing.T) {
	t.Parallel()
	s := newEphemeralScene(t)
	qc := newQConn(1)
	c := s.register(qc, s.sam)
	if err := s.e.hub.Attach(context.Background(), c, 41, 41); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.e.hub.PublishEphemeral(context.Background(), typingOf(s.alex, s.th)); err != nil {
			t.Fatal(err)
		}
	}
	if code := qc.closedWith(); code != 0 {
		t.Fatalf("closed with %d", code)
	}
	if n := qc.len(); n != 1 {
		t.Errorf("%d queued, want the one that fit", n)
	}
	if got := s.e.hub.PhaseOf(c); got != domain.PhaseLive {
		t.Errorf("phase %s", got)
	}
}

// TestEphemeral_PublishNeverBlocks: with a full queue and no writer, PublishEphemeral returns at
// once.
func TestEphemeral_PublishNeverBlocks(t *testing.T) {
	t.Parallel()
	s := newEphemeralScene(t)
	qc := newQConn(1)
	c := s.register(qc, s.sam)
	if err := s.e.hub.Attach(context.Background(), c, 41, 41); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		var err error
		for range 100 {
			if err = s.e.hub.PublishEphemeral(context.Background(), typingOf(s.alex, s.th)); err != nil {
				break
			}
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PublishEphemeral blocked on a full queue")
	}
}

// TestEphemeral_UnregisteredTypeRejected: an unregistered type, a schema version below 1, a
// payload that is no object and a payload of sync.event_payload_max_bytes are programming
// faults; nothing is sent.
func TestEphemeral_UnregisteredTypeRejected(t *testing.T) {
	t.Parallel()
	s := newEphemeralScene(t)
	qc := newQConn(10)
	c := s.register(qc, s.sam)
	if err := s.e.hub.Attach(context.Background(), c, 41, 41); err != nil {
		t.Fatal(err)
	}
	big := typingOf(s.alex, s.th)
	big.Payload = map[string]string{"x": strings.Repeat("a", 65536)}
	noObject := typingOf(s.alex, s.th)
	noObject.Payload = []int{1}
	nullPayload := typingOf(s.alex, s.th)
	nullPayload.Payload = nil
	unknown := typingOf(s.alex, s.th)
	unknown.Type = "message.created"
	noVersion := typingOf(s.alex, s.th)
	noVersion.SchemaVersion = 0
	for name, ev := range map[string]eventlog.EphemeralEvent{
		"unregistered type": unknown, "schema version 0": noVersion, "payload too large": big, "array payload": noObject,
		"null payload": nullPayload,
	} {
		if err := s.e.hub.PublishEphemeral(context.Background(), ev); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if n := qc.len(); n != 0 {
		t.Errorf("%d frames sent", n)
	}
}

// TestEphemeral_AudienceAndPurge: an ephemeral event with an audience goes only to that
// principal's live connections; an access.changed removal purges queued ephemeral frames of the
// removed channel like dispatches.
func TestEphemeral_AudienceAndPurge(t *testing.T) {
	t.Parallel()
	s := newEphemeralScene(t)
	samQ, alexQ := newQConn(10), newQConn(10)
	samC, alexC := s.register(samQ, s.sam), s.register(alexQ, s.alex)
	for _, c := range []*app.Connection{samC, alexC} {
		if err := s.e.hub.Attach(context.Background(), c, 41, 41); err != nil {
			t.Fatal(err)
		}
	}
	ev := typingOf(s.alex, s.th)
	ev.Type, ev.Audience = "run.updated", s.sam.ID
	ev.Resource = eventlog.ResourceRef{Kind: "run", ID: ids.New()}
	if err := s.e.hub.PublishEphemeral(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	// run.updated is focus-bound: Alex focuses the thread too, but the audience is Sam.
	if alexQ.len() != 0 || samQ.len() != 1 {
		t.Fatalf("Alex %d frames, Sam %d frames; want 0 and 1", alexQ.len(), samQ.len())
	}
	samQ.drain()
	th := s.e.w.threads[s.th]
	typing := typingOf(s.alex, s.th)
	typing.Scope = eventlog.Scope{SpaceID: th.SpaceID, ChannelID: th.ChannelID, ThreadID: th.ID}
	if err := s.e.hub.PublishEphemeral(context.Background(), typing); err != nil {
		t.Fatal(err)
	}
	if n := samQ.len(); n != 1 {
		t.Fatalf("Sam has %d queued frames, want the typing frame", n)
	}
	s.e.w.grant(s.sam.ID, th.ChannelID, 0)
	s.e.hub.Dispatch(context.Background(), accessChanged(42, s.sam.ID, th.ChannelID))
	if n := samQ.len(); n != 1 {
		t.Fatalf("Sam has %d queued frames, want only access.changed", n)
	}
	all := framesOf(t, samQ.drain())
	if last := all[len(all)-1]; last.op != "dispatch" || last.seq != 42 {
		t.Errorf("queued frame %+v, want access.changed", last)
	}
}
