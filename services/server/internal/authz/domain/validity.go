// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

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

// AssumeValid names grants and memberships that a snapshot treats as not expired. The expiry
// sweep (M1-30) uses it for both views of a batch: at sweep time the expired rows are already
// invalid for authorize, so without it the view before would equal the view after and the
// difference would be empty. Unreported treats every row whose expiry is not reported yet
// (ExpiryReportedAt zero) as not expired; the sweep sets it for the view before and the view
// after, so exactly the rows a batch marks as reported fall out between the two, and access that
// needs two expired rows at once (a membership and a grant of its group) is reported by the batch
// that marks the first of them. It never makes a revoked grant, a removed membership or a
// membership of a deleted group valid. The zero value assumes nothing.
type AssumeValid struct {
	// GrantIDs are the grants treated as not expired.
	GrantIDs []ids.ID
	// MembershipIDs are the memberships treated as not expired.
	MembershipIDs []ids.ID
	// Unreported treats every grant and membership without ExpiryReportedAt as not expired.
	Unreported bool
}

// Grant returns g with its expiry cleared when a assumes it valid, else g unchanged.
func (a AssumeValid) Grant(g Grant) Grant {
	if slices.Contains(a.GrantIDs, g.ID) || (a.Unreported && g.ExpiryReportedAt.IsZero()) {
		g.ExpiresAt = time.Time{}
	}
	return g
}

// Membership returns m with its expiry cleared when a assumes it valid, else m unchanged.
func (a AssumeValid) Membership(m Membership) Membership {
	if slices.Contains(a.MembershipIDs, m.ID) || (a.Unreported && m.ExpiryReportedAt.IsZero()) {
		m.ExpiresAt = time.Time{}
	}
	return m
}

// Empty reports that a assumes nothing.
func (a AssumeValid) Empty() bool {
	return len(a.GrantIDs) == 0 && len(a.MembershipIDs) == 0 && !a.Unreported
}
