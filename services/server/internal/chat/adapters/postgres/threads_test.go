// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRepositoryThreads (M1-55): insert, read, lock, save, the thread sequence with reopen, the
// keyset lists with their filters, the tag rows and the unique parent message.
func TestRepositoryThreads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	space, forum := ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'bugs', 'a0', 20, $3)`, forum.String(), space.String(), alex.String())
	tag := ids.New()
	e.exec(t, `INSERT INTO forum_tags (id, channel_id, name, sort_key) VALUES ($1, $2, 'bug', 'a0')`, tag.String(), forum.String())
	newThread := func(i int, parent ids.ID) domain.Thread {
		at := t0.Add(time.Duration(i) * time.Minute)
		return domain.Thread{ID: ids.New(), ChannelID: forum, Kind: domain.ThreadKindThread, ParentMessageID: parent, Title: "t",
			TitleSource: domain.TitleAuto, State: domain.ThreadOpen, CreatedBy: alex, CreatedAt: at, LastActivityAt: at, Version: 1}
	}
	if _, err := e.repo.InsertThread(ctx, newThread(0, ids.ID{})); err == nil {
		t.Fatal("insert outside a unit of work")
	}
	parent := ids.New()
	var made []domain.Thread
	if err := e.write(t, func(ctx context.Context) error {
		for i := range 3 {
			p := ids.ID{}
			if i == 0 {
				p = parent
			}
			th, err := e.repo.InsertThread(ctx, newThread(i, p))
			if err != nil {
				return err
			}
			made = append(made, th)
		}
		return e.repo.ReplaceThreadTags(ctx, made[2].ID, []ids.ID{tag}, alex, t0)
	}); err != nil {
		t.Fatal(err)
	}
	err := e.write(t, func(ctx context.Context) error { _, err := e.repo.InsertThread(ctx, newThread(4, parent)); return err })
	if ee, ok := errs.As(err); !ok || ee.Code != errs.CodeConflict || ee.Fields[0].Pointer != "/parent_message_id" {
		t.Fatalf("second thread of the parent: %v", err)
	}
	got, found, err := e.repo.Thread(ctx, made[0].ID)
	if err != nil || !found || got.ParentMessageID != parent || got.Kind != domain.ThreadKindThread || !got.CreatedAt.Equal(t0) {
		t.Fatalf("thread %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.repo.Thread(ctx, ids.New()); found || err != nil {
		t.Fatalf("unknown thread %v %v", found, err)
	}
	if _, _, err := e.repo.LockThread(ctx, made[0].ID); err == nil {
		t.Fatal("lock outside a unit of work")
	}
	if err := e.write(t, func(ctx context.Context) error {
		th, found, err := e.repo.LockThread(ctx, made[0].ID)
		if err != nil || !found {
			return err
		}
		if _, err := th.Close(alex, t0.Add(time.Hour)); err != nil {
			return err
		}
		if err := e.repo.SaveThread(ctx, th); err != nil {
			return err
		}
		th, err = e.repo.AppendToThread(ctx, th.ID, true, t0.Add(2*time.Hour))
		if err != nil || th.LastMessageSeq != 1 || th.State != domain.ThreadOpen || !th.ClosedAt.IsZero() || th.Version != 3 {
			t.Errorf("append %+v, %v", th, err)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.AppendToThread(ctx, made[0].ID, false, t0); err == nil {
		t.Fatal("append outside a unit of work")
	}
	ids3 := func(list []domain.Thread) []ids.ID {
		var out []ids.ID
		for _, th := range list {
			out = append(out, th.ID)
		}
		return out
	}
	list, err := e.repo.ListThreads(ctx, app.ThreadQuery{ChannelID: forum, Limit: 10})
	if err != nil || !slices.Equal(ids3(list), []ids.ID{made[0].ID, made[2].ID, made[1].ID}) {
		t.Fatalf("by activity %v, %v", ids3(list), err)
	}
	list, err = e.repo.ListThreads(ctx, app.ThreadQuery{ChannelID: forum, ByCreated: true, Limit: 10, HasAfter: true,
		AfterAt: made[2].CreatedAt, AfterID: made[2].ID})
	if err != nil || !slices.Equal(ids3(list), []ids.ID{made[1].ID, made[0].ID}) {
		t.Fatalf("by created after %v, %v", ids3(list), err)
	}
	list, err = e.repo.ListThreads(ctx, app.ThreadQuery{ChannelID: forum, TagID: tag, State: "open", Limit: 10})
	if err != nil || !slices.Equal(ids3(list), []ids.ID{made[2].ID}) {
		t.Fatalf("tag filter %v, %v", ids3(list), err)
	}
	if _, err := e.repo.ListThreads(ctx, app.ThreadQuery{ChannelID: forum}); err == nil {
		t.Fatal("a list without a limit")
	}
	tags, err := e.repo.ThreadTags(ctx, []ids.ID{made[1].ID, made[2].ID})
	if err != nil || len(tags) != 1 || !slices.Equal(tags[made[2].ID], []ids.ID{tag}) {
		t.Fatalf("tags %v, %v", tags, err)
	}
	if err := e.repo.DeleteTagAssignments(ctx, tag); err == nil {
		t.Fatal("delete outside a unit of work")
	}
	if err := e.repo.SaveThread(ctx, made[1]); err == nil {
		t.Fatal("save outside a unit of work")
	}
	if err := e.repo.ReplaceThreadTags(ctx, made[1].ID, nil, alex, t0); err == nil {
		t.Fatal("replace outside a unit of work")
	}
	if err := e.write(t, func(ctx context.Context) error { return e.repo.DeleteTagAssignments(ctx, tag) }); err != nil {
		t.Fatal(err)
	}
	if tags, err := e.repo.ThreadTags(ctx, []ids.ID{made[2].ID}); err != nil || len(tags) != 0 {
		t.Fatalf("tags after the delete %v, %v", tags, err)
	}
}
