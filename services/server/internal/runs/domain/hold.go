// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// HoldUntil is the release time of a held agent.invocation (04 15.3 point 1, 12 10, R-FI-09): the
// zero time when no attachment of the triggering message is pending (not held), else now plus
// holdMax, the value of file.invocation_hold_max. The acceptance takes now from one clock reading
// and stores the result in runs.invocation_held_until and as the scheduled time of the job
// runs.invocation_hold_timeout, so a later change of the key does not move held runs (D-M3-07).
//
// Example:
//
//	at := domain.HoldUntil(clk.Now(), pending, cfg.InvocationHoldMax) // zero: invoke at once
func HoldUntil(now time.Time, pending bool, holdMax time.Duration) time.Time {
	if !pending {
		return time.Time{}
	}
	return now.Add(holdMax)
}

// ReleaseCause is why ReleaseInvocation runs for a held run.
type ReleaseCause int

// The causes of a release.
const (
	// ReleaseFinal: an attachment of the triggering message became final (files.FileFinalized);
	// the run is released only when no other attachment is still pending.
	ReleaseFinal ReleaseCause = iota + 1
	// ReleaseTimeout: file.invocation_hold_max passed (the job runs.invocation_hold_timeout); the
	// run is released with the attachments in their current state.
	ReleaseTimeout
)

// String returns final or timeout; "" for an unknown cause.
func (c ReleaseCause) String() string {
	switch c {
	case ReleaseFinal:
		return "final"
	case ReleaseTimeout:
		return "timeout"
	}
	return ""
}

// ReleaseVerdict reports whether a held agent.invocation is written now: only for a run in
// queued whose heldUntil (runs.invocation_held_until) is set, and for ReleaseFinal only when
// nothing is pending. A run that ended while held keeps its column and is never released
// (D-M3-07); a released run has the column NULL, so a second release finds nothing to do.
func ReleaseVerdict(state State, heldUntil time.Time, cause ReleaseCause, pending bool) bool {
	if state != StateQueued || heldUntil.IsZero() {
		return false
	}
	switch cause {
	case ReleaseFinal:
		return !pending
	case ReleaseTimeout:
		return true
	}
	return false
}
