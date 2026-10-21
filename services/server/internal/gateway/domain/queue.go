// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Class is the kind of an outbound frame: a dispatch of a durable event, which access removal can
// purge, or a control frame (cursor, catchup.complete, error, heartbeat_ack and the frames of
// connection setup), which is never purged.
type Class int

// The classes of outbound frames.
const (
	// ClassControl is every frame that is not a dispatch.
	ClassControl Class = iota
	// ClassDispatch is a dispatch frame of a durable event.
	ClassDispatch
)

// Entry is one frame of the outbound queue of a connection: the encoded frame and, for a
// dispatch, the seq and scope of its event, so access removal can purge it (04 section 8.3 item 2).
type Entry struct {
	// Frame is the encoded JSON text frame.
	Frame []byte
	// Class tells dispatch from control frames.
	Class Class
	// Seq is the seq of the event (dispatch) or of the cursor frame; 0 for none.
	Seq int64
	// SpaceID and ChannelID are the scope of the event; zero where it has none.
	SpaceID, ChannelID ids.ID
	// OccurredAt is the time of the event.
	OccurredAt time.Time
	// Live marks a dispatch of live delivery (phase live, durable or ephemeral): the writer
	// measures bothy.delivery.duration for these entries only, not for catch-up and its flush.
	Live bool
	// Ephemeral marks a dispatch of an ephemeral event.
	Ephemeral bool
}

// Targets is a set of removed targets of access.changed: spaces and channels, and the seq of
// that access.changed. A removal applies only to the events before it: an event after it was
// admitted with the rights of its commit or later (M1-65/15).
type Targets struct {
	spaces, channels map[ids.ID]bool
	before           int64
}

// NewTargets returns the set of the removed spaces and channels.
func NewTargets(spaces, channels []ids.ID) Targets {
	t := Targets{spaces: map[ids.ID]bool{}, channels: map[ids.ID]bool{}}
	t.Add(spaces, channels)
	return t
}

// Add adds spaces and channels to t, which must come from NewTargets.
func (t Targets) Add(spaces, channels []ids.ID) {
	for _, s := range spaces {
		t.spaces[s] = true
	}
	for _, c := range channels {
		t.channels[c] = true
	}
}

// Before returns t bound to the events before seq, the seq of its access.changed; 0 keeps every
// event covered.
func (t Targets) Before(seq int64) Targets {
	t.before = seq
	return t
}

// Empty reports whether t names no target.
func (t Targets) Empty() bool { return len(t.spaces) == 0 && len(t.channels) == 0 }

// Covers reports whether a dispatch of seq with the scope space and channel belongs to a removed
// target: it lies before the removal, and its channel was removed or the space it lies in.
func (t Targets) Covers(space, channel ids.ID, seq int64) bool {
	if t.before > 0 && seq >= t.before {
		return false
	}
	return !channel.IsZero() && t.channels[channel] || !space.IsZero() && t.spaces[space]
}

// Queue is the bounded outbound queue of one connection (sync.outbound_queue_max frames, 02
// "Hub and delivery"). It is not safe for concurrent use; the adapter that owns it locks it.
type Queue struct {
	max   int
	items []Entry
}

// NewQueue returns an empty queue of at most maxFrames entries.
func NewQueue(maxFrames int) *Queue {
	return &Queue{max: maxFrames}
}

// Push appends e and reports false, without appending, when the queue is full.
func (q *Queue) Push(e Entry) bool {
	if len(q.items) >= q.max {
		return false
	}
	q.items = append(q.items, e)
	return true
}

// Pop removes and returns the oldest entry; false when the queue is empty.
func (q *Queue) Pop() (Entry, bool) {
	if len(q.items) == 0 {
		return Entry{}, false
	}
	e := q.items[0]
	q.items[0] = Entry{}
	q.items = q.items[1:]
	return e, true
}

// Len returns the number of entries.
func (q *Queue) Len() int { return len(q.items) }

// Full reports whether a Push would be refused.
func (q *Queue) Full() bool { return len(q.items) >= q.max }

// ControlReserve is the number of entries the catch-up runner leaves free below
// sync.outbound_queue_max for the control frames a client's ops cause while it catches up
// (heartbeat_ack, reauth_required, error, and one spare); without it a heartbeat during a long
// catch-up would overflow the queue and close with 4014. TestQueue_RoomLeavesControlReserve holds
// the value.
const ControlReserve = 4

// Roomy reports whether the catch-up runner may queue another frame: the queue holds fewer than
// its bound minus ControlReserve entries; a queue of at most ControlReserve entries keeps one
// entry for the catch-up.
func (q *Queue) Roomy() bool { return len(q.items) < q.max-min(ControlReserve, q.max-1) }

// Purge drops every dispatch entry whose scope t covers and returns how many it dropped; control
// frames stay, and the others keep their order.
func (q *Queue) Purge(t Targets) int {
	kept := q.items[:0]
	n := 0
	for _, e := range q.items {
		if e.Class == ClassDispatch && t.Covers(e.SpaceID, e.ChannelID, e.Seq) {
			n++
			continue
		}
		kept = append(kept, e)
	}
	clear(q.items[len(kept):])
	q.items = kept
	return n
}

// Clear drops every entry (a connection that is closing).
func (q *Queue) Clear() {
	clear(q.items)
	q.items = q.items[:0]
}

// Step is one frame of a catch-up page: a dispatch of the admitted event Seq, or a cursor frame
// with Seq.
type Step struct {
	Seq      int64
	Dispatch bool
}

// PageSteps returns the frames of one catch-up page (04 sections 7.1 and 7.2, R-SY-03): the
// admitted seqs in order, a cursor frame each time every sequence numbers were skipped since the
// last frame that carried a seq (starting from last), and a cursor frame with end, the highest seq
// the page checked. every is sync.cursor_advance_every and at least 1.
func PageSteps(last int64, admitted []int64, end, every int64) []Step {
	var out []Step
	for _, s := range admitted {
		for s-last-1 >= every {
			last += every
			out = append(out, Step{Seq: last})
		}
		out = append(out, Step{Seq: s, Dispatch: true})
		last = s
	}
	for end-last > every {
		last += every
		out = append(out, Step{Seq: last})
	}
	return append(out, Step{Seq: end})
}
