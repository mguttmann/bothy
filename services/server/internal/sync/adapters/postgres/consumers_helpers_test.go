// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"log/slog"
	"strings"
	stdsync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// jobsSchema is the schema of River's tables in the consumer tests.
const jobsSchema = "jobs"

// probeArgs are the arguments of the test job kind sync.consumer_probe: only an ID and a
// sequence number, as jobs.ValidateArgsType requires (design note 15 of M2-01).
type probeArgs struct {
	Seq       int64  `json:"seq"`
	Recipient ids.ID `json:"recipient"`
}

// Kind implements jobs.Args.
func (probeArgs) Kind() string { return "sync.consumer_probe" }

// probeWork is the no-op handler of the probe kind; the runtime of role api never runs it.
type probeWork struct{}

func (probeWork) Work(context.Context, jobs.Job[probeArgs]) error { return nil }

// The two recipients of every probe event: Alex and Sam.
var (
	alex = ids.New()
	sam  = ids.New()
)

// consumerEnv is the event log of env plus River's tables and an insert-only job runtime.
type consumerEnv struct {
	*env
	inserter jobs.Inserter
}

// newConsumerEnv returns a consumer environment over a fresh database.
func newConsumerEnv(t *testing.T, o envOptions) *consumerEnv {
	t.Helper()
	if o.maxConns == 0 {
		o.maxConns = 12
	}
	e := newEnv(t, o)
	jobstest.Migrate(t, e.db, jobsSchema)
	return &consumerEnv{env: e, inserter: newInserter(t, e.pool)}
}

// newInserter returns the inserter of a runtime in role api over pool with the probe kind.
func newInserter(t *testing.T, pool *pg.Pool) jobs.Inserter {
	t.Helper()
	reg := jobs.NewRegistry()
	if err := jobs.Register[probeArgs](reg, probeWork{}, jobs.HandlerOptions{Queue: jobs.QueueDefault}); err != nil {
		t.Fatal(err)
	}
	rt, err := jobs.New(pool, jobs.Config{Schema: jobsSchema, MaxWorkers: map[string]int{
		jobs.QueueDefault: 1, jobs.QueueFiles: 1, jobs.QueueDelivery: 1, jobs.QueueMaintenance: 1, jobs.QueuePrivacy: 1,
	}, SoftStopTimeout: time.Second}, jobs.RoleAPI, reg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rt.Inserter()
}

// consumerCfg is a fast configuration of the worker consumers.
func consumerCfg() app.ConsumerConfig {
	return app.ConsumerConfig{BatchSize: 5, PollInterval: 100 * time.Millisecond, ReconnectBackoffCap: 400 * time.Millisecond, StatesTimeout: 5 * time.Second}
}

// settingsEvent is an event of the catalogue type settings.updated of Alex (resource user,
// receive action self).
func settingsEvent() eventlog.Event {
	return eventlog.Event{
		Type: "settings.updated", SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: alex,
		ResourceKind: "user", ResourceID: alex, ReceiveAction: "self", AudiencePrincipalID: alex,
		Payload: []byte(`{"theme":"dark"}`),
	}
}

// appendSettings commits n settings events in one unit.
func (e *consumerEnv) appendSettings(t *testing.T, n int) {
	t.Helper()
	events := make([]eventlog.Event, n)
	for i := range events {
		events[i] = settingsEvent()
	}
	if _, err := e.appendUnit(context.Background(), events...); err != nil {
		t.Fatal(err)
	}
}

// worker is one worker process of the tests: its own pool, unit of work, repository, service
// and job inserter.
type worker struct {
	pool     *pg.Pool
	uow      *pg.UnitOfWork
	repo     *postgres.Repository
	svc      *app.ConsumerService
	inserter jobs.Inserter
	logs     *syncBuffer
	log      *slog.Logger
}

// newWorker builds a worker over e's database with cfg.
func (e *consumerEnv) newWorker(t *testing.T, cfg app.ConsumerConfig) *worker {
	t.Helper()
	pc := e.db.Config(pg.RoleApp)
	pc.MaxConns = 12
	pool := e.db.OpenPool(t, pc)
	logs := &syncBuffer{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	reg := txn.NewRegistry()
	if err := reg.SetHook(txn.PhaseSync, wire.NewModules(pool, log).Sync.Run); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 5}, reg)
	repo := postgres.NewRepository(pool, log)
	svc, err := app.NewConsumerService(cfg, uow, repo, repo, log)
	if err != nil {
		t.Fatal(err)
	}
	return &worker{pool: pool, uow: uow, repo: repo, svc: svc, inserter: newInserter(t, pool), logs: logs, log: log}
}

