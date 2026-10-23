// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/authz"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	notifyhttp "github.com/OWNER/bothy/services/server/internal/notify/adapters/httpapi"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	notifymetrics "github.com/OWNER/bothy/services/server/internal/notify/adapters/otelmetrics"
	notifypg "github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
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

// PushConfig binds the keys of the push path (notify.coalesce_window,
// notify.mobile_delay_when_desktop_active, notify.default_admin_enabled,
// notify.default_input_needed_breaks_quiet_hours, notify.badge_sync_min_interval, push.max_age,
// push.retry_base, push.retry_cap, push.max_concurrent_requests, push.max_consecutive_failures;
// M3-19). The error names the key whose binding failed, never a value.
func PushConfig(v *config.Values) (notifyapp.PushConfig, error) {
	var c notifyapp.PushConfig
	if err := config.Bind(v, &c); err != nil {
		return notifyapp.PushConfig{}, err
	}
	return c, nil
}

// Push is the push path of notify (M3-19): the consumer push, the use cases of the five jobs of
// notify, the scheduler over the job runtime and the read port of the push registrations.
type Push struct {
	// Push holds the use cases; Push.Consumer is the handler of the worker consumer push.
	Push *notifyapp.Push
	// Scheduler inserts the jobs of notify; the jobs step binds its inserter (BindJobs).
	Scheduler *notifyjobs.Scheduler
	// Registrations implements notify.PushRegistrations and the registration store.
	Registrations *notifypg.Registrations
}

// PushDeps are the collaborators of NewPush from the other modules.
type PushDeps struct {
	// Notify is the notify module of the chat step (the facts of M3-18).
	Notify *Notify
	// Authorizer decides push.deliver; Recipients gives the candidate readers (authz).
	Authorizer authorize.Authorizer
	Recipients authz.Recipients
	// Directory gives kind and status of people; Devices their active devices (identity).
	Directory notifyapp.PrincipalSource
	Devices   notifyapp.DeviceSource
	// Settings gives the personal switches (sync.Settings).
	Settings notifyapp.SettingsSource
	// Chat gives the thread facts; Messaging the anchors, agent messages and mentions.
	Chat      *chatapp.Service
	Messaging *messagingapp.Service
	// History reads retained events (sync.History); Positions the read positions (sync).
	History   sync.History
	Positions notifyapp.ReadPositions
	// InstanceID is the instance record's ID of every Notification.
	InstanceID ids.ID
	// Providers are the registered push providers; production registers none until M3-20 and
	// M3-21, tests pass fakepush providers.
	Providers []notifyapp.PushProvider
	// Presence gives the live connections; nil means presence.None (until M3-23).
	Presence presence.View
	// Routes takes the gateway and delayed routes; nil means notifyapp.FailClosedRoutes.
	Routes notifyapp.LiveRoutes
	// Metrics records the push signals; nil means notify/adapters/otelmetrics on the global meter
	// provider, which also gets the gauge bothy.push.registrations.active. Tests pass their own.
	Metrics notifyapp.Metrics
}

// NewPush builds the push path over pool with the unit of work uow, the configuration cfg, the
// collaborators d, the clock (nil means clock.System()) and log. Without d.Metrics it registers
// the gauge bothy.push.registrations.active on the global meter provider. It opens no connection; the scheduler stays unbound
// until Push.BindJobs. Errors: a missing collaborator or a value the push path refuses.
func NewPush(cfg notifyapp.PushConfig, pool *pg.Pool, uow txn.UnitOfWork, d PushDeps, clk clock.Clock, log *slog.Logger) (*Push, error) {
	if d.Notify == nil || d.Chat == nil || d.Messaging == nil || d.Recipients == nil || d.History == nil {
		return nil, errors.New("wire: the push path needs notify, chat, messaging, the recipients of authz and the history of sync")
	}
	regs := notifypg.NewRegistrations(pool)
	metrics := d.Metrics
	if metrics == nil {
		m, err := notifymetrics.New(nil)
		if err != nil {
			return nil, err
		}
		if _, err := notifymetrics.RegisterActiveGauge(nil, regs); err != nil {
			return nil, err
		}
		metrics = m
	}
	sched := notifyjobs.NewScheduler(nil)
	p, err := notifyapp.NewPush(cfg, notifyapp.PushDeps{
		Facts: d.Notify.Service.Facts(), Settings: d.Settings, Authorizer: d.Authorizer, Readers: d.Recipients,
		Principals: d.Directory, Devices: d.Devices, Registrations: regs, Store: regs, Presence: d.Presence, Routes: d.Routes,
		Scheduler: sched, History: d.History, Positions: d.Positions, Threads: d.Chat, Messages: d.Messaging,
		Mentions: d.Messaging.Mentions(), Providers: notifyjobs.Traced(d.Providers, nil), UoW: uow, InstanceID: d.InstanceID,
		Clock: clk, Metrics: metrics, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &Push{Push: p, Scheduler: sched, Registrations: regs}, nil
}

// BindJobs gives the scheduler of the push path the inserter of the job runtime of the process.
func (p *Push) BindJobs(ins jobs.Inserter) { p.Scheduler.Bind(ins) }

// Consumer returns the handler of the worker consumer push (sync.ConsumerPush).
func (p *Push) Consumer() sync.ConsumerHandler { return p.Push.Consumer() }
