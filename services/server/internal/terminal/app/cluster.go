// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The reads and decisions behind the signals of M2-47 (Design notes 3, 7, 8 and 9): the hub of
// adapters/wsrelay owns the connections and calls these; every read is bounded by
// database.statement_timeout, and a receiver always reads the state from the database, so a
// signal that arrives late, twice, out of order or forged causes nothing the committed rows do not
// allow.

// HostState is the state of one host for the reconciliation after a lost listener (Design note 9).
type HostState struct {
	ID             ids.ID
	State          domain.State
	DisabledReason string
}

// bounded returns ctx with database.statement_timeout as its deadline (none for zero) and the
// system as principal.
func (s *Service) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = system(context.WithoutCancel(ctx))
	if s.cfg.StatementTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.cfg.StatementTimeout)
}

// HandleSignal implements SignalHandler for the kinds the use cases decide (the hub of
// adapters/wsrelay hands them on): terminal.session_open registers the expectation in the relay of
// this node and, where an eligible control connection of an active host exists, sends
// session.open; terminal.host_close closes the control connections of the host on this node with
// the code of its state. Every other kind is the hub's. Failures of the reads are dropped: the
// deadline of the opening, the next heartbeat or the reconciliation decide instead.
func (s *Service) HandleSignal(ctx context.Context, sig domain.Signal) {
	switch sig.Kind {
	case domain.SignalSessionOpen:
		s.onSessionOpen(ctx, sig)
	case domain.SignalHostClose:
		s.onHostClose(ctx, sig.HostID)
	}
}

// onSessionOpen is the handler of terminal.session_open (Design note 3): a session of the host
// that is not closed gets an expectation whose deadline counts from the receipt of the signal on
// this node; the device of the row is the expected device; whether the PTY started comes from the
// row. Only an eligible control connection of an active host sends session.open (ControlLinks
// answers ErrHostNotConnected or ErrHostNotReady elsewhere; a connection in probation defers it).
func (s *Service) onSessionOpen(ctx context.Context, sig domain.Signal) {
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	wdi := s.notes.take(sig.SessionID)
	ses, found, err := s.d.Store.Session(rctx, sig.SessionID)
	if err != nil || !found || ses.State == domain.SessionClosed || ses.HostID != sig.HostID {
		return
	}
	h, found, err := s.d.Store.Host(rctx, sig.HostID)
	if err != nil || !found {
		return
	}
	started := ses.State != domain.SessionOpening
	if started || domain.ProfileKind(h.Profiles, ses.Profile) != domain.ProfileKindClaude {
		wdi = nil
	}
	deadline := s.d.Clock.Now().Add(s.cfg.OpenTimeout)
	confirm := h.RequireLocalConfirm != nil && *h.RequireLocalConfirm
	s.relayControl().Expect(Expectation{SessionID: ses.ID, HostID: h.ID, DeviceID: ses.DeviceID, Deadline: deadline, Started: started,
		LocalConfirm: confirm, WorkdirIndex: wdi})
	if h.State != domain.StateActive {
		return
	}
	// An error means no eligible link here; the node of the link sends it, or the deadline ends the
	// opening.
	_ = s.control().OpenSession(ctx, h.ID, SessionOpen{SessionID: ses.ID, DataPath: DataPath(ses.ID), Profile: ses.Profile, ExpiresAt: deadline})
}

// onHostClose closes the control connections of hostID on this node with the code of the state
// read now (domain.HostCloseCode); a host that may stay connected keeps them, so a stale or forged
// signal closes nothing.
func (s *Service) onHostClose(ctx context.Context, hostID ids.ID) {
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	h, found, err := s.d.Store.Host(rctx, hostID)
	if err != nil || !found {
		return
	}
	if code := domain.HostCloseCode(h.State, h.DisabledReason); code != 0 {
		s.control().CloseHost(hostID, code)
	}
}

// HostCloseCodes returns, for the hosts of list that may no longer hold a connection, the close
// code of their state (the reconciliation after a lost listener, Design note 9). Errors: those of
// the store.
func (s *Service) HostCloseCodes(ctx context.Context, list []ids.ID) (map[ids.ID]domain.CloseCode, error) {
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	states, err := s.d.Store.HostStatesByIDs(rctx, list)
	if err != nil {
		return nil, err
	}
	out := map[ids.ID]domain.CloseCode{}
	for _, h := range states {
		if code := domain.HostCloseCode(h.State, h.DisabledReason); code != 0 {
			out[h.ID] = code
		}
	}
	return out, nil
}

// SessionOpenValid reports whether a session.open that waited for the end of a probation may
// still go out (Design note 7 d): the session of the host is not closed and the host is active.
// Errors: those of the store.
func (s *Service) SessionOpenValid(ctx context.Context, hostID, sessionID ids.ID) (bool, error) {
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	ses, found, err := s.d.Store.Session(rctx, sessionID)
	if err != nil || !found || ses.HostID != hostID || ses.State == domain.SessionClosed {
		return false, err
	}
	h, found, err := s.d.Store.Host(rctx, hostID)
	if err != nil || !found {
		return false, err
	}
	return h.State == domain.StateActive, nil
}

// RelaySessionState returns the state of the session id read now (the deadline of an
// expectation decides by it, not by a local started mark, Design note 3); found is false for an
// unknown id. Errors: those of the store.
func (s *Service) RelaySessionState(ctx context.Context, id ids.ID) (string, bool, error) {
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	ses, found, err := s.d.Store.Session(rctx, id)
	if err != nil || !found {
		return "", false, err
	}
	return ses.State, true, nil
}

// RelayOpeningTimedOut ends a session that is still in opening with open_failed and
// close_detail open_timeout through the one end path (Design note 8 c): a compare-and-swap under
// the row lock, so a second node that times out the same session writes nothing (one audit entry,
// one event).
func (s *Service) RelayOpeningTimedOut(ctx context.Context, id ids.ID) error {
	return s.lockForRelay(ctx, id, func(ctx context.Context, ses domain.Session) error {
		if ses.State != domain.SessionOpening {
			return nil
		}
		return s.endSession(ctx, ses, domain.HostSessionEnd{Reason: domain.CloseOpenFailed, Detail: domain.DetailOpenTimeout}, true)
	})
}