// probe is the probe handler: two jobs per event (Alex and Sam), so a half-done event would
// show. hook, when set, runs after the first job of every event and may block or fail.
type probe struct {
	inserter jobs.Inserter
	hook     func(ctx context.Context, ev sync.Event) error
	handled  atomic.Int64
}

// HandleEvent implements sync.ConsumerHandler.
func (p *probe) HandleEvent(ctx context.Context, ev sync.Event) error {
	if err := p.inserter.InsertTx(ctx, probeArgs{Seq: ev.Seq, Recipient: alex}, nil); err != nil {
		return err
	}
	if p.hook != nil {
		if err := p.hook(ctx, ev); err != nil {
			return err
		}
	}
	if err := p.inserter.InsertTx(ctx, probeArgs{Seq: ev.Seq, Recipient: sam}, nil); err != nil {
		return err
	}
	p.handled.Add(1)
	return nil
}

// register registers h as consumer name of w.
func (w *worker) register(t *testing.T, name sync.ConsumerName, h sync.ConsumerHandler) {
	t.Helper()
	if err := w.svc.Register(name, h); err != nil {
		t.Fatal(err)
	}
}

// runner starts a runner of w with its own waker (or waker, when set) and stops it at the end.
func (w *worker) runner(t *testing.T, cfg app.ConsumerConfig, waker app.Waker, rounds app.Rounds) {
	t.Helper()
	if waker == nil {
		waker = postgres.NewWaker(w.pool, cfg, w.log)
	}
	if rounds == nil {
		rounds = w.svc
	}
	r, err := app.NewConsumerRunner(rounds, waker, cfg, app.RunnerOptions{Logger: w.log})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.svc.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		if err := r.Stop(ctx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})
}

// countingRounds wraps the rounds of a runner and counts the outcomes.
type countingRounds struct {
	app.Rounds
	locked atomic.Int64
	failed atomic.Int64
	rounds atomic.Int64
}

func (c *countingRounds) Round(ctx context.Context, name sync.ConsumerName) (app.RoundResult, error) {
	res, err := c.Rounds.Round(ctx, name)
	c.rounds.Add(1)
	if err != nil {
		c.failed.Add(1)
	} else if res.Outcome == app.RoundLocked {
		c.locked.Add(1)
	}
	return res, err
}

// silentWaker never signals: only the poll wakes the consumers.
type silentWaker struct{}

func (silentWaker) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (silentWaker) Wake() <-chan struct{} { return nil }

// cursor returns last_seq of name, or -1 without a row.
func (e *consumerEnv) cursor(t *testing.T, name sync.ConsumerName) int64 {
	t.Helper()
	var v int64 = -1
	rows, err := e.db.SuperConn(t).Query(context.Background(), "SELECT last_seq FROM event_consumer_cursors WHERE name = $1", string(name))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return v
}

// head returns max(seq) of event_log.
func (e *consumerEnv) head(t *testing.T) int64 {
	t.Helper()
	var v int64
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT COALESCE(max(seq), 0) FROM event_log").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// probeJobs returns the number of probe jobs per seq in River's job table.
func (e *consumerEnv) probeJobs(t *testing.T) map[int64]int {
	t.Helper()
	rows, err := e.db.SuperConn(t).Query(context.Background(),
		"SELECT (args->>'seq')::bigint, count(*) FROM "+jobsSchema+".river_job WHERE kind = 'sync.consumer_probe' GROUP BY 1")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var seq int64
		var n int
		if err := rows.Scan(&seq, &n); err != nil {
			t.Fatal(err)
		}
		out[seq] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// exactlyTwoJobs fails t unless every seq from..to has exactly two probe jobs and no other seq
// has one.
func (e *consumerEnv) exactlyTwoJobs(t *testing.T, from, to int64) {
	t.Helper()
	got := e.probeJobs(t)
	for s := from; s <= to; s++ {
		if got[s] != 2 {
			t.Errorf("seq %d has %d probe jobs, want 2", s, got[s])
		}
		delete(got, s)
	}
	for s, n := range got {
		t.Errorf("seq %d outside %d..%d has %d probe jobs", s, from, to, n)
	}
}

// waitCursor waits until the cursor of name equals want.
func (e *consumerEnv) waitCursor(t *testing.T, name sync.ConsumerName, want int64) {
	t.Helper()
	testkit.Until(t, "the cursor of "+string(name)+" at the head", func() bool { return e.cursor(t, name) == want })
}

// countLines counts the log lines of buf with msg.
func countLines(buf *syncBuffer, msg string) int {
	return strings.Count(buf.String(), `"msg":"`+msg+`"`)
}

// latch is a one-shot gate shared by goroutines.
type latch struct {
	once stdsync.Once
	ch   chan struct{}
}

func newLatch() *latch { return &latch{ch: make(chan struct{})} }

func (l *latch) open() { l.once.Do(func() { close(l.ch) }) }
