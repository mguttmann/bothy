// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// env is a migrated database with a scratch table ranges (lo, hi) bothy_app may write, a pool of
// bothy_app and the counters over it.
type env struct {
	db       *pgtest.DB
	pool     *pg.Pool
	cfg      pg.UnitConfig
	counters instance.Counters
}

func newEnv(t *testing.T, maxConns int) *env {
	t.Helper()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	for _, q := range []string{
		"CREATE TABLE ranges (lo bigint PRIMARY KEY, hi bigint NOT NULL)",
		"GRANT SELECT, INSERT ON ranges TO bothy_app",
	} {
		if _, err := super.Exec(context.Background(), q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = maxConns
	pool := db.OpenPool(t, cfg)
	return &env{
		db: db, pool: pool,
		cfg:      pg.UnitConfig{RetryMax: 5, StatementTimeout: cfg.StatementTimeout, LockTimeout: cfg.LockTimeout, IdleInTxTimeout: cfg.IdleInTxTimeout},
		counters: app.NewCounters(postgres.NewRepository(pool)),
	}
}

// uow seals reg and returns a unit of work over the pool of e.
func (e *env) uow(reg *txn.Registry) *pg.UnitOfWork {
	reg.Seal()
	return pg.NewUnitOfWork(e.pool, e.cfg, reg)
}

// committed returns the committed value of a counter, read as bothy_app outside any unit.
func (e *env) committed(t *testing.T, name string) int64 {
	t.Helper()
	var v int64
	if err := e.pool.QueryRow(context.Background(), "SELECT value FROM sequence_counters WHERE name = $1", name).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// span is a committed range of numbers.
type span struct{ lo, hi int64 }

// visible reads the committed ranges ordered by lo.
func (e *env) visible(ctx context.Context) ([]span, error) {
	rows, err := e.pool.Query(ctx, "SELECT lo, hi FROM ranges ORDER BY lo")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (span, error) {
		var s span
		err := r.Scan(&s.lo, &s.hi)
		return s, err
	})
}

// contiguous reports whether spans form one block 1..end without gap or overlap, and its end.
func contiguous(spans []span) (int64, error) {
	next := int64(1)
	for _, s := range spans {
		if s.lo != next || s.hi < s.lo {
			return 0, fmt.Errorf("range %d..%d after %d: gap or overlap", s.lo, s.hi, next-1)
		}
		next = s.hi + 1
	}
	return next - 1, nil
}

// countKey carries the k of one unit from the work phase to its sync hook.
var countKey = txn.NewKey[int64]("numbers of this unit")

// errRollback makes a unit roll back after its hook advanced the counter.
var errRollback = errors.New("rollback on purpose")

// syncHook is the test sync hook: Advance(event_seq, k), a random wait while it holds the lock,
// the range into ranges, and a rollback when the unit asked for one. It is a function of this
// test package, so the registry derives the module instance from its path.
type syncHook struct {
	counters instance.Counters
	rollback func(ctx context.Context) bool
}

func (h *syncHook) run(ctx context.Context) error {
	k, ok := txn.Load(ctx, countKey)
	if !ok {
		return errors.New("no k stored in the work phase")
	}
	last, err := h.counters.Advance(ctx, instance.CounterEventSeq, k)
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(rand.IntN(3000)) * time.Microsecond) //nolint:gosec // test jitter
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO ranges (lo, hi) VALUES ($1, $2)", last-k+1, last); err != nil {
		return err
	}
	if h.rollback(ctx) {
		return errRollback
	}
	return nil
}

// rollbackKey marks a unit that must roll back.
var rollbackKey = txn.NewKey[bool]("roll back")

