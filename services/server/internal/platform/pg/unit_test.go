// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/internal/fakemodule"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// unitEnv is the database, pool and scratch table of one unit-of-work test.
type unitEnv struct {
	db   *pgtest.DB
	pool *pg.Pool
	cfg  pg.UnitConfig
}

// newUnitEnv creates a database with the scratch table items (id int primary key, v text) that
// bothy_app may write, and a pool of bothy_app.
func newUnitEnv(t *testing.T) *unitEnv {
	t.Helper()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	super := db.SuperConn(t)
	for _, q := range []string{
		"CREATE TABLE items (id int PRIMARY KEY, v text NOT NULL)",
		"INSERT INTO items VALUES (1, 'a'), (2, 'b')",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON items TO bothy_app",
	} {
		if _, err := super.Exec(context.Background(), q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	cfg := db.Config(pg.RoleApp)
	return &unitEnv{
		db: db, pool: db.Pool(t, pg.RoleApp),
		cfg: pg.UnitConfig{RetryMax: 2, StatementTimeout: cfg.StatementTimeout, LockTimeout: cfg.LockTimeout, IdleInTxTimeout: cfg.IdleInTxTimeout},
	}
}

// uow returns a unit of work over the pool of e with reg, sealing reg.
func (e *unitEnv) uow(reg *txn.Registry) *pg.UnitOfWork {
	reg.Seal()
	return pg.NewUnitOfWork(e.pool, e.cfg, reg)
}

// insert writes the row (id, v) in the ambient transaction of ctx.
func insert(ctx context.Context, id int, v string) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO items VALUES ($1, $2)", id, v)
	return err
}

// rowExists reports whether the row id is committed.
func (e *unitEnv) rowExists(t *testing.T, id int) bool {
	t.Helper()
	var n int
	if err := pg.Querier(context.Background(), e.pool).QueryRow(context.Background(),
		"SELECT count(*) FROM items WHERE id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// phaseLog records the phases hooks and the work function ran in.
type phaseLog struct {
	mu     sync.Mutex
	phases []string
}

func (l *phaseLog) add(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.phases = append(l.phases, txn.CurrentPhase(ctx).String())
}

func (l *phaseLog) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.phases)
}

// recordingRegistry returns a registry whose four hooks record their phase into l; fail, when
// set for a phase, is returned by that hook.
func recordingRegistry(t *testing.T, l *phaseLog, fail map[txn.Phase]error) *txn.Registry {
	t.Helper()
	reg := txn.NewRegistry()
	for _, p := range []txn.Phase{txn.PhaseRightsBegin, txn.PhaseRightsCheck, txn.PhaseSync, txn.PhaseAudit} {
		if err := reg.SetHook(p, fakemodule.Hook(func(ctx context.Context) error {
			l.add(ctx)
			return fail[p]
		})); err != nil {
			t.Fatal(err)
		}
	}
	return reg
}

func TestUnitOfWork_PhaseOrderWithRightsChange(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	uow := e.uow(recordingRegistry(t, &l, nil))
	p := ids.New()
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		l.add(ctx)
		if got, ok := txn.AffectedPrincipals(ctx); !ok || !slices.Equal(got, []ids.ID{p}) {
			t.Errorf("AffectedPrincipals = %v, %v", got, ok)
		}
		return nil
	}, txn.RightsChange(p))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"rights_begin", "work", "rights_check", "sync", "audit"}; !slices.Equal(l.get(), want) {
		t.Fatalf("phases = %v, want %v", l.get(), want)
	}
}

func TestUnitOfWork_PhaseOrderWithoutRightsChange(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	uow := e.uow(recordingRegistry(t, &l, nil))
	if err := uow.Do(context.Background(), func(ctx context.Context) error { l.add(ctx); return nil }); err != nil {
		t.Fatal(err)
	}
	if want := []string{"work", "sync", "audit"}; !slices.Equal(l.get(), want) {
		t.Fatalf("phases = %v, want %v", l.get(), want)
	}
}

