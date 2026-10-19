// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/internal/fakemodule"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// The schema of River's tables in these tests: not the default, so nothing depends on it.
const testSchema = "jobs_test"

// env is the database of one test: River's tables in testSchema, a pool of bothy_app with
// room for River's connections, and a unit of work over it.
type env struct {
	db   *pgtest.DB
	pool *pg.Pool
	uow  *pg.UnitOfWork
}

func newEnv(t *testing.T, opts ...func(*txn.Registry)) *env {
	t.Helper()
	db := pgtest.New(t)
	jobstest.Migrate(t, db, testSchema)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 12
	pool := db.OpenPool(t, cfg)
	reg := txn.NewRegistry()
	for _, o := range opts {
		o(reg)
	}
	reg.Seal()
	ucfg := pg.UnitConfig{StatementTimeout: cfg.StatementTimeout, LockTimeout: cfg.LockTimeout, IdleInTxTimeout: cfg.IdleInTxTimeout}
	return &env{db: db, pool: pool, uow: pg.NewUnitOfWork(pool, ucfg, reg)}
}

// testConfig is the job configuration of the tests: two workers per queue and soft.
func testConfig(t *testing.T, soft time.Duration) jobs.Config {
	t.Helper()
	mw := map[string]int{}
	for _, q := range jobs.Queues() {
		mw[q] = 2
	}
	return jobs.Config{Schema: testSchema, MaxWorkers: mw, SoftStopTimeout: soft, Logger: slog.New(slog.NewTextHandler(testWriter{t}, nil))}
}

// testWriter sends River's log lines to the test log.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// runtime builds and starts a runtime and stops it when the test ends.
func (e *env) runtime(t *testing.T, cfg jobs.Config, role jobs.Role, reg *jobs.Registry) *jobs.Runtime {
	t.Helper()
	rt, err := jobs.New(e.pool, cfg, role, reg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := rt.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		if err := rt.Stop(ctx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})
	return rt
}

// insert inserts args in a unit of work of its own.
func (e *env) insert(t *testing.T, ins jobs.Inserter, args jobs.Args, opts *jobs.InsertOptions) {
	t.Helper()
	err := e.uow.Do(t.Context(), func(ctx context.Context) error { return ins.InsertTx(ctx, args, opts) })
	if err != nil {
		t.Fatalf("insert %s: %v", args.Kind(), err)
	}
}

// query runs q on a superuser connection of its own, outside every transaction of the test.
func (e *env) query(t *testing.T, q string, dest ...any) {
	t.Helper()
	if err := e.db.SuperConn(t).QueryRow(context.Background(), q).Scan(dest...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// table returns the qualified name of a River table in testSchema.
func table(name string) string { return pgx.Identifier{testSchema, name}.Sanitize() }

// jobRow is the state of the only job of a test.
type jobRow struct {
	state   string
	attempt int
	errors  int
}

func (e *env) onlyJob(t *testing.T) jobRow {
	t.Helper()
	var r jobRow
	e.query(t, "SELECT state::text, attempt, coalesce(array_length(errors, 1), 0) FROM "+table("river_job"), &r.state, &r.attempt, &r.errors)
	return r
}

func (e *env) jobCount(t *testing.T) int {
	t.Helper()
	var n int
	e.query(t, "SELECT count(*) FROM "+table("river_job"), &n)
	return n
}

// recorder is a handler that reports each job on a channel.
func recorder[A jobs.Args](ch chan<- jobs.Job[A]) jobs.Handler[A] {
	return fakemodule.Func[A](func(_ context.Context, j jobs.Job[A]) error {
		ch <- j
		return nil
	})
}

// registry returns a registry with the handler h for A.
func registry[A jobs.Args](t *testing.T, h jobs.Handler[A], o jobs.HandlerOptions) *jobs.Registry {
	t.Helper()
	reg := jobs.NewRegistry()
	if err := jobs.Register(reg, h, o); err != nil {
		t.Fatal(err)
	}
	return reg
}

func receive[A jobs.Args](t *testing.T, ch <-chan jobs.Job[A]) jobs.Job[A] {
	t.Helper()
	select {
	case j := <-ch:
		return j
	case <-time.After(testkit.Patience):
		t.Fatalf("no job ran within %s", testkit.Patience)
		return jobs.Job[A]{}
	}
}

func none[A jobs.Args](t *testing.T, ch <-chan jobs.Job[A], d time.Duration) {
	t.Helper()
	select {
	case j := <-ch:
		t.Fatalf("job %d ran, want none within %s", j.ID, d)
	case <-time.After(d):
	}
}

func TestInsertTx_VisibleOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, registry(t, recorder(ran), jobs.HandlerOptions{}))

	target := ids.New()
	inserted, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- e.uow.Do(context.Background(), func(ctx context.Context) error {
			if err := rt.Inserter().InsertTx(ctx, fakemodule.Echo{Target: target, Seq: 7}, nil); err != nil {
				return err
			}
			close(inserted)
			<-release
			return nil
		})
	}()
	select {
	case <-inserted:
	case err := <-done:
		t.Fatalf("unit ended before the insert: %v", err)
	}
	if n := e.jobCount(t); n != 0 {
		t.Fatalf("a second connection sees %d jobs before the commit, want 0", n)
	}
	none(t, ran, 1500*time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Do: %v", err)
	}
	j := receive(t, ran)
	if j.Args.Target != target || j.Args.Seq != 7 || j.Attempt != 1 || j.ID == 0 {
		t.Fatalf("job = %+v, want the inserted arguments on attempt 1", j)
	}
}

