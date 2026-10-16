// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	notifyhttp "github.com/OWNER/bothy/services/server/internal/notify/adapters/httpapi"
	notifypg "github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// NotifyConfig binds the keys of the notify use cases (sync.event_payload_max_bytes,
// notify.user_default_level). The error names the key whose binding failed, never a value.
func NotifyConfig(v *config.Values) (notifyapp.Config, error) {
	var c notifyapp.Config
	if err := config.Bind(v, &c); err != nil {
		return notifyapp.Config{}, err
	}
	return c, nil
}

// Notify is the notify module (M3-18): the use cases of the notification levels and quiet hours,
// the reads of the push path and the REST handlers.
type Notify struct {
	// Service implements notify.Preferences and notify.QuietHours (Service.Preferences,
	// Service.QuietHours) and the reads of the push path (Service.Facts).
	Service *notifyapp.Service
	// Handlers are the REST handlers of the group Preferences.
	Handlers *notifyhttp.Preferences
}

// NotifyDeps are the collaborators of NewNotify from the other modules.
type NotifyDeps struct {
	// Chat gives the target paths (chat.Structure).
	Chat *chatapp.Service
	// Settings gives the stored personal default (sync.Settings).
	Settings notifyapp.SettingsSource
	// Users gives users.time_zone (identity.Directory).
	Users notifyapp.UserSource
}

// NewNotify builds the notify use cases over pool with the unit of work uow, the Authorizer az,
// the appender events, the ports of chat, sync and identity in d, the clock (nil means
// clock.System()) and log. It opens no connection. Errors: a missing collaborator or a value the
// use cases refuse.
func NewNotify(cfg notifyapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, events eventlog.Appender, d NotifyDeps,
	clk clock.Clock, log *slog.Logger) (*Notify, error) {
	if d.Chat == nil {
		return nil, errors.New("wire: notify needs the chat service")
	}
	svc, err := notifyapp.New(cfg, notifyapp.Deps{
		Store: notifypg.NewRepository(pool), UoW: uow, Authorizer: az, Events: events, Paths: d.Chat, Settings: d.Settings, Users: d.Users,
		Clock: clk, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &Notify{Service: svc, Handlers: notifyhttp.New(svc.Preferences(), svc.QuietHours())}, nil
}

// fillHandlers sets the nil notify handler of h to the one n built; a nil n leaves h unchanged.
func (n *Notify) fillHandlers(h *Handlers) {
	if n != nil && h.Preferences == nil {
		h.Preferences = n.Handlers
	}
}

// newNotifySubscribers returns the in-transaction subscribers of notify over pool (ChannelArchived
// and SpaceArchived remove preferences, M3-18). It opens no connection; the chat step binds the
// appender and the chat paths (Subscribers.Bind).
func newNotifySubscribers(pool *pg.Pool) *notifyapp.Subscribers {
	return notifyapp.NewSubscribers(notifypg.NewRepository(pool))
}