func TestUnitOfWork_MissingHooksAreSkipped(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	if err := uow.Do(context.Background(), func(ctx context.Context) error { return insert(ctx, 3, "c") },
		txn.RightsChange()); err != nil {
		t.Fatal(err)
	}
	if !e.rowExists(t, 3) {
		t.Fatal("the row was not committed")
	}
}

func TestUnitOfWork_SubscriberVetoRollsBack(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	reg := recordingRegistry(t, &l, nil)
	veto := errs.New(errs.CodeConflict, "vetoed")
	if err := txn.Subscribe(reg, fakemodule.Handler(func(ctx context.Context, ev fakemodule.Event) error {
		if err := insert(ctx, 11, "from subscriber"); err != nil {
			return err
		}
		return veto
	})); err != nil {
		t.Fatal(err)
	}
	uow := e.uow(reg)
	buffered := txn.NewKey[[]int]("buffered events")
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		if err := insert(ctx, 10, "x"); err != nil {
			return err
		}
		if err := txn.Store(ctx, buffered, []int{1}); err != nil {
			return err
		}
		return txn.Publish(ctx, fakemodule.Event{N: 1})
	})
	if !errors.Is(err, veto) {
		t.Fatalf("Do = %v, want the veto", err)
	}
	if e.rowExists(t, 10) || e.rowExists(t, 11) {
		t.Fatal("rows of a vetoed unit were committed")
	}
	if got := l.get(); len(got) != 0 {
		t.Fatalf("hooks ran after the veto: %v", got)
	}
	err = uow.Do(context.Background(), func(ctx context.Context) error {
		if v, ok := txn.Load(ctx, buffered); ok {
			t.Errorf("a new unit sees the value %v of the vetoed one", v)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnitOfWork_RightsCheckVetoRollsBack(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	violation := errs.New(errs.CodeI1Violation, "no owner left")
	uow := e.uow(recordingRegistry(t, &l, map[txn.Phase]error{txn.PhaseRightsCheck: violation}))
	err := uow.Do(context.Background(), func(ctx context.Context) error { return insert(ctx, 20, "x") }, txn.RightsChange())
	if !errors.Is(err, violation) || errs.KindOf(err) != errs.KindInvariantViolation {
		t.Fatalf("Do = %v, want the invariant_violation", err)
	}
	if e.rowExists(t, 20) {
		t.Fatal("the row of a vetoed unit was committed")
	}
	if want := []string{"rights_begin", "rights_check"}; !slices.Equal(l.get(), want) {
		t.Fatalf("phases = %v, want %v (sync and audit must not run)", l.get(), want)
	}
}

func TestUnitOfWork_HookErrorRollsBack(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	boom := errors.New("audit failed")
	uow := e.uow(recordingRegistry(t, &l, map[txn.Phase]error{txn.PhaseAudit: boom}))
	if err := uow.Do(context.Background(), func(ctx context.Context) error { return insert(ctx, 21, "x") }); !errors.Is(err, boom) {
		t.Fatalf("Do = %v", err)
	}
	if e.rowExists(t, 21) {
		t.Fatal("the row was committed despite a failed audit hook")
	}
}

// failFirst returns a work function that fails its first k calls with the SQLSTATE code and
// counts all calls in n.
func failFirst(k int32, code string, n *atomic.Int32) func(context.Context) error {
	return func(context.Context) error {
		if n.Add(1) <= k {
			return &pgconn.PgError{Code: code, Message: "simulated"}
		}
		return nil
	}
}

func testRetriesUpToMax(t *testing.T, code string) {
	t.Helper()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	var n atomic.Int32
	if err := uow.Do(context.Background(), failFirst(2, code, &n)); err != nil {
		t.Fatalf("Do with %s twice = %v, want success", code, err)
	}
	if n.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", n.Load())
	}
	n.Store(0)
	err := uow.Do(context.Background(), failFirst(3, code, &n))
	if n.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", n.Load())
	}
	var pgErr *pgconn.PgError
	if errs.KindOf(err) != errs.KindUnavailable || !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("Do = %v, want kind unavailable wrapping the %s error", err, code)
	}
	if e2, _ := errs.As(err); e2.PublicCode() != errs.CodeUnavailable {
		t.Fatalf("code = %s, want unavailable", e2.PublicCode())
	}
}

