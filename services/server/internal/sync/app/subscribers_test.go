// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// fakeDraftRows is the draft part of the personal store: drafts keyed by (user, thread,
// channel); it counts every call so a test can prove that nothing else is written.
type fakeDraftRows struct {
	mu      sync.Mutex
	drafts  map[[3]ids.ID]syncmod.Draft
	deletes int
}

func (f *fakeDraftRows) DeleteDraft(_ context.Context, user, thread, channel ids.ID) (syncmod.Draft, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes++
	k := [3]ids.ID{user, thread, channel}
	if thread.IsZero() == channel.IsZero() {
		panic("DeleteDraft needs exactly one of thread and channel")
	}
	d, ok := f.drafts[k]
	delete(f.drafts, k)
	return d, ok, nil
}

// recordingAppender records the appended events.
type recordingAppender struct {
	mu     sync.Mutex
	events []eventlog.Event
}

func (r *recordingAppender) Append(_ context.Context, events ...eventlog.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
	return nil
}

// workCtx is a context inside a unit of work in phase work.
func workCtx() context.Context {
	ctx, _ := unitCtx(txn.PhaseWork)
	return ctx
}

// TestOnMessagePostedWritesNoReadState: a message, also one with mentions and one by the person
// itself, touches no draft and appends nothing without clear_draft; the subscriber has no read
// state port at all (brief note 5).
func TestOnMessagePostedWritesNoReadState(t *testing.T) {
	t.Parallel()
	rows := &fakeDraftRows{drafts: map[[3]ids.ID]syncmod.Draft{}}
	ev := &recordingAppender{}
	sub := app.NewSubscribers(rows)
	if err := sub.Bind(ev); err != nil {
		t.Fatal(err)
	}
	alex, sam := ids.New(), ids.New()
	posted := messaging.MessagePosted{MessageID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(),
		AuthorID: alex, AuthorKind: "user", MentionedIDs: []ids.ID{sam, alex}, CreatedAt: time.Now()}
	if err := sub.OnMessagePosted(workCtx(), posted); err != nil {
		t.Fatal(err)
	}
	if rows.deletes != 0 || len(ev.events) != 0 {
		t.Fatalf("%d draft deletes, %d events", rows.deletes, len(ev.events))
	}
}

// TestOnMessagePostedClearDraftThreadAndForumPost: clear_draft deletes the author's draft of the
// thread, or for the first message of a new thread the channel draft of the forum, and appends
// draft.deleted for the author; no draft deletes nothing and appends nothing.
func TestOnMessagePostedClearDraftThreadAndForumPost(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	space, text, forum, thread, post := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	threadDraft := syncmod.Draft{ID: ids.New(), ThreadID: thread, ChannelID: text, Version: 3}
	forumDraft := syncmod.Draft{ID: ids.New(), ChannelID: forum, Version: 1}
	rows := &fakeDraftRows{drafts: map[[3]ids.ID]syncmod.Draft{
		{alex, thread, {}}: threadDraft, {alex, {}, forum}: forumDraft, {sam, thread, {}}: {ID: ids.New(), ThreadID: thread, ChannelID: text},
	}}
	ev := &recordingAppender{}
	sub := app.NewSubscribers(rows)
	ctx := workCtx()
	if err := sub.OnMessagePosted(ctx, messaging.MessagePosted{ThreadID: thread, ChannelID: text, SpaceID: space, AuthorID: alex,
		AuthorKind: "user", ClearDraft: true}); err == nil {
		t.Fatal("an unbound subscriber deleted a draft")
	}
	if err := sub.Bind(ev); err != nil {
		t.Fatal(err)
	}
	if err := sub.OnMessagePosted(ctx, messaging.MessagePosted{ThreadID: thread, ChannelID: text, SpaceID: space, AuthorID: alex,
		AuthorKind: "user", ClearDraft: true}); err != nil {
		t.Fatal(err)
	}
	if err := sub.OnMessagePosted(ctx, messaging.MessagePosted{ThreadID: post, ChannelID: forum, SpaceID: space, AuthorID: alex,
		AuthorKind: "user", ClearDraft: true, CreatedThread: true}); err != nil {
		t.Fatal(err)
	}
	if err := sub.OnMessagePosted(ctx, messaging.MessagePosted{ThreadID: post, ChannelID: forum, SpaceID: space, AuthorID: alex,
		AuthorKind: "user", ClearDraft: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := rows.drafts[[3]ids.ID{sam, thread, {}}]; !ok || len(rows.drafts) != 1 {
		t.Fatalf("drafts left %v", rows.drafts)
	}
	if len(ev.events) != 2 {
		t.Fatalf("%d events", len(ev.events))
	}
	for i, want := range []struct {
		d      syncmod.Draft
		thread ids.ID
	}{{threadDraft, thread}, {forumDraft, ids.ID{}}} {
		e := ev.events[i]
		if e.Type != "draft.deleted" || e.AudiencePrincipalID != alex || e.ReceiveAction != "self" || e.ResourceKind != "draft" ||
			e.ResourceID != want.d.ID || e.ResourceVersion != 0 || e.ThreadID != want.thread || e.ChannelID != want.d.ChannelID ||
			e.SpaceID != space || e.ActorPrincipalID != alex || e.ActorKind != eventlog.ActorUser {
			t.Errorf("event %d: %+v", i, e)
		}
		var p map[string]any
		if err := json.Unmarshal(e.Payload, &p); err != nil || p["draft_id"] != want.d.ID.String() || p["channel_id"] != want.d.ChannelID.String() {
			t.Errorf("payload %d: %s", i, e.Payload)
		}
		if _, has := p["thread_id"]; has != !want.thread.IsZero() {
			t.Errorf("payload %d thread: %s", i, e.Payload)
		}
	}
}
