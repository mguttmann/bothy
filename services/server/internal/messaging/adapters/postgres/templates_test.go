// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestRepository_Templates (M1-60): insert, count and list per scope, the visible lock with and
// without deleted rows, update and soft delete raising the version, and writes outside a unit of
// work refused.
func TestRepository_Templates(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	_, ch, _ := w.TextChannel()
	sam := w.Person("sam", false)
	repo := postgres.NewRepository(w.Pool)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(w.Pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg)
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	personal := domain.Template{ID: ids.New(), OwnerUserID: sam, Name: "beta", ContentMD: "b", CreatedBy: sam, CreatedAt: at}
	channel := domain.Template{ID: ids.New(), ChannelID: ch, Name: "Alpha", ContentMD: "a", CreatedBy: w.OwnerID, CreatedAt: at}
	if _, err := repo.InsertTemplate(ctx, personal); err == nil {
		t.Fatal("insert outside a unit of work")
	}
	if err := repo.LockTemplateScope(ctx, 1); err == nil {
		t.Fatal("lock outside a unit of work")
	}
	err := uow.Do(ctx, func(ctx context.Context) error {
		if err := repo.LockTemplateScope(ctx, domain.ScopeLockKey(domain.ScopePersonal, sam)); err != nil {
			return err
		}
		for _, tpl := range []domain.Template{personal, channel} {
			got, err := repo.InsertTemplate(ctx, tpl)
			if err != nil {
				return err
			}
			if got.Version != 1 || got.Name != tpl.Name || got.Scope() != tpl.Scope() {
				t.Errorf("inserted %+v", got)
			}
			if n, err := repo.CountTemplates(ctx, tpl); err != nil || n != 1 {
				t.Errorf("count %d %v", n, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if list, err := repo.PersonalTemplates(ctx, sam); err != nil || len(list) != 1 || list[0].ID != personal.ID {
		t.Fatalf("personal %+v %v", list, err)
	}
	list, err := repo.ChannelTemplates(ctx, sam, ch, []ids.ID{ch})
	if err != nil || len(list) != 2 || list[0].ID != channel.ID || list[1].ID != personal.ID {
		t.Fatalf("channel list %+v %v", list, err)
	}
	if list, err := repo.ChannelTemplates(ctx, ids.ID{}, ch, nil); err != nil || len(list) != 0 {
		t.Fatalf("invisible channel %+v %v", list, err)
	}
	err = uow.Do(ctx, func(ctx context.Context) error {
		if _, found, err := repo.LockVisibleTemplate(ctx, channel.ID, sam, nil, false); found || err != nil {
			t.Errorf("channel template without visibility: %v %v", found, err)
		}
		got, found, err := repo.LockVisibleTemplate(ctx, personal.ID, sam, nil, false)
		if err != nil || !found {
			t.Fatalf("own template: %v %v", found, err)
		}
		got.Name = "gamma"
		up, err := repo.UpdateTemplate(ctx, got)
		if err != nil || up.Version != 2 || up.Name != "gamma" {
			t.Fatalf("update %+v %v", up, err)
		}
		return repo.DeleteTemplate(ctx, personal.ID, at.Add(time.Minute))
	})
	if err != nil {
		t.Fatal(err)
	}
	err = uow.Do(ctx, func(ctx context.Context) error {
		if _, found, err := repo.LockVisibleTemplate(ctx, personal.ID, sam, nil, false); found || err != nil {
			t.Errorf("deleted without include: %v %v", found, err)
		}
		got, found, err := repo.LockVisibleTemplate(ctx, personal.ID, sam, nil, true)
		if err != nil || !found || got.DeletedAt.IsZero() || got.Version != 3 {
			t.Errorf("deleted with include: %+v %v %v", got, found, err)
		}
		if n, err := repo.CountTemplates(ctx, personal); err != nil || n != 0 {
			t.Errorf("count after delete %d %v", n, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRepository_MentionQueries (M1-60): the mention page in keyset order and the unread counts
// per thread above the read sequence.
func TestRepository_MentionQueries(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	_, ch, main := w.TextChannel()
	sam := w.Person("sam", false)
	repo := postgres.NewRepository(w.Pool)
	ctx := context.Background()
	for range 3 {
		w.Clock.Advance(time.Second)
		w.Send("owner", main, "@sam hi", http.StatusCreated)
	}
	rows, err := repo.MyMentions(ctx, app.MentionQuery{PrincipalID: sam, Visible: []ids.ID{ch}, Limit: 2})
	if err != nil || len(rows) != 2 || !rows[0].MentionedAt.After(rows[1].MentionedAt) {
		t.Fatalf("first page %+v %v", rows, err)
	}
	rest, err := repo.MyMentions(ctx, app.MentionQuery{PrincipalID: sam, Visible: []ids.ID{ch}, Limit: 2,
		HasAfter: true, AfterAt: rows[1].MentionedAt, AfterID: rows[1].Message.ID})
	if err != nil || len(rest) != 1 || rest[0].Message.ThreadSeq != 1 {
		t.Fatalf("second page %+v %v", rest, err)
	}
	if none, err := repo.MyMentions(ctx, app.MentionQuery{PrincipalID: sam, Limit: 2}); err != nil || len(none) != 0 {
		t.Fatalf("no visible channel %+v %v", none, err)
	}
	counts, err := repo.CountUnreadMentions(ctx, sam, []ids.ID{ch}, map[ids.ID]int64{main: 1})
	if err != nil || len(counts) != 1 || counts[0].ThreadID != main || counts[0].ChannelID != ch || counts[0].Count != 2 {
		t.Fatalf("counts %+v %v", counts, err)
	}
}
