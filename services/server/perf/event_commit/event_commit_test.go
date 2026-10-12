// SPDX-License-Identifier: Apache-2.0

//go:build perf

package eventcommit_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	stdsync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

var (
	writers  = flag.Int("perf.writers", 0, "concurrent writers; 0 means 4 * GOMAXPROCS")
	duration = flag.Duration("perf.duration", 20*time.Second, "how long each measurement runs")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(pgtest.Main(m))
}

// target is the name of the measured threshold.
const target = "perf.event_commit_min_per_s"

// TestEventCommitThroughput measures durable events per second with the counter pattern on a
// database with the durability settings of production (ADR 0063 point 2): each unit of work
// updates one scratch row and appends one event through the production composition of
// internal/wire (sync hook, then audit hook). It fails below perf.event_commit_min_per_s; the
// second subtest repeats the measurement with an idle River worker on the same database and only
// prints its number (02 "Offen", PollOnly).
func TestEventCommitThroughput(t *testing.T) {
	targets, err := report.Load(report.ArchitectureChapter())
	if err != nil {
		t.Fatal(err)
	}
	threshold, err := targets.Minimum(target)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := targets.Text("perf.reference_profile")
	if err != nil {
		t.Fatal(err)
	}
	n := *writers
	if n <= 0 {
		n = 4 * runtime.GOMAXPROCS(0)
	}
	db := pgtest.New(t, pgtest.Durable())
	super := db.SuperConn(t)
	for _, setting := range []string{"fsync", "synchronous_commit"} {
		var v string
		if err := super.QueryRow(context.Background(), "SHOW "+setting).Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v != "on" {
			t.Fatalf("%s = %s; ADR 0063 point 2 measures only with fsync and synchronous_commit on", setting, v)
		}
	}
	owner := db.Pool(t, pg.RoleOwner)
	for _, stmt := range []string{
		"CREATE TABLE perf_scratch (id int PRIMARY KEY, n bigint NOT NULL DEFAULT 0)",
		fmt.Sprintf("INSERT INTO perf_scratch (id) SELECT generate_series(1, %d)", n),
		"GRANT SELECT, UPDATE ON perf_scratch TO bothy_app",
	} {
		if _, err := owner.Exec(context.Background(), stmt); err != nil {
			t.Fatal(err)
		}
	}
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = n + 4
	pool := db.OpenPool(t, cfg)
	reg, err := wire.NewTxnRegistry(wire.NewModules(pool, nil))
	if err != nil {
		t.Fatal(err)
	}
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 2, StatementTimeout: 5 * time.Second, LockTimeout: 2 * time.Second, IdleInTxTimeout: 10 * time.Second}, reg)
	sy, err := wire.NewSync(pool, uow, wire.SyncConfig{
		Appender:   syncapp.AppenderConfig{PayloadMaxBytes: 65536},
		Partitions: syncapp.PartitionConfig{Size: 1_000_000, Ahead: 3, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		Tail:       syncpg.TailConfig{PollInterval: time.Second, BatchSize: 1000, ReconnectBackoffCap: 10 * time.Second},
		Cleanup:    syncapp.IdempotencyCleanupConfig{Interval: time.Hour, BatchSize: 1000},
		Consumers: syncapp.ConsumerConfig{BatchSize: 500, PollInterval: time.Second, ReconnectBackoffCap: 10 * time.Second,
			StatesTimeout: 5 * time.Second},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sy.Maintenance().Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Logf("profile %s: %s; this machine: %d logical CPUs", "perf.reference_profile", profile, runtime.NumCPU())

	t.Run("counter_pattern", func(t *testing.T) {
		rate := measure(t, uow, pool, sy.Appender(), n)
		fmt.Printf("%s measured=%.0f threshold=%.0f cpus=%d writers=%d\n", target, rate, threshold, runtime.NumCPU(), n)
		if err := report.CheckMinimum(target, rate, threshold); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("with_idle_river_worker", func(t *testing.T) {
		jobstest.Migrate(t, db, "jobs")
		rt, err := jobs.New(pool, jobs.Config{
			Schema:          "jobs",
			MaxWorkers:      map[string]int{jobs.QueueDefault: 1, jobs.QueueFiles: 1, jobs.QueueDelivery: 1, jobs.QueueMaintenance: 1, jobs.QueuePrivacy: 1},
			SoftStopTimeout: 5 * time.Second,
		}, jobs.RoleWorker, jobs.NewRegistry(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := rt.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = rt.Stop(ctx)
		}()
		rate := measure(t, uow, pool, sy.Appender(), n)
		fmt.Printf("%s with_idle_river_worker measured=%.0f threshold=%.0f cpus=%d writers=%d\n", target, rate, threshold, runtime.NumCPU(), n)
	})
}

// measure runs n writers for the duration flag and returns committed events per second. Each
// unit updates the scratch row of its writer and appends one event.
func measure(t *testing.T, uow *pg.UnitOfWork, pool *pg.Pool, appender eventlog.Appender, n int) float64 {
	t.Helper()
	var count atomic.Int64
	var failures atomic.Int64
	deadline := time.Now().Add(*duration)
	start := time.Now()
	var wg stdsync.WaitGroup
	for w := range n {
		wg.Go(func() {
			ev := eventlog.Event{
				Type: "thread.updated", SchemaVersion: 1, ActorKind: eventlog.ActorSystem,
				ResourceKind: "thread", ReceiveAction: "thread.read", Payload: []byte(`{"n":1}`),
			}
			for time.Now().Before(deadline) {
				err := uow.Do(context.Background(), func(ctx context.Context) error {
					if _, err := pg.Querier(ctx, pool).Exec(ctx, "UPDATE perf_scratch SET n = n + 1 WHERE id = $1", w+1); err != nil {
						return err
					}
					return appender.Append(ctx, ev)
				})
				if err != nil {
					failures.Add(1)
					continue
				}
				count.Add(1)
			}
		})
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()
	if f := failures.Load(); f > 0 {
		t.Errorf("%d units failed during the measurement", f)
	}
	return float64(count.Load()) / elapsed
}
