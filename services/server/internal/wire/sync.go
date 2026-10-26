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

	"github.com/OWNER/bothy/services/server/internal/files"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	syncmetrics "github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
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
	// Idempotency holds sync.idempotency_key_ttl for the Idempotency-Key stage (M1-20).
	Idempotency httpserver.IdempotencyConfig
	// Cleanup holds the interval and batch size of the job sync.idempotency_cleanup (M1-20).
	Cleanup syncapp.IdempotencyCleanupConfig
	// Consumers holds consumer.batch_size, consumer.poll_interval,
	// database.listen_reconnect_backoff_cap and database.statement_timeout of the worker
	// consumers (M2-01).
	Consumers syncapp.ConsumerConfig
}

// SyncConfigFrom binds SyncConfig from v. The error names the key whose binding failed, never a
// value.
func SyncConfigFrom(v *config.Values) (SyncConfig, error) {
	var c SyncConfig
	for _, dst := range []any{&c.Appender, &c.Partitions, &c.Tail, &c.Idempotency, &c.Cleanup, &c.Consumers} {
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
// hooks receive them, TestCountersOnlyUsedByPhaseHooks), the repository of sync and the metrics
// of sync/adapters/otelmetrics on the global meter provider (M1-68). It opens no connection, so
// the golden test builds it without a database (pool nil). It panics when the instruments cannot
// be created, a programming error of their declaration.
func newSyncHook(pool *pg.Pool, log *slog.Logger) *syncapp.PhaseHook {
	counters := instanceapp.NewCounters(instancepg.NewRepository(pool))
	m, err := syncmetrics.New(nil)
	if err != nil {
		panic(err)
	}
	return syncapp.NewPhaseHook(counters, syncpg.NewRepository(pool, orDiscard(log))).WithMetrics(m, nil)
}

// Sync is the sync module of a process: the appender every module receives as eventlog.Appender,
// the partition maintenance (job and bothy migrate up), the log tail of the roles api and all,
// the idempotency store behind kernel idempotency.Store and its cleanup job (M1-20), and the
// worker consumers (every role builds them for the lag gauges; worker and all run them; M2-01).
type Sync struct {
	cfg         SyncConfig
	pool        *pg.Pool
	uow         txn.UnitOfWork
	clock       clock.Clock
	repo        *syncpg.Repository
	appender    *syncapp.Appender
	maintenance *syncapp.PartitionMaintenance
	keys        *syncapp.IdempotencyStore
	cleanup     *syncapp.IdempotencyCleanup
	consumers   *syncapp.ConsumerService
	log         *slog.Logger
}

// NewSync assembles the sync module over pool with the unit of work uow (whose registry holds the
// sync hook), cfg, the clock c of the retention cutoff and the key cleanup, and log. It opens no
// connection.
//
// Errors: a configuration the appender, the maintenance, the cleanup or the worker consumers refuse.
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
	keyRows := syncpg.NewIdempotencyRepository(pool)
	cleanup, err := syncapp.NewIdempotencyCleanup(cfg.Cleanup, uow, keyRows, c)
	if err != nil {
		return nil, err
	}
	// The worker consumers push and webhooks keep running in maintenance mode (ADR 0065 item 4,
	// M7-08), so their units of work may write.
	cs, err := syncapp.NewConsumerService(cfg.Consumers, txn.ExemptInMaintenance(uow), repo, repo, log)
	if err != nil {
		return nil, err
	}
	return &Sync{
		cfg: cfg, pool: pool, uow: uow, clock: c, repo: repo, appender: a, maintenance: m,
		keys: syncapp.NewIdempotencyStore(keyRows, uow), cleanup: cleanup, consumers: cs, log: log,
	}, nil
}

// IdempotencyStore returns the store of sync behind the kernel port (02 "Ports, events and
// sources per module": sync implements idempotency.Store).
func (s *Sync) IdempotencyStore() idempotency.Store { return s.keys }

// IdempotencyCleanup returns the use case of the job sync.idempotency_cleanup.
func (s *Sync) IdempotencyCleanup() *syncapp.IdempotencyCleanup { return s.cleanup }

// InstallIdempotency is the Idempotency-Key part of start step 7 for the roles api and all: it
// installs StageIdempotency (02 "Chain in the api node" step 10) in chain over the store and the
// unit of work of sy, with sync.idempotency_key_ttl from cfg and the clock of sy. Call it before
// the routes are mounted. The error is a configuration the stage refuses.
func InstallIdempotency(chain *httpserver.Chain, sy *Sync, cfg SyncConfig) error {
	stage, err := httpserver.NewIdempotencyStage(cfg.Idempotency, sy.keys, sy.uow, sy.clock)
	if err != nil {
		return err
	}
	chain.Use(httpserver.StageIdempotency, stage)
	return nil
}

// Consumers returns the worker consumers of the process (sync.Consumers): only internal/wire
// registers handlers, before the runner starts.
func (s *Sync) Consumers() *syncapp.ConsumerService { return s.consumers }

// NewConsumerRunner returns the runner of the worker consumers with its own LISTEN connection
// (application name bothy-consumer-wake, D-M2-01-7).
func (s *Sync) NewConsumerRunner() (*syncapp.ConsumerRunner, error) {
	return syncapp.NewConsumerRunner(s.consumers, syncpg.NewWaker(s.pool, s.cfg.Consumers, s.log), s.cfg.Consumers,
		syncapp.RunnerOptions{Logger: s.log})
}

// consumerHandlers returns the production worker consumers of the event log: webhooks over the
// apps step (M2-26) and push over the push path of notify (M3-19; ADR 0051, PLAN P-13). A nil ap
// or push (the step did not run) registers none of its consumer.
func consumerHandlers(ap *Apps, push *Push) map[sync.ConsumerName]sync.ConsumerHandler {
	out := map[sync.ConsumerName]sync.ConsumerHandler{}
	if ap != nil {
		out[sync.ConsumerWebhooks] = ap.Consumer()
	}
	if push != nil {
		out[sync.ConsumerPush] = push.Consumer()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// History returns the reads of retained events for the worker consumers (sync.History, M3-19).
func (s *Sync) History() sync.History { return s.repo }

// startConsumers is the start of the worker consumers in worker and all after River (02
// "Start" step 8): it registers handlers, inserts their cursor rows at the head (Ensure),
// registers the stop step event_consumers in lifecycle.PhaseConsumers without an own budget (the
// rest of shutdown.total_timeout; D-M2-01-9) and starts the runner; an unexpected end of the
// runner ends the process with exit code 1.
func startConsumers(ctx context.Context, r *lifecycle.Runner, sy *Sync, handlers map[sync.ConsumerName]sync.ConsumerHandler) error {
	svc := sy.Consumers()
	for name, h := range handlers {
		if err := svc.Register(name, h); err != nil {
			return err
		}
	}
	if err := svc.Ensure(ctx); err != nil {
		return err
	}
	runner, err := sy.NewConsumerRunner()
	if err != nil {
		return err
	}
	r.OnStop(consumerStopStep(runner))
	if err := runner.Start(ctx); err != nil {
		return err
	}
	r.FailOn("event_consumers", runner.Failed())
	return nil
}

// consumerStopStep is the stop step event_consumers (02 "Shutdown", worker step 2): the runner
// starts no new round and waits for the running ones; Budget 0 gives it the rest of
// shutdown.total_timeout, after which the rounds are cancelled and roll back.
func consumerStopStep(runner *syncapp.ConsumerRunner) lifecycle.StopStep {
	return lifecycle.StopStep{Name: "event_consumers", Phase: lifecycle.PhaseConsumers, Run: runner.Stop}
}

// LogReader returns the log reader of the delivery jobs (sync.LogReader, M2-26).
func (s *Sync) LogReader() sync.LogReader { return s.repo }

// Redactor returns the redactor of stored event payloads behind the kernel port (02 "Ports,
// events and sources per module": sync implements eventlog.Redactor, M1-58).
func (s *Sync) Redactor() eventlog.Redactor { return syncpg.NewRedactor(s.pool) }

// DraftFiles returns the source port files.DraftFileSource over drafts (02 "Ports, events and
// sources per module": sync implements files.DraftFileSource, M3-08): the cleanup of never
// attached files asks it whether a draft of the uploader still holds a file.
func (s *Sync) DraftFiles() files.DraftFileSource { return syncpg.NewDraftFiles(s.pool) }

// Appender returns the appender of durable events for the module constructors.
func (s *Sync) Appender() eventlog.Appender { return s.appender }

// Maintenance returns the partition maintenance.
func (s *Sync) Maintenance() *syncapp.PartitionMaintenance { return s.maintenance }

// NewTail returns the log tail of the process with sinks in delivery order (02 "Log tail per api
// process"); serve passes DeliverySink, the rights cache and then the hub.
func (s *Sync) NewTail(sinks ...sync.Sink) *syncpg.Tail {
	return syncpg.NewTail(s.pool, s.repo, s.cfg.Tail, sinks, s.log)
}

// startTail is start step 8 for api and all: it starts the log tail, waits until it has read the
// head it starts at (local_head, 02 "Start and graceful shutdown"), registers the stop step event_tail in
// lifecycle.PhaseEventTail and the liveness check event_tail, which fails once the goroutine of the
// tail and the hub has ended (02 "Health"). A tail that ends with an error (a sink error, a gap)
// ends the process with exit code 1 (02 "Panics"); a head that cannot be read ends the start.
func startTail(ctx context.Context, r *lifecycle.Runner, reg *health.Registry, tail *syncpg.Tail) error {
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
	reg.AddLiveness(CheckEventTail, TailLiveness(done))
	select {
	case <-tail.Started():
		return nil
	case <-done:
		return fmt.Errorf("log tail: %w", runErr)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CheckEventTail is the liveness check of the log tail and the hub.
const CheckEventTail = "event_tail"

// errTailStopped is the failure of the liveness check event_tail.
var errTailStopped = errors.New("the log tail and the hub have stopped")

// TailLiveness returns the liveness check of the goroutine of the log tail and the hub, which
// closes done when it ends: from then on /healthz fails (02 "Health"), since the process delivers
// nothing any more.
func TailLiveness(done <-chan struct{}) health.Check {
	return func(context.Context) error {
		select {
		case <-done:
			return errTailStopped
		default:
			return nil
		}
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
