// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/authzsource"
	agentshttp "github.com/OWNER/bothy/services/server/internal/agents/adapters/httpapi"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// AgentsConfig binds the keys of the agents use cases (api.page_size_default, api.page_size_max).
// The error names the key whose binding failed, never a value.
func AgentsConfig(v *config.Values) (agentsapp.Config, error) {
	var c agentsapp.Config
	if err := config.Bind(v, &c); err != nil {
		return agentsapp.Config{}, err
	}
	return c, nil
}

// NewAgentPolicySource returns the authz.AgentPolicySource of the process over pool: the agents
// records with the principal directory of identity (M1-51). It replaces the empty source of
// M1-28 for the Authorizer and the rights hooks.
func NewAgentPolicySource(pool *pg.Pool) authz.AgentPolicySource {
	return authzsource.New(agentspg.NewRepository(pool), identityapp.NewDirectory(identitypg.NewRepository(pool)))
}

// newAgentDirectory returns the authz.AgentDirectory of the process over pool, the same source as
// NewAgentPolicySource: the list "Permission hints" of authz reads every agent through it (M6-07).
func newAgentDirectory(pool *pg.Pool) authz.AgentDirectory {
	return authzsource.New(agentspg.NewRepository(pool), identityapp.NewDirectory(identitypg.NewRepository(pool)))
}

// Agents is the agents module of the process (M1-51): the use cases of agents.Agents and the REST
// handlers of the ten agent operations, and the consumer cursors with the three consumer operations
// (M2-07).
type Agents struct {
	// Service implements agents.Agents.
	Service *agentsapp.Service
	// Consumers implements agents.Consumers and agents.ConsumerAPI; the gateway binds its engine as
	// the feed (mountGateway).
	Consumers *agentsapp.ConsumerService
	// Handlers are the REST handlers.
	Handlers *agentshttp.Agents
	// Presence is the presence tracker of the process (M2-20, BindPresence); nil until bound.
	Presence *agentsapp.Tracker
	// Switch implements authz.AgentPolicySwitch over Service (M2-21); the start steps bind it into
	// the resolver of authz (AuthzAdmin.BindAgentPolicySwitch).
	Switch *authzsource.Switch
	// Commands is the command register (M2-16) and CommandHandlers its REST handlers; nil until
	// BindCommands, which the chat step calls once chat exists.
	Commands        *agentsapp.CommandService
	CommandHandlers *agentshttp.Commands
}

// CommandsConfig binds the keys of the command register (command.max_per_principal,
// command.description_max_chars, command.autocomplete_max_choices, sync.event_payload_max_bytes).
// The error names the key whose binding failed, never a value.
func CommandsConfig(v *config.Values) (agentsapp.CommandConfig, error) {
	var c agentsapp.CommandConfig
	if err := config.Bind(v, &c); err != nil {
		return agentsapp.CommandConfig{}, err
	}
	return c, nil
}

// CommandDeps are the collaborators of the command register from the other steps of the start.
type CommandDeps struct {
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	Events     eventlog.Appender
	// Principals locks the owner's principal row (identity).
	Principals agentsapp.PrincipalLocker
	// Channels gives the channel facts of a list and a resolve (chat).
	Channels chat.TriggerFacts
}

// BindCommands builds the command register of the agents module (M2-16) over pool and its REST
// handlers. The chat step calls it, because the register reads channels through chat. Errors: a
// missing collaborator or a limit below 1.
func (ag *Agents) BindCommands(cfg agentsapp.CommandConfig, pool *pg.Pool, d CommandDeps) error {
	c, err := agentsapp.NewCommands(cfg, agentsapp.CommandDeps{
		Store: agentspg.NewRepository(pool), UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit, Events: d.Events,
		Principals: d.Principals, Channels: d.Channels,
	})
	if err != nil {
		return err
	}
	ag.Commands, ag.CommandHandlers = c, agentshttp.NewCommands(c)
	return nil
}

// NewAgents builds the agents module over pool with the unit of work uow, the Authorizer az, the
// rights hook rights (it names agents as affected), the recorder and appender of w, the identity
// module id (principal writer, directory and token revocation), the clock (nil means
// clock.System()) and log. It opens no connection. Errors: a missing collaborator or a page size
// the use cases refuse.
func NewAgents(cfg agentsapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, rights authz.RightsChanges,
	w IdentityWrites, id *Identity, clk clock.Clock, _ *slog.Logger) (*Agents, error) {
	if id == nil || id.Principals == nil || id.Tokens == nil {
		return nil, errors.New("wire: the agents module needs the identity module")
	}
	svc, err := agentsapp.New(cfg, agentsapp.Deps{
		Store: agentspg.NewRepository(pool), Principals: id.Principals, Directory: identityapp.NewDirectory(identitypg.NewRepository(pool)),
		Tokens: id.Tokens, Rights: rights, UoW: uow, Authorizer: az, Audit: w.Audit, Events: w.Events,
		Cursors: agentshttp.Cursors{}, Clock: clk, Measure: w.Measure,
	})
	if err != nil {
		return nil, err
	}
	cons, err := agentsapp.NewConsumers(agentspg.NewRepository(pool), az)
	if err != nil {
		return nil, err
	}
	return &Agents{Service: svc, Consumers: cons, Handlers: agentshttp.New(svc, cons), Switch: authzsource.NewSwitch(svc)}, nil
}
