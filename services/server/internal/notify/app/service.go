// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the notify module that M3-18 builds: the caller's
// notification levels (notify.Preferences) and quiet hours (notify.QuietHours), the cleanup of
// preferences when a space or channel is archived (Subscribers), and the reads of the push path
// (Facts.LevelFacts, Facts.QuietRules) that the consumer push of M3-19 calls. SQL stays in
// adapters/postgres; the pure rules come from notify/domain (M3-17).
package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// Config holds the configuration of the use cases, bound in internal/wire (wire.NotifyConfig).
type Config struct {
	// EventPayloadMaxBytes is sync.event_payload_max_bytes: the encoded QuietHoursPayload of the
	// complete list must stay below it (04 payload rule 2), which bounds the number of rules.
	EventPayloadMaxBytes int `config:"sync.event_payload_max_bytes"`
	// UserDefaultLevel is notify.user_default_level, the last step of 13 section 3.1.
	UserDefaultLevel string `config:"notify.user_default_level"`
}

// PreferenceRow is one row of notification_preferences with its person.
type PreferenceRow struct {
	// UserID is the person of the row.
	UserID ids.ID
	notify.Preference
}

// Store is the outgoing port over notification_preferences and quiet_hours (implemented by
// adapters/postgres). Every statement names the person or the explicit list of people; reads run
// in the ambient transaction of ctx when there is one, writes only inside a unit of work.
type Store interface {
	// UpsertPreference inserts p for user with version 1 or replaces a stored row whose level or
	// muted_until differs with version + 1, in one statement; changed is false, and the row not
	// returned, when the stored values are equal.
	UpsertPreference(ctx context.Context, user ids.ID, p notify.Preference, at time.Time) (out notify.Preference, changed bool, err error)
	// Preference returns the row of user and target; found is false for none.
	Preference(ctx context.Context, user ids.ID, target notify.Target) (p notify.Preference, found bool, err error)
	// DeletePreference deletes the row of user and target and returns it; found is false for none.
	DeletePreference(ctx context.Context, user ids.ID, target notify.Target) (p notify.Preference, found bool, err error)
	// Preferences returns every row of user ordered by target kind, then target id.
	Preferences(ctx context.Context, user ids.ID) ([]notify.Preference, error)
	// DeletePreferencesOfTargets deletes every row of every person on the space (zero: none), the
	// channels and the threads and returns them in ascending (user, target kind, target id).
	DeletePreferencesOfTargets(ctx context.Context, space ids.ID, channels, threads []ids.ID) ([]PreferenceRow, error)
	// LevelRows returns the rows of the people on the thread, the channel and the space (zero:
	// left out) in one query.
	LevelRows(ctx context.Context, users []ids.ID, thread, channel, space ids.ID) ([]PreferenceRow, error)
	// LockRules serializes the quiet-hour writes of user until the end of the unit of work.
	LockRules(ctx context.Context, user ids.ID) error
	// Rules returns every rule of user, ascending by id.
	Rules(ctx context.Context, user ids.ID) ([]notify.QuietHoursRule, error)
	// Rule returns the rule id of user; found is false for none or a rule of another person.
	Rule(ctx context.Context, user, id ids.ID) (r notify.QuietHoursRule, found bool, err error)
	// InsertRule stores r (its ID set by the caller) for user with version 1.
	InsertRule(ctx context.Context, user ids.ID, r notify.QuietHoursRule) (notify.QuietHoursRule, error)
	// UpdateRule replaces the whole rule r.ID of user with version + 1; found is false for none.
	UpdateRule(ctx context.Context, user ids.ID, r notify.QuietHoursRule) (out notify.QuietHoursRule, found bool, err error)
	// DeleteRule deletes the rule id of user and returns it; found is false for none.
	DeleteRule(ctx context.Context, user, id ids.ID) (r notify.QuietHoursRule, found bool, err error)
	// RulesOfUsers returns the rules of the people, keyed by person, each list ascending by id.
	RulesOfUsers(ctx context.Context, users []ids.ID) (map[ids.ID][]notify.QuietHoursRule, error)
}

