// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// Windows are the time limits of a terminal session, each a configuration key: Idle is
// terminal.idle_timeout, IdleGrace terminal.idle_grace, MaxDuration terminal.max_session_duration,
// DetachGrace terminal.detach_grace and OpenTimeout terminal.open_timeout.
type Windows struct {
	Idle, IdleGrace, MaxDuration, DetachGrace, OpenTimeout time.Duration
}

// SessionTimes are the instants of a terminal session the deadlines are measured from; a zero time
// is a value not set. State is terminal_sessions.state. GrantedAt is the time of the last grant,
// not opened_at, because an attach to an opening session restarts the opening while opened_at
// never changes (Design note 8 a of M2-46).
type SessionTimes struct {
	State                                                   string
	OpenedAt, GrantedAt, StartedAt, DetachedAt, LastInputAt time.Time
}

// latest returns the latest of ts; zero times never win over a set one.
func latest(ts ...time.Time) time.Time {
	var out time.Time
	for _, t := range ts {
		if t.After(out) {
			out = t
		}
	}
	return out
}

// IdleReference returns the instant idleness is measured from (Design note 7 a of M2-46): the
// latest of the last device input this process saw, the stored last_input_at, started_at and
// opened_at. Only device input counts, because the server cannot see frame types (24 section 6.3).
//
// Parameters: s are the session times, lastDeviceInput the last binary message from the device
// after identify seen by this process (zero when none).
// Returns the reference instant.
func IdleReference(s SessionTimes, lastDeviceInput time.Time) time.Time {
	return latest(lastDeviceInput, s.LastInputAt, s.StartedAt, s.OpenedAt)
}

// Reached reports whether the deadline at is reached at now: the instant itself ends, one tick
// before keeps (Design note 7 c of M2-46).
//
// Parameters: at is the deadline, now the current time.
// Returns true when now is not before at.
func Reached(at, now time.Time) bool {
	return !now.Before(at)
}

// RelayDeadline returns the next deadline a relay holding a side of the session must act on
// (Design note 7 of M2-46): the earliest of the maximum duration from opened_at, which a reattach
// never moves; in state detached with a detached_at, the detach grace from it; and the idle limit
// from IdleReference plus the idle grace, so a lagging stored last_input_at never ends a session
// early. On a tie max_duration wins over detach_expired and detach_expired over idle_timeout, the
// order of SweepVerdict, so relay and sweep name the same reason. The caller ends the session when
// Reached(at, now) holds.
//
// Parameters: s are the session times, lastDeviceInput the last device input this process saw
// (zero when none), w the windows.
// Returns the deadline and its close reason.
func RelayDeadline(s SessionTimes, lastDeviceInput time.Time, w Windows) (at time.Time, reason CloseReason) {
	at, reason = s.OpenedAt.Add(w.MaxDuration), CloseMaxDuration
	if s.State == SessionDetached && !s.DetachedAt.IsZero() {
		if d := s.DetachedAt.Add(w.DetachGrace); d.Before(at) {
			at, reason = d, CloseDetachExpired
		}
	}
	if d := IdleReference(s, lastDeviceInput).Add(w.Idle + w.IdleGrace); d.Before(at) {
		at, reason = d, CloseIdleTimeout
	}
	return at, reason
}

// SweepVerdict decides whether terminal.sweep ends an open session (Design note 8 a of M2-46). It
// returns the first match in this order, security reasons before limits: the host left active
// (HostStateEnd); the login session ended (session_revoked); an opening past the last grant plus
// the open timeout (open_failed with open_timeout), skipped on a host that reports a local
// confirmation, because there the pairing meets the open deadline and the host service bounds
// start and consent; the maximum duration; the detach grace; the idle limit from the stored
// last_input_at. A zero GrantedAt falls back to opened_at, so a session without a recorded grant
// still meets its open deadline.
//
// Parameters: hostState is hosts.state, hostLocalConfirm hosts.reported_require_local_confirm,
// authSessionEnded whether the login session is revoked or expired, s the stored session times,
// now the current time, w the windows.
// Returns the close reason, its detail and true for an end; "", "" and false for a session that
// stays open or is already closed.
func SweepVerdict(hostState string, hostLocalConfirm, authSessionEnded bool, s SessionTimes, now time.Time,
	w Windows) (CloseReason, CloseDetail, bool) {
	if s.State == SessionClosed {
		return "", "", false
	}
	if r, ok := HostStateEnd(hostState); ok {
		return r, "", true
	}
	if authSessionEnded {
		return CloseSessionRevoked, "", true
	}
	if s.State == SessionOpening && !hostLocalConfirm {
		granted := s.GrantedAt
		if granted.IsZero() {
			granted = s.OpenedAt
		}
		if Reached(granted.Add(w.OpenTimeout), now) {
			return CloseOpenFailed, DetailOpenTimeout, true
		}
	}
	if Reached(s.OpenedAt.Add(w.MaxDuration), now) {
		return CloseMaxDuration, "", true
	}
	if s.State == SessionDetached && !s.DetachedAt.IsZero() && Reached(s.DetachedAt.Add(w.DetachGrace), now) {
		return CloseDetachExpired, "", true
	}
	if Reached(IdleReference(s, time.Time{}).Add(w.Idle+w.IdleGrace), now) {
		return CloseIdleTimeout, "", true
	}
	return "", "", false
}

// PersistDue reports whether a relay writes the progress of a held session now (Design note 7 d
// of M2-46): at most once per every (terminal.idle_grace), so the stored last_input_at lags the
// true one by less than the idle grace, and at once when nothing was written yet.
//
// Parameters: lastPersisted is the time of the last write (zero when none), now the current time,
// every the interval.
// Returns true when a write is due.
func PersistDue(lastPersisted, now time.Time, every time.Duration) bool {
	return lastPersisted.IsZero() || now.Sub(lastPersisted) >= every
}
