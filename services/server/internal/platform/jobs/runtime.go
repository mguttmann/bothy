// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// instrumentationName is the instrumentation scope of the job spans when New gets no tracer.
const instrumentationName = "github.com/OWNER/bothy/services/server/internal/platform/jobs"

// schemaPattern is the form of jobs.schema: an unquoted PostgreSQL identifier in lower case.
var schemaPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// ValidateSchema checks the value of jobs.schema: an unquoted lower-case identifier
// (^[a-z_][a-z0-9_]{0,62}$), not "public" (the domain schema must stay free of River's tables,
// R-AR-09), not "information_schema" and not starting with "pg_" (both belong to PostgreSQL).
// Reserved words such as user are allowed: River quotes the schema in every statement. internal/wire registers it as a
// cross-check of the configuration schema. The error names the key, never the value.
func ValidateSchema(schema string) error {
	switch {
	case !schemaPattern.MatchString(schema):
		return errors.New("jobs.schema must be an unquoted identifier: a lower-case letter or underscore, then up to 62 lower-case letters, digits or underscores")
	case schema == "public":
		return errors.New("jobs.schema must not name the schema of the domain tables")
	case schema == "information_schema":
		return errors.New("jobs.schema must not name a system schema of PostgreSQL")
	case len(schema) >= 3 && schema[:3] == "pg_":
		return errors.New("jobs.schema must not start with pg_, which PostgreSQL reserves")
	default:
		return nil
	}
}

// Config is the configuration of the job runtime, filled in internal/wire from jobs.schema,
// jobs.queue.<queue>.max_workers and shutdown.jobs_soft_timeout.
type Config struct {
	// Schema is jobs.schema, the PostgreSQL schema of River's tables.
	Schema string
	// MaxWorkers maps each of the five queues to its parallelism,
	// jobs.queue.<queue>.max_workers; every queue needs a positive value.
	MaxWorkers map[string]int
	// SoftStopTimeout is shutdown.jobs_soft_timeout: how long Stop lets running jobs finish
	// before their contexts are cancelled.
	SoftStopTimeout time.Duration
	// Logger receives River's log lines and the panic lines of the handler wrapper; nil means
	// slog.Default(). The server logger keeps only catalogue attributes (signals.md "Logs").
	Logger *slog.Logger
	// StatementTimeout is database.statement_timeout: it bounds the count query of bothy.job.queue
	// (0 means no bound of its own).
	StatementTimeout time.Duration
	// MeterProvider creates bothy.job.duration and bothy.job.queue; nil means the global provider
	// of platform/otel.
	MeterProvider metric.MeterProvider
}

// validate checks c.
func (c Config) validate() error {
	if err := ValidateSchema(c.Schema); err != nil {
		return fmt.Errorf("jobs: %w", err)
	}
	for _, q := range Queues() {
		if c.MaxWorkers[q] < 1 {
			return fmt.Errorf("jobs: jobs.queue.%s.max_workers must be positive", q)
		}
	}
	for q := range c.MaxWorkers {
		if !knownQueue(q) {
			return fmt.Errorf("jobs: unknown queue %q in MaxWorkers", q)
		}
	}
	if c.SoftStopTimeout <= 0 {
		return errors.New("jobs: shutdown.jobs_soft_timeout must be positive")
	}
	return nil
}

// Role says which part of River a process runs (02 "Roles of a process").
type Role string

// The roles; internal/wire maps server.role to them.
const (
	// RoleAPI only inserts jobs: its River client is never started and never joins the leader
	// election.
	RoleAPI Role = "api"
	// RoleWorker inserts and runs jobs, periodic jobs and River's maintenance.
	RoleWorker Role = "worker"
	// RoleAll is api and worker in one process.
	RoleAll Role = "all"
)

// works reports whether the role runs jobs.
func (r Role) works() bool { return r == RoleWorker || r == RoleAll }

// InsertOptions change one insert.
type InsertOptions struct {
	// ScheduledAt is the earliest time the job may run; the zero time means now.
	ScheduledAt time.Time
}