func TestInsertTx_RollbackNeverRuns(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, registry(t, recorder(ran), jobs.HandlerOptions{}))
	veto := errors.New("veto")
	err := e.uow.Do(t.Context(), func(ctx context.Context) error {
		if err := rt.Inserter().InsertTx(ctx, fakemodule.Echo{Target: ids.New()}, nil); err != nil {
			return err
		}
		return veto
	})
	if !errors.Is(err, veto) {
		t.Fatalf("Do = %v, want the veto", err)
	}
	none(t, ran, 3*time.Second)
	if n := e.jobCount(t); n != 0 {
		t.Fatalf("%d jobs after the rollback, want 0", n)
	}
}

func TestInsertTx_OutsideUnitFails(t *testing.T) {
	t.Parallel()
	var syncErr error
	var rtRef atomic.Pointer[jobs.Runtime]
	e := newEnv(t, func(reg *txn.Registry) {
		if err := reg.SetHook(txn.PhaseSync, fakemodule.Hook(func(ctx context.Context) error {
			syncErr = rtRef.Load().Inserter().InsertTx(ctx, fakemodule.Echo{}, nil)
			return nil
		})); err != nil {
			t.Fatal(err)
		}
	})
	rt, err := jobs.New(e.pool, testConfig(t, time.Second), jobs.RoleAPI, registry(t, recorder(make(chan jobs.Job[fakemodule.Echo])), jobs.HandlerOptions{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	rtRef.Store(rt)
	err = rt.Inserter().InsertTx(context.Background(), fakemodule.Echo{}, nil)
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("InsertTx outside a unit = %v, want kind internal", err)
	}
	if err := e.uow.Do(t.Context(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if errs.KindOf(syncErr) != errs.KindInternal || !strings.Contains(syncErr.Error(), "sync") {
		t.Fatalf("InsertTx in phase sync = %v, want kind internal naming the phase", syncErr)
	}
	if n := e.jobCount(t); n != 0 {
		t.Fatalf("%d jobs, want 0", n)
	}
}

func TestInsertTx_UnregisteredKindFails(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rt, err := jobs.New(e.pool, testConfig(t, time.Second), jobs.RoleAPI, registry(t, recorder(make(chan jobs.Job[fakemodule.Echo])), jobs.HandlerOptions{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range []jobs.Args{fakemodule.Tick{}, &fakemodule.Echo{}, nil} {
		err := e.uow.Do(t.Context(), func(ctx context.Context) error { return rt.Inserter().InsertTx(ctx, args, nil) })
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("InsertTx(%T) = %v, want kind internal", args, err)
		}
	}
	if n := e.jobCount(t); n != 0 {
		t.Fatalf("%d jobs, want 0", n)
	}
}

// TestInsertTx_ArgumentsMustRoundTrip: arguments River could not read back (a zero ID is no valid
// UUIDv7) fail at the insert, inside the unit of the caller, instead of failing every attempt.
func TestInsertTx_ArgumentsMustRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rt, err := jobs.New(e.pool, testConfig(t, time.Second), jobs.RoleAPI, registry(t, recorder(make(chan jobs.Job[fakemodule.Echo])), jobs.HandlerOptions{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = e.uow.Do(t.Context(), func(ctx context.Context) error { return rt.Inserter().InsertTx(ctx, fakemodule.Echo{}, nil) })
	if errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), "fakemodule.echo") {
		t.Fatalf("InsertTx with a zero ID = %v, want kind internal naming the kind", err)
	}
	if n := e.jobCount(t); n != 0 {
		t.Fatalf("%d jobs, want 0", n)
	}
}

func TestInsertTx_ScheduledAt(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, registry(t, recorder(ran), jobs.HandlerOptions{}))
	// River compares scheduled_at with the clock of the database, which in a container VM can run
	// ahead of or behind the clock of this process; both checks read that clock.
	const offset = 3 * time.Second
	// skew is read with the time before the query, so it errs towards the database being ahead and
	// the quiet window below ends early rather than late.
	var dbNow time.Time
	before := time.Now()
	e.query(t, "SELECT clock_timestamp()", &dbNow)
	skew := dbNow.Sub(before)
	at := dbNow.Add(offset)
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 1}, &jobs.InsertOptions{ScheduledAt: at})
	none(t, ran, time.Until(at.Add(-skew))-200*time.Millisecond)
	receive(t, ran)
	e.query(t, "SELECT clock_timestamp()", &dbNow)
	if dbNow.Before(at) {
		t.Fatalf("the job ran at %s by the database clock, before its time %s", dbNow, at)
	}
}

func TestRoleAPI_DoesNotWorkJobs(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleAPI, registry(t, recorder(ran), jobs.HandlerOptions{}))
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 1}, nil)
	// Several River fetch intervals (1s each).
	none(t, ran, 4*time.Second)
	if r := e.onlyJob(t); r.state != "available" || r.attempt != 0 {
		t.Fatalf("job = %+v, want available and never attempted", r)
	}
	var leaders int
	e.query(t, "SELECT count(*) FROM "+table("river_leader"), &leaders)
	if leaders != 0 {
		t.Fatalf("%d leader rows, want none from an insert-only client", leaders)
	}
	select {
	case err := <-rt.Failed():
		t.Fatalf("Failed of a runtime that never started delivered %v", err)
	default:
	}
}

