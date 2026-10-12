// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	stdsync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// TestEventConsumerCursorsTable compares event_consumer_cursors with docs/spec/03-data-model.md
// 4.10: columns, types, nullability, defaults, the primary key, no foreign key, the check equal to
// sync.ConsumerNames, and the rights of bothy_app (SELECT, INSERT, UPDATE, no DELETE).
func TestEventConsumerCursorsTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{noPartitions: true})
	super := e.db.SuperConn(t)
	rows, err := super.Query(ctx, `SELECT column_name, data_type, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'event_consumer_cursors'
		ORDER BY ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var name, typ, null, def string
		if err := rows.Scan(&name, &typ, &null, &def); err != nil {
			t.Fatal(err)
		}
		got = append(got, name+" "+typ+" "+null+" "+def)
	}
	rows.Close()
	want := []string{"name text NO ", "last_seq bigint NO 0", "updated_at timestamp with time zone NO now()"}
	if !slices.Equal(got, want) {
		t.Fatalf("columns\n got %v\nwant %v", got, want)
	}
	var cons []string
	rows, err = super.Query(ctx, `SELECT contype::text || ' ' || pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'public.event_consumer_cursors'::regclass AND contype <> 'n' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cons = append(cons, c)
	}
	rows.Close()
	wantCons := []string{"c CHECK ((name = ANY (ARRAY['push'::text, 'webhooks'::text])))", "p PRIMARY KEY (name)"}
	if !slices.Equal(cons, wantCons) {
		t.Fatalf("constraints\n got %v\nwant %v (no foreign key)", cons, wantCons)
	}
	appConn := e.db.Pool(t, pg.RoleApp)
	for _, n := range sync.ConsumerNames() {
		if _, err := appConn.Exec(ctx, "INSERT INTO event_consumer_cursors (name) VALUES ($1)", string(n)); err != nil {
			t.Errorf("insert %s: %v", n, err)
		}
	}
	for _, bad := range []string{"agents", "Push", ""} {
		_, err := appConn.Exec(ctx, "INSERT INTO event_consumer_cursors (name) VALUES ($1)", bad)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("insert %q = %v, want SQLSTATE 23514", bad, err)
		}
	}
	if _, err := appConn.Exec(ctx, "UPDATE event_consumer_cursors SET last_seq = 1 WHERE name = 'push'"); err != nil {
		t.Errorf("bothy_app cannot UPDATE: %v", err)
	}
	_, err = appConn.Exec(ctx, "DELETE FROM event_consumer_cursors WHERE name = 'push'")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("DELETE as bothy_app = %v, want SQLSTATE 42501", err)
	}
}

// TestConsumerEnsureStartsAtHead: Ensure puts a new cursor at max(seq), a second Ensure (a
// second worker) changes nothing, and only events committed after Ensure create jobs.
func TestConsumerEnsureStartsAtHead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	e.appendSettings(t, 3)
	w := e.newWorker(t, consumerCfg())
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter})
	if err := w.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 3 {
		t.Fatalf("cursor after Ensure = %d, want the head 3", got)
	}
	if got := e.cursor(t, sync.ConsumerPush); got != -1 {
		t.Fatalf("an unregistered consumer has a row at %d", got)
	}
	e.appendSettings(t, 2)
	other := e.newWorker(t, consumerCfg())
	other.register(t, sync.ConsumerWebhooks, &probe{inserter: other.inserter})
	if err := other.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 3 {
		t.Fatalf("cursor after the second Ensure = %d, want 3 unchanged", got)
	}
	res, err := w.svc.Round(ctx, sync.ConsumerWebhooks)
	if err != nil || res.Processed != 2 {
		t.Fatalf("Round = %+v, %v", res, err)
	}
	e.exactlyTwoJobs(t, 4, 5)
}

