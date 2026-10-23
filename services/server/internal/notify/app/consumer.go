// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// PushConfig holds the configuration keys of the push path (13 section 17, 03 section 9), bound in
// internal/wire.
type PushConfig struct {
	// CoalesceWindow is notify.coalesce_window, the bundling window of 13 4.5.
	CoalesceWindow time.Duration `config:"notify.coalesce_window"`
	// MobileDelay is notify.mobile_delay_when_desktop_active (13 4.4).
	MobileDelay time.Duration `config:"notify.mobile_delay_when_desktop_active"`
	// DefaultAdminEnabled is notify.default_admin_enabled, the default of the switch
	// notify.admin_enabled (13 3.3).
	DefaultAdminEnabled bool `config:"notify.default_admin_enabled"`
	// DefaultInputNeededBreaksQuietHours is notify.default_input_needed_breaks_quiet_hours.
	DefaultInputNeededBreaksQuietHours bool `config:"notify.default_input_needed_breaks_quiet_hours"`
	// BadgeSyncMinInterval is notify.badge_sync_min_interval (13 8).
	BadgeSyncMinInterval time.Duration `config:"notify.badge_sync_min_interval"`
	// MaxAge is push.max_age: an older hint is dropped (13 7.3).
	MaxAge time.Duration `config:"push.max_age"`
	// RetryBase and RetryCap are push.retry_base and push.retry_cap (13 7.2).
	RetryBase time.Duration `config:"push.retry_base"`
	RetryCap  time.Duration `config:"push.retry_cap"`
	// MaxConcurrentRequests is push.max_concurrent_requests per process (13 12 point 5).
	MaxConcurrentRequests int `config:"push.max_concurrent_requests"`
	// MaxConsecutiveFailures is push.max_consecutive_failures (03 section 9, 13 7.3).
	MaxConsecutiveFailures int `config:"push.max_consecutive_failures"`
}

// validate refuses values the push path cannot work with.
func (c PushConfig) validate() error {
	switch {
	case c.CoalesceWindow < 0, c.MobileDelay < 0, c.BadgeSyncMinInterval < 0:
		return errors.New("notify: the push windows must not be negative")
	case c.MaxAge <= 0:
		return errors.New("notify: push.max_age must be positive")
	case c.RetryBase <= 0 || c.RetryCap < c.RetryBase:
		return errors.New("notify: push.retry_base must be positive and push.retry_cap at least push.retry_base")
	case c.MaxConcurrentRequests < 1:
		return errors.New("notify: push.max_concurrent_requests must be at least 1")
	case c.MaxConsecutiveFailures < 1:
		return errors.New("notify: push.max_consecutive_failures must be at least 1")
	}
	return nil
}

// FactsSource is the part of Facts (M3-18) the push path reads; Service.Facts implements it.
type FactsSource interface {
	LevelFacts(ctx context.Context, threadID ids.ID, userIDs []ids.ID) (map[ids.ID]domain.LevelFacts, error)
	QuietRules(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]domain.QuietRule, error)
	LevelFactsOfUser(ctx context.Context, userID ids.ID, threadIDs []ids.ID) (map[ids.ID]domain.LevelFacts, error)
}

// PushDeps are the collaborators of the push path.
type PushDeps struct {
	// Facts gives the level facts and quiet-hour rules (M3-18).
	Facts FactsSource
	// Settings gives the personal switches (sync.Settings.ValuesFor).
	Settings SettingsSource
	// Authorizer decides push.deliver with the principal of the check without a request.
	Authorizer authorize.Authorizer
	// Readers gives the candidate readers of a channel (authz.Recipients).
	Readers ReaderSource
	// Principals gives kind and status of the candidates (identity.Directory).
	Principals PrincipalSource
	// Devices gives the active devices (identity.Devices.ActiveForUsers).
	Devices DeviceSource
	// Registrations gives the active push registrations (notify.PushRegistrations).
	Registrations RegistrationSource
	// Store reads and writes the registration of a send (adapters/postgres).
	Store RegistrationStore
	// Presence gives the live connections; nil means presence.None.
	Presence presence.View
	// Routes takes the routes gateway and push_delayed; nil means FailClosedRoutes.
	Routes LiveRoutes
	// Publisher hands notification.created to the gateway of this process (the hub in role all);
	// nil in a process without a gateway (role worker), where notify.gateway_send skips with
	// no_registration (M3-23 design note 4 b).
	Publisher eventlog.EphemeralPublisher
	// Scheduler inserts and reads the jobs of notify.
	Scheduler Scheduler
	// History reads retained events (sync.History).
	History HistorySource
	// Positions reads read_states.last_read_seq (sync).
	Positions ReadPositions
	// Threads reads thread facts (chat).
	Threads ThreadSource
	// Messages reads anchors and agent messages (messaging).
	Messages MessageSource
	// Mentions counts unread mentions for the badge (messaging).
	Mentions MentionSource
	// Providers are the registered push providers, at most one per Name; empty is allowed.
	Providers []PushProvider
	// UoW runs the steps of the jobs.
	UoW txn.UnitOfWork
	// InstanceID is the instance record's ID for Notification.InstanceID.
	InstanceID ids.ID
	// Clock gives "now"; nil means clock.System().
	Clock clock.Clock
	// Metrics records the push signals; nil means NopMetrics.
	Metrics Metrics
	// Log takes the Warn lines (never a person, a thread, a token or a text); nil means slog.Default().
	Log *slog.Logger
	// Jitter returns the retry jitter in [0.5, 1]; nil draws it at random.
	Jitter func() float64
}