// Inserter inserts jobs inside a unit of work. Modules receive it from internal/wire.
type Inserter interface {
	// InsertTx inserts a job with args in the ambient transaction of ctx (pg.Tx), so the job
	// becomes visible to workers only when the unit of work commits and vanishes with a
	// rollback (02 "What never happens in a transaction"). opts may be nil.
	//
	// Errors (kernel/errs, code internal_error): ctx outside a unit of work, or in the phases
	// sync or audit (after the work phase the unit only numbers events and audits); the kind
	// of args is not registered, or args has another type than the registered one. Other errors
	// are those of the insert statement.
	InsertTx(ctx context.Context, args Args, opts *InsertOptions) error
}

// Runtime is the job runtime of one process: a River client over the pool of the process, the
// handlers of the registry and the role. It is the only owner of River in the server.
type Runtime struct {
	role   Role
	cfg    Config
	log    *slog.Logger
	kinds  map[string]*entry
	client *river.Client[pgx.Tx]
	pool   *pg.Pool
	// metrics are bothy.job.duration and bothy.job.queue; queueReg is the callback of the queue
	// gauge while River runs.
	metrics  *jobMetrics
	queueReg metric.Registration
	// works is false for RoleAPI and for a worker without registered kinds: River refuses to
	// start a client without workers, and such a process has nothing to run.
	works bool

	mu       sync.Mutex
	started  bool
	stopping bool
	failed   chan error
}

// New builds the runtime of role over pool (the process pool of bothy_app) with the handlers
// and periodic jobs of reg; reg cannot take registrations afterwards. tracer starts the job
// spans; nil means the tracer of the global provider, which platform/otel sets at the start.
//
// RoleAPI builds an insert-only River client (Workers, no Queues) that is never started.
// RoleWorker and RoleAll build a full client with the five queues and their parallelism, the
// periodic jobs, cfg.Schema and cfg.SoftStopTimeout; Start starts it.
//
// The error reports an invalid cfg (naming the configuration key), an unknown role, a nil pool
// or registry, a pool below two connections for RoleWorker and RoleAll (River holds one for
// LISTEN), or a River configuration error.
func New(pool *pg.Pool, cfg Config, role Role, reg *Registry, tracer trace.Tracer) (*Runtime, error) {
	if pool == nil || reg == nil {
		return nil, errors.New("jobs: New needs a pool and a registry")
	}
	if role != RoleAPI && !role.works() {
		return nil, fmt.Errorf("jobs: unknown role %q", role)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if role.works() {
		if err := needConns(pool); err != nil {
			return nil, err
		}
	}
	if tracer == nil {
		tracer = otelapi.GetTracerProvider().Tracer(instrumentationName)
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	kinds, periodic := reg.seal()
	m, err := newJobMetrics(cfg.MeterProvider, sortedKinds(kinds))
	if err != nil {
		return nil, err
	}
	rt := &Runtime{role: role, cfg: cfg, log: log, kinds: kinds, pool: pool, metrics: m, failed: make(chan error, 1)}
	rt.works = role.works() && len(kinds) > 0
	rc, err := rt.riverConfig(periodic, workerDeps{tracer: tracer, log: log, metrics: m})
	if err != nil {
		return nil, err
	}
	client, err := river.NewClient(riverpgxv5.New(pool.PGX()), rc)
	if err != nil {
		return nil, fmt.Errorf("jobs: %w", err)
	}
	rt.client = client
	return rt, nil
}

// riverConfig builds River's configuration: the workers of every kind for every role (an
// insert-only client checks inserted kinds against them), and for a role that works also the
// queues, the soft stop timeout and the periodic jobs.
func (rt *Runtime) riverConfig(periodic []PeriodicJob, d workerDeps) (*river.Config, error) {
	workers := river.NewWorkers()
	for _, k := range slices.Sorted(maps.Keys(rt.kinds)) {
		if err := rt.kinds[k].add(workers, d); err != nil {
			return nil, fmt.Errorf("jobs: kind %q: %w", k, err)
		}
	}
	rc := &river.Config{
		Logger:  rt.log,
		Schema:  rt.cfg.Schema,
		Workers: workers,
		// bothy_app owns no index of River, so River's nightly REINDEX could only fail; the
		// owner role keeps the indexes (README.md "River defaults").
		ReindexerSchedule: river.NeverSchedule(),
	}
	if rt.works {
		rc.Queues = make(map[string]river.QueueConfig, len(rt.cfg.MaxWorkers))
		for q, n := range rt.cfg.MaxWorkers {
			rc.Queues[q] = river.QueueConfig{MaxWorkers: n}
		}
		rc.SoftStopTimeout = rt.cfg.SoftStopTimeout
		for _, p := range periodic {
			rc.PeriodicJobs = append(rc.PeriodicJobs, periodicJob(p, rt.kinds[p.Args.Kind()]))
		}
	}
	return rc, nil
}

// periodicJob converts p to River's periodic job with the queue and attempts of its kind.
func periodicJob(p PeriodicJob, e *entry) *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(p.Interval),
		func() (river.JobArgs, *river.InsertOpts) { return p.Args, e.insertOpts(time.Time{}) },
		&river.PeriodicJobOpts{RunOnStart: p.RunOnStart})
}

