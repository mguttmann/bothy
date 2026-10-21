// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	agentshttp "github.com/OWNER/bothy/services/server/internal/agents/adapters/httpapi"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
)

// SessionsConfig binds the key of the agent sessions (agent.session_ref_max_bytes, M2-12). The
// error names the key whose binding failed, never a value.
func SessionsConfig(v *config.Values) (agentsapp.SessionConfig, error) {
	var c agentsapp.SessionConfig
	if err := config.Bind(v, &c); err != nil {
		return agentsapp.SessionConfig{}, err
	}
	return c, nil
}

// SessionWrites are the writers of the session use cases: the audit recorder of the allow entry
// of agent_session.bind and the appender of agent_session.changed.
type SessionWrites struct {
	Audit  auditing.Recorder
	Events eventlog.Appender
}

// AgentSessions is the agent session part of the agents module (M2-12): the use cases of
// agents.Sessions, which runs calls at the invocation and the start of a run, and the REST
// handlers of listAgentSessions, resetAgentSession, bindAgentSession and reportAgentSession.
type AgentSessions struct {
	// Service implements agents.Sessions.
	Service *agentsapp.SessionService
	// Handlers are the REST handlers.
	Handlers *agentshttp.AgentSessions
}

// NewAgentSessions builds the agent sessions over pool with the unit of work uow, the Authorizer
// az, the writers w, the thread routing data and the channel facts of chat (threads, channels),
// the clock (nil means clock.System()) and log. The declared source agents.RunSource is the
// implementation of runs over pool (02 section 4.4; M2-12 design note 2), so agents never uses
// runs. It opens no connection. Errors: a missing collaborator or a limit the use cases refuse.
//
// Example:
//
//	ss, err := wire.NewAgentSessions(cfg, pool, uow, az, wire.SessionWrites{Audit: rec, Events: appender}, c.Service, c.Service, clk, log)
func NewAgentSessions(cfg agentsapp.SessionConfig, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, w SessionWrites,
	threads agentsapp.ThreadInfos, channels chat.TriggerFacts, clk clock.Clock, log *slog.Logger) (*AgentSessions, error) {
	if pool == nil {
		return nil, errors.New("wire: the agent sessions need the database pool")
	}
	svc, err := agentsapp.NewSessions(cfg, agentsapp.SessionDeps{
		Store: agentspg.NewSessionStore(pool), Threads: threads, Channels: channels, Runs: runspg.NewRunSource(pool), UoW: uow,
		Authorizer: az, Audit: w.Audit, Events: w.Events, Clock: clk, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &AgentSessions{Service: svc, Handlers: agentshttp.NewAgentSessions(svc)}, nil
}
