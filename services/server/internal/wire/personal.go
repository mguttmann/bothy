// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	synchttp "github.com/OWNER/bothy/services/server/internal/sync/adapters/httpapi"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// PersonalConfig binds the keys of the personal data use cases of sync (message.max_content_bytes,
// message.max_attachments, sync.event_payload_max_bytes). The error names the key whose binding
// failed, never a value.
func PersonalConfig(v *config.Values) (syncapp.PersonalConfig, error) {
	var c syncapp.PersonalConfig
	if err := config.Bind(v, &c); err != nil {
		return syncapp.PersonalConfig{}, err
	}
	return c, nil
}

// Personal is the personal data part of the sync module (M1-61): the use cases of
// sync.ReadStates, Drafts and Settings and their REST handlers.
type Personal struct {
	// Service implements sync.ReadStates; Service.Drafts and Service.Settings the other ports.
	Service *syncapp.Personal
	// Handlers are the REST handlers.
	Handlers *synchttp.Personal
}

// NewPersonal builds the personal data use cases over pool with the unit of work uow, the
// Authorizer az, the appender events, the chat service (threads, channels and sidebar) and the
// messaging service (unread mentions and reply targets), and the clock (nil means
// clock.System()). It opens no connection. Errors: a missing collaborator or a limit the use cases
// refuse.
func NewPersonal(cfg syncapp.PersonalConfig, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, events eventlog.Appender,
	c *chatapp.Service, m *messagingapp.Service, clk clock.Clock) (*Personal, error) {
	if c == nil || m == nil {
		return nil, errors.New("wire: the personal data of sync need the chat and messaging services")
	}
	svc, err := syncapp.NewPersonal(cfg, syncapp.PersonalDeps{
		Store: syncpg.NewPersonalRepository(pool), UoW: uow, Authorizer: az, Events: events, Threads: c, Channels: c,
		Mentions: m.Mentions(), Replies: m, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Personal{Service: svc, Handlers: synchttp.New(svc, svc.Drafts(), svc.Settings())}, nil
}

// fillHandlers sets the nil personal data handler of h to the one p built; a nil p leaves h
// unchanged.
func (p *Personal) fillHandlers(h *Handlers) {
	if p != nil && h.Personal == nil {
		h.Personal = p.Handlers
	}
}

// newSyncSubscribers returns the in-transaction subscribers of sync over pool (MessagePosted
// clears drafts, M1-61; ThreadMoved moves the read states and drafts of a thread, M1-62). It opens
// no connection; the start binds the appender (Subscribers.Bind).
func newSyncSubscribers(pool *pg.Pool) *syncapp.Subscribers {
	rows := syncpg.NewPersonalRepository(pool)
	return syncapp.NewSubscribers(rows, rows)
}