// TestCounters_ConcurrentAdvanceGaplessInCommitOrder: many units advance event_seq by random k in
// their sync hook, wait while they hold the lock and roll back at random; a reader polling the
// committed ranges always sees one block from 1, and at the end the block ends at the committed
// counter (04 "Muster mit Zähler-Zeile" items 1 to 4).
func TestCounters_ConcurrentAdvanceGaplessInCommitOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 12)
	reg := txn.NewRegistry()
	h := &syncHook{counters: e.counters, rollback: func(ctx context.Context) bool {
		v, _ := txn.Load(ctx, rollbackKey)
		return v
	}}
	if err := reg.SetHook(txn.PhaseSync, h.run); err != nil {
		t.Fatal(err)
	}
	uow := e.uow(reg)

	ctx, stop := context.WithCancel(context.Background())
	var reads, readerErr atomic.Value
	readerDone, firstRead := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(readerDone)
		n := 0
		defer func() {
			reads.Store(n)
			if n == 0 {
				close(firstRead)
			}
		}()
		for ctx.Err() == nil {
			spans, err := e.visible(ctx)
			if err != nil {
				if ctx.Err() == nil {
					readerErr.Store(err)
				}
				return
			}
			if _, err := contiguous(spans); err != nil {
				readerErr.Store(fmt.Errorf("read %d: %w", n, err))
				return
			}
			if n == 0 {
				close(firstRead)
			}
			n++
		}
	}()

	<-firstRead // the units start once the reader reads, so it observes them while they commit
	const workers, units = 10, 15
	var wg sync.WaitGroup
	var committed, rolledBack atomic.Int64
	errCh := make(chan error, workers*units)
	for w := range workers {
		wg.Go(func() {
			for u := range units {
				k := int64(1 + rand.IntN(5))  //nolint:gosec // test data
				rollback := rand.IntN(4) == 0 //nolint:gosec // test data
				err := uow.Do(context.Background(), func(ctx context.Context) error {
					if err := txn.Store(ctx, countKey, k); err != nil {
						return err
					}
					return txn.Store(ctx, rollbackKey, rollback)
				})
				switch {
				case rollback && errors.Is(err, errRollback):
					rolledBack.Add(1)
				case !rollback && err == nil:
					committed.Add(k)
				default:
					errCh <- fmt.Errorf("worker %d unit %d (rollback %v): %w", w, u, rollback, err)
				}
			}
		})
	}
	wg.Wait()
	stop()
	<-readerDone
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if err, ok := readerErr.Load().(error); ok {
		t.Fatalf("reader: %v", err)
	}
	spans, err := e.visible(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	end, err := contiguous(spans)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.committed(t, "event_seq"); end != got || end != committed.Load() {
		t.Fatalf("block ends at %d, committed counter %d, committed numbers %d", end, got, committed.Load())
	}
	if n, _ := reads.Load().(int); n < 2 {
		t.Fatalf("the reader read %d times; the test proves nothing about visibility", n)
	}
	if rolledBack.Load() == 0 {
		t.Fatal("no unit rolled back; the test proves nothing about rollbacks")
	}
	t.Logf("%d numbers committed, %d units rolled back, %v reads", end, rolledBack.Load(), reads.Load())
}

// TestCounters_RollbackLeavesNoGap: numbers of a rolled-back unit are handed out again.
func TestCounters_RollbackLeavesNoGap(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 4)
	reg := txn.NewRegistry()
	var got []int64
	h := &advanceHook{counters: e.counters, name: instance.CounterAuditSeq, n: 3, seen: &got}
	if err := reg.SetHook(txn.PhaseAudit, h.run); err != nil {
		t.Fatal(err)
	}
	uow := e.uow(reg)
	boom := errors.New("veto after the counter")
	h.fail = boom
	if err := uow.Do(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, boom) {
		t.Fatalf("first unit = %v, want the veto", err)
	}
	h.fail = nil
	if err := uow.Do(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 3 || got[1] != 3 {
		t.Fatalf("last values %v, want [3 3]: the rolled-back numbers must come back", got)
	}
	if v := e.committed(t, "audit_seq"); v != 3 {
		t.Fatalf("committed audit_seq %d, want 3", v)
	}
}

// advanceHook advances name by n and records the result; fail vetoes after the advance.
type advanceHook struct {
	counters instance.Counters
	name     instance.CounterName
	n        int64
	seen     *[]int64
	fail     error
}

func (h *advanceHook) run(ctx context.Context) error {
	last, err := h.counters.Advance(ctx, h.name, h.n)
	if err != nil {
		return err
	}
	*h.seen = append(*h.seen, last)
	return h.fail
}

// probeHook runs probe in its phase and records the result.
type probeHook struct {
	probe func(ctx context.Context) error
	err   error
}

func (h *probeHook) run(ctx context.Context) error {
	h.err = h.probe(ctx)
	return nil
}

