// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
)

// River fetches a marker job (notify.coalesce_window, notify.badge_sync_hold) and its companion
// (notify.coalesce_flush, the deferred notify.badge_sync) in the same pass when both are due at the
// same time, and does not promise that the marker is completed first. These helpers run the
// companions while the markers of the same moment still stand, then complete the markers.

// flushDueMarkersStanding runs every due notify.coalesce_flush while the due window markers still
// stand, then completes those markers.
func (w *world) flushDueMarkersStanding() {
	w.t.Helper()
	now := w.clk.Now()
	markers := w.pendingOf(notifyjobs.KindCoalesceWindow)
	for _, j := range w.pendingOf(notifyjobs.KindCoalesceFlush) {
		if j.ScheduledAt.After(now) {
			continue
		}
		var a notifyjobs.CoalesceFlush
		must(w.t, json.Unmarshal(j.Args, &a))
		must(w.t, w.push.Push.Flush(context.Background(), a.UserID, a.ThreadID, time.UnixMicro(a.EndUnixMicro).UTC()))
		w.complete(j.ID)
	}
	for _, j := range markers {
		if !j.ScheduledAt.After(now) {
			w.complete(j.ID)
		}
	}
}

// badgeSyncsDueHoldsStanding runs every due notify.badge_sync while the due holds still stand,
// then completes those holds.
func (w *world) badgeSyncsDueHoldsStanding() {
	w.t.Helper()
	now := w.clk.Now()
	holds := w.pendingOf(notifyjobs.KindBadgeSyncHold)
	for _, j := range w.pendingOf(notifyjobs.KindBadgeSync) {
		if j.ScheduledAt.After(now) {
			continue
		}
		var a notifyjobs.BadgeSync
		must(w.t, json.Unmarshal(j.Args, &a))
		must(w.t, w.push.Push.BadgeSync(context.Background(), a.UserID, a.AfterSeq))
		w.complete(j.ID)
	}
	for _, j := range holds {
		if !j.ScheduledAt.After(now) {
			w.complete(j.ID)
		}
	}
}

// TestFlushWithStandingMarker_OpensNextWindow (13 4.5 point 5): the flush at the window end opens
// the next window even while the marker of the ending window still stands, so a message shortly
// after the flush is collected and not sent at once.
func TestFlushWithStandingMarker_OpensNextWindow(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	w.setLevel("alex", "thread", th, "all")
	t0 := w.clk.Now()
	w.post("owner", th, "Update 1")
	w.consume()
	w.clk.Set(t0.Add(5 * time.Second))
	w.post("owner", th, "Update 2")
	w.consume()
	if n := len(w.hintsFor(w.alex)); n != 1 {
		t.Fatalf("Alex has %d hints inside the first window, want 1", n)
	}
	end := t0.Add(60 * time.Second) // notify.coalesce_window of pushEnv
	w.clk.Set(end)
	w.flushDueMarkersStanding()
	if n := len(w.hintsFor(w.alex)); n != 2 {
		t.Fatalf("Alex has %d hints after the flush, want 2", n)
	}
	next := w.windowsOf(notifyjobs.KindCoalesceWindow, w.alex, th)
	var open bool
	for _, j := range next {
		if j.pending() && j.ScheduledAt.Equal(end.Add(60*time.Second)) {
			open = true
		}
	}
	if !open {
		t.Fatalf("no window marker until %s after the flush: %+v", end.Add(60*time.Second), next)
	}
	w.clk.Set(end.Add(5 * time.Second))
	w.post("owner", th, "Update 3")
	w.consume()
	if n := len(w.hintsFor(w.alex)); n != 2 {
		t.Fatalf("Alex has %d hints after a message in the next window, want 2 (collected)", n)
	}
}

// threadAt is a thread and the thread_seq of its message.
type threadAt struct {
	th  ids.ID
	seq int64
}

// TestBadgeSyncWithStandingHold_KeepsInterval (13 8): a deferred badge sync that runs while the
// hold of the previous interval still stands inserts the next hold, so a further falling read
// inside the new interval waits for its end.
func TestBadgeSyncWithStandingHold_KeepsInterval(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var threads []threadAt
	for range 3 {
		th := w.newThread(w.allgemein)
		w.setLevel("alex", "thread", th, "all")
		m := w.post("owner", th, "Export numbers are in")
		threads = append(threads, threadAt{th: th, seq: m.ThreadSeq})
	}
	w.consume()
	t0 := w.clk.Now()
	w.readUpTo("alex", threads[0].th, threads[0].seq)
	w.consume()
	w.badgeSyncsDue()
	w.clk.Set(t0.Add(10 * time.Second))
	w.readUpTo("alex", threads[1].th, threads[1].seq)
	w.consume()
	w.clk.Set(t0.Add(badgeSyncInterval))
	w.badgeSyncsDueHoldsStanding()
	if n := len(w.syncJobs(w.alex)); n != 2 {
		t.Fatalf("%d sync pushes after the deferred sync, want 2", n)
	}
	w.clk.Set(t0.Add(badgeSyncInterval + 10*time.Second))
	w.readUpTo("alex", threads[2].th, threads[2].seq)
	w.consume()
	want := t0.Add(2 * badgeSyncInterval)
	syncs := w.pendingOf(notifyjobs.KindBadgeSync)
	if len(syncs) != 1 || !syncs[0].ScheduledAt.Equal(want) {
		t.Fatalf("badge syncs %+v, want one at the end of the second interval %s", syncs, want)
	}
}

// TestBadgeSyncRunning_KeepsLaterFallingRead (13 8): a falling read that arrives while the due
// badge sync already runs (it has read its events) schedules a follow-up sync after the interval,
// so the later read reaches the phone in Cleared and is not dropped.
func TestBadgeSyncRunning_KeepsLaterFallingRead(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var threads []threadAt
	for range 2 {
		th := w.newThread(w.allgemein)
		w.setLevel("alex", "thread", th, "all")
		m := w.post("owner", th, "Export numbers are in")
		threads = append(threads, threadAt{th: th, seq: m.ThreadSeq})
	}
	w.consume()
	t0 := w.clk.Now()
	w.readUpTo("alex", threads[0].th, threads[0].seq)
	w.consume()
	running := w.pendingOf(notifyjobs.KindBadgeSync)
	if len(running) != 1 {
		t.Fatalf("badge syncs %+v, want one", running)
	}
	// The sync runs its use case; River has not completed the job yet when the next read arrives.
	var a notifyjobs.BadgeSync
	must(t, json.Unmarshal(running[0].Args, &a))
	must(t, w.push.Push.BadgeSync(context.Background(), a.UserID, a.AfterSeq))
	w.clk.Set(t0.Add(time.Second))
	w.readUpTo("alex", threads[1].th, threads[1].seq)
	w.consume()
	w.complete(running[0].ID)
	// The badge is counted at send time, after both reads.
	w.wantSyncCall(w.apns, w.alex, w.alexPhone.Registration, 0, 0, threads[0].th)
	follow := w.pendingOf(notifyjobs.KindBadgeSync)
	if len(follow) != 1 || follow[0].ScheduledAt.Before(t0.Add(badgeSyncInterval)) {
		t.Fatalf("badge syncs %+v, want one follow-up not before %s", follow, t0.Add(badgeSyncInterval))
	}
	w.clk.Set(follow[0].ScheduledAt)
	w.badgeSyncsDue()
	w.wantSyncCall(w.apns, w.alex, w.alexPhone.Registration, 1, 0, threads[1].th)
}