// TestConsumerRoundJobsAndCursorCommitTogether: while the handler blocks after inserting its
// jobs, another connection sees neither the jobs nor the moved cursor; after the commit both.
func TestConsumerRoundJobsAndCursorCommitTogether(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	w := e.newWorker(t, consumerCfg())
	inside, release := make(chan struct{}), make(chan struct{})
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter, hook: func(_ context.Context, ev sync.Event) error {
		if ev.Seq == 3 {
			close(inside)
			<-release
		}
		return nil
	}})
	if err := w.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	e.appendSettings(t, 3)
	done := make(chan error, 1)
	go func() {
		_, err := w.svc.Round(ctx, sync.ConsumerWebhooks)
		done <- err
	}()
	<-inside
	if got := e.probeJobs(t); len(got) != 0 {
		t.Errorf("jobs visible before the commit: %v", got)
	}
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 0 {
		t.Errorf("cursor visible at %d before the commit", got)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 3 {
		t.Fatalf("cursor after the commit = %d", got)
	}
	e.exactlyTwoJobs(t, 1, 3)
}

// TestConsumerRoundLockedEndsAtOnce: while worker A's round holds the row, worker B's round
// returns RoundLocked far below database.lock_timeout and inserts nothing.
func TestConsumerRoundLockedEndsAtOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	a, b := e.newWorker(t, consumerCfg()), e.newWorker(t, consumerCfg())
	inside, release := make(chan struct{}), make(chan struct{})
	a.register(t, sync.ConsumerPush, &probe{inserter: a.inserter, hook: func(context.Context, sync.Event) error {
		select {
		case <-inside:
		default:
			close(inside)
		}
		<-release
		return nil
	}})
	bh := &probe{inserter: b.inserter}
	b.register(t, sync.ConsumerPush, bh)
	if err := a.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	e.appendSettings(t, 1)
	done := make(chan error, 1)
	go func() {
		_, err := a.svc.Round(ctx, sync.ConsumerPush)
		done <- err
	}()
	<-inside
	start := time.Now()
	res, err := b.svc.Round(ctx, sync.ConsumerPush)
	took := time.Since(start)
	if err != nil || res.Outcome != app.RoundLocked {
		t.Fatalf("B's round = %+v, %v; want RoundLocked", res, err)
	}
	if took > 500*time.Millisecond {
		t.Errorf("B's round took %s; SKIP LOCKED must not wait for the lock", took)
	}
	if bh.handled.Load() != 0 {
		t.Error("B handled an event")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	e.exactlyTwoJobs(t, 1, 1)
}

// TestConsumersTwoWorkersExactlyOnce (plan M2-01: "zwei worker parallel: jedes Ereignis erzeugt
// genau einmal Jobs"): two runners with their own pools and wakers while several writers append
// in units of one to three; a barrier forces one overlap, so at least one round is locked. At the
// end every seq has exactly two probe jobs and the cursor is at the head.
func TestConsumersTwoWorkersExactlyOnce(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	a, b := e.newWorker(t, cfg), e.newWorker(t, cfg)
	ca, cb := &countingRounds{Rounds: a.svc}, &countingRounds{Rounds: b.svc}
	overlap := newLatch()
	barrier := func(context.Context, sync.Event) error {
		select {
		case <-overlap.ch:
		case <-time.After(20 * time.Second):
			return errors.New("no overlap within the bound")
		}
		return nil
	}
	a.register(t, sync.ConsumerWebhooks, &probe{inserter: a.inserter, hook: barrier})
	b.register(t, sync.ConsumerWebhooks, &probe{inserter: b.inserter, hook: barrier})
	go func() {
		for ca.locked.Load()+cb.locked.Load() == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		overlap.open()
	}()
	a.runner(t, cfg, nil, ca)
	b.runner(t, cfg, nil, cb)
	var wg stdsync.WaitGroup
	for writer := range 4 {
		wg.Go(func() {
			for i := range 12 {
				e.appendSettings(t, 1+(writer+i)%3)
			}
		})
	}
	wg.Wait()
	head := e.head(t)
	e.waitCursor(t, sync.ConsumerWebhooks, head, 60*time.Second)
	if ca.locked.Load()+cb.locked.Load() == 0 {
		t.Fatal("no round was locked; the barrier did not force an overlap")
	}
	e.exactlyTwoJobs(t, 1, head)
}