func TestRoleWorker_RunsJobAsAppRole(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	h := fakemodule.Func[fakemodule.Echo](func(ctx context.Context, j jobs.Job[fakemodule.Echo]) error {
		// The handler runs on the pool of bothy_app: a unit of work of the job sees its role.
		var who string
		if err := e.pool.QueryRow(ctx, "SELECT current_user").Scan(&who); err != nil {
			return err
		}
		if who != string(pg.RoleApp) {
			return fmt.Errorf("job runs as %s", who)
		}
		ran <- j
		return nil
	})
	reg := registry(t, h, jobs.HandlerOptions{Queue: jobs.QueueDelivery})
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, reg)
	target := ids.New()
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: target}, nil)
	if j := receive(t, ran); j.Args.Target != target {
		t.Fatalf("job = %+v", j)
	}
	waitCompleted(t, e)
	var queue string
	e.query(t, "SELECT queue FROM "+table("river_job"), &queue)
	if queue != jobs.QueueDelivery {
		t.Fatalf("queue = %q, want %q", queue, jobs.QueueDelivery)
	}
}

// waitCompleted waits until the only job is completed.
func waitCompleted(t *testing.T, e *env) jobRow {
	t.Helper()
	const state = "completed"
	deadline := testkit.Deadline()
	for {
		r := e.onlyJob(t)
		if r.state == state {
			return r
		}
		if time.Now().After(deadline) {
			t.Fatalf("job = %+v, want state %s within %s", r, state, testkit.Patience)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRoleAll_InsertsAndWorks(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleAll, registry(t, recorder(ran), jobs.HandlerOptions{}))
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 3}, nil)
	if j := receive(t, ran); j.Args.Seq != 3 {
		t.Fatalf("job = %+v", j)
	}
	waitCompleted(t, e)
}

// blocker is a handler that reports its start and waits for its context; returnNil makes it
// swallow the cancellation.
func blocker(started chan<- jobs.Job[fakemodule.Block], returnNil bool) jobs.Handler[fakemodule.Block] {
	return fakemodule.Func[fakemodule.Block](func(ctx context.Context, j jobs.Job[fakemodule.Block]) error {
		started <- j
		<-ctx.Done()
		if returnNil {
			return nil
		}
		return ctx.Err()
	})
}

