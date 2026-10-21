// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// sessionEnv is one test database with the session store, the agent Lotse and the main threads of
// two channels of the space Support.
type sessionEnv struct {
	env
	store    *postgres.SessionStore
	lotse    ids.ID
	threadA  ids.ID
	threadB  ids.ID
	threadC  ids.ID
	baseTime time.Time
}

func newSessionEnv(t *testing.T) sessionEnv {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	e := env{
		repo: postgres.NewRepository(pool), owner: db.Pool(t, pg.RoleOwner),
		uow: pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg),
	}
	ctx := context.Background()
	alex := e.principal(t, "user", "alex")
	lotse := e.principal(t, "agent", "lotse")
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		return e.repo.InsertAgent(ctx, domain.NewAgent(lotse, "custom", "", domain.OperatorConfig{}))
	}); err != nil {
		t.Fatal(err)
	}
	space := ids.New()
	if _, err := e.owner.Exec(ctx, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`,
		space.String(), alex.String()); err != nil {
		t.Fatal(err)
	}
	thread := func(name string) ids.ID {
		channel, id := ids.New(), ids.New()
		if _, err := e.owner.Exec(ctx, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'text', $3, $3, 20, $4)`, channel.String(), space.String(), name, alex.String()); err != nil {
			t.Fatal(err)
		}
		if _, err := e.owner.Exec(ctx, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`,
			id.String(), channel.String(), alex.String()); err != nil {
			t.Fatal(err)
		}
		return id
	}
	return sessionEnv{
		env: e, store: postgres.NewSessionStore(pool), lotse: lotse,
		threadA: thread("general"), threadB: thread("billing"), threadC: thread("ops"),
		baseTime: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC),
	}
}

// insert inserts s in a unit of work and returns the row and whether it was inserted.
func (e sessionEnv) insert(t *testing.T, s app.NewSession) (agents.Session, bool) {
	t.Helper()
	var row agents.Session
	var inserted bool
	if err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		row, inserted, err = e.store.InsertSession(ctx, s)
		return err
	}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}
	return row, inserted
}

// TestSessionStore_InsertAndRead covers InsertSession (current, ended, both conflicts, the default
// currency, NULL as zero values), the reads with their order and their misses, and that writes and
// locks need a unit of work.
func TestSessionStore_InsertAndRead(t *testing.T) {
	t.Parallel()
	e := newSessionEnv(t)
	ctx := context.Background()
	if rows, err := e.store.ThreadSessions(ctx, e.threadA); err != nil || len(rows) != 0 {
		t.Fatalf("ThreadSessions of an empty thread = %+v, %v", rows, err)
	}
	first := app.NewSession{
		ID: ids.New(), ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-1", Model: "vendor/model-1",
		Started: true, IsCurrent: true, CostBaselineMicros: 1500, CreatedAt: e.baseTime,
	}
	if _, _, err := e.store.InsertSession(ctx, first); err == nil {
		t.Fatal("InsertSession outside a unit of work")
	}
	if _, _, err := e.store.LockCurrentSession(ctx, e.threadA); err == nil {
		t.Fatal("LockCurrentSession outside a unit of work")
	}
	row, inserted := e.insert(t, first)
	want := agents.Session{
		ID: first.ID, ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-1", IsCurrent: true, Started: true,
		Model: "vendor/model-1", CostBaselineMicros: 1500, Currency: "USD", CostIsEstimate: true, CreatedAt: e.baseTime, Version: 1,
	}
	if !inserted || row != want {
		t.Fatalf("InsertSession = %+v, %v; want %+v", row, inserted, want)
	}
	// A second current row of the thread and the same reference current in another thread are
	// refused without an error.
	if _, inserted := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadA, AgentID: e.lotse,
		ExternalSessionRef: "sess-2", IsCurrent: true, CreatedAt: e.baseTime}); inserted {
		t.Fatal("a second current row of the thread was inserted")
	}
	if _, inserted := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadB, AgentID: e.lotse,
		ExternalSessionRef: "sess-1", IsCurrent: true, CreatedAt: e.baseTime}); inserted {
		t.Fatal("a reference current in another thread was inserted")
	}
	// An ended row of the same reference in another thread, with its own currency; then a newer
	// one. Equal created_at orders by id descending.
	ended := app.NewSession{
		ID: ids.New(), ThreadID: e.threadB, AgentID: e.lotse, ExternalSessionRef: "sess-1", Started: true, CostBaselineMicros: 700,
		Currency: "EUR", CreatedAt: e.baseTime.Add(time.Minute), EndedAt: e.baseTime.Add(time.Minute), EndReason: agents.EndImport,
	}
	endedRow, inserted := e.insert(t, ended)
	if !inserted || endedRow.IsCurrent || endedRow.EndReason != agents.EndImport || !endedRow.EndedAt.Equal(ended.EndedAt) ||
		endedRow.Currency != "EUR" || endedRow.Model != "" || endedRow.Version != 1 {
		t.Fatalf("ended row = %+v, %v", endedRow, inserted)
	}
	later := ended
	later.ID, later.CreatedAt, later.EndedAt = ids.New(), e.baseTime.Add(2*time.Minute), e.baseTime.Add(2*time.Minute)
	laterRow, _ := e.insert(t, later)
	same := later
	same.ID = ids.New()
	sameRow, _ := e.insert(t, same)
	newest, older := laterRow, sameRow
	if bytes.Compare(sameRow.ID[:], laterRow.ID[:]) > 0 {
		newest, older = sameRow, laterRow
	}

	list, err := e.store.ThreadSessions(ctx, e.threadB)
	if err != nil || len(list) != 3 || list[0] != newest || list[1] != older || list[2] != endedRow {
		t.Fatalf("ThreadSessions = %+v, %v", list, err)
	}
	if got, found, err := e.store.CurrentSession(ctx, e.threadA); err != nil || !found || got != want {
		t.Fatalf("CurrentSession = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.store.CurrentSession(ctx, e.threadB); err != nil || found {
		t.Fatalf("CurrentSession of a thread with only ended rows = %v, %v", found, err)
	}
	if got, found, err := e.store.CurrentByRef(ctx, e.lotse, "sess-1"); err != nil || !found || got != want {
		t.Fatalf("CurrentByRef = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.store.CurrentByRef(ctx, e.lotse, "sess-9"); err != nil || found {
		t.Fatalf("CurrentByRef unknown = %v, %v", found, err)
	}
	if got, found, err := e.store.LatestByRef(ctx, e.lotse, "sess-1"); err != nil || !found || got != newest {
		t.Fatalf("LatestByRef = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.store.LatestByRef(ctx, ids.New(), "sess-1"); err != nil || found {
		t.Fatalf("LatestByRef of another agent = %v, %v", found, err)
	}
	if got, found, err := e.store.LatestInThreadByRef(ctx, e.threadA, e.lotse, "sess-1"); err != nil || !found || got != want {
		t.Fatalf("LatestInThreadByRef = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.store.LatestInThreadByRef(ctx, e.threadC, e.lotse, "sess-1"); err != nil || found {
		t.Fatalf("LatestInThreadByRef of a thread without the reference = %v, %v", found, err)
	}
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		got, found, err := e.store.LockCurrentSession(ctx, e.threadA)
		if err != nil || !found || got != want {
			t.Errorf("LockCurrentSession = %+v, %v, %v", got, found, err)
		}
		if _, found, err := e.store.LockCurrentSession(ctx, e.threadC); err != nil || found {
			t.Errorf("LockCurrentSession of a thread without sessions = %v, %v", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestSessionStore_EndAndUpdate covers EndSession and UpdateSession: a stale version changes
// nothing, an ended row cannot end again, an update writes every field and raises the version,
// and a reference current in another thread is ErrSessionRefTaken.
func TestSessionStore_EndAndUpdate(t *testing.T) {
	t.Parallel()
	e := newSessionEnv(t)
	ctx := context.Background()
	a, _ := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-a",
		IsCurrent: true, CreatedAt: e.baseTime})
	if _, inserted := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadB, AgentID: e.lotse, ExternalSessionRef: "sess-b",
		IsCurrent: true, CreatedAt: e.baseTime}); !inserted {
		t.Fatal("fixture sess-b")
	}
	if _, _, err := e.store.EndSession(ctx, app.SessionEnd{ID: a.ID, Version: 1, Reason: agents.EndReset, At: e.baseTime}); err == nil {
		t.Fatal("EndSession outside a unit of work")
	}
	if _, _, err := e.store.UpdateSession(ctx, app.SessionChange{ID: a.ID, Version: 1, ExternalSessionRef: "x"}); err == nil {
		t.Fatal("UpdateSession outside a unit of work")
	}

	var updated agents.Session
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		if _, changed, err := e.store.UpdateSession(ctx, app.SessionChange{ID: a.ID, Version: 2, ExternalSessionRef: "x"}); err != nil || changed {
			t.Errorf("UpdateSession at a stale version = %v, %v", changed, err)
		}
		var changed bool
		var err error
		updated, changed, err = e.store.UpdateSession(ctx, app.SessionChange{ID: a.ID, Version: 1, ExternalSessionRef: "sess-a2",
			Model: "vendor/model-2", Started: true})
		if err != nil || !changed {
			t.Errorf("UpdateSession = %v, %v", changed, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if updated.ExternalSessionRef != "sess-a2" || updated.Model != "vendor/model-2" || !updated.Started || updated.Version != 2 || !updated.IsCurrent {
		t.Fatalf("updated = %+v", updated)
	}
	// Model "" writes NULL; started false is written as given (the use case keeps it monotone).
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		row, changed, err := e.store.UpdateSession(ctx, app.SessionChange{ID: a.ID, Version: 2, ExternalSessionRef: "sess-a2"})
		if err != nil || !changed || row.Model != "" || row.Started || row.Version != 3 {
			t.Errorf("UpdateSession clearing the model = %+v, %v, %v", row, changed, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	err := e.uow.Do(ctx, func(ctx context.Context) error {
		_, _, err := e.store.UpdateSession(ctx, app.SessionChange{ID: a.ID, Version: 3, ExternalSessionRef: "sess-b"})
		return err
	})
	if !errors.Is(err, app.ErrSessionRefTaken) {
		t.Fatalf("UpdateSession to a reference current elsewhere = %v, want ErrSessionRefTaken", err)
	}

	end := app.SessionEnd{ID: a.ID, Version: 3, Reason: agents.EndRebind, At: e.baseTime.Add(time.Hour)}
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		if _, changed, err := e.store.EndSession(ctx, app.SessionEnd{ID: a.ID, Version: 1, Reason: agents.EndReset, At: end.At}); err != nil || changed {
			t.Errorf("EndSession at a stale version = %v, %v", changed, err)
		}
		row, changed, err := e.store.EndSession(ctx, end)
		if err != nil || !changed || row.IsCurrent || !row.EndedAt.Equal(end.At) || row.EndReason != agents.EndRebind || row.Version != 4 {
			t.Errorf("EndSession = %+v, %v, %v", row, changed, err)
		}
		if _, changed, err := e.store.EndSession(ctx, app.SessionEnd{ID: a.ID, Version: 4, Reason: agents.EndReset, At: end.At}); err != nil || changed {
			t.Errorf("EndSession of an ended row = %v, %v", changed, err)
		}
		if _, changed, err := e.store.EndSession(ctx, app.SessionEnd{ID: ids.New(), Version: 1, Reason: agents.EndReset, At: end.At}); err != nil || changed {
			t.Errorf("EndSession of an unknown row = %v, %v", changed, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The ended reference is free: another thread may make it current, here at a given first
	// version (the replacement row after bind_failed starts above the replaced one).
	if row, inserted := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadC, AgentID: e.lotse, ExternalSessionRef: "sess-a2",
		IsCurrent: true, CreatedAt: e.baseTime, Version: 5}); !inserted || row.Version != 5 {
		t.Fatalf("an ended reference made current in another thread at version 5 = %+v, %v", row, inserted)
	}
}

// TestSessionStore_LockSessions: the rows come back ordered by id, unknown and repeated ids are
// left out, and the locks are taken in ascending id. While another transaction holds the lower
// row, LockSessions waits on it and has not locked the higher row yet.
func TestSessionStore_LockSessions(t *testing.T) {
	t.Parallel()
	e := newSessionEnv(t)
	ctx := context.Background()
	x, _ := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-x",
		IsCurrent: true, CreatedAt: e.baseTime})
	y, _ := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadB, AgentID: e.lotse, ExternalSessionRef: "sess-y",
		IsCurrent: true, CreatedAt: e.baseTime})
	low, high := x, y
	if bytes.Compare(x.ID[:], y.ID[:]) > 0 {
		low, high = y, x
	}
	if _, err := e.store.LockSessions(ctx, []ids.ID{low.ID}); err == nil {
		t.Fatal("LockSessions outside a unit of work")
	}
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		rows, err := e.store.LockSessions(ctx, []ids.ID{high.ID, low.ID, ids.New(), high.ID})
		if err != nil || len(rows) != 2 || rows[0] != low || rows[1] != high {
			t.Errorf("LockSessions = %+v, %v", rows, err)
		}
		if rows, err := e.store.LockSessions(ctx, nil); err != nil || len(rows) != 0 {
			t.Errorf("LockSessions empty = %+v, %v", rows, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	holder, err := e.owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	var holderPID int32
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid() FROM agent_sessions WHERE id = $1 FOR UPDATE`, low.ID.String()).
		Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- e.uow.Do(ctx, func(ctx context.Context) error {
			_, err := e.store.LockSessions(ctx, []ids.ID{high.ID, low.ID})
			return err
		})
	}()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var waiting int
		if err := e.owner.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY (pg_blocking_pids(pid))`,
			holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("LockSessions never waited on the lower row")
		}
		time.Sleep(20 * time.Millisecond)
	}
	probe, err := e.owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := probe.Exec(ctx, `SELECT 1 FROM agent_sessions WHERE id = $1 FOR UPDATE NOWAIT`, high.ID.String()); err != nil {
		t.Errorf("the higher row was locked before the lower one: %v", err)
	}
	_ = probe.Rollback(ctx)
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("LockSessions after the holder ended: %v", err)
	}
}
