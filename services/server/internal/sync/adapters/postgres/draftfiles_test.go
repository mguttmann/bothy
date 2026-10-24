// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestDraftFilesHeldByUploader (M3-08): a thread or channel draft of the uploader holds the file;
// the same file ID in the draft of another principal holds nothing; an empty list asks nothing.
func TestDraftFilesHeldByUploader(t *testing.T) {
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
	src := postgres.NewDraftFiles(pool)
	su := db.SuperConn(t)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := su.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	alex, sam, space, ch, thread := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	for _, p := range []struct {
		id     ids.ID
		handle string
	}{{alex, "alex"}, {sam, "sam"}} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, p.id.String(), p.handle)
		exec(`INSERT INTO users (principal_id) VALUES ($1)`, p.id.String())
	}
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'ideas', 'a0', 20, $3)`, ch.String(), space.String(), alex.String())
	exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, thread.String(), ch.String(), alex.String())
	inThread, inChannel, foreign, loose := ids.New(), ids.New(), ids.New(), ids.New()
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	if err := uow.Do(ctx, func(ctx context.Context) error {
		if _, err := repo.UpsertDraft(ctx, alex, sync.Draft{ID: ids.New(), ThreadID: thread, ChannelID: ch, ContentMD: "a",
			FileIDs: []ids.ID{inThread}, UpdatedAt: at}); err != nil {
			return err
		}
		if _, err := repo.UpsertDraft(ctx, alex, sync.Draft{ID: ids.New(), ChannelID: ch, ContentMD: "b", FileIDs: []ids.ID{inChannel},
			UpdatedAt: at}); err != nil {
			return err
		}
		_, err := repo.UpsertDraft(ctx, sam, sync.Draft{ID: ids.New(), ThreadID: thread, ChannelID: ch, ContentMD: "c",
			FileIDs: []ids.ID{foreign}, UpdatedAt: at})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	held, err := src.HeldByUploader(ctx, []files.UploaderFile{{FileID: inThread, UploaderID: alex}, {FileID: inChannel, UploaderID: alex},
		{FileID: foreign, UploaderID: alex}, {FileID: loose, UploaderID: alex}, {FileID: inThread, UploaderID: sam}})
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 2 || !held[inThread] || !held[inChannel] {
		t.Fatalf("held %v", held)
	}
	if held, err := src.HeldByUploader(ctx, nil); err != nil || len(held) != 0 {
		t.Fatalf("empty %v %v", held, err)
	}
}
