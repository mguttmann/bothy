// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/wsrelay"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// terminalState is the part of the server state the terminal module fills.
type terminalState struct {
	// terminal is the terminal module (M2-39): the host records and their REST handlers.
	terminal *Terminal
	// terminalStreams are the relay and the stream routes of api and all (M2-44); the log tail
	// feeds the relay.
	terminalStreams *TerminalStreams
	// plan is what cluster.mode means for the role (M2-47); terminalLocal are the local signals of
	// one node, whose handler the hub becomes; terminalHub is the handler of the signals of an api
	// process with its configuration hubCfg.
	plan          ClusterPlan
	terminalLocal *terminalapp.LocalSignals
	terminalHub   *wsrelay.Hub
	hubCfg        ClusterHubConfig
}

// moduleTerminal is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleTerminal = "terminal"

// terminalWiring wires the terminal module: its start step (6g), the route bindings of the
// control connection and the streams, and their mount after the gateway (api and all).
func terminalWiring() moduleWiring {
	return moduleWiring{
		module: moduleTerminal,
		parts: []part{
			{order: 1500, step: moduleTerminal, start: (*server).startTerminal},
		},
		handlers: func(s *server, h *Handlers) { s.terminal.fillHandlers(h) },
		routeBindings: func() []authorize.RouteBinding {
			return append([]authorize.RouteBinding{HostConnectRouteBinding()}, TerminalStreamRouteBindings()...)
		},
		mounts: []moduleMount{{order: 200, mount: func(s *server, ctx context.Context, mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter,
			log *slog.Logger) error {
			return s.mountHostControl(ctx, mux, chain, limiter, log)
		}}},
		// M2-46: the job terminal.sweep over the use cases of the terminal step.
		jobs: func(s *server, m *JobModules) error {
			sweepCfg, err := TerminalSweepConfigFrom(s.values)
			if err != nil {
				return err
			}
			s.mu.Lock()
			tm := s.terminal
			s.mu.Unlock()
			if tm == nil {
				return errors.New("wire: the jobs step needs the terminal step")
			}
			m.TerminalSweep, m.TerminalSweepInterval = tm.Service, sweepCfg.Interval
			return m.registerRetention(terminalpg.NewRetention(s.currentPool()), m.RetentionConfig.SecurityAfterEnd)
		},
	}
}

// startTerminal is start step 6g (M2-39): the terminal module over the pool, the unit of work and
// the recorder of the audit step, the appender of the sync step, the Authorizer and the identity
// module.
func (s *server) startTerminal(context.Context) error {
	cfg, err := TerminalConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id := s.pool, s.audit, s.authorizer, s.sync, s.identity
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || id == nil {
		return errors.New("wire: the terminal step needs audit, sync, the Authorizer and identity")
	}
	tm, err := NewTerminal(cfg, pool, a.UoW(), az, IdentityWrites{Audit: a.Recorder(), Events: sy.Appender()}, id, s.deps.Clock,
		s.deps.TerminalBeforeOpenCommit, s.logger())
	if err != nil {
		return err
	}
	sig, local := terminalSignals(s.plan, pool, s.deps.TerminalSignals)
	tm.Service.BindSignals(sig)
	if err := tm.bindSessionMetrics(s.deps.TerminalSessionMetrics); err != nil {
		return err
	}
	// The subscribers of the txn registry end sessions from now on (M2-46).
	a.TerminalSubscribers().Bind(tm.Service)
	if s.deps.TerminalModule != nil {
		s.deps.TerminalModule(tm.Service)
	}
	s.mu.Lock()
	s.terminal, s.terminalLocal = tm, local
	s.mu.Unlock()
	return nil
}

// mountHostControl builds the control connection of the host services (M2-43) over the terminal
// module, the ephemeral publisher of the gateway and the signing keys, mounts GET
// /api/v1/hosts/connect on mux and registers its stop step.
func (s *server) mountHostControl(ctx context.Context, mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter, log *slog.Logger) error {
	cfg, err := HostControlConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	tm, gw, keys := s.terminal, s.gateway, s.signingKeys
	s.mu.Unlock()
	if tm == nil || gw == nil || keys == nil {
		return errors.New("wire: the control connection needs the terminal module, the gateway and the signing keys")
	}
	hc, err := NewHostControl(cfg, tm, gw.Publisher, keys.Keys, s.deps.Clock, log, terminalLimiter{l: limiter})
	if err != nil {
		return err
	}
	hc.Mount(mux, chain)
	s.runner.OnStop(hc.StopConnections())
	return s.mountTerminalStreams(ctx, mux, chain, tm, hc, limiter, log)
}

// mountTerminalStreams builds the relay and the two stream routes of the terminal sessions
// (M2-44) over the module, the control connection, identity's authenticator and nonces and the
// signing keys, builds the hub of the signals across nodes (M2-47), reconciles the sessions of the
// previous process (Design note 14, only with cluster.mode single, and only while the start lock
// of R-DM-50 is free), mounts both routes on mux and registers the stop step of the relay.
func (s *server) mountTerminalStreams(ctx context.Context, mux Mux, chain *httpserver.Chain, tm *Terminal, hc *HostControl,
	limiter *ratelimit.Limiter, log *slog.Logger) error {
	cfg, err := TerminalStreamsConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id, keys := s.identity, s.signingKeys
	s.mu.Unlock()
	if id == nil || id.Authenticator == nil || id.DPoP == nil || keys == nil {
		return errors.New("wire: the terminal streams need identity's authenticator and nonces and the signing keys")
	}
	ts, err := NewTerminalStreams(cfg, tm, hc, TerminalStreamsDeps{Authenticator: id.Authenticator, Nonces: id.DPoP, Signer: keys.Keys,
		Timers: s.deps.TerminalTimers, Metrics: s.deps.TerminalMetrics, Clock: s.deps.Clock, Log: log, Multi: s.plan.Multi,
		Limiter: terminalLimiter{l: limiter}})
	if err != nil {
		return err
	}
	s.mu.Lock()
	local := s.terminalLocal
	s.mu.Unlock()
	hub, hubCfg, err := newTerminalHub(s.values, s.plan, tm, hc, ts, local, s.deps.Clock, log)
	if err != nil {
		return err
	}
	if !s.plan.Multi && s.finalized() && !s.maint.ReadOnly {
		// One node: the slots and connections of the previous process are gone (M2-44 Design
		// note 14); with several nodes the other nodes still hold theirs. The reconcile ends and
		// detaches sessions with audit entries and events, so while the start lock holds it waits
		// for the next start after bothy restore finalize, and in maintenance mode, which writes
		// nothing, for the next start without it (M7-08).
		if err := tm.Service.Reconcile(ctx); err != nil {
			return err
		}
	}
	ts.Mount(mux, chain)
	s.runner.OnStop(ts.StopConnections())
	s.mu.Lock()
	s.terminalStreams, s.terminalHub, s.hubCfg = ts, hub, hubCfg
	s.mu.Unlock()
	return nil
}