// TestConsumerCrashMidRoundNoDuplicates (plan M2-01: "Absturz mitten in einer Runde erzeugt
// keine Doppel"; ADR 0051 "abort before the commit"): a round that ends before the commit leaves
// neither jobs nor a moved cursor, and a fresh worker then creates every job exactly once.
func TestConsumerCrashMidRoundNoDuplicates(t *testing.T) {
	t.Parallel()
	type crash struct {
		// hook runs after the first job of seq 3; abort may end the round from outside.
		hook  func(ctx context.Context, pid *atomic.Int64, e *consumerEnv, abort chan<- struct{}) error
		abort func(t *testing.T, e *consumerEnv, pid int64, cancel context.CancelFunc)
	}
	cases := map[string]crash{
		"backend_terminated": {
			hook: func(ctx context.Context, pid *atomic.Int64, _ *consumerEnv, abort chan<- struct{}) error {
				tx, err := pg.Tx(ctx)
				if err != nil {
					return err
				}
				var p int64
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&p); err != nil {
					return err
				}
				pid.Store(p)
				abort <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			},
			abort: func(t *testing.T, e *consumerEnv, pid int64, cancel context.CancelFunc) {
				var ok bool
				if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT pg_terminate_backend($1)", pid).Scan(&ok); err != nil || !ok {
					t.Fatalf("pg_terminate_backend = %v, %v", ok, err)
				}
				// The worker process dies with its backend: its goroutine never commits.
				cancel()
			},
		},
		"handler_error": {
			hook: func(context.Context, *atomic.Int64, *consumerEnv, chan<- struct{}) error {
				return errors.New("probe failure after inserting jobs")
			},
		},
		"handler_panic": {
			hook: func(context.Context, *atomic.Int64, *consumerEnv, chan<- struct{}) error {
				panic("probe panic after inserting jobs")
			},
		},
		"context_cancelled": {
			hook: func(ctx context.Context, _ *atomic.Int64, _ *consumerEnv, abort chan<- struct{}) error {
				abort <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			},
			abort: func(_ *testing.T, _ *consumerEnv, _ int64, cancel context.CancelFunc) { cancel() },
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newConsumerEnv(t, envOptions{})
			cfg := consumerCfg()
			w := e.newWorker(t, cfg)
			var pid atomic.Int64
			abort := make(chan struct{}, 1)
			w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter, hook: func(ctx context.Context, ev sync.Event) error {
				if ev.Seq != 3 {
					return nil
				}
				return c.hook(ctx, &pid, e, abort)
			}})
			if err := w.svc.Ensure(context.Background()); err != nil {
				t.Fatal(err)
			}
			e.appendSettings(t, 4)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := w.svc.Round(ctx, sync.ConsumerWebhooks)
				done <- err
			}()
			if c.abort != nil {
				select {
				case <-abort:
				case <-time.After(10 * time.Second):
					t.Fatal("the handler did not reach seq 3")
				}
				c.abort(t, e, pid.Load(), cancel)
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("the round committed")
				}
				if name == "handler_panic" && errs.KindOf(err) != errs.KindInternal {
					t.Errorf("a panic became %v, want kind internal", err)
				}
			case <-time.After(20 * time.Second):
				t.Fatal("the round did not end")
			}
			if got := e.probeJobs(t); len(got) != 0 {
				t.Fatalf("jobs of the aborted round: %v", got)
			}
			if got := e.cursor(t, sync.ConsumerWebhooks); got != 0 {
				t.Fatalf("cursor at %d after the aborted round", got)
			}
			restarted := e.newWorker(t, cfg)
			restarted.register(t, sync.ConsumerWebhooks, &probe{inserter: restarted.inserter})
			restarted.runner(t, cfg, nil, nil)
			e.waitCursor(t, sync.ConsumerWebhooks, 4, 20*time.Second)
			e.exactlyTwoJobs(t, 1, 4)
		})
	}
}

