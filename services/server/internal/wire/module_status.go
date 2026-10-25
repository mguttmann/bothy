// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/files"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/search"
	statspg "github.com/OWNER/bothy/services/server/internal/stats/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/checks"
	statuspg "github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	statusapp "github.com/OWNER/bothy/services/server/internal/status/app"
)

// statusState is the part of the server state the status module fills.
type statusState struct {
	// status is the status module (M7-09), built in the step status after files and search,
	// whose ports its checks read.
	status *Status
	// statusCfg is its configuration; the jobs step reads the period of status.probe and
	// retention.ops from it.
	statusCfg StatusConfig
}

// moduleStatus is the name of the module in architecture/modules.yaml and of its start step.
const moduleStatus = "status"

// statusWiring wires the status module (M7-09): its start step after apps, the REST handlers of
// the status page, the job status.probe and the participant of retention.ops.
func statusWiring() moduleWiring {
	return moduleWiring{
		module:   moduleStatus,
		parts:    []part{{order: 1650, step: moduleStatus, start: (*server).startStatus}},
		handlers: func(s *server, h *Handlers) { s.status.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			st, cfg, pool := s.status, s.statusCfg, s.pool
			s.mu.Unlock()
			if st == nil {
				return errors.New("wire: the jobs step needs the status step")
			}
			m.StatusProbe, m.StatusProbeInterval = st.Probe, cfg.CheckInterval
			return m.registerRetention(statuspg.NewRetention(pool), cfg.RetentionOps)
		},
	}
}

// startStatus builds the status module over the pool, the unit of work and recorder of audit, the
// Authorizer, the appender of sync and the checks over the ports of stats, files, search and
// agents, and binds the in-transaction subscriber of agents.AgentDisabled to the appender.
// Deps.StatusSources, when set, replaces the checks (tests simulate outages with it).
func (s *server) startStatus(context.Context) error {
	cfg, err := StatusConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, f, sr := s.pool, s.audit, s.authorizer, s.sync, s.files, s.searchModule
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || f == nil || sr == nil {
		return errors.New("wire: status needs audit, the Authorizer, sync, files and search")
	}
	src, err := s.statusSources(cfg, f, sr.Coordinator)
	if err != nil {
		return err
	}
	if s.deps.StatusSources != nil {
		src = s.deps.StatusSources(src)
	}
	st, err := NewStatus(cfg, pool, StatusDeps{UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Events: sy.Appender(),
		Sources: src, Clock: s.deps.Clock, Log: s.logger()})
	if err != nil {
		return err
	}
	a.StatusSubscribers().Bind(sy.Appender())
	s.mu.Lock()
	s.status, s.statusCfg = st, cfg
	s.mu.Unlock()
	return nil
}

// statusSources returns the checks of the process: the own measurement of every process in
// stats_minutes, the probe object of the object store, the PING of clamd by this worker (none in
// the role api), the test query of search and the stored presence of the agents.
func (s *server) statusSources(cfg StatusConfig, f *Files, sr search.Probe) (statusapp.Sources, error) {
	presence, err := PresenceConfigFrom(s.values)
	if err != nil {
		return statusapp.Sources{}, err
	}
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	agentsSrc, err := agentsapp.NewStatusSource(agentspg.NewRepository(pool), identityapp.NewDirectory(identitypg.NewRepository(pool)),
		presence, s.deps.Clock)
	if err != nil {
		return statusapp.Sources{}, err
	}
	storage, err := filesapp.NewStorageCheck(f.store, ids.New())
	if err != nil {
		return statusapp.Sources{}, err
	}
	d := checks.Deps{SLI: statspg.NewRepository(pool), Storage: storage, Search: sr, Agents: agentsSrc}
	if f.Health != nil {
		var h files.ScannerHealth = f.Health
		d.Scanner = h
	}
	return checks.Sources(cfg.checksConfig(), d), nil
}
