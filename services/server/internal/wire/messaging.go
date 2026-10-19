// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messaginghttp "github.com/OWNER/bothy/services/server/internal/messaging/adapters/httpapi"
	messagingmetrics "github.com/OWNER/bothy/services/server/internal/messaging/adapters/otelmetrics"
	messagingpg "github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	messagingunicode "github.com/OWNER/bothy/services/server/internal/messaging/adapters/unicode"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// MessagingConfig binds the keys of the messaging use cases (message.max_content_bytes,
// message.max_mentions, sync.event_payload_max_bytes, message.list_content_max_bytes,
// template.name_max_chars, template.max_per_scope, api.page_size_default, api.page_size_max,
// reaction.emoji_max_bytes, reaction.max_distinct_per_message, pin.max_per_thread). The error names
// the key whose binding failed, never a value.
func MessagingConfig(v *config.Values) (messagingapp.Config, error) {
	var c messagingapp.Config
	if err := config.Bind(v, &c); err != nil {
		return messagingapp.Config{}, err
	}
	return c, nil
}

// Messaging is the messaging module of the process (M1-56 to M1-60): the use cases of
// messaging.Messages, Mentions, Templates, Reactions and Pins and their REST handlers. Filter mismatches of the
// history and mention lists count in bothy.authz.errors on the global meter provider of
// platform/otel. Its in-transaction events messaging.MessagePosted and messaging.MessageDeleted
// have no production subscriber yet (runs, sync and notify subscribe from M1-61 and M2 on,
// registerTxn); the golden file of the registrations is unchanged.
type Messaging struct {
	// Service implements messaging.Messages; Service.Mentions and Service.Templates the other
	// two ports.
	Service *messagingapp.Service
	// Handlers are the REST handlers.
	Handlers *messaginghttp.Messages
}

// MessagingWrites are the write ports of other modules the messaging module receives: the
// appender of sync, the redactor of sync (M1-58) and the audit recorder (M1-58, the deletion of
// another principal's message).
type MessagingWrites struct {
	Events   eventlog.Appender
	Redactor eventlog.Redactor
	Audit    auditing.Recorder
	// Metrics counts the created messages; nil means bothy.message.created of
	// messaging/adapters/otelmetrics on the global meter provider of platform/otel (M1-68).
	Metrics messagingapp.Metrics
}

// NewMessaging builds the messaging module over pool with the unit of work uow, the Authorizer
// az, the write ports w, the thread port of chat, the principal directory dir of identity with
// the search language configuration (Identity.Directory: mentions, authors and the language of
// every message, M3-13), the clock (nil means clock.System()) and log (nil means slog.Default()),
// which with bothy.authz.errors on the global meter provider of platform/otel records a history
// item that fails message.read (filter_mismatch, M1-57). It opens no connection. Errors: a missing
// collaborator, a limit the use cases refuse or the counter.
func NewMessaging(cfg messagingapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, w MessagingWrites,
	threads chat.Threads, dir identity.Directory, clk clock.Clock, log *slog.Logger) (*Messaging, error) {
	if az == nil || threads == nil || dir == nil {
		return nil, errors.New("wire: the messaging module needs the Authorizer, the threads of chat and the directory of identity")
	}
	counter, err := authztelemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	if w.Metrics == nil {
		m, err := messagingmetrics.New(nil)
		if err != nil {
			return nil, err
		}
		w.Metrics = m
	}
	svc, err := messagingapp.New(cfg, messagingapp.Deps{
		Store: messagingpg.NewRepository(pool), UoW: uow, Authorizer: az, Events: w.Events, Redactor: w.Redactor, Audit: w.Audit,
		Threads:   threads,
		Directory: identityapp.NewDirectory(identitypg.NewRepository(pool)), Cursors: messaginghttp.Cursors{}, Symbols: messagingunicode.Normalizer{}, Errors: counter, Clock: clk, Log: log,
		Metrics: w.Metrics,
	})
	if err != nil {
		return nil, err
	}
	return &Messaging{Service: svc, Handlers: messaginghttp.New(svc, svc.Mentions(), svc.Templates(), svc.Reactions(), svc.Pins(), threads)}, nil
}

// fillHandlers sets the nil messaging handler of h to the one m built; a nil m (a role or a test
// without the module) leaves h unchanged.
func (m *Messaging) fillHandlers(h *Handlers) {
	if m != nil && h.Messages == nil {
		h.Messages = m.Handlers
	}
}

// newMessagingSubscribers returns the in-transaction subscribers of messaging over pool
// (ThreadMoved moves messages.channel_id, M1-62). It opens no connection.
func newMessagingSubscribers(pool *pg.Pool) *messagingapp.Subscribers {
	subs, err := messagingapp.NewSubscribers(messagingpg.NewRepository(pool))
	if err != nil {
		// The repository is never nil, so this is a programming error.
		panic(err)
	}
	return subs
}
