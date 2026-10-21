// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// seqs returns the seq of every entry left in q, in order.
func seqs(q *domain.Queue) []int64 {
	var out []int64
	for {
		e, ok := q.Pop()
		if !ok {
			return out
		}
		out = append(out, e.Seq)
	}
}

// TestQueue_BoundedAndPurgeByScope: the queue holds at most its bound (sync.outbound_queue_max)
// and Purge drops the dispatch entries of a removed channel or space, keeping the order of the
// others (04 section 8.3 item 2).
func TestQueue_BoundedAndPurgeByScope(t *testing.T) {
	t.Parallel()
	support, marketing := ids.New(), ids.New()
	eskalation, kampagnen := ids.New(), ids.New()
	q := domain.NewQueue(4)
	for i, e := range []domain.Entry{
		{Class: domain.ClassDispatch, Seq: 1, SpaceID: support, ChannelID: eskalation},
		{Class: domain.ClassDispatch, Seq: 2, SpaceID: marketing, ChannelID: kampagnen},
		{Class: domain.ClassDispatch, Seq: 3, SpaceID: support},
		{Class: domain.ClassDispatch, Seq: 4},
	} {
		if !q.Push(e) {
			t.Fatalf("entry %d refused below the bound", i)
		}
	}
	if q.Push(domain.Entry{Class: domain.ClassDispatch, Seq: 5}) || q.Len() != 4 {
		t.Fatalf("the queue grew past its bound: %d", q.Len())
	}
	if n := q.Purge(domain.NewTargets(nil, []ids.ID{eskalation})); n != 1 {
		t.Fatalf("purged %d entries of the removed channel, want 1", n)
	}
	if n := q.Purge(domain.NewTargets([]ids.ID{support}, nil)); n != 1 {
		t.Fatalf("purged %d entries of the removed space, want 1", n)
	}
	if got := seqs(q); !slices.Equal(got, []int64{2, 4}) {
		t.Fatalf("left %v, want [2 4]", got)
	}
}

// TestQueue_ControlFramesSurvivePurge: control frames (cursor, catchup.complete, error,
// heartbeat_ack) are never purged, even with the scope of a removed target.
func TestQueue_ControlFramesSurvivePurge(t *testing.T) {
	t.Parallel()
	support := ids.New()
	q := domain.NewQueue(3)
	q.Push(domain.Entry{Class: domain.ClassControl, Frame: []byte(`{"op":"heartbeat_ack"}`)})
	q.Push(domain.Entry{Class: domain.ClassDispatch, Seq: 7, SpaceID: support})
	q.Push(domain.Entry{Class: domain.ClassControl, Seq: 7, SpaceID: support, Frame: []byte(`{"op":"cursor"}`)})
	q.Purge(domain.NewTargets([]ids.ID{support}, nil))
	var ops []string
	for {
		e, ok := q.Pop()
		if !ok {
			break
		}
		if e.Class != domain.ClassControl {
			t.Fatalf("a dispatch entry survived: %+v", e)
		}
		ops = append(ops, string(e.Frame))
	}
	if len(ops) != 2 {
		t.Fatalf("control frames %v", ops)
	}
}

// TestPageSteps: a cursor frame whenever sync.cursor_advance_every sequence numbers were skipped
// since the last frame with a seq, and one at the end of the page with its last checked seq (04
// section 7.1, R-SY-03).
func TestPageSteps(t *testing.T) {
	t.Parallel()
	got := domain.PageSteps(10, []int64{12, 20}, 26, 3)
	want := []domain.Step{
		{Seq: 12, Dispatch: true}, {Seq: 15}, {Seq: 18}, {Seq: 20, Dispatch: true}, {Seq: 23}, {Seq: 26},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("steps %v, want %v", got, want)
	}
	if got := domain.PageSteps(10, []int64{11}, 11, 3); !slices.Equal(got, []domain.Step{{Seq: 11, Dispatch: true}, {Seq: 11}}) {
		t.Fatalf("a page ending on a dispatched event: %v", got)
	}
}

// TestQueue_RoomLeavesControlReserve: Roomy holds back ControlReserve entries for control frames
// (at most all but one of a very small queue); Push still accepts up to the bound.
func TestQueue_RoomLeavesControlReserve(t *testing.T) {
	t.Parallel()
	if domain.ControlReserve != 4 {
		t.Fatalf("ControlReserve = %d, want 4", domain.ControlReserve)
	}
	q := domain.NewQueue(domain.ControlReserve + 2)
	for range 2 {
		if !q.Roomy() || !q.Push(domain.Entry{Class: domain.ClassDispatch}) {
			t.Fatal("no room below the reserve")
		}
	}
	if q.Roomy() {
		t.Fatal("room inside the control reserve")
	}
	for range domain.ControlReserve {
		if !q.Push(domain.Entry{Class: domain.ClassControl}) {
			t.Fatal("the reserve does not take control frames")
		}
	}
	small := domain.NewQueue(2)
	if !small.Roomy() || !small.Push(domain.Entry{}) || small.Roomy() {
		t.Fatal("a queue of two keeps one slot for catch-up and one in reserve")
	}
}

// TestQueue_PurgeOnlyBeforeTheRemoval: a removal bound to its access.changed at seq 5 drops the
// queued dispatches of the target before seq 5 and keeps the later ones, which were admitted with
// newer rights (M1-65/15).
func TestQueue_PurgeOnlyBeforeTheRemoval(t *testing.T) {
	t.Parallel()
	support := ids.New()
	q := domain.NewQueue(4)
	for _, seq := range []int64{3, 4, 6, 7} {
		q.Push(domain.Entry{Class: domain.ClassDispatch, Seq: seq, SpaceID: support})
	}
	if n := q.Purge(domain.NewTargets([]ids.ID{support}, nil).Before(5)); n != 2 {
		t.Fatalf("purged %d, want 2", n)
	}
	if got := seqs(q); !slices.Equal(got, []int64{6, 7}) {
		t.Fatalf("left %v, want [6 7]", got)
	}
}
