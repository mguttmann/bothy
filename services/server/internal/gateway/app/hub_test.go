// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// TestHub_DispatchNeverBlocksOnSlowConnection: one connection's writer never drains; Dispatch
// returns within the test deadline for every event, the slow connection is closed with 4014 and
// the other one receives every frame (02 "Hub and delivery", 04 section 7.2 step 5).
func TestHub_DispatchNeverBlocksOnSlowConnection(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	support := ids.New()
	allgemein := w.channel(support)
	slow, fast := person(), person()
	w.grantChannel(slow.ID, allgemein)
	w.grantChannel(fast.ID, allgemein)
	h := newHub(t, w, hubCfg)
	blocked := connect(t, h, slow, 2, 0, 0)
	released := connect(t, h, fast, 1000, 0, 0)
	for seq := int64(1); seq <= 50; seq++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			h.Dispatch(context.Background(), msg(seq, support, allgemein))
		}()
		select {
		case <-done:
		case <-time.After(testkit.Patience):
			t.Fatalf("Dispatch of seq %d waited on a connection", seq)
		}
	}
	if code := blocked.closedWith(); code != domain.CloseSlowConsumer {
		t.Fatalf("slow connection closed with %d, want 4014", code)
	}
	got := dispatched(framesOf(t, released.drain()))
	if len(got) != 50 || got[0] != 1 || got[49] != 50 {
		t.Fatalf("fast connection got %v", got)
	}
}

// TestHub_SlowConsumerClosedWith4014: a queue of sync.outbound_queue_max frames that is full
// closes its connection with 4014 at the next frame.
func TestHub_SlowConsumerClosedWith4014(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	support := ids.New()
	allgemein := w.channel(support)
	alex := person()
	w.grantChannel(alex.ID, allgemein)
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 3, 0, 0)
	for seq := int64(1); seq <= 3; seq++ {
		h.Dispatch(context.Background(), msg(seq, support, allgemein))
	}
	if code := qc.closedWith(); code != 0 {
		t.Fatalf("closed at the bound with %d", code)
	}
	h.Dispatch(context.Background(), msg(4, support, allgemein))
	if code := qc.closedWith(); code != domain.CloseSlowConsumer {
		t.Fatalf("closed with %d, want 4014", code)
	}
}

// TestHub_PersonalEventsOnlyToAudience: a personal event reaches only the connections of its
// audience principal (05 "Durable event types and receive action", 07 section 8.1).
func TestHub_PersonalEventsOnlyToAudience(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	alex, sam := person(), person()
	w.instanceWide[sam.ID] = true
	h := newHub(t, w, hubCfg)
	a1 := connect(t, h, alex, 10, 0, 0)
	a2 := connect(t, h, alex, 10, 0, 0)
	s1 := connect(t, h, sam, 10, 0, 0)
	h.Dispatch(context.Background(), personal(1, alex.ID))
	for name, qc := range map[string]*qconn{"first of Alex": a1, "second of Alex": a2} {
		if got := dispatched(framesOf(t, qc.drain())); !slices.Equal(got, []int64{1}) {
			t.Errorf("%s got %v", name, got)
		}
	}
	if got := dispatched(framesOf(t, s1.drain())); len(got) != 0 {
		t.Errorf("Sam (instance-wide) got Alex's personal event: %v", got)
	}
}

// TestHub_CandidatesSupersetOfAllowed is a property test in random worlds: the connections the
// hub delivers an event to are exactly those Admit allows by brute force over every connection,
// also after new channels appear without access.changed and after grants change with
// access.changed (02 "Hub and delivery": the candidate set is a superset of the allowed one).
func TestHub_CandidatesSupersetOfAllowed(t *testing.T) {
	t.Parallel()
	for round := range 30 {
		seed := uint64(round)*7919 + 17
		t.Run("", func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // G404: a seeded, printed generator makes a failing world reproducible.
			defer func() {
				if t.Failed() {
					t.Logf("seed %d", seed)
				}
			}()
			candidatesWorld(t, r)
		})
	}
}

