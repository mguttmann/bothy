// SPDX-License-Identifier: Apache-2.0

package app_test

// Tests of the focus member background_thread_ids (M2-51; 04 6.2, 11.3, 13 4.4, R-SY-27): a
// background thread gets the run document once and run.updated like a focused thread, no
// typing.started, never enters the presence focus, and both lists count against
// sync.max_focused_threads.

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// focusBoth sends focus with thread_ids and background_thread_ids.
func (e *fenv) focusBoth(cl *client, threads, background []ids.ID) {
	e.t.Helper()
	if threads == nil {
		threads = []ids.ID{}
	}
	e.send(cl.c, "focus", map[string]any{"thread_ids": threads, "background_thread_ids": background})
}

// TestFocus_BackgroundThreadsGetRunUpdates: a thread only in background_thread_ids gets the whole
// document once and then run.updated, but no typing.started.
func TestFocus_BackgroundThreadsGetRunUpdates(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, 65536)
	sam := e.newPerson()
	th := e.w.thread(e.support)
	e.w.grant(sam.ID, th.ChannelID, levelRead)
	run := ids.New()
	e.box.set(runDoc(run, th.ID, 3))
	conn, fc := plain()
	cl := e.joinOn(sam, conn, fc)
	r := &frameReader{cl: cl}

	e.focusBoth(cl, nil, []ids.ID{th.ID})
	wantOpen(t, cl.fc)
	if n := len(cl.errorFrames()); n != 0 {
		t.Fatalf("%d error frames", n)
	}
	e.tick()
	fs := r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames after the focus, want one snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], run, 3)

	e.publish(th, run, 3, 4, false, replaceOp("/status_line", "Reading export.log"))
	e.box.set(runDoc(run, th.ID, 4))
	e.tick()
	e.clk.Advance(liveInterval)
	e.tick()
	fs = r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames after a patch, want one run.updated", len(fs))
	}
	wantOps(t, fs[0], run, 3, 4, replaceOp("/status_line", "Reading export.log"))

	// No typing.started for a background thread: the typing part runs on the plain focus
	// gateway, whose connections record dispatch frames (backgroundTyping).
	backgroundTyping(t)
}

// backgroundTyping: Alex types in a thread; Robin, who shows it in thread_ids, gets
// typing.started; Sam, who holds it only in background_thread_ids, gets none.
func backgroundTyping(t *testing.T) {
	t.Helper()
	s := newTypingScene(t)
	e := s.e
	robin := e.newPerson()
	e.w.grant(robin.ID, e.w.threads[s.th].ChannelID, levelRead)
	shower := e.join(robin)
	e.focus(shower, false, s.th)
	e.focusBoth(s.sam, nil, []ids.ID{s.th})
	e.typing(s.alex, "typing.start", s.th)
	if evs := shower.events(t); len(evs) != 1 || evs[0].D.Type != "typing.started" {
		t.Fatalf("Robin received %+v, want one typing.started", evs)
	}
	if evs := s.sam.events(t); len(evs) != 0 {
		t.Fatalf("a background thread got %+v", evs)
	}
}

// TestFocus_BackgroundThreadsNotInPresence: the presence focus (13 4.4, gateway_presence) holds
// only the threads of thread_ids; a thread in both lists counts as in thread_ids.
func TestFocus_BackgroundThreadsNotInPresence(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	alex := e.newPerson()
	shown, card, both := e.w.thread(e.support), e.w.thread(e.support), e.w.thread(e.support)
	for _, th := range []ids.ID{shown.ChannelID, card.ChannelID, both.ChannelID} {
		e.w.grant(alex.ID, th, levelRead)
	}
	cl := e.join(alex)
	e.focusBoth(cl, []ids.ID{shown.ID, both.ID}, []ids.ID{card.ID, both.ID})
	if got := e.focused(cl); !sameSet(got, []ids.ID{shown.ID, card.ID, both.ID}) {
		t.Fatalf("hub focus %v", got)
	}
	c := e.connections(alex.ID)[alex.ID]
	if len(c) != 1 || !sameSet(c[0].Focus, []ids.ID{shown.ID, both.ID}) {
		t.Fatalf("presence %+v, want only the threads of thread_ids", c)
	}
	rows := e.hub.PresenceRows()
	if len(rows) != 1 || !sameSet(rows[0].Focus, []ids.ID{shown.ID, both.ID}) {
		t.Fatalf("gateway_presence rows %+v", rows)
	}
	// The session keeps both kinds of entries with their mark, which resume restores.
	got := e.gw.SessionOf(cl.c).FocusState()
	background := 0
	for _, en := range got.Entries {
		if en.Background {
			background++
		}
	}
	if len(got.Entries) != 3 || background != 1 {
		t.Fatalf("session focus %+v", got)
	}
}

// TestFocus_BudgetCountsBothLists: more than sync.max_focused_threads distinct threads across both
// lists answer validation_failed and keep the previous focus; a thread in both lists counts once.
func TestFocus_BudgetCountsBothLists(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	var threads []ids.ID
	for range maxFocused + 1 {
		th := e.w.thread(e.support)
		e.w.grant(sam.ID, th.ChannelID, levelRead)
		threads = append(threads, th.ID)
	}
	cl := e.join(sam)
	e.focus(cl, false, threads[0])
	half := maxFocused / 2
	e.focusBoth(cl, threads[:half], threads[half:])
	wantOpen(t, cl.fc)
	errsSent := cl.errorFrames()
	if len(errsSent) != 1 || errsSent[0] != (app.ErrorFrame{Op: "focus", Code: "validation_failed", Detail: "sync.max_focused_threads"}) {
		t.Fatalf("error frames %+v", errsSent)
	}
	if got := e.focused(cl); !sameSet(got, threads[:1]) {
		t.Errorf("focus %v, want the previous set", got)
	}
	// Exactly the limit, with one thread in both lists, is accepted.
	e.focusBoth(cl, threads[:half+1], threads[half:maxFocused])
	if got := e.focused(cl); !sameSet(got, threads[:maxFocused]) {
		t.Errorf("focus %v, want %v", got, threads[:maxFocused])
	}
	if n := len(cl.errorFrames()); n != 1 {
		t.Errorf("%d error frames", n)
	}
}
