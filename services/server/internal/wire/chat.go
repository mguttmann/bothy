// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/authz"
	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/authzsource"
	chathttp "github.com/OWNER/bothy/services/server/internal/chat/adapters/httpapi"
	chatpg "github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	filespg "github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity"
	importerpg "github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	importerapp "github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// ChatConfig binds the keys of the chat use cases (space.name_max_chars, the channel keys of
// M1-53, the thread and page keys of M1-55 and sidebar.order_max_items of M4-51). The error names the key whose binding failed, never a value.
func ChatConfig(v *config.Values) (chatapp.Config, error) {
	var c chatapp.Config
	if err := config.Bind(v, &c); err != nil {
		return chatapp.Config{}, err
	}
	return c, nil
}

// chatStructure is the authz.StructureSource of the process: the structure of chat (M1-52 to
// M1-55) and the empty dm part until V3 (empty_sources.go).
type chatStructure struct {
	*authzsource.Source
	emptyDMs
}

// Compile-time check: the joined source is the structure port of authz.
var _ authz.StructureSource = chatStructure{}

// NewStructureSource returns the authz.StructureSource of the process over pool: spaces,
// channels, threads and the served targets of I1 from the tables of chat (M1-52 to M1-55), no dm
// participation yet. The Authorizer, the rights hooks and the Admin use cases of authz read it.
func NewStructureSource(pool *pg.Pool) authz.StructureSource {
	return chatStructure{Source: authzsource.New(chatpg.NewRepository(pool))}
}

// ChatWrites are the write collaborators of the chat use cases: the recorder, the appender and
// the rights hook that names the affected principals of an operator change (M1-54).
type ChatWrites struct {
	Audit  auditing.Recorder
	Events eventlog.Appender
	Rights authz.RightsChanges
	// Directory is the principal directory of identity with the search language configuration
	// (Identity.Directory, M3-13): every thread takes the language of its creator from it
	// (14 3.1). The operator checks of M1-54 read the agents through it as well.
	Directory identity.Directory
	// Resolver is the guided flow of authz (AuthzAdmin.Resolver, M2-21) for the operator actions
	// with resolve_i1; nil refuses such a request with kind internal.
	Resolver authz.I1Resolver
}

// Chat is the chat module of the process (M1-52 to M1-55): the space, sidebar, channel, forum tag
// and operator use cases of chat.Structure, the thread use cases of chat.Threads, the behaviour
// use cases of chat.AgentConfig (M2-02) and the REST handlers of their operations.
type Chat struct {
	// Service implements chat.Structure and chat.Threads.
	Service *chatapp.Service
	// Handlers are the REST handlers of spaces and the sidebar.
	Handlers *chathttp.Spaces
	// ChannelHandlers are the REST handlers of channels and forum tags (M1-53).
	ChannelHandlers *chathttp.Channels
	// ThreadHandlers are the REST handlers of threads (M1-55).
	ThreadHandlers *chathttp.Threads
	// BehaviorHandlers are the REST handlers of channel behaviour and agent configuration
	// (M2-02).
	BehaviorHandlers *chathttp.Behavior
	// ReadOnlyHandlers are the REST handler of setChannelReadOnly (M8-03).
	ReadOnlyHandlers *chathttp.ReadOnly
	// DeletionHandlers are the REST handlers of deleting and restoring spaces, channels and
	// threads (FEAT-delete-spaces-chats).
	DeletionHandlers *chathttp.Deletion
}

// NewChat builds the chat module over pool with the unit of work uow, the Authorizer az (also the
// rights query of SpaceView), the recorder, appender, rights hook and principal directory of w,
// the agent policy source policies (the agents rows of M1-54 and
// the billing mode of a thread operator; nil means the source of agents, NewAgentPolicySource)
// and the clock (nil means clock.System()); a filter mismatch of listServedChannels is counted in
// bothy.authz.errors on the global meter provider of platform/otel (M2-02). It opens no
// connection. Errors: a missing collaborator, a limit the use cases refuse or the counter.
func NewChat(cfg chatapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, rights authz.RightsQuery,
	w ChatWrites, policies authz.AgentPolicySource, clk clock.Clock) (*Chat, error) {
	if az == nil || rights == nil || w.Directory == nil {
		return nil, errors.New("wire: the chat module needs the Authorizer and the directory of identity")
	}
	if policies == nil {
		policies = NewAgentPolicySource(pool)
	}
	counter, err := authztelemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	repo := chatpg.NewRepository(pool)
	guard, err := importerapp.NewGuard(importerpg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	svc, err := chatapp.New(cfg, chatapp.Deps{
		Store: repo, UoW: uow, Authorizer: az, Rights: rights, Audit: w.Audit, Events: w.Events,
		RightsChanges: w.Rights, Directory: w.Directory,
		Policies: policies, Cursors: chathttp.Cursors{}, Clock: clk, Behaviors: repo, Errors: counter, Resolver: w.Resolver, Guard: guard,
		Files: filespg.NewRepository(pool), Sessions: agentspg.NewRepository(pool),
	})
	if err != nil {
		return nil, err
	}
	return &Chat{Service: svc, Handlers: chathttp.New(svc), ChannelHandlers: chathttp.NewChannels(svc), ThreadHandlers: chathttp.NewThreads(svc),
		BehaviorHandlers: chathttp.NewBehavior(svc, svc), ReadOnlyHandlers: chathttp.NewReadOnly(svc), DeletionHandlers: chathttp.NewDeletion(svc)}, nil
}

// fillHandlers sets the nil chat handlers of h to the ones c built: spaces and the sidebar
// (M1-52), channels and forum tags (M1-53), threads (M1-55), behaviour and agent configuration
// (M2-02).
func (c *Chat) fillHandlers(h *Handlers) {
	if h.Spaces == nil {
		h.Spaces = c.Handlers
	}
	if h.Channels == nil {
		h.Channels = c.ChannelHandlers
	}
	if h.Threads == nil {
		h.Threads = c.ThreadHandlers
	}
	if h.Behavior == nil {
		h.Behavior = c.BehaviorHandlers
	}
	if h.ReadOnly == nil {
		h.ReadOnly = c.ReadOnlyHandlers
	}
	if h.Deletion == nil {
		h.Deletion = c.DeletionHandlers
	}
}