// waitWakeConnection waits until the one LISTEN connection of the waker is open and idle.
func waitWakeConnection(t *testing.T, e *consumerEnv) {
	t.Helper()
	waitFor(t, 10*time.Second, "the wake connection", func() bool {
		var got int
		if err := e.db.SuperConn(t).QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity
			WHERE application_name = $1 AND datname = current_database() AND state = 'idle'`, postgres.WakerAppName).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got == 1
	})
}

// TestConsumerWakesOnNotify (plan M2-01: "`NOTIFY` and polling wake the consumers"): with consumer.poll_interval
// far above the wait bound, a committed event gets its jobs within the bound, so only the
// notification can have woken the round.
func TestConsumerWakesOnNotify(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	cfg.PollInterval = time.Hour
	cfg.ReconnectBackoffCap = time.Hour
	w := e.newWorker(t, cfg)
	counting := &countingRounds{Rounds: w.svc}
	w.register(t, sync.ConsumerPush, &probe{inserter: w.inserter})
	w.runner(t, cfg, nil, counting)
	waitWakeConnection(t, e)
	waitFor(t, 10*time.Second, "the rounds after the start", func() bool { return counting.rounds.Load() >= 1 })
	time.Sleep(200 * time.Millisecond)
	before := counting.rounds.Load()
	e.appendSettings(t, 1)
	waitFor(t, 5*time.Second, "the jobs of the notified event", func() bool { return len(e.probeJobs(t)) == 1 })
	if counting.rounds.Load() <= before {
		t.Fatal("no round ran after the notification")
	}
	e.exactlyTwoJobs(t, 1, 1)
}

// TestConsumerWakesOnPoll (plan M2-01, same item): with a waker that never signals, a committed
// event gets its jobs within a small multiple of consumer.poll_interval.
func TestConsumerWakesOnPoll(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	cfg.PollInterval = 200 * time.Millisecond
	w := e.newWorker(t, cfg)
	w.register(t, sync.ConsumerPush, &probe{inserter: w.inserter})
	w.runner(t, cfg, silentWaker{}, nil)
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	e.appendSettings(t, 1)
	waitFor(t, 5*cfg.PollInterval, "the jobs of the polled event", func() bool { return len(e.probeJobs(t)) == 1 })
	t.Logf("jobs after %s with poll interval %s", time.Since(start), cfg.PollInterval)
	e.waitCursor(t, sync.ConsumerPush, 1, 5*time.Second)
	e.exactlyTwoJobs(t, 1, 1)
}

// TestConsumerListenLossLosesNothing: the wake connection is terminated several times while
// writers append; the waker reconnects and every seq has exactly two probe jobs.
func TestConsumerListenLossLosesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	w := e.newWorker(t, cfg)
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter})
	w.runner(t, cfg, nil, nil)
	waitWakeConnection(t, e)
	var wg stdsync.WaitGroup
	for writer := range 3 {
		wg.Go(func() {
			for i := range 10 {
				e.appendSettings(t, 1+(writer+i)%3)
				time.Sleep(15 * time.Millisecond)
			}
		})
	}
	super := e.db.SuperConn(t)
	for range 3 {
		waitWakeConnection(t, e)
		if _, err := super.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity
			WHERE application_name = $1 AND datname = current_database()`, postgres.WakerAppName); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
	head := e.head(t)
	e.waitCursor(t, sync.ConsumerWebhooks, head, 30*time.Second)
	e.exactlyTwoJobs(t, 1, head)
	if countLines(w.logs, "event consumer LISTEN connection lost, polling and reconnecting") == 0 {
		t.Error("no line for the lost wake connection")
	}
	waitWakeConnection(t, e)
}

// TestConsumerRetentionLossSkipsAndLogs: with small partitions and a clock past
// sync.event_retention, the partition maintenance drops the partitions below the cursor's next
// number; the round processes from min(seq), returns no error and logs one line with the
// consumer.
func TestConsumerRetentionLossSkipsAndLogs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pc := app.PartitionConfig{Size: 10, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second}
	e := newConsumerEnv(t, envOptions{partitions: pc})
	cfg := consumerCfg()
	cfg.BatchSize = 100
	w := e.newWorker(t, cfg)
	w.register(t, sync.ConsumerPush, &probe{inserter: w.inserter})
	if err := w.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		e.appendSettings(t, 5)
		// The clock is still before the retention cutoff: this run only adds partitions ahead.
		if _, err := e.maintenance(t, pc).Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	e.clock.Set(time.Now().Add(pc.Retention + time.Hour))
	res, err := e.maintenance(t, pc).Run(ctx)
	if err != nil || res.Dropped == 0 {
		t.Fatalf("maintenance = %+v, %v; want dropped partitions", res, err)
	}
	minSeq, err := e.repo.MinRetainedSeq(ctx)
	if err != nil || minSeq <= 1 {
		t.Fatalf("min(seq) = %d, %v", minSeq, err)
	}
	got, err := w.svc.Round(ctx, sync.ConsumerPush)
	if err != nil || got.Outcome != app.RoundProcessed {
		t.Fatalf("Round = %+v, %v", got, err)
	}
	e.exactlyTwoJobs(t, minSeq, 25)
	if got := e.cursor(t, sync.ConsumerPush); got != 25 {
		t.Fatalf("cursor = %d, want 25", got)
	}
	logs := w.logs.String()
	if n := countLines(w.logs, "event consumer lost events to retention"); n != 1 {
		t.Fatalf("%d retention lines, want 1:\n%s", n, logs)
	}
	for _, want := range []string{`"` + attr.ConsumerName + `":"push"`, `"` + attr.EventSeq + `":`, `"level":"ERROR"`} {
		if !strings.Contains(logs, want) {
			t.Errorf("log lacks %s:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "theme") {
		t.Errorf("the log carries a payload:\n%s", logs)
	}
}

// insertRaw inserts an event_log row with seq as the owner, bypassing the counter.
func insertRaw(t *testing.T, e *consumerEnv, seq int64) {
	t.Helper()
	if _, err := e.db.SuperConn(t).Exec(context.Background(), `INSERT INTO event_log
		(seq, id, type, resource_kind, receive_action, payload, permission_version)
		VALUES ($1::bigint, gen_random_uuid(), 'settings.updated', 'user', 'settings.read', '{}'::jsonb, 0)`, seq); err != nil {
		t.Fatal(err)
	}
}

// TestConsumerHoleFailsClosed: a hole the retention does not explain fails every round with
// ErrConsumerGap without moving the cursor or creating jobs; repeated failures log one line, and
// after the hole is closed the consumer recovers with one info line.
func TestConsumerHoleFailsClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	w := e.newWorker(t, cfg)
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter})
	e.appendSettings(t, 2)
	if err := w.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	insertRaw(t, e, 4)
	if _, err := w.svc.Round(ctx, sync.ConsumerWebhooks); !errors.Is(err, domain.ErrConsumerGap) {
		t.Fatalf("Round over the hole = %v, want ErrConsumerGap", err)
	}
	counting := &countingRounds{Rounds: w.svc}
	w.runner(t, cfg, nil, counting)
	waitFor(t, 10*time.Second, "several failed rounds", func() bool { return counting.failed.Load() >= 3 })
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 2 {
		t.Fatalf("cursor at %d over the hole", got)
	}
	if got := e.probeJobs(t); len(got) != 0 {
		t.Fatalf("jobs over the hole: %v", got)
	}
	if n := countLines(w.logs, "event consumer round failed"); n != 1 {
		t.Fatalf("%d failure lines for %d failed rounds, want 1:\n%s", n, counting.failed.Load(), w.logs)
	}
	if !strings.Contains(w.logs.String(), `"`+attr.ErrorType+`":"consumer_gap"`) {
		t.Errorf("the failure line lacks error.type consumer_gap:\n%s", w.logs)
	}
	insertRaw(t, e, 3)
	e.waitCursor(t, sync.ConsumerWebhooks, 4, 10*time.Second)
	e.exactlyTwoJobs(t, 3, 4)
	waitFor(t, 5*time.Second, "the recovery line", func() bool { return countLines(w.logs, "event consumer recovered") == 1 })
}