// candidatesWorld runs one random world of TestHub_CandidatesSupersetOfAllowed.
func candidatesWorld(t *testing.T, r *rand.Rand) {
	w := newHubWorld()
	spaces := make([]ids.ID, 1+r.IntN(3))
	for i := range spaces {
		spaces[i] = ids.New()
	}
	var channels []ids.ID
	for range 2 + r.IntN(6) {
		channels = append(channels, w.channel(spaces[r.IntN(len(spaces))]))
	}
	people := make([]actor.Actor, 2+r.IntN(4))
	for i := range people {
		people[i] = person()
		p := people[i].ID
		switch r.IntN(4) {
		case 0:
			w.instanceWide[p] = true
		case 1:
			w.grantSpace(p, spaces[r.IntN(len(spaces))])
		}
		for _, c := range channels {
			if r.IntN(3) == 0 {
				w.grantChannel(p, c)
			}
		}
	}
	h := newHub(t, w, app.HubConfig{QueueMax: 10000, PageSize: 4, CursorEvery: 1 << 30, RetryBase: time.Millisecond, RetryCap: time.Millisecond, PayloadMax: 65536})
	type conn struct {
		a  actor.Actor
		qc *qconn
	}
	var conns []conn
	for _, a := range people {
		for range 1 + r.IntN(2) {
			qc := connect(t, h, a, 10000, 0, 0)
			conns = append(conns, conn{a, qc})
		}
	}
	for seq := int64(1); seq <= 200; seq++ {
		var ev sync.Event
		switch r.IntN(10) {
		case 0:
			ev = personal(seq, people[r.IntN(len(people))].ID)
		case 1:
			ev = msg(seq, ids.ID{}, ids.ID{})
		case 2:
			// A new channel in a space, without access.changed.
			c := w.channel(spaces[r.IntN(len(spaces))])
			channels = append(channels, c)
			ev = msg(seq, w.spaceOf[c], c)
		case 3:
			// A grant change with access.changed for its principal.
			p, c := people[r.IntN(len(people))].ID, channels[r.IntN(len(channels))]
			var removed []ids.ID
			if r.IntN(2) == 0 {
				w.grantChannel(p, c)
			} else {
				w.revokeChannel(p, c)
				removed = append(removed, c)
			}
			ev = accessChanged(seq, p, removed...)
		default:
			c := channels[r.IntN(len(channels))]
			ev = msg(seq, w.spaceOf[c], c)
		}
		var want []int
		for i, c := range conns {
			if ok, _ := w.Admit(context.Background(), c.a, ev); ok {
				want = append(want, i)
			}
		}
		before := make([]int, len(conns))
		for i, c := range conns {
			before[i] = c.qc.dispatches()
		}
		h.Dispatch(context.Background(), ev)
		var got []int
		for i, c := range conns {
			if c.qc.dispatches() > before[i] {
				got = append(got, i)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("seq %d (%s): hub delivered to %v, admission allows %v", seq, ev.Type, got, want)
		}
	}
}

// TestHub_UnauthenticatedDecisionCloses4012: an admission that answers unauthenticated means the
// credential was revoked; the connection closes with 4012.
func TestHub_UnauthenticatedDecisionCloses4012(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	support := ids.New()
	allgemein := w.channel(support)
	alex := person()
	w.grantChannel(alex.ID, allgemein)
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 10, 0, 0)
	w.mu.Lock()
	w.revoked[alex.ID] = true
	w.mu.Unlock()
	h.Dispatch(context.Background(), msg(1, support, allgemein))
	if code := qc.closedWith(); code != domain.CloseRevoked {
		t.Fatalf("closed with %d, want 4012", code)
	}
}

// TestHub_ForbiddenConnectAtRevalidationCloses: when access.changed revalidates a connection and
// gateway.connect is now forbidden, the connection closes like at identify: 4016 for a pending
// device, 4018 for any other forbidden.
func TestHub_ForbiddenConnectAtRevalidationCloses(t *testing.T) {
	t.Parallel()
	for reason, want := range map[authorize.Reason]domain.CloseCode{"device_pending": domain.CloseDevicePending, "subject_kind_not_allowed": domain.CloseForbidden} {
		w := newHubWorld()
		support := ids.New()
		allgemein := w.channel(support)
		alex := person()
		w.grantChannel(alex.ID, allgemein)
		h := newHub(t, w, hubCfg)
		qc := connect(t, h, alex, 10, 0, 0)
		w.mu.Lock()
		w.connect = map[ids.ID]authorize.Decision{alex.ID: {Outcome: authorize.OutcomeForbidden, Reason: reason}}
		w.mu.Unlock()
		h.Dispatch(context.Background(), accessChanged(1, alex.ID))
		if code := qc.closedWith(); code != want {
			t.Fatalf("%s: closed with %d, want %d", reason, code, want)
		}
	}
}

// TestHub_LiveCursorFrameAfterAdvanceEvery: every sync.cursor_advance_every processed events a
// live connection whose last frame with a seq is older gets cursor {seq} (04 section 7.1).
func TestHub_LiveCursorFrameAfterAdvanceEvery(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	support := ids.New()
	allgemein, eskalation := w.channel(support), w.channel(support)
	alex := person()
	w.grantChannel(alex.ID, allgemein)
	h := newHub(t, w, hubCfg) // CursorEvery 3
	qc := connect(t, h, alex, 100, 10, 10)
	for seq := int64(11); seq <= 16; seq++ {
		ch := eskalation
		if seq == 11 {
			ch = allgemein
		}
		h.Dispatch(context.Background(), msg(seq, support, ch))
	}
	got := framesOf(t, qc.drain())
	want := []frame{{"dispatch", 11}, {"cursor", 13}, {"cursor", 16}}
	if !slices.Equal(got, want) {
		t.Fatalf("frames %v, want %v", got, want)
	}
}

