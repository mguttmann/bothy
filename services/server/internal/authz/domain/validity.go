// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// The validity rules of 07 section 2.5 at one time t. t is the server clock read once per
// authorize call, so every check of one call sees the same time. At expires_at = t a grant or
// membership is already invalid.
//
// Subject activity and the installation state of an app are not judged here: authorize checks
// the principal's own status before it reads any grant (07 SR7), and a grant to another subject
// never acts for the caller.

// InEffect reports whether g is valid at t: not revoked, and no expiry or an expiry after t.
// A set RevokedAt counts whatever its value, because revocation takes effect when written.
func (g Grant) InEffect(t time.Time) bool {
	return g.RevokedAt.IsZero() && (g.ExpiresAt.IsZero() || g.ExpiresAt.After(t))
}

// InEffect reports whether m is valid at t: not removed, its group not deleted, and no expiry
// or an expiry after t. group must be the row of m.GroupID.
func (m Membership) InEffect(group Group, t time.Time) bool {
	return m.RemovedAt.IsZero() && group.DeletedAt.IsZero() && (m.ExpiresAt.IsZero() || m.ExpiresAt.After(t))
}

// NextExpiry returns the smallest non-zero value of expiries strictly after t, or the zero time
// when there is none. The rights snapshot uses it as the time its validity ends (07 section 7.2
// step 7).
//
// Example: NextExpiry(t, t, t.Add(time.Hour), time.Time{}) is t.Add(time.Hour).
func NextExpiry(t time.Time, expiries ...time.Time) time.Time {
	var next time.Time
	for _, e := range expiries {
		if !e.IsZero() && e.After(t) && (next.IsZero() || e.Before(next)) {
			next = e
		}
	}
	return next
}
