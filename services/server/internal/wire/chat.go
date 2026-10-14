// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/authzsource"
	chathttp "github.com/OWNER/bothy/services/server/internal/chat/adapters/httpapi"
	chatpg "github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// ChatConfig binds the keys of the chat use cases (space.name_max_chars). The error names the key
// whose binding failed, never a value.
func ChatConfig(v *config.Values) (chatapp.Config, error) {
	var c chatapp.Config
	if err := config.Bind(v, &c); err != nil {
		return chatapp.Config{}, err
	}
	return c, nil
}

// chatStructure is the authz.StructureSource of the process: the space part of chat (M1-52) and
// the empty channel and thread part until M1-53 and M1-55 (empty_sources.go).
type chatStructure struct {
	*authzsource.Source
	emptyChannels
}

// Compile-time check: the joined source is the structure port of authz.
var _ authz.StructureSource = chatStructure{}

// NewStructureSource returns the authz.StructureSource of the process over pool: spaces from the
// table of chat (M1-52), no channel, thread or dm participation yet. The Authorizer, the rights
// hooks and the Admin use cases of authz read it.
func NewStructureSource(pool *pg.Pool) authz.StructureSource {
	return chatStructure{Source: authzsource.New(chatpg.NewRepository(pool))}
}

// ChatWrites are the write collaborators of the chat use cases: the recorder and the appender.
type ChatWrites struct {
	Audit  auditing.Recorder
	Events eventlog.Appender
}

// Chat is the chat module of the process (M1-52): the space and sidebar use cases of
// chat.Structure and the REST handlers of their seven operations.
type Chat struct {
	// Service implements chat.Structure.
	Service *chatapp.Service
	// Handlers are the REST handlers.
	Handlers *chathttp.Spaces
}

// NewChat builds the chat module over pool with the unit of work uow, the Authorizer az (also the
// rights query of SpaceView), the recorder and appender of w and the clock (nil means
// clock.System()). It opens no connection. Errors: a missing collaborator or a name limit the use
// cases refuse.
func NewChat(cfg chatapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, rights authz.RightsQuery,
	w ChatWrites, clk clock.Clock) (*Chat, error) {
	if az == nil || rights == nil {
		return nil, errors.New("wire: the chat module needs the Authorizer")
	}
	svc, err := chatapp.New(cfg, chatapp.Deps{
		Store: chatpg.NewRepository(pool), UoW: uow, Authorizer: az, Rights: rights, Audit: w.Audit, Events: w.Events, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Chat{Service: svc, Handlers: chathttp.New(svc)}, nil
}