// TestConsumerRoundRejectsAppendedEvents: a handler that appends a durable event fails the
// round: no event, no job, no cursor move (D-M2-01-4).
func TestConsumerRoundRejectsAppendedEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	w := e.newWorker(t, consumerCfg())
	w.register(t, sync.ConsumerPush, &probe{inserter: w.inserter, hook: func(ctx context.Context, _ sync.Event) error {
		return e.appender.Append(ctx, settingsEvent())
	}})
	if err := w.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	e.appendSettings(t, 1)
	if _, err := w.svc.Round(ctx, sync.ConsumerPush); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Round = %v, want kind internal", err)
	}
	if got := e.head(t); got != 1 {
		t.Fatalf("head = %d; the appended event was committed", got)
	}
	if got := e.probeJobs(t); len(got) != 0 {
		t.Fatalf("jobs: %v", got)
	}
	if got := e.cursor(t, sync.ConsumerPush); got != 0 {
		t.Fatalf("cursor = %d", got)
	}
}

// alwaysFails is a handler whose every call fails.
type alwaysFails struct{}

func (alwaysFails) HandleEvent(context.Context, sync.Event) error { return errors.New("always fails") }

// TestConsumersIndependent: a push consumer that always fails does not hold back webhooks; only
// push shows a lag.
func TestConsumersIndependent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	w := e.newWorker(t, cfg)
	w.register(t, sync.ConsumerPush, alwaysFails{})
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter})
	w.runner(t, cfg, nil, nil)
	for range 4 {
		e.appendSettings(t, 3)
	}
	e.waitCursor(t, sync.ConsumerWebhooks, 12, 20*time.Second)
	if got := e.cursor(t, sync.ConsumerPush); got != 0 {
		t.Fatalf("push cursor = %d, want 0", got)
	}
	states, err := w.svc.States(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0].Name != sync.ConsumerPush || states[0].HeadSeq-states[0].LastSeq != 12 ||
		states[1].HeadSeq-states[1].LastSeq != 0 || !states[1].OldestPendingAt.IsZero() || states[0].OldestPendingAt.IsZero() {
		t.Fatalf("states = %+v", states)
	}
	e.exactlyTwoJobs(t, 1, 12)
}