// TestHub_AccessChangedPurgesRemovedTargets: frames of a removed channel queued before
// access.changed (the writer held) are not written; access.changed itself is, then nothing more of
// the channel, while another channel keeps arriving (04 section 8.3 items 1 and 2).
func TestHub_AccessChangedPurgesRemovedTargets(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	support, marketing := ids.New(), ids.New()
	eskalation, kampagnen := w.channel(support), w.channel(marketing)
	sam := person()
	w.grantChannel(sam.ID, eskalation)
	w.grantChannel(sam.ID, kampagnen)
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, sam, 100, 0, 0)
	h.Dispatch(context.Background(), msg(1, support, eskalation))
	h.Dispatch(context.Background(), msg(2, marketing, kampagnen))
	w.revokeChannel(sam.ID, eskalation)
	h.Dispatch(context.Background(), accessChanged(3, sam.ID, eskalation))
	h.Dispatch(context.Background(), msg(4, support, eskalation))
	h.Dispatch(context.Background(), msg(5, marketing, kampagnen))
	if got := dispatched(framesOf(t, qc.drain())); !slices.Equal(got, []int64{2, 3, 5}) {
		t.Fatalf("written %v, want [2 3 5]", got)
	}
}

// TestHub_SessionRevokedCloses4012: session.revoked closes the connections of that session with
// 4012 and reaches the person's other connections.
func TestHub_SessionRevokedCloses4012(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	h := newHub(t, w, hubCfg)
	phone := person()
	laptop := phone
	laptop.SessionID, laptop.DeviceID = ids.New(), ids.New()
	qPhone := connect(t, h, phone, 10, 0, 0)
	qLaptop := connect(t, h, laptop, 10, 0, 0)
	ev := personal(1, phone.ID)
	ev.Type, ev.ResourceKind, ev.ResourceID = "session.revoked", "auth_session", phone.SessionID
	ev.Payload = []byte(`{"session_id":"` + phone.SessionID.String() + `","reason":"admin"}`)
	h.Dispatch(context.Background(), ev)
	if code := qPhone.closedWith(); code != domain.CloseRevoked {
		t.Fatalf("revoked session closed with %d, want 4012", code)
	}
	if code := qLaptop.closedWith(); code != 0 {
		t.Fatalf("other session closed with %d", code)
	}
	if got := dispatched(framesOf(t, qLaptop.drain())); !slices.Equal(got, []int64{1}) {
		t.Fatalf("other session got %v", got)
	}
}

// TestHub_AdmitErrorClosesResumable: an admission check that fails (database briefly gone) neither
// drops the event silently nor lets a later frame move the client's cursor past it; the
// connection closes with 4000, and the client resumes from its own cursor (04 section 9 item 1).
func TestHub_AdmitErrorClosesResumable(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	space := ids.New()
	ch := w.channel(space)
	alex := person()
	w.grantChannel(alex.ID, ch)
	w.admitFail = map[int64]bool{11: true}
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 10, 10)
	for seq := int64(11); seq <= 13; seq++ {
		h.Dispatch(context.Background(), msg(seq, space, ch))
	}
	got := framesOf(t, qc.drain())
	if code := qc.closedWith(); code != domain.CloseUnknownError || len(got) != 0 {
		t.Fatalf("close %d, frames %v; want 4000 and no frame after the failed check", code, got)
	}
}

// TestHub_OneStructureScopePerEvent: the checks of every connection for one live event share one
// structure memo, and the next event gets a new one, so the thread and channel of an event are
// read once per event instead of once per connection (perf.fanout_local_p99, M1-72).
func TestHub_OneStructureScopePerEvent(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	support := ids.New()
	allgemein := w.channel(support)
	alex, sam := person(), person()
	w.grantChannel(alex.ID, allgemein)
	w.grantChannel(sam.ID, allgemein)
	h := newHub(t, w, hubCfg)
	connect(t, h, alex, 100, 0, 0)
	connect(t, h, sam, 100, 0, 0)
	h.Dispatch(context.Background(), msg(1, support, allgemein))
	h.Dispatch(context.Background(), msg(2, support, allgemein))
	w.mu.Lock()
	defer w.mu.Unlock()
	first, second := w.memos[1], w.memos[2]
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("structure memos per event: %d and %d, want one per connection", len(first), len(second))
	}
	if first[0] != first[1] || second[0] != second[1] {
		t.Error("the connections of one event did not share the structure memo")
	}
	if first[0] == second[0] {
		t.Error("two events shared a structure memo")
	}
}
