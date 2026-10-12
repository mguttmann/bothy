// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
	stdsync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// noCorrelation is a correlator without request context.
type noCorrelation struct{}

func (noCorrelation) Correlation(context.Context) (ids.ID, string) { return ids.ID{}, "" }

// env is a database with the event log wired as in production: the sync hook of app over the
// counters of instance and the repository of this package, run by the unit of work of platform/pg
// as bothy_app.
type env struct {
	db       *pgtest.DB
	pool     *pg.Pool
	repo     *postgres.Repository
	uow      *pg.UnitOfWork
	appender *app.Appender
	clock    *clock.Fake
	logs     *syncBuffer
}

// envOptions change newEnv.
type envOptions struct {
	// audit is a hook of the phase after sync; nil for none.
	audit txn.Hook
	// maxConns is the pool size; 0 keeps the pgtest default.
	maxConns int
	// noPartitions leaves event_log without partitions.
	noPartitions bool
	// partitions is the configuration of the maintenance; zero means size 1000, ahead 2.
	partitions app.PartitionConfig
}

// syncBuffer is a bytes.Buffer safe for concurrent writes of a logger.
type syncBuffer struct {
	mu stdsync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newEnv(t *testing.T, o envOptions) *env {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	if o.maxConns > 0 {
		cfg.MaxConns = o.maxConns
	}
	pool := db.OpenPool(t, cfg)
	logs := &syncBuffer{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	repo := postgres.NewRepository(pool, log)
	// The hook as internal/wire builds it; only wire and the hook packages may name the counters
	// (TestCountersOnlyUsedByPhaseHooks), and its repository logs into logs as well.
	reg := txn.NewRegistry()
	if err := reg.SetHook(txn.PhaseSync, wire.NewModules(pool, log).Sync.Run); err != nil {
		t.Fatal(err)
	}
	if o.audit != nil {
		if err := reg.SetHook(txn.PhaseAudit, o.audit); err != nil {
			t.Fatal(err)
		}
	}
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 5}, reg)
	a, err := app.NewAppender(app.AppenderConfig{PayloadMaxBytes: 65536}, noCorrelation{})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{db: db, pool: pool, repo: repo, uow: uow, appender: a, logs: logs,
		clock: clock.NewFake(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))}
	if !o.noPartitions {
		pc := o.partitions
		if pc.Size == 0 {
			pc = app.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second}
		}
		if _, err := e.maintenance(t, pc).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// maintenance returns the partition maintenance of e with cfg.
func (e *env) maintenance(t *testing.T, cfg app.PartitionConfig) *app.PartitionMaintenance {
	t.Helper()
	m, err := app.NewPartitionMaintenance(cfg, e.uow, e.repo, e.clock)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// testEvent returns a valid event of type typ.
func testEvent(typ string) eventlog.Event {
	return eventlog.Event{
		Type: typ, SchemaVersion: 1, ActorKind: eventlog.ActorSystem, ResourceKind: "thread",
		ResourceID: ids.New(), ReceiveAction: "thread.read", Payload: []byte(`{"n":1}`),
	}
}

// appendUnit commits one unit with events and returns their IDs.
func (e *env) appendUnit(ctx context.Context, events ...eventlog.Event) ([]ids.ID, error) {
	var out []ids.ID
	err := e.uow.Do(ctx, func(ctx context.Context) error {
		out = out[:0]
		for i := range events {
			if events[i].ID.IsZero() {
				events[i].ID = ids.New()
			}
			out = append(out, events[i].ID)
		}
		return e.appender.Append(ctx, events...)
	})
	return out, err
}

// mustAppend commits n events in one unit.
func (e *env) mustAppend(t *testing.T, n int) []ids.ID {
	t.Helper()
	events := make([]eventlog.Event, n)
	for i := range events {
		events[i] = testEvent(fmt.Sprintf("thread.e_%c", 'a'+rune(i%26)))
	}
	got, err := e.appendUnit(context.Background(), events...)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// collector is a sink that keeps copies of every batch.
type collector struct {
	mu     stdsync.Mutex
	events []sync.Event
}

func (c *collector) sink(_ context.Context, batch []sync.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, batch...)
	return nil
}

func (c *collector) snapshot() []sync.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.events)
}

// waitFor polls cond every 20ms until it holds or d has passed.
func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout after %s waiting for %s", d, what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// tailConfig is a fast tail configuration for tests.
func tailConfig() postgres.TailConfig {
	return postgres.TailConfig{PollInterval: 100 * time.Millisecond, BatchSize: 7, ReconnectBackoffCap: 400 * time.Millisecond}
}

// runTail starts tail and returns the channel with the result of Run and the cancel function.
func runTail(t *testing.T, tail *postgres.Tail) (<-chan error, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- tail.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("the tail did not stop")
		}
	})
	return done, cancel
}

// checkContiguous fails t unless events are exactly seqs 1..n in order.
func checkContiguous(t *testing.T, events []sync.Event, n int64) {
	t.Helper()
	if int64(len(events)) != n {
		t.Fatalf("collected %d events, want %d", len(events), n)
	}
	for i, ev := range events {
		if ev.Seq != int64(i)+1 {
			t.Fatalf("event %d has seq %d, want %d", i, ev.Seq, i+1)
		}
	}
}

// started waits until tail has read the head it starts at.
func started(t *testing.T, tail *postgres.Tail) {
	t.Helper()
	select {
	case <-tail.Started():
	case <-time.After(10 * time.Second):
		t.Fatal("the tail did not start")
	}
}
