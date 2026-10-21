// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"crypto/rand"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/agents"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	runsactingfor "github.com/OWNER/bothy/services/server/internal/runs/adapters/actingfor"
	runshttp "github.com/OWNER/bothy/services/server/internal/runs/adapters/httpapi"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	runsmetrics "github.com/OWNER/bothy/services/server/internal/runs/adapters/otelmetrics"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
)

// RunsConfig binds the keys of the runs use cases (api.page_size_default, api.page_size_max,
// run.step_detail_max_bytes, run.queued_max_age, and for M2-09 auth.clock_leeway,
// agent.cancel_grace, agent.presence_timeout, run.orphan_timeout, audit.reason_max_chars, and for
// M2-10 run.excerpt_max_chars, run.max_steps_persisted, sync.event_payload_max_bytes). The
// error names the key whose binding failed, never a value.
func RunsConfig(v *config.Values) (runsapp.Config, error) {
	var c runsapp.Config
	if err := config.Bind(v, &c); err != nil {
		return runsapp.Config{}, err
	}
	return c, nil
}

// Runs is the runs module of the process (M2-05): the acceptance of a triggering message, the
// lifecycle Open and Transition, the reads of runs.Runs with their REST handlers, and the metrics
// of signals.md 4.3 on the global meter provider of platform/otel.
type Runs struct {
	// Service implements runs.Runs, Open, Transition and the source of bothy.run.active.
	Service *runsapp.Service
	// Handlers are the REST handlers.
	Handlers *runshttp.Runs
	// Invocations issues and verifies the invocation tokens (M2-06); nil until
	// BindInvocationTokens, which the signing keys step calls.
	Invocations *runsapp.InvocationTokens
	// Expiry schedules runs.queued_expiry (M2-08); the jobs step binds its inserter (BindJobs).
	Expiry *runsjobs.Scheduler
	// CancelTimeouts schedules runs.cancel_timeout (M2-09); the jobs step binds its inserter
	// (BindJobs).
	CancelTimeouts *runsjobs.CancelScheduler
	// Sessions are the agent sessions the runs use at the invocation and the start (M2-12);
	// buildRuns sets them, and fillHandlers mounts their REST handlers. nil in a world without
	// them.
	Sessions *AgentSessions
}

// RunsDeps are the collaborators of the runs module from the other steps of the start: the
// messaging use cases (anchor and trigger state), the thread and channel ports of chat and the
// agent facts of agents.
type RunsDeps struct {
	Events   eventlog.Appender
	Messages runsapp.MessageWriter
	Threads  runsapp.ThreadPort
	Channels chat.TriggerFacts
	Agents   agents.Facts
	// Metrics records committed transitions; nil means runs/adapters/otelmetrics on the global
	// meter provider of platform/otel.
	Metrics runsapp.Metrics
	// Versions reads the permission version a delivery check pins; Audit writes the audit entry
	// of a denial at delivery (M2-08). Without them the delivery refuses with an internal error.
	Versions runsapp.PermissionVersions
	Audit    runsapp.AuditRecorder
	// Expiry schedules runs.queued_expiry; nil means the scheduler over the job runtime of the
	// process (Runs.BindJobs). A test world without jobs passes one that schedules nothing.
	Expiry runsapp.ExpiryScheduler
	// CancelTimeouts schedules runs.cancel_timeout (M2-09); nil means the scheduler over the job
	// runtime of the process (Runs.BindJobs).
	CancelTimeouts runsapp.CancelTimeoutScheduler
	// Heartbeats reads the persisted heartbeat for runs.stall_check (M2-09); nil means the reader
	// of the agents module over pool.
	Heartbeats runsapp.HeartbeatReader
	// Sessions decides the session of every operator invocation and attaches it at the start
	// (agents, M2-12); nil gives every operator run session_mode new without a row (a test world
	// without the sessions). The process always sets it (buildRuns).
	Sessions agents.Sessions
}

// NewRuns builds the runs module over pool with the unit of work uow, the Authorizer az and the
// collaborators d, the clock (nil means clock.System()) and log (nil means slog.Default()), which
// with bothy.authz.errors records a listed run that fails run.read (filter_mismatch). It opens no
// connection. Errors: a missing collaborator, a limit the use cases refuse, or an instrument.
func NewRuns(cfg runsapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, d RunsDeps, clk clock.Clock,
	log *slog.Logger) (*Runs, error) {
	if az == nil || d.Events == nil || d.Messages == nil || d.Threads == nil || d.Channels == nil || d.Agents == nil {
		return nil, errors.New("wire: the runs module needs the Authorizer, the appender, messaging, chat and agents")
	}
	counter, err := authztelemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	if d.Metrics == nil {
		m, err := runsmetrics.New(nil)
		if err != nil {
			return nil, err
		}
		d.Metrics = m
	}
	telemetry, err := runsmetrics.NewDelivery(nil, nil)
	if err != nil {
		return nil, err
	}
	var sched *runsjobs.Scheduler
	if d.Expiry == nil {
		sched = &runsjobs.Scheduler{}
		d.Expiry = sched
	}
	var cancels *runsjobs.CancelScheduler
	if d.CancelTimeouts == nil {
		cancels = &runsjobs.CancelScheduler{}
		d.CancelTimeouts = cancels
	}
	if d.Heartbeats == nil {
		hb, err := agentsapp.NewHeartbeatReader(agentspg.NewRepository(pool))
		if err != nil {
			return nil, err
		}
		d.Heartbeats = hb
	}
	mismatches, err := runsmetrics.NewMismatches(nil)
	if err != nil {
		return nil, err
	}
	svc, err := runsapp.New(cfg, runsapp.Deps{
		Repo: runspg.NewRepository(pool), UoW: uow, Authorizer: az, Events: d.Events, Messages: d.Messages, Threads: d.Threads,
		Channels: d.Channels, Agents: d.Agents, Cursors: runshttp.Cursors{}, Errors: counter, Log: log, Clock: clk, Metrics: d.Metrics,
		Versions: d.Versions, Audit: d.Audit, Telemetry: telemetry, Expiry: d.Expiry, CancelTimeouts: d.CancelTimeouts,
		Heartbeats: d.Heartbeats, Mismatches: mismatches, Followups: runsapp.NoFollowups{}, Steps: stepMetrics(d.Metrics),
		Sessions: d.Sessions,
	})
	if err != nil {
		return nil, err
	}
	return &Runs{Service: svc, Handlers: runshttp.New(svc), Expiry: sched, CancelTimeouts: cancels}, nil
}

