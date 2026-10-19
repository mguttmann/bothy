// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// Lockout is the failure state of one local login name (docs/spec/03-data-model.md
// local_credentials, 08 section 13): the failures of the running window, its start and the end of
// a lock. Zero times stand for NULL; FailedCount is 0 exactly when WindowStartedAt is zero (the
// check of the table).
type Lockout struct {
	FailedCount     int
	WindowStartedAt time.Time
	LockedUntil     time.Time
}

// LockoutPolicy are auth.login_fail_limit, auth.login_fail_window and auth.login_lock_duration.
type LockoutPolicy struct {
	Limit        int
	Window       time.Duration
	LockDuration time.Duration
}

// Locked reports a lock of the password path at now: locked_until > now (R-ID-10, only the
// password path).
func (l Lockout) Locked(now time.Time) bool {
	return !l.LockedUntil.IsZero() && now.Before(l.LockedUntil)
}

// RecordFailure returns the state after one failed password login at now (brief M1-39
// "LoginWithPassword" step 6). Without a window, or at or after its end, the failure starts a new
// window; otherwise it counts in the running one. Reaching p.Limit locks the password path for
// p.LockDuration and clears the window; locked reports that this failure set the lock. The caller
// only counts a failure while the account is not locked.
func RecordFailure(l Lockout, p LockoutPolicy, now time.Time) (next Lockout, locked bool) {
	if l.WindowStartedAt.IsZero() || !now.Before(l.WindowStartedAt.Add(p.Window)) {
		l.WindowStartedAt, l.FailedCount = now, 1
	} else {
		l.FailedCount++
	}
	if l.FailedCount >= p.Limit {
		return Lockout{LockedUntil: now.Add(p.LockDuration)}, true
	}
	return l, false
}
