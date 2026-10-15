// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRepositoryChannels (M1-53): every channel and forum tag method of app.Store against the
// tables, with the name conflicts and the facts of the space.
func TestRepositoryChannels(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	sp := domain.NewSpace(ids.New(), "Support", "", "", "a0", alex)
	sp.CreatedAt = t0
	text := domain.NewChannel(ids.New(), sp.ID, domain.ChannelText, "tickets", "Customer", "a0", alex, 20)
	text.CreatedAt = t0
	forum := domain.NewChannel(ids.New(), sp.ID, domain.ChannelForum, "bugs", "", "a1", alex, 20)
	forum.CreatedAt, forum.ForumDefaultSort = t0, "created"
	var main domain.Thread
	if err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertSpace(ctx, sp); err != nil {
			return err
		}
		if err := e.repo.InsertChannel(ctx, text); err != nil {
			return err
		}
		var err error
		if main, err = e.repo.InsertMainThread(ctx, mainThread(text.ID, alex)); err != nil {
			return err
		}
		return e.repo.InsertChannel(ctx, forum)
	}); err != nil {
		t.Fatal(err)
	}
	if main.Kind != "main" || main.State != "open" || main.Version != 1 || !main.CreatedAt.Equal(t0) {
		t.Fatalf("main thread %+v", main)
	}
	row, found, err := e.repo.Channel(ctx, text.ID)
	if err != nil || !found || row.Channel.MainThreadID != main.ID || row.Channel.Topic != "Customer" || row.SpaceArchived || !row.Channel.RequireMention {
		t.Fatalf("channel %+v %v %v", row, found, err)
	}
	if _, found, err := e.repo.Channel(ctx, ids.New()); found || err != nil {
		t.Fatal("unknown channel found")
	}
	order, err := e.repo.ChannelOrder(ctx, sp.ID)
	if err != nil || len(order) != 2 || order[0].ID != text.ID {
		t.Fatalf("order %+v, %v", order, err)
	}
	vis, err := e.repo.VisibleChannels(ctx, sp.ID, false, []ids.ID{forum.ID}, nil)
	if err != nil || len(vis) != 1 || vis[0].Channel.ForumDefaultSort != "created" {
		t.Fatalf("visible %+v, %v", vis, err)
	}
	if vis, err = e.repo.VisibleChannels(ctx, sp.ID, false, nil, []ids.ID{sp.ID}); err != nil || len(vis) != 2 {
		t.Fatalf("fully visible %+v, %v", vis, err)
	}
	dup := domain.NewChannel(ids.New(), sp.ID, domain.ChannelText, "TICKETS", "", "a2", alex, 20)
	dup.CreatedAt = t0
	err = e.write(t, func(ctx context.Context) error { return e.repo.InsertChannel(ctx, dup) })
	if ee, ok := errs.As(err); !ok || ee.Code != errs.CodeConflict {
		t.Fatalf("duplicate %v", err)
	}
	tag := domain.ForumTag{ID: ids.New(), ChannelID: forum.ID, Name: "bug", Color: "moss", SortKey: "a0"}
	if err := e.write(t, func(ctx context.Context) error {
		c, found, err := e.repo.LockChannel(ctx, forum.ID)
		if err != nil || !found {
			t.Fatalf("lock %v %v", found, err)
		}
		c.Channel.Name, c.Channel.DefaultNotificationLevel = "issues", "mute"
		c.Channel.Archive(t0)
		if err := e.repo.SaveChannel(ctx, c.Channel); err != nil {
			return err
		}
		return e.repo.InsertForumTag(ctx, tag)
	}); err != nil {
		t.Fatal(err)
	}
	err = e.write(t, func(ctx context.Context) error {
		return e.repo.InsertForumTag(ctx, domain.ForumTag{ID: ids.New(), ChannelID: forum.ID, Name: "BUG", SortKey: "a1"})
	})
	if ee, ok := errs.As(err); !ok || ee.Code != errs.CodeConflict {
		t.Fatalf("duplicate tag %v", err)
	}
	all, err := e.repo.AllChannels(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("all %+v, %v", all, err)
	}
	got, found, err := e.repo.Channel(ctx, forum.ID)
	if err != nil || !found || got.Channel.Name != "issues" || got.Channel.DefaultNotificationLevel != "mute" || !got.Channel.Archived() || got.Channel.Version != 2 {
		t.Fatalf("saved %+v, %v", got, err)
	}
	tag.Color, tag.DeletedAt = "", t0
	if err := e.write(t, func(ctx context.Context) error { return e.repo.SaveForumTag(ctx, tag) }); err != nil {
		t.Fatal(err)
	}
	if tags, err := e.repo.ForumTags(ctx, forum.ID); err != nil || len(tags) != 0 {
		t.Fatalf("tags %+v, %v", tags, err)
	}
	if st, found, err := e.repo.ForumTag(ctx, tag.ID); err != nil || !found || !st.Deleted() || st.Color != "" {
		t.Fatalf("tag %+v %v %v", st, found, err)
	}
	if _, found, err := e.repo.ForumTag(ctx, ids.New()); found || err != nil {
		t.Fatal("unknown tag found")
	}
	for _, f := range []func(context.Context) error{
		func(ctx context.Context) error { _, _, err := e.repo.LockChannel(ctx, text.ID); return err },
		func(ctx context.Context) error { return e.repo.InsertChannel(ctx, text) },
		func(ctx context.Context) error { return e.repo.SaveChannel(ctx, text) },
		func(ctx context.Context) error { _, err := e.repo.InsertMainThread(ctx, main); return err },
		func(ctx context.Context) error { return e.repo.InsertForumTag(ctx, tag) },
		func(ctx context.Context) error { return e.repo.SaveForumTag(ctx, tag) },
	} {
		if err := f(ctx); err == nil {
			t.Error("a write outside a unit of work passed")
		}
	}
}