// stepMetrics returns m as the counter of bothy.run.steps when it is one (runs/adapters/otelmetrics
// is), nil otherwise (a test world that brings its own transition metrics counts no steps).
func stepMetrics(m runsapp.Metrics) runsapp.StepMetrics {
	if sm, ok := m.(runsapp.StepMetrics); ok {
		return sm
	}
	return nil
}

// BindJobs gives the schedulers of runs.queued_expiry and runs.cancel_timeout the inserter of the
// job runtime; the jobs step calls it once the runtime exists. A Runs built with its own
// scheduler ignores it for that scheduler.
func (r *Runs) BindJobs(ins jobs.Inserter) {
	if r == nil {
		return
	}
	if r.Expiry != nil {
		r.Expiry.Bind(ins)
	}
	if r.CancelTimeouts != nil {
		r.CancelTimeouts.Bind(ins)
	}
}

// RegisterActiveGauge registers the observable gauge bothy.run.active over the runs of r on the
// global meter provider of platform/otel for the life of the process; the worker role calls it
// (signals.md 4.3, D-M2-05-15). Errors: those of the meter.
func (r *Runs) RegisterActiveGauge() error {
	_, err := runsmetrics.RegisterActiveGauge(nil, r.Service)
	return err
}

// fillHandlers sets the nil runs handler of h to the one r built; a nil r leaves h unchanged.
func (r *Runs) fillHandlers(h *Handlers) {
	if r != nil && h.Runs == nil {
		h.Runs = r.Handlers
	}
	if r != nil && r.Sessions != nil && h.AgentSessions == nil {
		h.AgentSessions = r.Sessions.Handlers
	}
}

// InvocationTokenConfigFrom binds the keys of the invocation tokens (agent.invocation_token_ttl,
// auth.clock_leeway; M2-06). The error names the key whose binding failed, never a value.
func InvocationTokenConfigFrom(v *config.Values) (runsapp.InvocationTokenConfig, error) {
	var c runsapp.InvocationTokenConfig
	if err := config.Bind(v, &c); err != nil {
		return runsapp.InvocationTokenConfig{}, err
	}
	return c, nil
}

// Compile-time check (brief M2-14 design note 3h): the acting-for source of runs implements the
// source port identity declares; it has only kernel types in its signature, so runs does not
// import identity and no module edge is added.
var _ identity.ActingForSource = (*runsactingfor.Source)(nil)

// ActingForSource returns the source of the acting-for binding of guard rule SR9 (brief M2-14):
// the resolution of the Invocation-Token header over the invocation tokens of BindInvocationTokens
// and the binding read of the runs table on pool, outside any unit of work. Errors: the invocation
// tokens are not bound yet.
func (r *Runs) ActingForSource(pool *pg.Pool) (*runsactingfor.Source, error) {
	if r.Invocations == nil {
		return nil, errors.New("wire: the acting-for source needs the invocation tokens (BindInvocationTokens)")
	}
	uc, err := runsapp.NewActingFor(r.Invocations, runspg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	return runsactingfor.NewSource(uc), nil
}

// BindInvocationTokens builds the invocation tokens of the runs module (M2-06) once per process
// over the signing keys of instance (purpose invocation), the instance row (the issuer), the
// process clock and a UUIDv7 generator over it. The start calls it in the signing keys step, which
// runs after the chat step that built r. Errors: a second bind, a configuration the service
// refuses.
func (r *Runs) BindInvocationTokens(cfg runsapp.InvocationTokenConfig, keys runsapp.InvocationKeys, state runsapp.InstanceState,
	clk clock.Clock) error {
	if r.Invocations != nil {
		return errors.New("wire: the invocation tokens are bound already")
	}
	if clk == nil {
		clk = clock.System()
	}
	it, err := runsapp.NewInvocationTokens(cfg, runsapp.InvocationTokenDeps{Keys: keys, State: state, Clock: clk,
		IDs: ids.NewGenerator(clk, rand.Reader)})
	if err != nil {
		return err
	}
	if r.Service != nil {
		// The delivery check signs the token of every allowed delivery (M2-08).
		if err := r.Service.BindTokens(it); err != nil {
			return err
		}
	}
	r.Invocations = it
	return nil
}