// Push is the push path of notify (M3-19): the consumer push, the flush of a bundling window, the
// job notify.push_send, the badge and the badge sync. It is safe for concurrent use.
type Push struct {
	cfg       PushConfig
	d         PushDeps
	providers map[string]PushProvider
	sem       chan struct{}
	// noGateway logs the Warn line of a gateway hint without a gateway once per process.
	noGateway stdsync.Once
}

// NewPush returns the push path. Errors: a missing collaborator, two providers of one name, or a
// configuration value the path cannot work with.
func NewPush(cfg PushConfig, d PushDeps) (*Push, error) {
	if d.Facts == nil || d.Settings == nil || d.Authorizer == nil || d.Readers == nil || d.Principals == nil ||
		d.Devices == nil || d.Registrations == nil || d.Store == nil || d.Scheduler == nil || d.History == nil ||
		d.Positions == nil || d.Threads == nil || d.Messages == nil || d.Mentions == nil || d.UoW == nil {
		return nil, errors.New("notify: the push path needs every source, the scheduler, the store and the unit of work")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if d.Presence == nil {
		d.Presence = presence.None{}
	}
	if d.Routes == nil {
		d.Routes = FailClosedRoutes{}
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Metrics == nil {
		d.Metrics = NopMetrics{}
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Jitter == nil {
		d.Jitter = func() float64 { return 0.5 + rand.Float64()/2 } // #nosec G404 -- retry jitter, not a secret
	}
	providers := make(map[string]PushProvider, len(d.Providers))
	for _, p := range d.Providers {
		if p == nil || p.Name() == "" {
			return nil, errors.New("notify: a push provider without a name")
		}
		if _, dup := providers[p.Name()]; dup {
			return nil, errors.New("notify: two push providers of one name")
		}
		providers[p.Name()] = p
	}
	return &Push{cfg: cfg, d: d, providers: providers, sem: make(chan struct{}, cfg.MaxConcurrentRequests)}, nil
}

// Consumer returns the handler of the worker consumer push (sync.ConsumerPush).
func (p *Push) Consumer() *PushConsumer { return &PushConsumer{p: p} }

// PushConsumer is the sync.ConsumerHandler of the consumer push (13 12 point 1, ADR 0051): it
// turns the events message.created, run.state_changed, access.expiring and device.pending into
// hint jobs and read_state.updated into the badge sync, inside the round's transaction, so the
// jobs and the cursor move commit together (R-PN-09). Every other type is ignored.
type PushConsumer struct{ p *Push }

// Compile-time check.
var _ sync.ConsumerHandler = (*PushConsumer)(nil)

// HandleEvent implements sync.ConsumerHandler. It appends no durable event (M2-01 note 6) and
// returns the same jobs for the same event when the round runs again. An error rolls the round
// back.
func (c *PushConsumer) HandleEvent(ctx context.Context, ev sync.Event) error {
	typ, known := authorize.ParseEventType(ev.Type)
	if !known {
		return nil
	}
	// Every type outside this table moves the cursor without effect (13 12 point 1).
	handlers := map[authorize.EventType]func(context.Context, sync.Event) error{
		authorize.EventMessageCreated:   c.p.onMessage,
		authorize.EventRunStateChanged:  c.p.onRun,
		authorize.EventAccessExpiring:   c.p.onAccessExpiring,
		authorize.EventDevicePending:    c.p.onDevicePending,
		authorize.EventReadStateUpdated: c.p.onReadState,
	}
	if h, ok := handlers[typ]; ok {
		return h(eventScope(ctx), ev)
	}
	return nil
}

// eventScope returns ctx with one rights version and one read of the thread and channel facts
// for all push.deliver checks of one event: every recipient of the event is decided against the
// same rights version, as the checks of one request are (authorize.WithRequestScope), and the
// structure of the event's thread is read once instead of once per recipient
// (authorize.WithStructureScope). Without them the consumer read both again for every reader of
// the channel and fell behind the sustained load (perf.push_handoff_p95, M3-24).
func eventScope(ctx context.Context) context.Context {
	return authorize.WithStructureScope(authorize.WithRequestScope(authorize.WithoutRequestScope(ctx)))
}

// FailClosedRoutes is the default LiveRoutes until M3-23: every route over the gateway or a
// delayed push is an internal error, so the round rolls back instead of losing the hint
// (D-M3-19-9). With presence.None no decision yields such a route.
type FailClosedRoutes struct{}

// Gateway implements LiveRoutes.
func (FailClosedRoutes) Gateway(context.Context, Hint, domain.Route) error {
	return internal("notify: the gateway route is not built before M3-23")
}

// Delayed implements LiveRoutes.
func (FailClosedRoutes) Delayed(context.Context, Hint, domain.Route) error {
	return internal("notify: the delayed push is not built before M3-23")
}
