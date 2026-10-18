// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// WindowConfig holds the configuration keys of the delivery window of an agent consumer.
type WindowConfig struct {
	// MaxInflight is agent.max_inflight_events: the most unacknowledged dispatched events.
	MaxInflight int
	// AckTimeout is agent.ack_timeout: the time an inflight event may stay unacknowledged before
	// the window delivers again from acked + 1.
	AckTimeout time.Duration
	// CursorEvery is sync.cursor_advance_every: in the live phase a cursor frame follows once that
	// many numbers were passed without a frame.
	CursorEvery int64
}

// FrameKind is the kind of a control frame the window asks for after a page.
type FrameKind int

// The control frames of a consumer window.
const (
	// FrameCursor is cursor {seq}.
	FrameCursor FrameKind = iota + 1
	// FrameComplete is catchup.complete {head_seq}.
	FrameComplete
)

// WindowFrame is one control frame of a consumer window: its kind and its seq (the head_seq of
// catchup.complete).
type WindowFrame struct {
	Kind FrameKind
	Seq  int64
}

// inflight is one dispatched durable event that is not acknowledged yet: its seq, its scope (for
// the purge of a removed target) and the time it was queued.
type inflight struct {
	seq            int64
	space, channel ids.ID
	queued         time.Time
}

// ConsumerWindow is the delivery window of one agent consumer connection or REST window (brief
// M2-07 note 5; 04 sections 15.1 and 15.2). It is pure: no I/O, the time is passed in, and it is
// not safe for concurrent use (the runner that owns it locks it).
//
// The runner reads pages after Read and, for each admitted event while Budget is above zero,
// calls Dispatch; when the page was not cut it calls Checked with the highest seq the page
// checked; then Frames returns the cursor and catchup.complete frames to send. Ack applies an
// acknowledgement, Expire the ack timeout, Purge the removal of targets and Restart a resync.
type ConsumerWindow struct {
	cfg WindowConfig

	acked, read   int64
	entries       []inflight
	redeliverUpTo int64
	highestSent   int64
	// readyHead is the head_seq of ready or resumed; catchup.complete names it once read reaches it.
	readyHead int64
	live      bool
	// lastFrame is the seq of the last frame that carried one (dispatch or cursor).
	lastFrame int64
}

// NewConsumerWindow returns a window in the backlog phase that starts after acked (the stored or
// held acknowledgement) and ends its catch-up at readyHead.
//
// Example:
//
//	w := domain.NewConsumerWindow(cfg, state.AckedSeq, ready.HeadSeq)
func NewConsumerWindow(cfg WindowConfig, acked, readyHead int64) *ConsumerWindow {
	return &ConsumerWindow{cfg: cfg, acked: acked, read: acked, readyHead: readyHead, lastFrame: acked}
}

// ConsumerResync reports whether the reason of resync_required restarts a consumer at the head:
// retention (the events after the acknowledgement were removed) and cursor_ahead (only after a
// restore without reset). too_far_behind does not apply, because the window bounds the backlog,
// and epoch_changed cannot occur, because the client epoch is ignored (brief M2-07 note 5).
func ConsumerResync(reason string) bool {
	return reason == "retention" || reason == "cursor_ahead"
}

// Acked returns the highest acknowledged seq.
func (w *ConsumerWindow) Acked() int64 { return w.acked }

// Read returns the highest seq this window checked; the next page starts after it.
func (w *ConsumerWindow) Read() int64 { return w.read }

// Live reports whether the window has left the backlog phase.
func (w *ConsumerWindow) Live() bool { return w.live }

// Inflight returns the number of dispatched unacknowledged events.
func (w *ConsumerWindow) Inflight() int { return len(w.entries) }

// Budget returns how many more events may be dispatched: agent.max_inflight_events minus the
// inflight ones, never below 0.
func (w *ConsumerWindow) Budget() int { return max(w.cfg.MaxInflight-len(w.entries), 0) }

// Again reports whether a dispatch of seq now would be a redelivery after an ack timeout, so the
// runner can build the envelope with the delivery field before it calls Dispatch.
func (w *ConsumerWindow) Again(seq int64) bool { return seq <= w.redeliverUpTo }

// Dispatch records the dispatch of the event seq with its scope at now and reports whether it is
// a redelivery after an ack timeout (the delivery field redelivered). read becomes seq. The caller
// checks Budget first.
func (w *ConsumerWindow) Dispatch(seq int64, space, channel ids.ID, now time.Time) bool {
	again := w.Again(seq)
	w.entries = append(w.entries, inflight{seq: seq, space: space, channel: channel, queued: now})
	w.highestSent = max(w.highestSent, seq)
	w.read = max(w.read, seq)
	w.lastFrame = max(w.lastFrame, seq)
	return again
}