// TestConsumerShutdownFinishesRound: Stop during a blocked round starts no new round, and a round
// released within the budget commits; with an expired budget the round rolls back, and a
// restarted runner creates every job exactly once.
func TestConsumerShutdownFinishesRound(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t, envOptions{})
	cfg := consumerCfg()
	w := e.newWorker(t, cfg)
	inside, release := make(chan struct{}, 1), make(chan struct{})
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter, hook: func(ctx context.Context, ev sync.Event) error {
		if ev.Seq != 1 && ev.Seq != 6 {
			return nil
		}
		inside <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	counting := &countingRounds{Rounds: w.svc}
	r, err := app.NewConsumerRunner(counting, postgres.NewWaker(w.pool, cfg, w.log), cfg, app.RunnerOptions{Logger: w.log})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.svc.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.appendSettings(t, 5)
	e.appendSettings(t, 5)
	<-inside
	stopped := make(chan error, 1)
	go func() { stopped <- r.Stop(context.Background()) }()
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned %v during the round", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	rounds := counting.rounds.Load()
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 5 {
		t.Fatalf("cursor after the stop = %d, want the full batch 5 committed", got)
	}
	time.Sleep(300 * time.Millisecond)
	if counting.rounds.Load() != rounds || e.cursor(t, sync.ConsumerWebhooks) != 5 {
		t.Fatal("a round ran after Stop although the batch was full")
	}

	// Expired budget: the round blocks at seq 6 and is cancelled.
	second := e.newWorker(t, cfg)
	release2 := make(chan struct{})
	second.register(t, sync.ConsumerWebhooks, &probe{inserter: second.inserter, hook: func(ctx context.Context, ev sync.Event) error {
		if ev.Seq != 6 {
			return nil
		}
		inside <- struct{}{}
		select {
		case <-release2:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	r2, err := app.NewConsumerRunner(second.svc, postgres.NewWaker(second.pool, cfg, second.log), cfg, app.RunnerOptions{Logger: second.log})
	if err != nil {
		t.Fatal(err)
	}
	if err := r2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-inside
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := r2.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop with an expired budget = %v", err)
	}
	waitFor(t, 10*time.Second, "the cancelled round to roll back", func() bool {
		return r2.Stop(context.Background()) == nil
	})
	if got := e.cursor(t, sync.ConsumerWebhooks); got != 5 {
		t.Fatalf("cursor after the cancelled round = %d, want 5", got)
	}
	e.exactlyTwoJobs(t, 1, 5)
	restarted := e.newWorker(t, cfg)
	restarted.register(t, sync.ConsumerWebhooks, &probe{inserter: restarted.inserter})
	restarted.runner(t, cfg, nil, nil)
	e.waitCursor(t, sync.ConsumerWebhooks, 10, 20*time.Second)
	e.exactlyTwoJobs(t, 1, 10)
}

// gaugePoints collects both gauges from reader: consumer name to value.
func gaugePoints(t *testing.T, reader *sdkmetric.ManualReader) (lag, age map[string]float64) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	lag, age = map[string]float64{}, map[string]float64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Gauge[int64]:
				for _, p := range d.DataPoints {
					if p.Attributes.Len() != 1 {
						t.Errorf("%s has attributes %v", m.Name, p.Attributes.ToSlice())
					}
					v, _ := p.Attributes.Value(attr.ConsumerName)
					lag[v.AsString()] = float64(p.Value)
				}
			case metricdata.Gauge[float64]:
				for _, p := range d.DataPoints {
					if p.Attributes.Len() != 1 {
						t.Errorf("%s has attributes %v", m.Name, p.Attributes.ToSlice())
					}
					v, _ := p.Attributes.Value(attr.ConsumerName)
					age[v.AsString()] = p.Value
				}
			}
		}
	}
	return lag, age
}