func TestUnitOfWork_RetriesDeadlockUpToMax(t *testing.T) {
	t.Parallel()
	testRetriesUpToMax(t, "40P01")
}

func TestUnitOfWork_RetriesLockTimeoutUpToMax(t *testing.T) {
	t.Parallel()
	testRetriesUpToMax(t, "55P03")
}

func TestUnitOfWork_RetryMaxZero(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	e.cfg.RetryMax = 0
	uow := e.uow(txn.NewRegistry())
	var n atomic.Int32
	if err := uow.Do(context.Background(), failFirst(1, "40P01", &n)); errs.KindOf(err) != errs.KindUnavailable || n.Load() != 1 {
		t.Fatalf("Do = %v after %d attempts, want unavailable after 1", err, n.Load())
	}
}

func TestUnitOfWork_DoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	for _, code := range []string{"23505", "57014"} {
		var n atomic.Int32
		err := uow.Do(context.Background(), failFirst(5, code, &n))
		var pgErr *pgconn.PgError
		if n.Load() != 1 || !errors.As(err, &pgErr) || pgErr.Code != code || errs.KindOf(err) == errs.KindUnavailable {
			t.Errorf("%s: Do = %v after %d attempts, want the error unchanged after 1", code, err, n.Load())
		}
	}
	plain := errors.New("plain")
	var n atomic.Int32
	if err := uow.Do(context.Background(), func(context.Context) error { n.Add(1); return plain }); err != plain || n.Load() != 1 { //nolint:errorlint // unchanged
		t.Errorf("Do = %v after %d attempts", err, n.Load())
	}
}

func TestUnitOfWork_RealDeadlockIsRetried(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	var first sync.WaitGroup
	first.Add(2)
	var attempts [2]atomic.Int32
	// update runs one unit: lock row a, wait until both units hold their first row (first
	// attempt only), then lock row b.
	update := func(i, a, b int) error {
		return uow.Do(context.Background(), func(ctx context.Context) error {
			n := attempts[i].Add(1)
			tx, err := pg.Tx(ctx)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "UPDATE items SET v = v || $2 WHERE id = $1", a, "x"); err != nil {
				return err
			}
			if n == 1 {
				first.Done()
				first.Wait()
			}
			_, err = tx.Exec(ctx, "UPDATE items SET v = v || $2 WHERE id = $1", b, "y")
			return err
		})
	}
	errsCh := make(chan error, 2)
	go func() { errsCh <- update(0, 1, 2) }()
	go func() { errsCh <- update(1, 2, 1) }()
	for range 2 {
		if err := <-errsCh; err != nil {
			t.Fatalf("a unit failed: %v", err)
		}
	}
	// At least one unit is a deadlock victim and runs again, and each unit commits exactly once
	// (checked below). How many attempts each side needs is not fixed: CI runs saw both units
	// retry (2 and 2), so the test does not demand exactly one retry.
	if total := attempts[0].Load() + attempts[1].Load(); total < 3 {
		t.Fatalf("attempts = %d and %d, want at least one retry", attempts[0].Load(), attempts[1].Load())
	}
	var v1, v2 string
	q := pg.Querier(context.Background(), e.pool)
	if err := q.QueryRow(context.Background(), "SELECT (SELECT v FROM items WHERE id = 1), (SELECT v FROM items WHERE id = 2)").Scan(&v1, &v2); err != nil {
		t.Fatal(err)
	}
	if len(v1) != 3 || len(v2) != 3 {
		t.Fatalf("rows = %q, %q; want both units committed once", v1, v2)
	}
}

