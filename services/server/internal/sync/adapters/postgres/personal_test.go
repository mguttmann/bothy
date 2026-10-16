// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestPersonalRepositoryRoundTrip (M1-61): every method of PersonalRepository against
// PostgreSQL: the upserts only move forward or replace a different value, the deletes return the
// row, writes and locks need a unit of work.
func TestPersonalRepositoryRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg, err := wire.NewTxnRegistry(wire.NewModules(pool, nil))
	if err != nil {
		t.Fatal(err)
	}
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 1, LockTimeout: 5 * time.Second, StatementTimeout: 30 * time.Second}, reg)
	repo := postgres.NewPersonalRepository(pool)
	su := db.SuperConn(t)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := su.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	alex, space, ch, thread, msg := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, alex.String())
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, alex.String())
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'ideas', 'a0', 20, $3)`, ch.String(), space.String(), alex.String())
	exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, thread.String(), ch.String(), alex.String())
	exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin)
		VALUES ($1, $2, $3, 1, $4, $5, '\x00', 'human')`, msg.String(), thread.String(), ch.String(), alex.String(), ids.New().String())
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	do := func(fn func(ctx context.Context) error) {
		t.Helper()
		if err := uow.Do(ctx, fn); err != nil {
			t.Fatal(err)
		}
	}

	// Read states.
	if _, _, err := repo.AdvanceReadState(ctx, alex, sync.ReadState{ThreadID: thread, ChannelID: ch, LastReadSeq: 1}); err == nil {
		t.Fatal("advance outside a unit of work")
	}
	if _, _, err := repo.LockReadState(ctx, alex, thread); err == nil {
		t.Fatal("lock outside a unit of work")
	}
	do(func(ctx context.Context) error {
		rs, changed, err := repo.AdvanceReadState(ctx, alex, sync.ReadState{ThreadID: thread, ChannelID: ch, LastReadSeq: 5, UpdatedAt: at})
		if err != nil || !changed || rs.Version != 1 || rs.LastReadSeq != 5 {
			t.Errorf("first advance %+v %v %v", rs, changed, err)
		}
		if _, changed, err := repo.AdvanceReadState(ctx, alex, sync.ReadState{ThreadID: thread, ChannelID: ch, LastReadSeq: 3, UpdatedAt: at}); changed || err != nil {
			t.Errorf("backwards advance %v %v", changed, err)
		}
		locked, found, err := repo.LockReadState(ctx, alex, thread)
		if err != nil || !found || locked.LastReadSeq != 5 {
			t.Errorf("lock %+v %v %v", locked, found, err)
		}
		back, err := repo.SetReadState(ctx, alex, thread, 2, ids.ID{}, at)
		if err != nil || back.LastReadSeq != 2 || back.Version != 2 || !back.UpdatedByDeviceID.IsZero() {
			t.Errorf("set %+v %v", back, err)
		}
		return nil
	})
	if rs, found, err := repo.ReadState(ctx, alex, thread); err != nil || !found || rs.LastReadSeq != 2 || !rs.UpdatedAt.Equal(at) {
		t.Fatalf("read %+v %v %v", rs, found, err)
	}
	if _, found, err := repo.ReadState(ctx, alex, ids.New()); found || err != nil {
		t.Fatalf("unknown %v %v", found, err)
	}
	if list, err := repo.ReadStatesOfChannel(ctx, alex, ch); err != nil || len(list) != 1 {
		t.Fatalf("list %+v %v", list, err)
	}

	// Drafts.
	if _, err := repo.UpsertDraft(ctx, alex, sync.Draft{ID: ids.New(), ThreadID: thread, ChannelID: ch}); err == nil {
		t.Fatal("upsert outside a unit of work")
	}
	if _, _, err := repo.DeleteDraft(ctx, alex, thread, ch); err == nil {
		t.Fatal("delete with both thread and channel")
	}
	var first sync.Draft
	do(func(ctx context.Context) error {
		var err error
		first, err = repo.UpsertDraft(ctx, alex, sync.Draft{ID: ids.New(), ThreadID: thread, ChannelID: ch, ContentMD: "a", ReplyToID: msg,
			FileIDs: []ids.ID{ids.New()}, UpdatedAt: at})
		if err != nil || first.Version != 1 || first.ReplyToID != msg || len(first.FileIDs) != 1 {
			t.Errorf("insert %+v %v", first, err)
		}
		again, err := repo.UpsertDraft(ctx, alex, sync.Draft{ID: ids.New(), ThreadID: thread, ChannelID: ch, ContentMD: "b", UpdatedAt: at})
		if err != nil || again.ID != first.ID || again.Version != 2 || len(again.FileIDs) != 0 || !again.ReplyToID.IsZero() {
			t.Errorf("update %+v %v", again, err)
		}
		post, err := repo.UpsertDraft(ctx, alex, sync.Draft{ID: ids.New(), ChannelID: ch, ContentMD: "post", UpdatedAt: at})
		if err != nil || post.Version != 1 || !post.ThreadID.IsZero() {
			t.Errorf("channel draft %+v %v", post, err)
		}
		return nil
	})
	if list, err := repo.DraftsInChannels(ctx, alex, []ids.ID{ch}); err != nil || len(list) != 2 {
		t.Fatalf("drafts %+v %v", list, err)
	}
	if list, err := repo.DraftsInChannels(ctx, alex, nil); err != nil || len(list) != 0 {
		t.Fatalf("drafts without channels %+v %v", list, err)
	}
	do(func(ctx context.Context) error {
		d, found, err := repo.DeleteDraft(ctx, alex, thread, ids.ID{})
		if err != nil || !found || d.ID != first.ID || d.Version != 2 {
			t.Errorf("delete thread draft %+v %v %v", d, found, err)
		}
		if _, found, err := repo.DeleteDraft(ctx, alex, thread, ids.ID{}); found || err != nil {
			t.Errorf("delete again %v %v", found, err)
		}
		if _, found, err := repo.DeleteDraft(ctx, alex, ids.ID{}, ch); !found || err != nil {
			t.Errorf("delete channel draft %v %v", found, err)
		}
		return nil
	})

	// Settings.
	if _, _, err := repo.PutSetting(ctx, alex, sync.UserSetting{Key: "appearance.theme", Value: json.RawMessage(`"dark"`)}); err == nil {
		t.Fatal("put outside a unit of work")
	}
	do(func(ctx context.Context) error {
		st, changed, err := repo.PutSetting(ctx, alex, sync.UserSetting{Key: "layout.column_widths", Value: json.RawMessage(`{"a":1,"b":2}`), UpdatedAt: at})
		if err != nil || !changed || st.Version != 1 {
			t.Errorf("insert %+v %v %v", st, changed, err)
		}
		if _, changed, err := repo.PutSetting(ctx, alex, sync.UserSetting{Key: "layout.column_widths", Value: json.RawMessage(`{ "b": 2, "a": 1 }`), UpdatedAt: at}); changed || err != nil {
			t.Errorf("same value %v %v", changed, err)
		}
		st, changed, err = repo.PutSetting(ctx, alex, sync.UserSetting{Key: "layout.column_widths", Value: json.RawMessage(`{"a":3}`), UpdatedAt: at})
		if err != nil || !changed || st.Version != 2 {
			t.Errorf("update %+v %v %v", st, changed, err)
		}
		return nil
	})
	if st, found, err := repo.Setting(ctx, alex, "layout.column_widths"); err != nil || !found || string(st.Value) != `{"a": 3}` {
		t.Fatalf("setting %+v %v %v", st, found, err)
	}
	if list, err := repo.Settings(ctx, alex); err != nil || len(list) != 1 {
		t.Fatalf("settings %+v %v", list, err)
	}
	do(func(ctx context.Context) error {
		if st, found, err := repo.DeleteSetting(ctx, alex, "layout.column_widths"); err != nil || !found || st.Version != 2 {
			t.Errorf("delete %+v %v %v", st, found, err)
		}
		if _, found, err := repo.DeleteSetting(ctx, alex, "layout.column_widths"); found || err != nil {
			t.Errorf("delete again %v %v", found, err)
		}
		return nil
	})
	if _, found, err := repo.Setting(ctx, alex, "layout.column_widths"); found || err != nil {
		t.Fatalf("after delete %v %v", found, err)
	}
}
