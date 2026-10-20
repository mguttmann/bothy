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
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
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
		Cursors: agentshttp.Cursors{}, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	cons, err := agentsapp.NewConsumers(agentspg.NewRepository(pool), az)
	if err != nil {
		return nil, err
	}
	return &Agents{Service: svc, Consumers: cons, Handlers: agentshttp.New(svc, cons)}, nil
}
