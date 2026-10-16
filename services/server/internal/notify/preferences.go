// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Preferences reads and writes the caller's notification levels (13 section 3.1, M3-18). The
// caller is the actor of ctx (kernel/actor); every method calls authorize itself and runs in a
// unit of work. Agents and apps get forbidden with reason subject_kind_not_allowed.
//
// Example:
//
//	p, err := prefs.Put(ctx, notify.PutPreference{Target: notify.Target{Kind: "thread", ID: threadID}, Level: "mute"})
type Preferences interface {
	// List returns the caller's rows on targets the caller can still read, ordered by target
	// kind, then target id (settings.read). Rows on targets the caller lost stay stored.
	List(ctx context.Context) ([]Preference, error)
	// Put sets the level of one target (notification_pref.update, "self and read"). An unchanged
	// value answers the stored row without event. Errors: not_found for an unknown or unreadable
	// target (the same answer), conflict target_archived for an archived target,
	// validation_failed for an unknown level.
	Put(ctx context.Context, cmd PutPreference) (Preference, error)
	// Delete removes the caller's row of target; no row is no error and writes no event. Errors:
	// not_found as Put. An archived target may be cleared.
	Delete(ctx context.Context, target Target) error
}

// QuietHours reads and writes the caller's quiet-hour rules (13 section 3.4, M3-18). The caller
// is the actor of ctx; every method calls authorize itself and runs in a unit of work.
type QuietHours interface {
	// List returns every rule of the caller, ascending by id, disabled ones included
	// (settings.read).
	List(ctx context.Context) ([]QuietHoursRule, error)
	// Create adds a rule with a new id and version 1 (quiet_hours.update). Errors:
	// validation_failed (unknown_time_zone on /time_zone, limit_reached when the list would no
	// longer fit into one quiet_hours.updated payload).
	Create(ctx context.Context, in QuietHoursInput) (QuietHoursRule, error)
	// Put replaces the whole rule ruleID; equal values answer the stored rule without event.
	// Errors: not_found for an unknown or foreign rule, validation_failed as Create.
	Put(ctx context.Context, ruleID ids.ID, in QuietHoursInput) (QuietHoursRule, error)
	// Delete removes the rule ruleID. Errors: not_found for an unknown, foreign or already
	// deleted rule.
	Delete(ctx context.Context, ruleID ids.ID) error
}

// Target is the target of a notification preference.
type Target struct {
	// Kind is space, channel or thread.
	Kind string
	// ID is the space, channel or thread.
	ID ids.ID
}

// PutPreference is the input of Preferences.Put.
type PutPreference struct {
	// Target is the space, channel or thread.
	Target Target
	// Level is all, mentions or mute.
	Level string
	// MutedUntil mutes the target until this instant; the zero time is none.
	MutedUntil time.Time
}

// Preference is one stored notification preference of the caller.
type Preference struct {
	// Target is the space, channel or thread.
	Target Target
	// Level is all, mentions or mute.
	Level string
	// MutedUntil is the zero time when none is stored; otherwise whole microseconds.
	MutedUntil time.Time
	// Version counts the changes of the row, from 1.
	Version int64
}

// QuietHoursInput is the input of QuietHours.Create and QuietHours.Put (schema
// QuietHoursRuleInput).
type QuietHoursInput struct {
	// Weekdays is the bit mask, bit 0 = Monday, from 1 to 127.
	Weekdays int
	// StartLocal and EndLocal are local times HH:MM; EndLocal <= StartLocal crosses midnight.
	StartLocal, EndLocal string
	// TimeZone is an IANA zone name; nil stores NULL, the rule then follows users.time_zone. A
	// pointer, unlike the brief's string, so that an explicit "" is refused with
	// unknown_time_zone instead of being read as absent.
	TimeZone *string
	// Enabled switches the rule on.
	Enabled bool
}

// QuietHoursRule is one stored quiet-hour rule of the caller (schema QuietHoursRule).
type QuietHoursRule struct {
	// ID is the rule.
	ID ids.ID
	// Weekdays is the bit mask, bit 0 = Monday.
	Weekdays int
	// StartLocal and EndLocal are local times HH:MM.
	StartLocal, EndLocal string
	// TimeZone is "" when NULL (the person's users.time_zone applies).
	TimeZone string
	// Enabled reports whether the rule is on.
	Enabled bool
	// Version counts the changes of the rule, from 1.
	Version int64
}