// Inserter returns the inserter of the runtime; every role has one.
func (rt *Runtime) Inserter() Inserter { return inserter{rt: rt} }

// Start starts River for RoleWorker and RoleAll: the producers of the five queues, the leader
// election, periodic jobs and River's maintenance. It returns once River runs; River keeps
// running independent of ctx until Stop. For RoleAPI, and for a worker without registered kinds
// (nothing to run, and River refuses such a client), it does nothing. A second Start does
// nothing either.
//
// The error is River's (for example the database is unreachable); the runtime is then not
// running and Failed stays silent.
func (rt *Runtime) Start(ctx context.Context) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if !rt.works || rt.started || rt.stopping {
		return nil
	}
	// River ties its life to the context of Start; the lifecycle cancels the start context
	// when the start ends early, so River gets a context that only Stop ends.
	if err := rt.client.Start(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("jobs: start River: %w", err)
	}
	// Only a process that works jobs observes the queue (role worker, signals.md 4.4).
	reg, err := rt.metrics.registerQueue(rt.pool, rt.cfg.Schema, rt.cfg.StatementTimeout)
	if err != nil {
		return errors.Join(fmt.Errorf("jobs: register bothy.job.queue: %w", err), rt.client.StopAndCancel(ctx))
	}
	rt.queueReg = reg
	rt.started = true
	go rt.watch(rt.client.Stopped())
	return nil
}

// watch reports a River client that stopped without Stop on Failed and closes Failed in every
// case.
func (rt *Runtime) watch(stopped <-chan struct{}) {
	<-stopped
	rt.mu.Lock()
	expected := rt.stopping
	rt.mu.Unlock()
	if !expected {
		rt.failed <- errors.New("jobs: the River client stopped unexpectedly")
	}
	close(rt.failed)
}

// Failed returns a channel that delivers one error and is closed when the River client of a
// started runtime stops without Stop; after Stop it is closed without a value. The process ends
// then (02 "Panics": without River a worker is useless). For a runtime that never started it
// never delivers.
func (rt *Runtime) Failed() <-chan error { return rt.failed }

// Stop stops River for RoleWorker and RoleAll in the order of 02 "Shutdown": a soft stop that
// lets running jobs finish within cfg.SoftStopTimeout, then the contexts of the jobs still
// running are cancelled (River escalates by itself when the soft timeout ends); cancelled jobs
// return an error and run again on the next start. Stop waits until River has stopped or ctx
// ends. When ctx ends first, Stop cancels the running jobs at once and returns the context
// error; a later Stop waits again, so internal/wire gives what the fixed shutdown budgets leave
// of shutdown.total_timeout to a second call. For a runtime that never started Stop does
// nothing.
func (rt *Runtime) Stop(ctx context.Context) error {
	rt.mu.Lock()
	rt.stopping = true
	started, reg := rt.started, rt.queueReg
	rt.queueReg = nil
	rt.mu.Unlock()
	if !started {
		return nil
	}
	if reg != nil {
		// The queue is not observed after the stop; the error only says it was unregistered.
		_ = reg.Unregister()
	}
	if err := rt.client.Stop(ctx); err == nil {
		return nil
	}
	// The budget ended before River did: cancel the jobs now; StopAndCancel waits with the
	// ended ctx, so it returns at once and River finishes in the background.
	return rt.client.StopAndCancel(ctx)
}
