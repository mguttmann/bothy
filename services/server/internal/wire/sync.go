// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	stdsync "sync"
	"time"

	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// SyncConfig is the configuration of the event log (M1-16), bound from the sync.* keys and the
// database keys the tail and the maintenance read.
type SyncConfig struct {
	// Appender holds sync.event_payload_max_bytes.
	Appender syncapp.AppenderConfig
	// Partitions holds the partition size, the partitions ahead, the retention and
	// database.lock_timeout.
	Partitions syncapp.PartitionConfig
	// Tail holds the poll interval, the batch size and the reconnect cap of the log tail.
	Tail syncpg.TailConfig
	// MaintenanceInterval is sync.partition_maintenance_interval.
	MaintenanceInterval time.Duration `config:"sync.partition_maintenance_interval"`
}

// SyncConfigFrom binds SyncConfig from v. The error names the key whose binding failed, never a
// value.
func SyncConfigFrom(v *config.Values) (SyncConfig, error) {
	var c SyncConfig
	for _, dst := range []any{&c.Appender, &c.Partitions, &c.Tail} {
		if err := config.Bind(v, dst); err != nil {
			return SyncConfig{}, err
		}
	}
	var interval struct {
		Interval time.Duration `config:"sync.partition_maintenance_interval"`
	}
	if err := config.Bind(v, &interval); err != nil {
		return SyncConfig{}, err
	}
	c.MaintenanceInterval = interval.Interval
	return c, nil
}

// newSyncHook returns the sync phase hook over pool: the counters of instance (only the phase
// hooks receive them, TestCountersOnlyUsedByPhaseHooks) and the repository of sync. It opens no
// connection, so the golden test builds it without a database (pool nil).
func newSyncHook(pool *pg.Pool, log *slog.Logger) *syncapp.PhaseHook {
	counters := instanceapp.NewCounters(instancepg.NewRepository(pool))
	return syncapp.NewPhaseHook(counters, syncpg.NewRepository(pool, orDiscard(log)))
}

// Sync is the event log of a process: the appender every module receives as eventlog.Appender,
// the partition maintenance (job and bothy migrate up) and the log tail of the roles api and all.
type Sync struct {
	cfg         SyncConfig
	pool        *pg.Pool
	repo        *syncpg.Repository
	appender    *syncapp.Appender
	maintenance *syncapp.PartitionMaintenance
	log         *slog.Logger
}

// NewSync assembles the event log over pool with the unit of work uow (whose registry holds the
// sync hook), cfg, the clock c of the retention cutoff and log. It opens no connection.
//
// Errors: a configuration the appender or the maintenance refuse.
func NewSync(pool *pg.Pool, uow txn.UnitOfWork, cfg SyncConfig, c clock.Clock, log *slog.Logger) (*Sync, error) {
	log = orDiscard(log)
	if c == nil {
		c = clock.System()
	}
	repo := syncpg.NewRepository(pool, log)
	a, err := syncapp.NewAppender(cfg.Appender, telemetry.Correlator{})
	if err != nil {
		return nil, err
	}
	m, err := syncapp.NewPartitionMaintenance(cfg.Partitions, uow, repo, c)
	if err != nil {
		return nil, err
	}
	return &Sync{cfg: cfg, pool: pool, repo: repo, appender: a, maintenance: m, log: log}, nil
}

// Appender returns the appender of durable events for the module constructors.
func (s *Sync) Appender() eventlog.Appender { return s.appender }

// Maintenance returns the partition maintenance.
func (s *Sync) Maintenance() *syncapp.PartitionMaintenance { return s.maintenance }

// NewTail returns the log tail of the process with sinks in delivery order: M1-28 adds the
// rights cache, then M1-65 the hub (02 "Log tail per api process"); today the list is empty.
func (s *Sync) NewTail(sinks ...sync.Sink) *syncpg.Tail {
	return syncpg.NewTail(s.pool, s.repo, s.cfg.Tail, sinks, s.log)
}

// tailSinks are the sinks of the log tail of api and all, in this order: the rights cache
// (M1-28), then the hub (M1-65). Empty until those tickets.
func tailSinks() []sync.Sink { return nil }

// startTail is start step 8 for api and all: it starts the log tail, waits until it has read the
// head it starts at (local_head, 02 "Start and graceful shutdown"), and registers the stop step event_tail in
// lifecycle.PhaseEventTail. A tail that ends with an error (a sink error, a gap) ends the process
// with exit code 1 (02 "Panics"); a head that cannot be read ends the start.
func startTail(ctx context.Context, r *lifecycle.Runner, tail *syncpg.Tail) error {
	// The stop step event_tail calls cancel.
	tctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var runErr error
	var once stdsync.Once
	go func() {
		defer close(done)
		if err := tail.Run(tctx); err != nil {
			runErr = err
			r.Fail(fmt.Errorf("log tail: %w", err))
		}
	}()
	stop := func(sctx context.Context) error {
		once.Do(cancel)
		select {
		case <-done:
			return nil
		case <-sctx.Done():
			return sctx.Err()
		}
	}
	r.OnStop(lifecycle.StopStep{Name: "event_tail", Phase: lifecycle.PhaseEventTail, Run: stop})
	select {
	case <-tail.Started():
		return nil
	case <-done:
		return fmt.Errorf("log tail: %w", runErr)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// partitionStep is the after-step of bothy migrate up that creates the partitions of event_log
// once (upgrade.md rule M10, 03 event_log), so a fresh or upgraded database has partitions
// before serve starts.
type partitionStep struct {
	maintenance *syncapp.PartitionMaintenance
}

// Name names the step in errors.
func (s partitionStep) Name() string { return "event_log_partitions" }

// Run runs the partition maintenance on the pool of migrate; the *sql.DB of the migrator is the
// same pool and is not used. A deferred drop (a long reader such as a backup holds event_log) is
// no failure of migrate up: the partitions ahead are committed, and the job drops later.
func (s partitionStep) Run(ctx context.Context, _ *sql.DB) error {
	_, err := s.maintenance.Run(ctx)
	if errors.Is(err, syncapp.ErrDropDeferred) {
		return nil
	}
	return err
}

// errNoSyncHook reports Modules without the sync hook: every unit of work must number its
// durable events, so a registry without it is refused.
var errNoSyncHook = errors.New("wire: the txn registry needs the sync hook (Modules.Sync)")
