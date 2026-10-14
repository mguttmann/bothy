// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/authzsource"
	chathttp "github.com/OWNER/bothy/services/server/internal/chat/adapters/httpapi"
	chatpg "github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
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

// ChatConfig binds the keys of the chat use cases (space.name_max_chars, the channel keys of
// M1-53 and the thread and page keys of M1-55). The error names the key whose binding failed, never a value.
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
}

// Chat is the chat module of the process (M1-52 to M1-55): the space, sidebar, channel, forum tag
// and operator use cases of chat.Structure, the thread use cases of chat.Threads and the REST
// handlers of their twenty-six operations.
type Chat struct {
	// Service implements chat.Structure and chat.Threads.
	Service *chatapp.Service
	// Handlers are the REST handlers of spaces and the sidebar.
	Handlers *chathttp.Spaces
	// ChannelHandlers are the REST handlers of channels and forum tags (M1-53).
	ChannelHandlers *chathttp.Channels
	// ThreadHandlers are the REST handlers of threads (M1-55).
	ThreadHandlers *chathttp.Threads
}

// NewChat builds the chat module over pool with the unit of work uow, the Authorizer az (also the
// rights query of SpaceView), the recorder, appender and rights hook of w, the principal
// directory of identity over pool, the agent policy source policies (the agents rows of M1-54 and
// the billing mode of a thread operator; nil means the source of agents, NewAgentPolicySource)
// and the clock (nil means clock.System()). It opens no connection. Errors: a missing
// collaborator or a limit the use cases refuse.
func NewChat(cfg chatapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, rights authz.RightsQuery,
	w ChatWrites, policies authz.AgentPolicySource, clk clock.Clock) (*Chat, error) {
	if az == nil || rights == nil {
		return nil, errors.New("wire: the chat module needs the Authorizer")
	}
	if policies == nil {
		policies = NewAgentPolicySource(pool)
	}
	svc, err := chatapp.New(cfg, chatapp.Deps{
		Store: chatpg.NewRepository(pool), UoW: uow, Authorizer: az, Rights: rights, Audit: w.Audit, Events: w.Events,
		RightsChanges: w.Rights, Directory: identityapp.NewDirectory(identitypg.NewRepository(pool)),
		Policies: policies, Cursors: chathttp.Cursors{}, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Chat{Service: svc, Handlers: chathttp.New(svc), ChannelHandlers: chathttp.NewChannels(svc), ThreadHandlers: chathttp.NewThreads(svc)}, nil
}

// fillHandlers sets the nil chat handlers of h to the ones c built: spaces and the sidebar
// (M1-52), channels and forum tags (M1-53), threads (M1-55).
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
}
