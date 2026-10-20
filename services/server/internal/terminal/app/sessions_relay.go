// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The callbacks of the relay (Design note 7 of M2-44): adapters/wsrelay owns sockets, queues and
// timers and calls these for every state change and for the counters; each runs in its own unit
// of work as the system (no actor, 05 rule 1.10), locks the host before the session (03 section
// 6.3) and changes nothing on a session in another state, so a late callback is harmless.

// lockForRelay locks the host and the session id and calls f with the session when it exists.
func (s *Service) lockForRelay(ctx context.Context, id ids.ID, f func(context.Context, domain.Session) error) error {
	return s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		cur, found, err := s.d.Store.Session(ctx, id)
		if err != nil || !found {
			return err
		}
		if _, _, err := s.d.Store.LockHost(ctx, cur.HostID); err != nil {
			return err
		}
		ses, found, err := s.d.Store.LockSession(ctx, id)
		if err != nil || !found {
			return err
		}
		return f(ctx, ses)
	})
}

// RelayActivate turns a started session that paired again from detached to active (detached_at
// cleared, terminal_session.changed); other states stay.
func (s *Service) RelayActivate(ctx context.Context, id ids.ID) error {
	return s.lockForRelay(ctx, id, func(ctx context.Context, ses domain.Session) error {
		if ses.State != domain.SessionDetached {
			return nil
		}
		if err := s.d.Store.ActivateSession(ctx, ses.ID); err != nil {
			return err
		}
		ses.State, ses.DetachedAt, ses.Version = domain.SessionActive, time.Time{}, ses.Version+1
		return s.emitSessionChanged(ctx, ses)
	})
}

// RelayDetach turns an active session detached (detached_at, terminal_session.changed); other
// states stay.
func (s *Service) RelayDetach(ctx context.Context, id ids.ID) error {
	return s.lockForRelay(ctx, id, func(ctx context.Context, ses domain.Session) error {
		return s.detach(ctx, ses, s.d.Clock.Now())
	})
}

// RelayHostLost is RelayDetach after the loss of the host side of a started session: it
// detaches an active session and reports, for a session that is already closed, its end (close
// reason and detail) and true, so the relay closes the device with that end instead of 4102
// (found by the conformance suite of M2-48).
func (s *Service) RelayHostLost(ctx context.Context, id ids.ID) (domain.End, bool, error) {
	var end domain.End
	closed := false
	err := s.lockForRelay(ctx, id, func(ctx context.Context, ses domain.Session) error {
		if ses.State == domain.SessionClosed {
			end, closed = domain.End{Reason: ses.CloseReason, Detail: ses.CloseDetail}, true
			return nil
		}
		return s.detach(ctx, ses, s.d.Clock.Now())
	})
	return end, closed, err
}

// RelayOpenFailed ends a session that is not closed with open_failed and detail through the one
// end path (terminal.terminate with the detail, terminal_session.changed).
func (s *Service) RelayOpenFailed(ctx context.Context, id ids.ID, detail domain.CloseDetail) error {
	return s.lockForRelay(ctx, id, func(ctx context.Context, ses domain.Session) error {
		if ses.State == domain.SessionClosed {
			return nil
		}
		return s.endSession(ctx, ses, domain.End{Reason: domain.CloseOpenFailed, Detail: detail, Source: domain.EndByServer})
	})
}

// RelayCounters adds the counts of a pairing that ended for another reason than the end of its
// session (ruling D-M2-44-5): bytes_in, bytes_out and last_input_at, one update without an event
// and without a new version.
func (s *Service) RelayCounters(ctx context.Context, id ids.ID, c Counts) error {
	if c.In == 0 && c.Out == 0 && c.LastInput.IsZero() {
		return nil
	}
	return s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		return s.d.Store.AddCounters(ctx, id, c.In, c.Out, c.LastInput)
	})
}

// RelayEnd ends the session id at a limit of the relay (idle_timeout, max_duration or
// detach_expired, M2-46 Design note 7) through the one end function with source server and the
// counts the relay had not written yet; a session that is already closed changes nothing.
func (s *Service) RelayEnd(ctx context.Context, id ids.ID, reason domain.CloseReason, p domain.Progress) error {
	return s.EndSession(ctx, SessionSelector{SessionID: id}, domain.End{Reason: reason, Source: domain.EndByServer, Progress: &p})
}

// RelayUnpaired counts bothy.terminal.relay.unpaired once with side (D-W-105).
func (s *Service) RelayUnpaired(ctx context.Context, side domain.Side) {
	s.relayMetrics().RelayUnpaired(ctx, side)
}
