// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// AuthSessions is the outgoing port of terminal.sweep to identity (identity.Auth.EndedSessions,
// bound in internal/wire): which of the login sessions ended by revocation or by expires_at.
type AuthSessions interface {
	// EndedSessions returns those of sessionIDs that are revoked or whose expires_at is not after
	// at, ascending.
	EndedSessions(ctx context.Context, sessionIDs []ids.ID, at time.Time) ([]ids.ID, error)
}

// SweepResult is what one run of terminal.sweep ended.
type SweepResult struct {
	// Ended counts the sessions this run ended.
	Ended int
}

// windows returns the limits of 24 section 6.3 that the sweep and the relay check.
func (s *Service) windows() domain.Windows {
	return domain.Windows{Idle: s.cfg.IdleTimeout, IdleGrace: s.cfg.IdleGrace, MaxDuration: s.cfg.MaxSessionDuration,
		DetachGrace: s.cfg.DetachGrace, OpenTimeout: s.cfg.OpenTimeout}
}

// Windows returns the limits of a session (terminal.idle_timeout, terminal.idle_grace,
// terminal.max_session_duration, terminal.detach_grace, terminal.open_timeout) for the relay of
// adapters/wsrelay, so relay and sweep check the same values.
func (s *Service) Windows() domain.Windows { return s.windows() }

// sessionTimes returns the times of ses for the deadlines: the time of the last grant is the UUIDv7
// timestamp of its token jti, which every grant renews (Design note 8 a).
func sessionTimes(ses domain.Session) domain.SessionTimes {
	return domain.SessionTimes{State: ses.State, OpenedAt: ses.OpenedAt, GrantedAt: ses.TokenJTI.Time(), StartedAt: ses.StartedAt,
		DetachedAt: ses.DetachedAt, LastInputAt: ses.LastInputAt}
}

// Sweep is the job terminal.sweep (worker, every terminal.sweep_interval; 02 section 12.3, M2-46
// Design note 8): it repeats the limits of the relay in the database for the sessions of a node
// that failed and ends the sessions whose login session ended. It reads every session that is not
// closed with the state of its host, asks identity which login sessions ended, and ends each
// session for which domain.SweepVerdict matches, one unit of work per session (host row, then the
// session row, the verdict decided again on the locked rows), through the one end function with
// source server. A session another path ended first is skipped. No actor (system). Errors: a
// window that is not positive or no identity port (fail closed), and those of the store and the
// units; the sessions ended before the error stay ended.
func (s *Service) Sweep(ctx context.Context) (SweepResult, error) {
	w := s.windows()
	if w.Idle <= 0 || w.IdleGrace <= 0 || w.MaxDuration <= 0 || w.DetachGrace <= 0 || w.OpenTimeout <= 0 || s.d.AuthSessions == nil {
		return SweepResult{}, errs.New(errs.CodeInternalError, "terminal: terminal.sweep needs its windows and the login sessions of identity")
	}
	ctx = system(ctx)
	list, err := s.d.Store.SweepCandidates(ctx)
	if err != nil || len(list) == 0 {
		return SweepResult{}, err
	}
	now := s.d.Clock.Now()
	auth := make([]ids.ID, 0, len(list))
	for _, c := range list {
		auth = append(auth, c.AuthSessionID)
	}
	endedList, err := s.d.AuthSessions.EndedSessions(ctx, auth, now)
	if err != nil {
		return SweepResult{}, err
	}
	ended := make(map[ids.ID]bool, len(endedList))
	for _, id := range endedList {
		ended[id] = true
	}
	var res SweepResult
	for _, c := range list {
		done, err := s.sweepOne(ctx, c, ended[c.AuthSessionID], now, w)
		if err != nil {
			return res, err
		}
		if done {
			res.Ended++
		}
	}
	return res, nil
}

// sweepOne decides and ends one candidate in its own unit of work; it reports whether it ended it.
func (s *Service) sweepOne(ctx context.Context, c SweepCandidate, authEnded bool, now time.Time, w domain.Windows) (bool, error) {
	var done bool
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		done = false
		cur, found, err := s.d.Store.Session(ctx, c.SessionID)
		if err != nil || !found || cur.State == domain.SessionClosed {
			return err
		}
		h, found, err := s.d.Store.LockHost(ctx, cur.HostID)
		if err != nil || !found {
			return err
		}
		ses, found, err := s.d.Store.LockSession(ctx, c.SessionID)
		if err != nil || !found || ses.State == domain.SessionClosed {
			return err
		}
		confirm := h.RequireLocalConfirm != nil && *h.RequireLocalConfirm
		reason, detail, ok := domain.SweepVerdict(string(h.State), confirm, authEnded, sessionTimes(ses), now, w)
		if !ok {
			return nil
		}
		done = true
		return s.finish(ctx, ses, domain.End{Reason: reason, Detail: detail, Source: domain.EndByServer}, "")
	})
	return done, err
}
