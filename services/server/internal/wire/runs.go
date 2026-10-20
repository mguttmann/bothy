// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"crypto/rand"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/agents"
	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	runshttp "github.com/OWNER/bothy/services/server/internal/runs/adapters/httpapi"
	runsmetrics "github.com/OWNER/bothy/services/server/internal/runs/adapters/otelmetrics"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
)

// RunsConfig binds the keys of the runs use cases (api.page_size_default, api.page_size_max,
// run.step_detail_max_bytes). The error names the key whose binding failed, never a value.
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
	svc, err := runsapp.New(cfg, runsapp.Deps{
		Repo: runspg.NewRepository(pool), UoW: uow, Authorizer: az, Events: d.Events, Messages: d.Messages, Threads: d.Threads,
		Channels: d.Channels, Agents: d.Agents, Cursors: runshttp.Cursors{}, Errors: counter, Log: log, Clock: clk, Metrics: d.Metrics,
	})
	if err != nil {
		return nil, err
	}
	return &Runs{Service: svc, Handlers: runshttp.New(svc)}, nil
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
	r.Invocations = it
	return nil
}
