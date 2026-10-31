// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

var t0 = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// env is the repository over PostgreSQL 18 with a unit of work without hooks.
type env struct {
	repo *postgres.Repository
	uow  *pg.UnitOfWork
	pool *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := pgtest.New(t).Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	return env{repo: postgres.NewRepository(pool), uow: uow, pool: pool}
}

func (e env) write(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	return e.uow.Do(context.Background(), fn)
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// person inserts the person alex with a device and returns both (INT-W17/5: every caller used
// the same handle).
func (e env) person(t *testing.T) (ids.ID, ids.ID) {
	t.Helper()
	id, dev := ids.New(), ids.New()
	e.exec(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'alex')`, id.String())
	e.exec(t, `INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	e.exec(t, `INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'web', 'active')`, dev.String(), id.String())
	return id, dev
}

// TestRepositoryRoundTrip: every method of app.Store against the tables.
func TestRepositoryRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, dev := e.person(t)
	support := domain.NewSpace(ids.New(), "Support", "headset", "moss", "a1", alex)
	support.CreatedAt = t0
	research := domain.NewSpace(ids.New(), "Research", "", "", "a0", alex)
	research.CreatedAt = t0
	if err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.LockActiveSpaces(ctx); err != nil {
			return err
		}
		if err := e.repo.InsertSpace(ctx, support); err != nil {
			return err
		}
		return e.repo.InsertSpace(ctx, research)
	}); err != nil {
		t.Fatal(err)
	}
	got, found, err := e.repo.Space(ctx, support.ID)
	if err != nil || !found || got != support {
		t.Fatalf("Space = %+v, %v, %v; want %+v", got, found, err, support)
	}
	if _, found, err := e.repo.Space(ctx, ids.New()); err != nil || found {
		t.Fatalf("unknown space: %v, %v", found, err)
	}
	order, err := e.repo.ActiveOrder(ctx)
	if err != nil || len(order) != 2 || order[0].ID != research.ID || order[1].SortKey != "a1" {
		t.Fatalf("order %+v, %v", order, err)
	}
	vis, err := e.repo.VisibleSpaces(ctx, false, []ids.ID{support.ID})
	if err != nil || len(vis) != 1 || vis[0].ID != support.ID {
		t.Fatalf("visible %+v, %v", vis, err)
	}
	all, err := e.repo.VisibleSpaces(ctx, true, nil)
	if err != nil || len(all) != 2 || all[0].ID != research.ID {
		t.Fatalf("all %+v, %v", all, err)
	}
	// Writes and locks need a unit of work.
	if err := e.repo.InsertSpace(ctx, support); err == nil {
		t.Fatal("insert outside a unit of work")
	}
	if err := e.repo.LockActiveSpaces(ctx); err == nil {
		t.Fatal("lock outside a unit of work")
	}
	if _, _, err := e.repo.LockSpace(ctx, support.ID); err == nil {
		t.Fatal("row lock outside a unit of work")
	}
	dup := domain.NewSpace(ids.New(), "SUPPORT", "", "", "a2", alex)
	dup.CreatedAt = t0
	err = e.write(t, func(ctx context.Context) error { return e.repo.InsertSpace(ctx, dup) })
	if ee, ok := errs.As(err); !ok || ee.Code != errs.CodeConflict || len(ee.Fields) != 1 || ee.Fields[0].Pointer != "/name" {
		t.Fatalf("taken name: %v", err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		sp, found, err := e.repo.LockSpace(ctx, support.ID)
		if err != nil || !found {
			return err
		}
		name, color := "Help", "rowan"
		sp.Update(&name, nil, &color)
		if err := e.repo.UpdateSpace(ctx, sp); err != nil {
			return err
		}
		sp.Move("Zz")
		if err := e.repo.MoveSpace(ctx, sp); err != nil {
			return err
		}
		sp.Archive(t0.Add(time.Hour))
		return e.repo.ArchiveSpace(ctx, sp)
	}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = e.repo.Space(ctx, support.ID)
	if got.Name != "Help" || got.Color != "rowan" || got.SortKey != "Zz" || got.Version != 4 || !got.ArchivedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("after writes %+v", got)
	}
	if _, found, err := e.write2(t, ids.New()); err != nil || found {
		t.Fatalf("lock unknown: %v, %v", found, err)
	}
	idsAll, err := e.repo.SpaceIDs(ctx)
	if err != nil || len(idsAll) != 2 || !slices.Contains(idsAll, support.ID) {
		t.Fatalf("space ids %v, %v", idsAll, err)
	}
	// The sidebar: insert, keep, replace, delete the key; rows of archived spaces are not listed.
	var st domain.SidebarState
	put := func(s domain.SidebarState, keep app.SidebarKeep) {
		t.Helper()
		if err := e.write(t, func(ctx context.Context) error {
			var err error
			st, err = e.repo.PutSidebarState(ctx, s, keep)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	order1 := []ids.ID{ids.New(), ids.New()}
	put(domain.SidebarState{UserID: alex, SpaceID: research.ID, Collapsed: true, SortKey: "a0V", ChannelOrder: order1, UpdatedByDeviceID: dev}, app.SidebarKeep{})
	if st.Version != 1 || st.SortKey != "a0V" || st.UpdatedByDeviceID != dev || !slices.Equal(st.ChannelOrder, order1) {
		t.Fatalf("first put %+v", st)
	}
	put(domain.SidebarState{UserID: alex, SpaceID: research.ID}, app.SidebarKeep{Key: true, ChannelOrder: true})
	if st.Version != 2 || st.SortKey != "a0V" || st.Collapsed || !st.UpdatedByDeviceID.IsZero() || !slices.Equal(st.ChannelOrder, order1) {
		t.Fatalf("keep %+v", st)
	}
	put(domain.SidebarState{UserID: alex, SpaceID: research.ID}, app.SidebarKeep{ChannelOrder: true})
	if st.SortKey != "" || st.Version != 3 || !slices.Equal(st.ChannelOrder, order1) {
		t.Fatalf("replace the key %+v", st)
	}
	// M4-51: no list stores NULL, the default order.
	put(domain.SidebarState{UserID: alex, SpaceID: research.ID}, app.SidebarKeep{})
	if st.ChannelOrder != nil || st.Version != 4 {
		t.Fatalf("clear the channel order %+v", st)
	}
	put(domain.SidebarState{UserID: alex, SpaceID: support.ID, Collapsed: true}, app.SidebarKeep{})
	list, err := e.repo.SidebarStates(ctx, alex, true, nil)
	if err != nil || len(list) != 1 || list[0].SpaceID != research.ID {
		t.Fatalf("sidebar states %+v, %v", list, err)
	}
	// The visible spaces of Scope (07 section 7.4 rule 1, AZ-P12): a space outside them is no row.
	if list, err := e.repo.SidebarStates(ctx, alex, false, []ids.ID{research.ID}); err != nil || len(list) != 1 {
		t.Fatalf("scoped sidebar states %+v, %v", list, err)
	}
	if list, err := e.repo.SidebarStates(ctx, alex, false, []ids.ID{support.ID}); err != nil || len(list) != 0 {
		t.Fatalf("sidebar states outside the scope %+v, %v", list, err)
	}
	if _, err := e.repo.PutSidebarState(ctx, st, app.SidebarKeep{}); err == nil {
		t.Fatal("sidebar write outside a unit of work")
	}
}

// write2 locks id in a unit of work.
func (e env) write2(t *testing.T, id ids.ID) (domain.Space, bool, error) {
	t.Helper()
	var (
		sp    domain.Space
		found bool
	)
	err := e.write(t, func(ctx context.Context) error {
		var err error
		sp, found, err = e.repo.LockSpace(ctx, id)
		return err
	})
	return sp, found, err
}

// mainThread returns the main thread of channel by creator at t0 with a search language, as the
// use case sets it before the insert (M3-13).
func mainThread(channel, creator ids.ID) domain.Thread {
	th := domain.NewMainThread(ids.New(), channel, creator, t0)
	th.SearchLanguage = "german"
	return th
}