// Checked raises read to seq, the highest seq a page checked or an event that was checked but not
// delivered.
func (w *ConsumerWindow) Checked(seq int64) { w.read = max(w.read, seq) }

// Ack applies the acknowledgement seq and reports whether it advanced: every inflight entry up to
// seq is dropped and read is raised to seq. A value at or below the acknowledgement changes
// nothing (cumulative and monotone, 04 section 15.2 item 1).
func (w *ConsumerWindow) Ack(seq int64) bool {
	if seq <= w.acked {
		return false
	}
	w.acked = seq
	kept := w.entries[:0]
	for _, e := range w.entries {
		if e.seq > seq {
			kept = append(kept, e)
		}
	}
	clear(w.entries[len(kept):])
	w.entries = kept
	w.read = max(w.read, seq)
	w.lastFrame = max(w.lastFrame, seq)
	return true
}

// OldestInflight returns the lowest seq that is dispatched and not acknowledged; false when
// nothing is inflight.
func (w *ConsumerWindow) OldestInflight() (int64, bool) {
	if len(w.entries) == 0 {
		return 0, false
	}
	low := w.entries[0].seq
	for _, e := range w.entries[1:] {
		low = min(low, e.seq)
	}
	return low, true
}

// Deadline returns the instant the oldest inflight entry times out (its queue time plus
// agent.ack_timeout); false when nothing is inflight.
func (w *ConsumerWindow) Deadline() (time.Time, bool) {
	if len(w.entries) == 0 {
		return time.Time{}, false
	}
	oldest := w.entries[0].queued
	for _, e := range w.entries[1:] {
		if e.queued.Before(oldest) {
			oldest = e.queued
		}
	}
	return oldest.Add(w.cfg.AckTimeout), true
}

// Expire applies the ack timeout at now and reports whether it fired: every number dispatched so
// far is marked for redelivery, inflight is emptied and reading restarts after acked (04 section
// 15.2 item 3).
func (w *ConsumerWindow) Expire(now time.Time) bool {
	d, ok := w.Deadline()
	if !ok || now.Before(d) {
		return false
	}
	w.redeliverUpTo = max(w.redeliverUpTo, w.highestSent)
	clear(w.entries)
	w.entries = w.entries[:0]
	w.read, w.lastFrame = w.acked, w.acked
	return true
}

// Purge drops the inflight entries whose scope t covers, so they hold no budget, and returns how
// many it dropped (the queue purge of access.changed, 04 section 8.3 item 2).
func (w *ConsumerWindow) Purge(t Targets) int {
	kept := w.entries[:0]
	n := 0
	for _, e := range w.entries {
		if t.Covers(e.space, e.channel, e.seq) {
			n++
			continue
		}
		kept = append(kept, e)
	}
	clear(w.entries[len(kept):])
	w.entries = kept
	return n
}

// Restart moves the window to seq after resync_required: acknowledged and read up to seq, nothing
// inflight, live without catchup.complete.
func (w *ConsumerWindow) Restart(seq int64) {
	clear(w.entries)
	w.entries = w.entries[:0]
	w.acked, w.read, w.lastFrame, w.live = seq, seq, seq, true
}

// Frames returns the control frames due after a page, in order: in the backlog phase a cursor
// {read} when read is above the last seq a frame carried, then catchup.complete {readyHead} once
// read reaches readyHead, which starts the live phase; in the live phase a cursor {read} once
// sync.cursor_advance_every numbers were passed without a frame (04 sections 7.1 and 7.2, R-SY-03).
func (w *ConsumerWindow) Frames() []WindowFrame {
	var out []WindowFrame
	switch {
	case !w.live && w.read > w.lastFrame:
		out = append(out, WindowFrame{Kind: FrameCursor, Seq: w.read})
		w.lastFrame = w.read
	case w.live && w.read-w.lastFrame >= max(w.cfg.CursorEvery, 1):
		out = append(out, WindowFrame{Kind: FrameCursor, Seq: w.read})
		w.lastFrame = w.read
	}
	if !w.live && w.read >= w.readyHead {
		out = append(out, WindowFrame{Kind: FrameComplete, Seq: w.readyHead})
		w.live = true
	}
	return out
}
