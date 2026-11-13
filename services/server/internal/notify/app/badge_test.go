// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// readEvent returns read_state.updated of person for thread with the read position pos at log
// seq.
func (w *pushWorld) readEvent(seq int64, person, thread ids.ID, pos int64) sync.Event {
	payload := map[string]any{"read_state": map[string]any{"thread_id": thread, "last_read_seq": pos}}
	return w.event(seq, "read_state.updated", "thread", thread, person, payload)
}

// unreadThread makes thread an unread candidate of its channel with last as its newest message.
func (w *pushWorld) unreadThread(thread ids.ID, last int64) {
	info := w.threads[thread]
	info.LastMessageSeq = last
	w.threads[thread] = info
	w.unread = append(w.unread, chat.ThreadPosition{ThreadID: thread, ChannelID: info.ChannelID, ChannelKind: info.ChannelKind, LastMessageSeq: last})
}

// TestReadState_PreviousPositionIsOneEvent (finding performance-5,
// docs/decisions.d/AUD-srv-notify-01.md AUD-SN01/3): Alex reads a thread that 300 other people
// read before him. The consumer asks the history for the one earlier read of Alex in that thread
// and gets one event, never the reads of the other people; the read lowers the badge from the
// previous position 3, so one badge sync is scheduled. A read that does not pass the previous
// position schedules none, and without an earlier read the previous position is 0.
func TestReadState_PreviousPositionIsOneEvent(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	w.scope[w.alex] = []ids.ID{w.channel}
	w.unreadThread(w.thread, 10)
	for i := range 300 {
		w.remember(w.readEvent(nextSeq(), ids.New(), w.thread, int64(i%10)+1))
	}
	w.remember(w.readEvent(nextSeq(), w.alex, w.thread, 3))
	w.remember(w.readEvent(nextSeq(), w.alex, ids.New(), 9))
	for range 50 {
		w.remember(w.readEvent(nextSeq(), ids.New(), w.thread, 10))
	}

	// Not past the previous position 3: nothing to lower.
	w.mustHandle(w.readEvent(nextSeq(), w.alex, w.thread, 3))
	if _, pending := w.sched.syncs[w.alex]; pending {
		t.Fatal("a read at the previous position scheduled a badge sync")
	}

	read := w.readEvent(nextSeq(), w.alex, w.thread, 10)
	w.mustHandle(read)
	if at, pending := w.sched.syncs[w.alex]; !pending || !at.Equal(w.clock.Now()) || w.sched.syncArg[w.alex] != read.Seq-1 {
		t.Fatalf("badge sync pending %v at %s after seq %d, want one now after seq %d", pending, at, w.sched.syncArg[w.alex], read.Seq-1)
	}
	if n := w.calls.get("LastPersonalEvent"); n != 2 {
		t.Fatalf("%d reads of the previous read, want one per event (2)", n)
	}
	if n := w.calls.get("ResourceEvents"); n != 0 {
		t.Fatalf("the consumer read the events of the thread %d times, want never", n)
	}
	if w.historyRows != 2 {
		t.Fatalf("the history handed out %d events for two reads of Alex, want one each", w.historyRows)
	}

	// Sam never read the thread: the previous position is 0, so his first read lowers the badge.
	w.scope[w.sam] = []ids.ID{w.channel}
	first := w.readEvent(nextSeq(), w.sam, w.thread, 10)
	w.mustHandle(first)
	if _, pending := w.sched.syncs[w.sam]; !pending || w.sched.syncArg[w.sam] != first.Seq-1 {
		t.Fatal("the first read of a counted thread scheduled no badge sync")
	}
}

// TestBadge_OneScopePerCount (findings srv-notify-2 and performance-7, AUD-SN01/4): the
// push.deliver checks of one badge count outside an event (the job notify.push_send, the badge
// sync, the preview) share one request memo and one structure memo, so the rights version is read
// once and the facts of a channel once per count instead of once per counted thread; the next
// count gets fresh ones, so it sees a rights change. A count inside a request (the preview) keeps
// the request memo of that request.
func TestBadge_OneScopePerCount(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	w.scope[w.alex] = []ids.ID{w.channel}
	for range 5 {
		th := ids.New()
		w.threads[th] = chat.ThreadInfo{ID: th, ChannelID: w.channel, SpaceID: w.space, ChannelKind: "text", Kind: "thread"}
		w.unreadThread(th, 4)
	}
	p := w.push()
	var scopes [2][2]*authorize.RequestMemo
	for i := range scopes {
		before := len(w.authzScopes)
		n, err := p.Badge().For(context.Background(), w.alex)
		if err != nil || n != 5 {
			t.Fatalf("badge %d, %v; want 5", n, err)
		}
		checks := w.authzScopes[before:]
		if len(checks) != 5 {
			t.Fatalf("%d push.deliver checks, want one per counted thread (5)", len(checks))
		}
		scopes[i] = checks[0]
		if scopes[i][0] == nil || scopes[i][1] == nil {
			t.Fatalf("a check without request memo (%v) or structure memo (%v)", scopes[i][0] != nil, scopes[i][1] != nil)
		}
		for _, c := range checks {
			if c != scopes[i] {
				t.Fatal("the checks of one count run under different scopes")
			}
		}
	}
	if scopes[0][0] == scopes[1][0] || scopes[0][1] == scopes[1][1] {
		t.Fatal("two counts share a scope: the second would not see a rights change")
	}

	request := authorize.WithRequestScope(context.Background())
	own, _ := authorize.RequestMemoFrom(request)
	before := len(w.authzScopes)
	if _, err := p.Badge().For(request, w.alex); err != nil {
		t.Fatal(err)
	}
	for _, c := range w.authzScopes[before:] {
		if c[0] != own || c[1] == nil {
			t.Fatal("a count inside a request does not use the request memo of the request and a structure memo")
		}
	}
}