// PathSource is the part of chat.Structure notify uses (brief M3-18 note 10). It never
// authorizes.
type PathSource interface {
	NotificationPath(ctx context.Context, kind string, id ids.ID) (chat.NotificationPath, error)
	LockNotificationPath(ctx context.Context, kind string, id ids.ID) (chat.NotificationPath, error)
	ChannelIDsOfSpace(ctx context.Context, spaceID ids.ID) ([]ids.ID, error)
	ThreadIDsOfChannels(ctx context.Context, channelIDs []ids.ID) ([]ids.ID, error)
}

// SettingsSource is the part of sync.Settings notify uses: the stored personal default.
type SettingsSource interface {
	ValuesFor(ctx context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error)
}

// UserSource is the part of identity.Directory notify uses: the time zones of people.
type UserSource interface {
	Users(ctx context.Context, ids []ids.ID) (map[ids.ID]identity.UserView, error)
}

// Deps are the collaborators of the use cases.
type Deps struct {
	// Store reads and writes the two tables.
	Store Store
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Authorizer decides every operation.
	Authorizer authorize.Authorizer
	// Events appends notification_pref.updated and quiet_hours.updated.
	Events eventlog.Appender
	// Paths resolves and locks the path of a target (chat).
	Paths PathSource
	// Settings gives the stored notify.default_level (sync).
	Settings SettingsSource
	// Users gives users.time_zone (identity).
	Users UserSource
	// Clock gives updated_at; nil means clock.System().
	Clock clock.Clock
	// Log takes the Warn lines of LevelFacts and QuietRules; nil means slog.Default().
	Log *slog.Logger
}

// Service implements notify.Preferences (through Service.Preferences) and notify.QuietHours
// (through Service.QuietHours), and the reads of the push path (Service.Facts). It is safe for
// concurrent use.
type Service struct {
	cfg Config
	d   Deps
}

// Prefs implements notify.Preferences over the use cases.
type Prefs struct{ *Service }

// Quiet implements notify.QuietHours over the use cases.
type Quiet struct{ *Service }

// Facts gathers the facts of the push path (13 sections 3.1 and 3.4) for M3-19.
type Facts struct{ *Service }

// Compile-time checks: the incoming ports.
var (
	_ notify.Preferences = Prefs{}
	_ notify.QuietHours  = Quiet{}
)

// New returns the use cases. Errors: a missing collaborator, a payload limit below 1 or an
// instance default that is not all, mentions or mute.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Events == nil || d.Paths == nil || d.Settings == nil || d.Users == nil {
		return nil, errors.New("notify: the use cases need store, unit of work, authorizer, appender, chat, sync and identity")
	}
	if cfg.EventPayloadMaxBytes < 1 {
		return nil, errors.New("notify: sync.event_payload_max_bytes must be at least 1")
	}
	if !domain.Level(cfg.UserDefaultLevel).Valid() {
		return nil, errors.New("notify: notify.user_default_level must be all, mentions or mute")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{cfg: cfg, d: d}, nil
}

// Preferences returns the incoming port of the notification levels.
func (s *Service) Preferences() Prefs { return Prefs{s} }

// QuietHours returns the incoming port of the quiet hours.
func (s *Service) QuietHours() Quiet { return Quiet{s} }

// Facts returns the reads of the push path.
func (s *Service) Facts() Facts { return Facts{s} }

// caller returns the actor of ctx; anonymous when none.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// check authorizes act on r for a and returns the decision as an error (nil for allow).
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// self is the resource user of the caller (settings.read, quiet_hours.update).
func self(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceUser, ID: id, OwnerID: id}
}

// notFound is the answer for an unknown, foreign or unreadable target or rule (AZ-P4).
func notFound() error { return errs.New(errs.CodeNotFound, "") }
