// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The regression tests of the phase rule (docs/DECISIONS.md M1-65/15), one per reproduced
// instance; TestHub_DeliveryModel covers the class.

// channelCreated returns channel.created at seq for channel in space.
func channelCreated(seq int64, space, channel ids.ID) sync.Event {
	e := msg(seq, space, channel)
	e.Type, e.ResourceKind, e.ResourceID, e.ReceiveAction = "channel.created", "channel", channel, "channel.read"
	return e
}

// operatorChannel adds a channel in a new space that p reads as its default operator: no grant of
// its own and no access.changed.
func operatorChannel(w *hubWorld, p ids.ID) (space, channel ids.ID) {
	space = ids.New()
	w.mu.Lock()
	w.operator[p] = map[ids.ID]bool{space: true}
	w.mu.Unlock()
	return space, w.channel(space)
}

// TestHub_OperatorNewChannelWhileCatchingUp (red team round 4): the default operator agent of a
// space catches up when a channel is created in the space and its first message is posted; the
// message is delivered after catchup.complete, and no cursor frame passes it.
func TestHub_OperatorNewChannelWhileCatchingUp(t *testing.T) {
	t.Parallel()
	w, alex, _, _, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	fired := false
	w.onPage = func(int64) {
		if fired {
			return
		}
		fired = true
		s2, c := operatorChannel(w, alex.ID)
		w.store(channelCreated(11, s2, c), msg(12, s2, c))
		h.Dispatch(context.Background(), channelCreated(11, s2, c))
		h.Dispatch(context.Background(), msg(12, s2, c))
	}
	qc := connect(t, h, alex, 100, 0, 10)
	fs := waitComplete(t, qc)
	other := ids.New()
	for seq := int64(13); seq <= 18; seq++ {
		h.Dispatch(context.Background(), msg(seq, other, ids.ID{}))
	}
	waitFor(t, "the cursor of the tail", func() bool {
		fs = framesOf(t, qc.drain())
		return slices.Contains(fs, frame{"cursor", 16})
	})
	if got := dispatched(fs); !slices.Contains(got, 12) {
		t.Fatalf("message 12 of the new channel never delivered; frames %v", fs)
	}
}

// TestHub_NewChannelUpToHeadIsLearned: the tail lags, so channel.created of a channel the
// default operator reads reaches a connection only after its head was read past it; the event is
// covered by the snapshot, but the later messages of the channel are still delivered.
func TestHub_NewChannelUpToHeadIsLearned(t *testing.T) {
	t.Parallel()
	for _, between := range []bool{true, false} {
		w := newHubWorld()
		alex := person()
		h := newHub(t, w, hubCfg)
		qc := newQConn(100)
		conn, sess := app.NewTestConnection(qc, alex)
		h.Register(context.Background(), conn, sess)
		s2, c := operatorChannel(w, alex.ID)
		if between {
			// channel.created reaches the registered connection before Attach.
			h.Dispatch(context.Background(), channelCreated(21, s2, c))
		}
		if err := h.Attach(context.Background(), conn, 21, 21); err != nil {
			t.Fatal(err)
		}
		if !between {
			h.Dispatch(context.Background(), channelCreated(21, s2, c))
		}
		h.Dispatch(context.Background(), msg(22, s2, c))
		if got := dispatched(framesOf(t, qc.drain())); !slices.Equal(got, []int64{22}) {
			t.Fatalf("between Register and Attach %v: dispatched %v, want [22]", between, got)
		}
	}
}

// TestHub_LiveSkipsTailEventsUpToHead (red team round 1, Minor): the tail lags behind the head a
// connection was attached at; the events up to the head are covered by the snapshot or the
// catch-up and are not sent again, and no frame goes back behind the cursor.
func TestHub_LiveSkipsTailEventsUpToHead(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	space := ids.New()
	ch := w.channel(space)
	alex := person()
	w.grantChannel(alex.ID, ch)
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 10, 10)
	for seq := int64(9); seq <= 11; seq++ {
		h.Dispatch(context.Background(), msg(seq, space, ch))
	}
	if got := dispatched(framesOf(t, qc.drain())); !slices.Equal(got, []int64{11}) {
		t.Fatalf("dispatched %v, want [11]", got)
	}
}

// staleRemovalWorld is the world of the tests of a removal the tail processes after the pages
// read past it: at seq 1 alex loses channel c, at seq 2 he gets it back, seq 3 is a message in c
// he reads since its commit.
func staleRemovalWorld() (w *hubWorld, alex actor.Actor, removal sync.Event) {
	w = newHubWorld()
	space := ids.New()
	c := w.channel(space)
	alex = person()
	w.grantChannel(alex.ID, c)
	removal = accessChanged(1, alex.ID, c)
	w.store(removal, accessGranted(2, alex.ID, c), msg(3, space, c))
	return w, alex, removal
}

// TestHub_StaleRemovalKeepsLaterPageEvents (model seed 1508888741): the tail delivers a removal
// of seq 1 while the catch-up pages already read seq 3 with the rights after the re-grant of
// seq 2; the removal drops only events before seq 1, so seq 3 is delivered.
func TestHub_StaleRemovalKeepsLaterPageEvents(t *testing.T) {
	t.Parallel()
	w, alex, removal := staleRemovalWorld()
	h := newHub(t, w, hubCfg)
	w.onPage = func(int64) { h.Dispatch(context.Background(), removal) }
	qc := connect(t, h, alex, 100, 0, 3)
	if got := dispatched(waitComplete(t, qc)); !slices.Contains(got, 3) {
		t.Fatalf("dispatched %v, want seq 3", got)
	}
}

// TestHub_StaleRemovalKeepsQueuedLaterEvents: the same removal reaches a connection whose queue
// still holds seq 3 from its catch-up; the purge drops only queued events before seq 1.
func TestHub_StaleRemovalKeepsQueuedLaterEvents(t *testing.T) {
	t.Parallel()
	w, alex, removal := staleRemovalWorld()
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 0, 3)
	waitFor(t, "catchup.complete queued", func() bool { return qc.len() >= 4 })
	h.Dispatch(context.Background(), removal)
	if got := dispatched(framesOf(t, qc.drain())); !slices.Contains(got, 3) {
		t.Fatalf("dispatched %v, want seq 3", got)
	}
}