// stopAfterSoftTimeout runs a blocking job on a worker with soft timeout soft, stops the worker
// and checks that Stop took at least soft and that the job is waiting for its next attempt.
func stopAfterSoftTimeout(t *testing.T, e *env, returnNil bool) {
	t.Helper()
	const soft = 700 * time.Millisecond
	started := make(chan jobs.Job[fakemodule.Block], 1)
	zero := jobs.HandlerOptions{NextRetry: func(int) time.Duration { return 0 }}
	rt, err := jobs.New(e.pool, testConfig(t, soft), jobs.RoleWorker, registry(t, blocker(started, returnNil), zero), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	e.insert(t, rt.Inserter(), fakemodule.Block{}, nil)
	if j := receive(t, started); j.Attempt != 1 {
		t.Fatalf("first run on attempt %d", j.Attempt)
	}
	begin := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if err := rt.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if took := time.Since(begin); took < soft {
		t.Fatalf("Stop took %s, less than the soft timeout %s", took, soft)
	}
	// River 0.49 puts a job its shutdown cancelled back as available without counting the
	// attempt (docs/DECISIONS.md M1-14); it never becomes completed.
	if r := e.onlyJob(t); r.state != "available" || r.attempt != 0 {
		t.Fatalf("job after the stop = %+v, want available for another attempt", r)
	}
	select {
	case err, ok := <-rt.Failed():
		if ok {
			t.Fatalf("Failed delivered %v after Stop", err)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("Failed was not closed after Stop")
	}

	// A new worker runs the job again to completion.
	again := make(chan jobs.Job[fakemodule.Block], 1)
	done := fakemodule.Func[fakemodule.Block](func(_ context.Context, j jobs.Job[fakemodule.Block]) error {
		again <- j
		return nil
	})
	e.runtime(t, testConfig(t, soft), jobs.RoleWorker, registry(t, done, zero))
	if j := receive(t, again); j.Attempt != 1 {
		t.Fatalf("second run on attempt %d, want 1 (the interrupted run is not counted)", j.Attempt)
	}
	waitCompleted(t, e)
}

func TestShutdown_SoftTimeoutCancelsAndRetries(t *testing.T) {
	t.Parallel()
	stopAfterSoftTimeout(t, newEnv(t), false)
}

func TestHandler_NilAfterCancelBecomesError(t *testing.T) {
	t.Parallel()
	stopAfterSoftTimeout(t, newEnv(t), true)
}

func TestHandler_PanicIsJobError(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 2)
	h := fakemodule.Func[fakemodule.Echo](func(_ context.Context, j jobs.Job[fakemodule.Echo]) error {
		ran <- j
		if j.Attempt == 1 {
			panic("secret argument text")
		}
		return nil
	})
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker,
		registry(t, h, jobs.HandlerOptions{NextRetry: func(int) time.Duration { return 0 }}))
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: ids.New()}, nil)
	if j := receive(t, ran); j.Attempt != 1 {
		t.Fatalf("first run on attempt %d", j.Attempt)
	}
	if j := receive(t, ran); j.Attempt != 2 {
		t.Fatalf("second run on attempt %d", j.Attempt)
	}
	r := waitCompleted(t, e)
	if r.errors != 1 {
		t.Fatalf("job = %+v, want one recorded error", r)
	}
	var msg string
	e.query(t, "SELECT errors[1]->>'error' FROM "+table("river_job"), &msg)
	if !strings.Contains(msg, "panicked") || strings.Contains(msg, "secret") {
		t.Fatalf("recorded error %q: want the panic, never its value", msg)
	}
	select {
	case err := <-rt.Failed():
		t.Fatalf("the runtime failed after a job panic: %v", err)
	default:
	}
}

func TestFailed_UnexpectedStopEndsProcess(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker,
		registry(t, recorder(make(chan jobs.Job[fakemodule.Echo], 1)), jobs.HandlerOptions{}))
	if err := jobs.StopClientForTest(t.Context(), rt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-rt.Failed():
		if err == nil {
			t.Fatal("Failed was closed without an error")
		}
	case <-time.After(testkit.Patience):
		t.Fatal("Failed delivered nothing after the client stopped")
	}
}

func TestPeriodicJobRunsOnStart(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Tick], 1)
	reg := registry(t, recorder(ran), jobs.HandlerOptions{Queue: jobs.QueueMaintenance})
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: fakemodule.Tick{}, Interval: time.Hour, RunOnStart: true}); err != nil {
		t.Fatal(err)
	}
	e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, reg)
	receive(t, ran)
	var queue string
	e.query(t, "SELECT queue FROM "+table("river_job"), &queue)
	if queue != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, want %q", queue, jobs.QueueMaintenance)
	}
}

