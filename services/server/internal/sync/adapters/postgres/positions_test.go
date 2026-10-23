// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestReadPositions (M3-19 design notes 9 d and 16 a): PositionsFor gives every requested person
// of one thread an entry and PositionsOfUser every requested thread of one person, the stored
// last_read_seq or 0 without a row; inside a unit of work they read its transaction.
func TestReadPositions(t *testing.T) {
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
	var rp sync.ReadPositions = repo
	su := db.SuperConn(t)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := su.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	alex, sam, robin, space, ch := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	for handle, id := range map[string]ids.ID{"alex": alex, "sam": sam, "robin": robin} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id.String(), handle)
		exec(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	}
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'ideas', 'a0', 20, $3)`, ch.String(), space.String(), alex.String())
	t1, t2, t3 := ids.New(), ids.New(), ids.New()
	for _, th := range []ids.ID{t1, t2, t3} {
		exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, th.String(), ch.String(), alex.String())
	}
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	advance := func(ctx context.Context, user, thread ids.ID, seq int64) error {
		_, _, err := repo.AdvanceReadState(ctx, user, sync.ReadState{ThreadID: thread, ChannelID: ch, LastReadSeq: seq, UpdatedAt: at})
		return err
	}
	if err := uow.Do(ctx, func(ctx context.Context) error {
		return errors.Join(advance(ctx, alex, t1, 5), advance(ctx, sam, t1, 3), advance(ctx, alex, t2, 7))
	}); err != nil {
		t.Fatal(err)
	}

	got, err := rp.PositionsFor(ctx, t1, []ids.ID{alex, sam, robin})
	if want := map[ids.ID]int64{alex: 5, sam: 3, robin: 0}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("PositionsFor(t1) = %v %v, want %v", got, err, want)
	}
	got, err = rp.PositionsFor(ctx, t3, []ids.ID{alex})
	if want := map[ids.ID]int64{alex: 0}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("PositionsFor(t3) = %v %v, want %v", got, err, want)
	}
	got, err = rp.PositionsOfUser(ctx, alex, []ids.ID{t1, t2, t3})
	if want := map[ids.ID]int64{t1: 5, t2: 7, t3: 0}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("PositionsOfUser(alex) = %v %v, want %v", got, err, want)
	}
	got, err = rp.PositionsOfUser(ctx, robin, []ids.ID{t1})
	if want := map[ids.ID]int64{t1: 0}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("PositionsOfUser(robin) = %v %v, want %v", got, err, want)
	}
	for name, read := range map[string]func() (map[ids.ID]int64, error){
		"PositionsFor":    func() (map[ids.ID]int64, error) { return rp.PositionsFor(ctx, t1, nil) },
		"PositionsOfUser": func() (map[ids.ID]int64, error) { return rp.PositionsOfUser(ctx, alex, nil) },
	} {
		if got, err := read(); err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s without ids = %v %v, want an empty map", name, got, err)
		}
	}

	// Inside a unit of work the reads see its uncommitted rows.
	rollback := errors.New("roll back")
	err = uow.Do(ctx, func(ctx context.Context) error {
		if err := advance(ctx, robin, t1, 2); err != nil {
			return err
		}
		of, err := rp.PositionsFor(ctx, t1, []ids.ID{robin})
		if err != nil {
			return err
		}
		ou, err := rp.PositionsOfUser(ctx, robin, []ids.ID{t1})
		if err != nil {
			return err
		}
		if of[robin] != 2 || ou[t1] != 2 {
			t.Errorf("inside the unit: PositionsFor %v and PositionsOfUser %v, want 2", of, ou)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("Do = %v, want the rollback", err)
	}
}
