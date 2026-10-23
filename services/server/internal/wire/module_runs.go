// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// runsState is the part of the server state the runs module fills.
type runsState struct {
	// runs is the runs module (M2-05), built in the chat step over messaging, chat and agents; the
	// chat step binds the acceptance of the unit of work to it.
	runs *Runs
}

// moduleRuns is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleRuns = "runs"

// runsWiring wires the runs module into the start step chat, after messaging, binds its
// acting-for source into identity at the start of step sessions (SR9, M2-14), and wires its jobs
// runs.queued_expiry (M2-08), runs.cancel_timeout and runs.stall_check (M2-09).
func runsWiring() moduleWiring {
	return moduleWiring{
		module: moduleRuns,
		parts: []part{
			{order: 1320, step: moduleChat, start: (*server).startRuns},
			{order: 1890, step: "sessions", start: (*server).bindActingFor},
		},
		handlers: func(s *server, h *Handlers) { s.runs.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			rn := s.runs
			s.mu.Unlock()
			if rn != nil {
				m.QueuedExpiry, m.RunsLifecycle = rn.Service, rn.Service
			}
			// runs.stall_check runs every agent.presence_timeout (M2-09, D-M2-09-19).
			m.StallInterval = s.values.Duration(keyPresenceTimeout)
			return nil
		},
		bindJobs: func(s *server, ins jobs.Inserter) {
			s.mu.Lock()
			rn := s.runs
			s.mu.Unlock()
			if rn != nil {
				rn.BindJobs(ins)
			}
		},
	}
}

// startRuns builds the runs module (M2-05) in the chat step over messaging (anchor and trigger
// state), the threads and channel facts of chat and the agent facts of the agents step, binds the
// acceptance of the unit of work to it and, in the worker role, registers the gauge
// bothy.run.active on the global meter provider.
func (s *server) startRuns(context.Context) error {
	cfg, err := RunsConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, c, m, ag := s.pool, s.audit, s.authorizer, s.sync, s.chat, s.messaging, s.agents
	s.mu.Unlock()
	if ag == nil {
		return errors.New("wire: the runs module needs the agents step")
	}
	if a == nil || sy == nil || c == nil || m == nil {
		return errors.New("wire: the runs module needs audit, sync, chat and messaging")
	}
	rn, err := NewRuns(cfg, pool, a.UoW(), az, RunsDeps{
		Events: sy.Appender(), Messages: m.Service, Threads: c.Service, Channels: c.Service, Agents: ag.Service,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Audit: a.Recorder(),
	}, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	if err := s.bindCommands(pool, a, az, sy, c, ag, rn); err != nil {
		return err
	}
	if err := a.RunsAcceptor().Bind(rn.Service); err != nil {
		return err
	}
	if s.worker() {
		if err := rn.RegisterActiveGauge(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.runs = rn
	s.mu.Unlock()
	return nil
}

// bindCommands builds the command register of agents over chat and binds it to the command
// execution of runs (M2-16).
func (s *server) bindCommands(pool *pg.Pool, a *Audit, az authorize.Authorizer, sy *Sync, c *Chat, ag *Agents, rn *Runs) error {
	cfg, err := CommandsConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id := s.identity
	s.mu.Unlock()
	if id == nil || id.Principals == nil {
		return errors.New("wire: the command register needs the identity module")
	}
	if err := ag.BindCommands(cfg, pool, CommandDeps{UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Events: sy.Appender(),
		Principals: id.Principals, Channels: c.Service}); err != nil {
		return err
	}
	return rn.Service.BindCommands(ag.Commands)
}

// bindActingFor binds the acting-for source of runs (07 guard rule SR9, M2-14) into identity at
// the start of step sessions, before startSessions builds the Authenticator: the Invocation-Token
// header of an agent's request resolves through runs. The signing keys step bound the invocation
// tokens before this step. Without runs the header stays invalid (identity.NoActingForSource).
func (s *server) bindActingFor(context.Context) error {
	s.mu.Lock()
	id, rn, pool := s.identity, s.runs, s.pool
	s.mu.Unlock()
	if id == nil {
		return errors.New("wire: the acting-for binding needs the identity step")
	}
	if rn == nil {
		return nil
	}
	src, err := rn.ActingForSource(pool)
	if err != nil {
		return err
	}
	id.BindActingFor(src)
	return nil
}