func TestRegisterAfterNewFails(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	reg := jobs.NewRegistry()
	if _, err := jobs.New(e.pool, testConfig(t, time.Second), jobs.RoleAPI, reg, nil); err != nil {
		t.Fatal(err)
	}
	if err := jobs.Register(reg, recorder(make(chan jobs.Job[fakemodule.Echo])), jobs.HandlerOptions{}); err == nil {
		t.Fatal("Register after New succeeded")
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	good := testConfig(t, time.Second)
	cases := map[string]func(*jobs.Config){
		"schema public":   func(c *jobs.Config) { c.Schema = "public" },
		"schema quoted":   func(c *jobs.Config) { c.Schema = `jobs"x` },
		"no soft timeout": func(c *jobs.Config) { c.SoftStopTimeout = 0 },
		"missing queue":   func(c *jobs.Config) { delete(c.MaxWorkers, jobs.QueuePrivacy) },
		"unknown queue":   func(c *jobs.Config) { c.MaxWorkers["urgent"] = 1 },
		"zero workers":    func(c *jobs.Config) { c.MaxWorkers[jobs.QueueFiles] = 0 },
	}
	for name, change := range cases {
		cfg := good
		cfg.MaxWorkers = map[string]int{}
		for k, v := range good.MaxWorkers {
			cfg.MaxWorkers[k] = v
		}
		change(&cfg)
		if _, err := jobs.New(e.pool, cfg, jobs.RoleWorker, jobs.NewRegistry(), nil); err == nil {
			t.Errorf("%s: New accepted the configuration", name)
		}
	}
	if _, err := jobs.New(e.pool, good, jobs.Role("batch"), jobs.NewRegistry(), nil); err == nil {
		t.Error("New accepted an unknown role")
	}
}

func TestWorkerWithoutKindsDoesNotStart(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, jobs.NewRegistry())
	time.Sleep(2 * time.Second)
	var leaders int
	e.query(t, "SELECT count(*) FROM "+table("river_leader"), &leaders)
	if leaders != 0 {
		t.Fatalf("%d leader rows from a worker without kinds", leaders)
	}
	select {
	case err := <-rt.Failed():
		t.Fatalf("Failed of a runtime that never started delivered %v", err)
	default:
	}
}

// TestMigrateNeedsTwoConnections: the advisory lock holds one connection while River migrates on
// others, so a pool of one connection is refused with the key to raise instead of a hang until
// the acquire timeout.
func TestMigrateNeedsTwoConnections(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleOwner).ForMigrate(time.Second)
	cfg.MaxConns = 1
	pool := db.OpenPool(t, cfg)
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	_, err := jobs.Migrate(ctx, pool, testSchema)
	if err == nil || !strings.Contains(err.Error(), "database.max_conns") {
		t.Fatalf("Migrate on one connection = %v, want an error naming database.max_conns", err)
	}
	if _, err := jobs.New(pool, testConfig(t, time.Second), jobs.RoleWorker, jobs.NewRegistry(), nil); err == nil || !strings.Contains(err.Error(), "database.max_conns") {
		t.Fatalf("New of a worker on one connection = %v, want an error naming database.max_conns", err)
	}
	if _, err := jobs.New(pool, testConfig(t, time.Second), jobs.RoleAPI, jobs.NewRegistry(), nil); err != nil {
		t.Fatalf("New of api on one connection = %v; an insert-only client holds no connection", err)
	}
}

// TestMigrateWaitsForAConcurrentRun: a second migrate up waits for the advisory lock of the
// first beyond migrate.lock_timeout, as goose waits for its own lock, and then finds nothing
// left to do; lock_timeout bounds the locks of statements, not the wait for another run.
func TestMigrateWaitsForAConcurrentRun(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(100*time.Millisecond))
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	holder, err := pool.PGX().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_lock($1)", jobs.MigrateLockIDForTest); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := jobs.Migrate(ctx, pool, testSchema)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("Migrate returned %v while another run held the lock", err)
	case <-time.After(time.Second): // nosemgrep: bothy-go-test-wall-clock-wait -- a wait for something that must not happen
	}
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_unlock($1)", jobs.MigrateLockIDForTest); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Migrate after the other run = %v", err)
	}
}
