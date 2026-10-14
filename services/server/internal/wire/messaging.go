// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messaginghttp "github.com/OWNER/bothy/services/server/internal/messaging/adapters/httpapi"
	messagingpg "github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// MessagingConfig binds the keys of the messaging use cases (message.max_content_bytes,
// message.max_mentions, sync.event_payload_max_bytes). The error names the key whose binding
// failed, never a value.
func MessagingConfig(v *config.Values) (messagingapp.Config, error) {
	var c messagingapp.Config
	if err := config.Bind(v, &c); err != nil {
		return messagingapp.Config{}, err
	}
	return c, nil
}

// Messaging is the messaging module of the process (M1-56): the use cases of messaging.Messages
// and the REST handlers of createMessage and createThread. Its in-transaction event
// messaging.MessagePosted has no production subscriber yet (runs, sync and notify subscribe from
// M1-61 and M2 on, registerTxn); the golden file of the registrations is unchanged.
type Messaging struct {
	// Service implements messaging.Messages.
	Service *messagingapp.Service
	// Handlers are the REST handlers.
	Handlers *messaginghttp.Messages
}

// NewMessaging builds the messaging module over pool with the unit of work uow, the Authorizer
// az, the appender events, the thread port of chat, the principal directory of identity over
// pool and the clock (nil means clock.System()). It opens no connection. Errors: a missing
// collaborator or a limit the use cases refuse.
func NewMessaging(cfg messagingapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, events eventlog.Appender,
	threads chat.Threads, clk clock.Clock) (*Messaging, error) {
	if az == nil || threads == nil {
		return nil, errors.New("wire: the messaging module needs the Authorizer and the threads of chat")
	}
	svc, err := messagingapp.New(cfg, messagingapp.Deps{
		Store: messagingpg.NewRepository(pool), UoW: uow, Authorizer: az, Events: events, Threads: threads,
		Directory: identityapp.NewDirectory(identitypg.NewRepository(pool)), Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Messaging{Service: svc, Handlers: messaginghttp.New(svc, threads)}, nil
}

// fillHandlers sets the nil messaging handler of h to the one m built; a nil m (a role or a test
// without the module) leaves h unchanged.
func (m *Messaging) fillHandlers(h *Handlers) {
	if m != nil && h.Messages == nil {
		h.Messages = m.Handlers
	}
}
