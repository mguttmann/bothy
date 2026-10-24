// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// CleanupFacts are the stored times of a file and its upload session that decide its age for the
// cleanup of 12 section 12 (docs/DECISIONS.md D-M3-08-9). A zero time is a NULL column; a file
// without a session has an empty SessionState.
type CleanupFacts struct {
	// CreatedAt is files.created_at; ScannedAt is files.scanned_at.
	CreatedAt, ScannedAt time.Time
	// SessionState, CompletedAt and ExpiresAt are the session's state, completed_at and
	// expires_at.
	SessionState           SessionState
	CompletedAt, ExpiresAt time.Time
}

// LastChange is the latest stored time of f: created_at, completed_at, scanned_at, and expires_at
// when the session expired. An aborted session stores no time of its own, so its file counts from
// the latest of the others.
//
// Example:
//
//	t := domain.LastChange(domain.CleanupFacts{CreatedAt: c, CompletedAt: c.Add(time.Hour)}) // c + 1h
func LastChange(f CleanupFacts) time.Time {
	last := f.CreatedAt
	for _, t := range []time.Time{f.CompletedAt, f.ScannedAt} {
		if t.After(last) {
			last = t
		}
	}
	if f.SessionState == SessionExpired && f.ExpiresAt.After(last) {
		last = f.ExpiresAt
	}
	return last
}

// UnattachedExpired reports that a never attached file is older than ttl (file.unattached_ttl):
// now is at least ttl after LastChange.
//
// Example:
//
//	domain.UnattachedExpired(f, domain.LastChange(f).Add(ttl), ttl) // true
func UnattachedExpired(f CleanupFacts, now time.Time, ttl time.Duration) bool {
	return !now.Before(LastChange(f).Add(ttl))
}

// FailedRowExpired reports that the row of a failed or blocked file is older than ttl
// (file.failed_row_ttl), by the same age as UnattachedExpired.
//
// Example:
//
//	domain.FailedRowExpired(f, domain.LastChange(f).Add(ttl-time.Nanosecond), ttl) // false
func FailedRowExpired(f CleanupFacts, now time.Time, ttl time.Duration) bool {
	return !now.Before(LastChange(f).Add(ttl))
}

// SessionPastExpiry reports an open session whose expires_at is reached (12 3.6): the same edge at
// which the upload answers 410 gone. A completed, aborted or expired session never expires again.
//
// Example:
//
//	domain.SessionPastExpiry(domain.SessionOpen, exp, exp) // true
func SessionPastExpiry(state SessionState, expiresAt, now time.Time) bool {
	return state == SessionOpen && !now.Before(expiresAt)
}