func TestUnitOfWork_FreshStatePerAttempt(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	k := txn.NewKey[string]("attempt")
	var n atomic.Int32
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		if v, ok := txn.Load(ctx, k); ok {
			t.Errorf("attempt %d sees %q of an earlier attempt", n.Load()+1, v)
		}
		if err := txn.Store(ctx, k, "first"); err != nil {
			return err
		}
		if n.Add(1) == 1 {
			return &pgconn.PgError{Code: "40P01"}
		}
		return nil
	})
	if err != nil || n.Load() != 2 {
		t.Fatalf("Do = %v after %d attempts", err, n.Load())
	}
}

// txid returns the transaction id of the ambient transaction.
func txid(t *testing.T, ctx context.Context) int64 {
	t.Helper()
	var id int64
	if err := pg.Querier(ctx, nil).QueryRow(ctx, "SELECT txid_current()").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestUnitOfWork_NestedJoins(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	uow := e.uow(recordingRegistry(t, &l, nil))
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		outer := txid(t, ctx)
		return uow.Do(ctx, func(ctx context.Context) error {
			if inner := txid(t, ctx); inner != outer {
				t.Errorf("inner txid %d, outer %d", inner, outer)
			}
			if txn.CurrentPhase(ctx) != txn.PhaseWork {
				t.Errorf("inner phase = %s", txn.CurrentPhase(ctx))
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"sync", "audit"}; !slices.Equal(l.get(), want) {
		t.Fatalf("hooks = %v, want one run of %v", l.get(), want)
	}

	boom := errors.New("inner failed")
	err = uow.Do(context.Background(), func(ctx context.Context) error {
		if err := insert(ctx, 30, "outer"); err != nil {
			return err
		}
		return uow.Do(ctx, func(context.Context) error { return boom })
	})
	if !errors.Is(err, boom) || e.rowExists(t, 30) {
		t.Fatalf("Do = %v; the inner error must roll back the outer work", err)
	}

	var outerCalls, innerCalls atomic.Int32
	err = uow.Do(context.Background(), func(ctx context.Context) error {
		outerCalls.Add(1)
		return uow.Do(ctx, func(context.Context) error {
			if innerCalls.Add(1) == 1 {
				return &pgconn.PgError{Code: "40P01"}
			}
			return nil
		})
	})
	if err != nil || outerCalls.Load() != 2 || innerCalls.Load() != 2 {
		t.Fatalf("Do = %v with %d outer and %d inner calls; only the outer unit may retry", err, outerCalls.Load(), innerCalls.Load())
	}
}

func TestUnitOfWork_NestedRightsChangeRejected(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	p, q := ids.New(), ids.New()
	nested := func(outer, inner []txn.Option) error {
		return uow.Do(context.Background(), func(ctx context.Context) error {
			return uow.Do(ctx, func(context.Context) error { return nil }, inner...)
		}, outer...)
	}
	if err := nested(nil, []txn.Option{txn.RightsChange(p)}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("nested RightsChange without outer = %v, want kind internal", err)
	}
	if err := nested([]txn.Option{txn.RightsChange(p)}, []txn.Option{txn.RightsChange(q)}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("nested RightsChange outside the outer principals = %v, want kind internal", err)
	}
	if err := nested([]txn.Option{txn.RightsChange(p, q)}, []txn.Option{txn.RightsChange(q)}); err != nil {
		t.Errorf("covered nested RightsChange = %v", err)
	}
}

func TestUnitOfWork_ReadCommittedEnforced(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	if _, err := e.db.SuperConn(t).Exec(context.Background(),
		"ALTER DATABASE "+pgx.Identifier{e.db.Name()}.Sanitize()+" SET default_transaction_isolation = 'serializable'"); err != nil {
		t.Fatal(err)
	}
	cfg := e.db.Config(pg.RoleApp)
	cfg.MaxConns = 1
	pool := e.db.OpenPool(t, cfg)
	// The pool starts its sessions with read committed; set the session default of its one
	// connection to serializable as well, so only the explicit isolation of Do is left.
	c, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(context.Background(), "SET default_transaction_isolation = 'serializable'"); err != nil {
		t.Fatal(err)
	}
	if got := show(t, c, "transaction_isolation"); got != "serializable" {
		t.Fatalf("session isolation = %q, want serializable before Do", got)
	}
	c.Release()
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, e.cfg, reg)
	err = uow.Do(context.Background(), func(ctx context.Context) error {
		tx, err := pg.Tx(ctx)
		if err != nil {
			return err
		}
		if got := show(t, tx, "transaction_isolation"); got != "read committed" {
			t.Errorf("transaction_isolation in Do = %q, want read committed", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnitOfWork_TimeoutsApplied(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	e.cfg.StatementTimeout, e.cfg.LockTimeout, e.cfg.IdleInTxTimeout = 1234*time.Millisecond, 2345*time.Millisecond, 3456*time.Millisecond
	uow := e.uow(txn.NewRegistry())
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		tx, err := pg.Tx(ctx)
		if err != nil {
			return err
		}
		for name, want := range map[string]string{
			"statement_timeout": "1234ms", "lock_timeout": "2345ms", "idle_in_transaction_session_timeout": "3456ms",
		} {
			if got := show(t, tx, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnitOfWork_PanicRollsBackAndRepanics(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Errorf("recovered %v, want the panic value boom", r)
			}
		}()
		_ = uow.Do(context.Background(), func(ctx context.Context) error {
			if err := insert(ctx, 40, "x"); err != nil {
				return err
			}
			panic("boom")
		})
		t.Error("Do returned after a panic")
	}()
	if e.rowExists(t, 40) {
		t.Fatal("the row of a panicking unit was committed")
	}
	// The connection went back to the pool usable.
	if err := uow.Do(context.Background(), func(ctx context.Context) error { return insert(ctx, 41, "y") }); err != nil || !e.rowExists(t, 41) {
		t.Fatalf("Do after a panic = %v", err)
	}
}

func TestUnitOfWork_NoRetryAfterCancel(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	var n atomic.Int32
	err := uow.Do(ctx, func(ctx context.Context) error {
		n.Add(1)
		if err := insert(ctx, 50, "x"); err != nil {
			return err
		}
		cancel()
		return &pgconn.PgError{Code: "40P01"}
	})
	var pgErr *pgconn.PgError
	if n.Load() != 1 || !errors.As(err, &pgErr) || errs.KindOf(err) == errs.KindUnavailable {
		t.Fatalf("Do = %v after %d attempts, want the error unchanged after 1", err, n.Load())
	}
	if e.rowExists(t, 50) {
		t.Fatal("the row of a canceled unit was committed")
	}
}

func TestUnitOfWork_CanceledContextAbortsCommit(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	err := uow.Do(ctx, func(ctx context.Context) error {
		if err := insert(ctx, 51, "x"); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do = %v, want the cancellation", err)
	}
	if e.rowExists(t, 51) {
		t.Fatal("the row of a canceled unit was committed")
	}
}

func TestUnitOfWork_RefusesBeforeSeal(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := pg.NewUnitOfWork(e.pool, e.cfg, txn.NewRegistry())
	ran := false
	err := uow.Do(context.Background(), func(context.Context) error { ran = true; return nil })
	if errs.KindOf(err) != errs.KindInternal || ran {
		t.Fatalf("Do before Seal = %v (ran %v), want kind internal without running", err, ran)
	}
	if err := pg.NewUnitOfWork(e.pool, e.cfg, nil).Do(context.Background(), func(context.Context) error { return nil }); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Do without registry = %v", err)
	}
	reg := txn.NewRegistry()
	reg.Seal()
	if err := pg.NewUnitOfWork(e.pool, e.cfg, reg).Do(context.Background(), nil); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Do with nil fn = %v", err)
	}
}

func TestTx(t *testing.T) {
	t.Parallel()
	if _, err := pg.Tx(context.Background()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Tx outside a unit = %v, want kind internal", err)
	}
	tu, _ := txn.NewUnit(txn.NewRegistry(), txn.Options{})
	foreign := txn.WithUnit(context.Background(), tu)
	if _, err := pg.Tx(foreign); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Tx in a foreign unit = %v, want kind internal", err)
	}
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	if err := uow.Do(foreign, func(context.Context) error { return nil }); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Do inside a foreign unit = %v, want kind internal", err)
	}
}

func TestQuerier(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	ctx := context.Background()
	q := pg.Querier(ctx, e.pool)
	if _, err := q.Exec(ctx, "INSERT INTO items VALUES (60, 'pool')"); err != nil {
		t.Fatal(err)
	}
	rows, err := q.Query(ctx, "SELECT id FROM items WHERE id >= $1 ORDER BY id", 2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil || !slices.Equal(got, []int32{2, 60}) {
		t.Fatalf("rows = %v, %v", got, err)
	}
	if err := q.QueryRow(ctx, "SELECT 1 FROM items WHERE id = 999").Scan(new(int)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("QueryRow without row = %v", err)
	}
	if _, err := q.Query(ctx, "SELECT nope"); err == nil {
		t.Fatal("a failing query reported no error")
	}
	if st := e.pool.Stat(); st.AcquiredConns() != 0 {
		t.Fatalf("%d connections still acquired after the pool queries", st.AcquiredConns())
	}

	uow := e.uow(txn.NewRegistry())
	boom := errors.New("roll back")
	err = uow.Do(ctx, func(ctx context.Context) error {
		if _, err := pg.Querier(ctx, e.pool).Exec(ctx, "INSERT INTO items VALUES (61, 'tx')"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || e.rowExists(t, 61) {
		t.Fatalf("Do = %v; Querier inside a unit must use the transaction", err)
	}
}

func TestQuerier_AcquireTimeout(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	cfg := e.db.Config(pg.RoleApp)
	cfg.MaxConns, cfg.AcquireTimeout = 1, 50*time.Millisecond
	pool := e.db.OpenPool(t, cfg)
	// The one connection is taken without the 50ms acquire timeout: dialling it can take longer
	// on a loaded machine, and the test is about the acquires that find the pool exhausted.
	c, err := pool.PGX().Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	q := pg.Querier(context.Background(), pool)
	if _, err := q.Exec(context.Background(), "SELECT 1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("Exec on an exhausted pool = %v", err)
	}
	if _, err := q.Query(context.Background(), "SELECT 1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("Query on an exhausted pool = %v", err)
	}
	if err := q.QueryRow(context.Background(), "SELECT 1").Scan(new(int)); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("QueryRow on an exhausted pool = %v", err)
	}
}

// TestUnitOfWork_RegistrationsNameTheOwningModule: the registry derives owner, publisher and
// event name from the Go packages of a real module package (the stand-in fakemodule).
func TestUnitOfWork_RegistrationsNameTheOwningModule(t *testing.T) {
	t.Parallel()
	reg := txn.NewRegistry()
	if err := reg.SetHook(txn.PhaseSync, fakemodule.Hook(func(context.Context) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if err := txn.Subscribe(reg, fakemodule.Handler(func(context.Context, fakemodule.Event) error { return nil })); err != nil {
		t.Fatal(err)
	}
	regs := reg.Registrations()
	if len(regs) != 2 {
		t.Fatalf("registrations = %+v", regs)
	}
	hook, sub := regs[0], regs[1]
	if hook.Kind != txn.KindHook || hook.Subscriber != "fakemodule" || !strings.Contains(hook.Handler, "/internal/fakemodule.Hook.") {
		t.Errorf("hook = %+v", hook)
	}
	if sub.Kind != txn.KindSubscribe || sub.Publisher != "fakemodule" || sub.Event != "fakemodule.Event" ||
		sub.Subscriber != "fakemodule" || !strings.Contains(sub.Handler, "/internal/fakemodule.Handler[") {
		t.Errorf("subscription = %+v", sub)
	}
	// A closure of platform/pg has no owning module.
	if err := reg.SetHook(txn.PhaseAudit, func(context.Context) error { return nil }); err == nil {
		t.Error("a hook of platform/pg was accepted")
	}
}