// TestCounters_PhaseGuard: Advance and Current against a real database in each phase of a unit.
func TestCounters_PhaseGuard(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 4)
	type call struct {
		phase txn.Phase
		what  string
		probe func(ctx context.Context) error
		ok    bool
	}
	adv := func(n instance.CounterName) func(context.Context) error {
		return func(ctx context.Context) error { _, err := e.counters.Advance(ctx, n, 1); return err }
	}
	cur := func(ctx context.Context) error {
		_, err := e.counters.Current(ctx, instance.CounterEventSeq)
		return err
	}
	calls := []call{
		{txn.PhaseWork, "Advance(event_seq) in work", adv(instance.CounterEventSeq), false},
		{txn.PhaseWork, "Advance(permission_version) in work", adv(instance.CounterPermissionVersion), false},
		{txn.PhaseAudit, "Advance(event_seq) in audit", adv(instance.CounterEventSeq), false},
		{txn.PhaseSync, "Advance(audit_seq) in sync", adv(instance.CounterAuditSeq), false},
		{txn.PhaseSync, "Advance(permission_version) in sync", adv(instance.CounterPermissionVersion), false},
		{txn.PhaseRightsCheck, "Advance(permission_version) in rights_check", adv(instance.CounterPermissionVersion), false},
		{txn.PhaseWork, "Current in work", cur, false},
		{txn.PhaseRightsBegin, "Advance(permission_version) in rights_begin", adv(instance.CounterPermissionVersion), true},
		{txn.PhaseSync, "Advance(event_seq) in sync", adv(instance.CounterEventSeq), true},
		{txn.PhaseAudit, "Advance(audit_seq) in audit", adv(instance.CounterAuditSeq), true},
		{txn.PhaseRightsBegin, "Current in rights_begin", cur, true},
		{txn.PhaseRightsCheck, "Current in rights_check", cur, true},
		{txn.PhaseSync, "Current in sync", cur, true},
		{txn.PhaseAudit, "Current in audit", cur, true},
	}
	for _, c := range calls {
		reg := txn.NewRegistry()
		var got error
		var h *probeHook
		if c.phase != txn.PhaseWork {
			h = &probeHook{probe: c.probe}
			if err := reg.SetHook(c.phase, h.run); err != nil {
				t.Fatal(err)
			}
		}
		uow := e.uow(reg)
		err := uow.Do(context.Background(), func(ctx context.Context) error {
			if c.phase == txn.PhaseWork {
				got = c.probe(ctx)
			}
			return nil
		}, txn.RightsChange())
		if err != nil {
			t.Fatalf("%s: unit failed: %v", c.what, err)
		}
		if h != nil {
			got = h.err
		}
		if c.ok != (got == nil) {
			t.Errorf("%s = %v, want ok %v", c.what, got, c.ok)
		}
		if got != nil && errs.KindOf(got) != errs.KindInternal {
			t.Errorf("%s = %v, want kind internal", c.what, got)
		}
	}
	// Outside a unit of work there is no phase at all.
	if _, err := e.counters.Advance(context.Background(), instance.CounterEventSeq, 1); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Advance outside a unit = %v", err)
	}
	if _, err := e.counters.Current(context.Background(), instance.CounterEventSeq); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Current outside a unit = %v", err)
	}
}

// TestCounters_RejectsUnknownNameAndNonPositiveN: rejected in the domain before SQL, so the
// committed value stays unchanged; the table CHECK and the missing row are the second line.
func TestCounters_RejectsUnknownNameAndNonPositiveN(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 4)
	reg := txn.NewRegistry()
	var errsSeen []error
	h := &probeHook{probe: func(ctx context.Context) error {
		for _, p := range []func() error{
			func() error { _, err := e.counters.Advance(ctx, "thread_seq", 1); return err },
			func() error { _, err := e.counters.Advance(ctx, instance.CounterEventSeq, 0); return err },
			func() error { _, err := e.counters.Advance(ctx, instance.CounterEventSeq, -3); return err },
			func() error { _, err := e.counters.Current(ctx, "thread_seq"); return err },
		} {
			errsSeen = append(errsSeen, p())
		}
		return nil
	}}
	if err := reg.SetHook(txn.PhaseSync, h.run); err != nil {
		t.Fatal(err)
	}
	if err := e.uow(reg).Do(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for i, err := range errsSeen {
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("call %d = %v, want kind internal", i, err)
		}
	}
	if v := e.committed(t, "event_seq"); v != 0 {
		t.Errorf("event_seq is %d after rejected calls", v)
	}
	// The repository alone (no domain check) meets the table: an unknown name updates no row.
	repo := postgres.NewRepository(e.pool)
	reg2 := txn.NewRegistry()
	h2 := &probeHook{probe: func(ctx context.Context) error { _, err := repo.AdvanceCounter(ctx, "thread_seq", 1); return err }}
	if err := reg2.SetHook(txn.PhaseSync, h2.run); err != nil {
		t.Fatal(err)
	}
	if err := e.uow(reg2).Do(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if h2.err == nil {
		t.Error("AdvanceCounter on a missing row succeeded")
	}
}