// TestConsumerLagGauges (plan M2-01: "metric of the backlog per consumer"): with an SDK
// manual reader, the lag is max(seq) minus last_seq per consumer and the age is the clock minus
// occurred_at of the oldest event after the cursor; both are zero at the head, and the only
// attribute is bothy.consumer.name. No runner runs, as in role api.
func TestConsumerLagGauges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newConsumerEnv(t, envOptions{})
	w := e.newWorker(t, consumerCfg())
	w.register(t, sync.ConsumerPush, &probe{inserter: w.inserter})
	w.register(t, sync.ConsumerWebhooks, &probe{inserter: w.inserter})
	if err := w.svc.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	if _, err := otelmetrics.RegisterConsumerGauges(mp, w.svc, e.clock); err != nil {
		t.Fatal(err)
	}
	lag, age := gaugePoints(t, reader)
	if lag["push"] != 0 || lag["webhooks"] != 0 || age["push"] != 0 || age["webhooks"] != 0 || len(lag) != 2 {
		t.Fatalf("empty log: lag %v, age %v", lag, age)
	}
	e.appendSettings(t, 3)
	var first time.Time
	if err := e.db.SuperConn(t).QueryRow(ctx, "SELECT occurred_at FROM event_log WHERE seq = 1").Scan(&first); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(first.Add(90 * time.Second))
	if _, err := w.svc.Round(ctx, sync.ConsumerWebhooks); err != nil {
		t.Fatal(err)
	}
	lag, age = gaugePoints(t, reader)
	if lag["push"] != 3 || lag["webhooks"] != 0 {
		t.Errorf("lag = %v, want push 3 and webhooks 0", lag)
	}
	if age["push"] != 90 || age["webhooks"] != 0 {
		t.Errorf("pending_age = %v, want push 90 and webhooks 0", age)
	}
	if _, err := w.svc.Round(ctx, sync.ConsumerPush); err != nil {
		t.Fatal(err)
	}
	lag, age = gaugePoints(t, reader)
	if lag["push"] != 0 || age["push"] != 0 {
		t.Errorf("at the head: lag %v, age %v", lag, age)
	}
}
