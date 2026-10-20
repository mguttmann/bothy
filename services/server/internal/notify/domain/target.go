// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"
)

// TargetKind is the kind of the target of a notification preference: the column
// notification_preferences.target_kind of 03 and the path parameter TargetKindPath.
type TargetKind string

// The targets of 13 section 3.1: a level is set per space, channel or thread.
const (
	// TargetSpace is a space.
	TargetSpace TargetKind = "space"
	// TargetChannel is a channel.
	TargetChannel TargetKind = "channel"
	// TargetThread is a thread, the main thread of a text channel included.
	TargetThread TargetKind = "thread"
)

var targetKinds = []TargetKind{TargetSpace, TargetChannel, TargetThread}

// Valid reports whether k is space, channel or thread.
func (k TargetKind) Valid() bool { return slices.Contains(targetKinds, k) }

// PreferenceUnchanged reports whether writing in over the stored row changes nothing: the same
// level and the same muted_until, compared as instants after the cut to whole microseconds (the
// precision of timestamptz); both zero (NULL) is equal. A write without change writes no event and
// keeps the version (brief M3-18 note 4 e; the SQL upsert compares the same way).
//
// Example:
//
//	domain.PreferenceUnchanged(domain.Preference{Level: domain.LevelAll}, domain.Preference{Level: domain.LevelAll}) // true
func PreferenceUnchanged(stored, in Preference) bool {
	return stored.Level == in.Level && TruncateMicro(stored.MutedUntil).Equal(TruncateMicro(in.MutedUntil))
}

// TruncateMicro cuts t to whole microseconds, the precision of a timestamptz column; the zero
// time stays zero.
func TruncateMicro(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.Truncate(time.Microsecond)
}
