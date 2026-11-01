// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// agentsState is the part of the server state the agents module fills.
type agentsState struct {
	// agents is the agents module (M1-51): the agent records and their REST handlers.
	agents *Agents
}

// moduleAgents is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleAgents = "agents"

// agentsWiring wires the agents module: its start step (6e), the readiness binding in the step
// chat and the presence tracker of api and all (M2-20).
func agentsWiring() moduleWiring {
	return moduleWiring{
		module: moduleAgents,
		parts: []part{
			{order: 1200, step: moduleAgents, start: (*server).startAgents},
			{order: 1340, step: moduleChat, start: (*server).bindAgentReadiness},
			{order: 2200, step: "agent_presence", when: (*server).api, start: whileFinalized((*server).startPresence)},
		},
		handlers: func(s *server, h *Handlers) {
			if h.Agents == nil && s.agents != nil {
				h.Agents = s.agents.Handlers
			}
			if h.Commands == nil && s.agents != nil {
				h.Commands = s.agents.CommandHandlers
			}
		},
	}
}

// bindAgentReadiness binds chat into the presence of the agents in the chat step: the heartbeat
// applies project readiness through chat (M2-03, D-M2-20-10).
func (s *server) bindAgentReadiness(context.Context) error {
	s.mu.Lock()
	ag, c := s.agents, s.chat
	s.mu.Unlock()
	if ag != nil && ag.Presence != nil {
		ag.Presence.BindReadiness(readinessInMaintenance{chat: c.Service})
	}
	return nil
}

// readinessInMaintenance passes the project readiness of a heartbeat to chat with
// txn.WithWriteInMaintenance: like the stored presence it is runtime state the agent reports and
// keeps writing in maintenance mode (M7-08, D-M7-08-9); outside the mode the mark changes nothing.
type readinessInMaintenance struct{ chat agentsapp.Readiness }

// ApplyProjectReadiness implements agentsapp.Readiness.
func (r readinessInMaintenance) ApplyProjectReadiness(ctx context.Context, agentID ids.ID, reports []chat.ProjectReadiness) error {
	return r.chat.ApplyProjectReadiness(txn.WithWriteInMaintenance(ctx), agentID, reports)
}

// ApplyProjectTrashResults implements agentsapp.Readiness: the outcomes of folder moves are
// runtime state of the runner too (D-DEL-11).
func (r readinessInMaintenance) ApplyProjectTrashResults(ctx context.Context, agentID ids.ID, results []chat.ProjectTrashResult) error {
	return r.chat.ApplyProjectTrashResults(txn.WithWriteInMaintenance(ctx), agentID, results)
}

// ProjectTrashRequests implements agentsapp.Readiness; a read.
func (r readinessInMaintenance) ProjectTrashRequests(ctx context.Context, agentID ids.ID) ([]chat.ProjectTrashRequest, error) {
	return r.chat.ProjectTrashRequests(ctx, agentID)
}

// BoundProjectFolders implements agentsapp.Readiness; a read, so it needs no mark (M4-44).
func (r readinessInMaintenance) BoundProjectFolders(ctx context.Context, names []string) ([]string, error) {
	return r.chat.BoundProjectFolders(ctx, names)
}

// startAgents is start step 6e (M1-51): the agents module over the pool, the unit of work, the
// recorder and the rights hook of the audit step, the appender of the sync step, the Authorizer
// and the identity module; it binds the switch of the module into the guided flow of authz
// (M2-21, bindGuidedFlow).
func (s *server) startAgents(context.Context) error {
	cfg, err := AgentsConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id := s.pool, s.audit, s.authorizer, s.sync, s.identity
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || id == nil {
		return errors.New("wire: the agents step needs audit, sync, the Authorizer and identity")
	}
	ag, err := NewAgents(cfg, pool, a.UoW(), az, a.RightsHook(), IdentityWrites{Audit: a.Recorder(), Events: sy.Appender(),
		Measure: s.measureRecorder()}, id, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	admin := s.authzAdmin
	s.mu.Unlock()
	if err := bindGuidedFlow(admin, ag); err != nil {
		return err
	}
	pcfg, err := PresenceConfigFrom(s.values)
	if err != nil {
		return err
	}
	if err := ag.BindPresence(pcfg, pool, PresenceDeps{UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Clock: s.deps.Clock,
		Log: s.logger()}); err != nil {
		return err
	}
	if s.api() {
		// Registered before the gateway's stop steps (mountGateway), so it runs after them: no
		// heartbeat arrives once it flushes (D-M2-20-5).
		s.runner.OnStop(ag.PresenceStopStep())
	}
	s.mu.Lock()
	s.agents = ag
	s.mu.Unlock()
	return nil
}

// bindGuidedFlow binds the switch of the agents module ag into the resolver of the guided flow of
// admin (M2-21): from then on resolve_i1 switches agents through the agents module. It fails the
// start when the resolver still holds authz.EmptyAgentPolicySwitch afterwards, so the production
// graph never serves the guided flow with the empty switch.
func bindGuidedFlow(admin *AuthzAdmin, ag *Agents) error {
	if admin == nil || admin.Resolver == nil || ag == nil || ag.Switch == nil {
		return errors.New("wire: the guided flow needs the resolver of authz and the switch of the agents module")
	}
	admin.BindAgentPolicySwitch(ag.Switch)
	if admin.Resolver.UsesEmptySwitch() {
		return errors.New("wire: the guided flow still uses the empty agent policy switch")
	}
	return nil
}

// startPresence starts the presence tracker of the agents in the roles api and all (M2-20): the
// start rule, the flush loop and the presence gauges.
func (s *server) startPresence(ctx context.Context) error {
	s.mu.Lock()
	ag := s.agents
	s.mu.Unlock()
	return ag.StartPresence(ctx, s.deps.PresenceGauges)
}
