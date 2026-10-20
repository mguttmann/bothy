// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// sessionResource is the descriptor of a terminal_session for authorize (Design note 3 a of
// M2-44). An unknown session gets a host id no host has, so authz answers not_found exactly like
// for an invisible one, after the gate of HumanSession (AZ-P4).
func sessionResource(id ids.ID, ses domain.Session, found, connected bool) authorize.Resource {
	if !found {
		return authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: id, HostID: ids.New(), Terminal: &authorize.TerminalFacts{}}
	}
	return authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: ses.ID, HostID: ses.HostID, OwnerID: ses.UserID,
		Terminal: &authorize.TerminalFacts{SessionState: ses.State, SessionDeviceID: ses.DeviceID, SessionAuthSessionID: ses.AuthSessionID,
			DetachedAt: ses.DetachedAt, HostConnected: connected, ProfileKnown: true}}
}

// errNoSessionUser is the answer for a session whose session user is no longer the person's valid
// assignment (Design note 6 c of M2-44).
func errNoSessionUser() error {
	return errs.New(errs.CodeTerminalDenied, "").WithReason("no_session_user")
}

// attachSession implements Sessions.Attach (Design note 6 of M2-44).
func (s *Service) attachSession(ctx context.Context, p actor.Actor, cmd terminal.AttachSession) (terminal.Grant, error) {
	ctx, err := withCaller(ctx, p)
	if err != nil {
		return terminal.Grant{}, err
	}
	if err := s.sessionsReady(); err != nil {
		return terminal.Grant{}, err
	}
	ses, found, err := s.d.Store.Session(ctx, cmd.SessionID)
	if err != nil {
		return terminal.Grant{}, err
	}
	connected := found && s.connected(ctx, ses.HostID)
	d, err := s.d.Authorizer.Authorize(ctx, p, authorize.ActionTerminalAttach, sessionResource(cmd.SessionID, ses, found, connected))
	if err != nil {
		return terminal.Grant{}, err
	}
	if err := d.Err(); err != nil {
		return terminal.Grant{}, err
	}
	h, hfound, err := s.d.Store.Host(ctx, ses.HostID)
	if err != nil {
		return terminal.Grant{}, err
	}
	if !found || !hfound {
		return terminal.Grant{}, errs.New(errs.CodeInternalError, "terminal: an attach was allowed without its session or host")
	}
	owner, err := s.d.People.IsActiveOwner(ctx, p.ID)
	if err != nil {
		return terminal.Grant{}, err
	}
	var g terminal.Grant
	s.noteAttach(h, ses)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		g, err = s.attachUnit(ctx, p, h, cmd, d.QuickReattach, owner)
		return err
	})
	if err != nil {
		s.notes.take(cmd.SessionID)
		return terminal.Grant{}, s.stepUpDenied(ctx, err, authorize.ActionTerminalAttach, authorize.ResourceTerminalSession, cmd.SessionID)
	}
	if err := s.recheck(ctx, p, authorize.ActionTerminalAttach, cmd.SessionID, func(domain.Host) (authorize.Resource, error) {
		cur, found, err := s.d.Store.Session(ctx, cmd.SessionID)
		if err != nil {
			return authorize.Resource{}, err
		}
		return sessionResource(cmd.SessionID, cur, found, true), nil
	}, h.ID); err != nil {
		return terminal.Grant{}, err
	}
	s.committed(ctx, sessionOpenSignal(h.ID, cmd.SessionID))
	return g, nil
}

// noteAttach remembers the working directory index the relay slot of this node keeps for an
// attach to an opening of a claude profile, so the handler of terminal.session_open keeps it in
// the new expectation (ruling D-M2-44-4).
func (s *Service) noteAttach(h domain.Host, ses domain.Session) {
	if ses.State != domain.SessionOpening || domain.ProfileKind(h.Profiles, ses.Profile) != domain.ProfileKindClaude {
		return
	}
	if wdi, ok := s.relayControl().OpeningWorkdir(ses.ID); ok {
		s.notes.put(ses.ID, wdi)
	}
}

// attachUnit is the unit of work of an attach (Design note 6 b to e): the step-up (unless quick
// reattach), the locked row, the session user, the token and the writes; it stages
// terminal.session_open (M2-47). It returns the grant.
func (s *Service) attachUnit(ctx context.Context, p actor.Actor, h domain.Host, cmd terminal.AttachSession, quick, owner bool) (terminal.Grant, error) {
	if !quick {
		if err := s.consume(ctx, p); err != nil {
			return terminal.Grant{}, err
		}
	}
	ses, att, u, err := s.lockAttach(ctx, p, cmd.SessionID, quick)
	if err != nil {
		return terminal.Grant{}, err
	}
	kind := domain.ProfileKind(h.Profiles, ses.Profile)
	wdi, err := s.openingWorkdir(ses.ID, att, kind)
	if err != nil {
		return terminal.Grant{}, err
	}
	ses.DeviceID, ses.AuthSessionID, ses.TokenJTI = p.DeviceID, p.SessionID, ids.New()
	if err := s.d.Store.AttachSession(ctx, ses.ID, ses.DeviceID, ses.AuthSessionID, ses.TokenJTI); err != nil {
		return terminal.Grant{}, err
	}
	ses.Version++
	tg := domain.TokenGrant{HostID: ses.HostID, UserID: ses.UserID, SessionID: ses.ID, DeviceID: ses.DeviceID, Profile: ses.Profile,
		ProfileKind: kind, CEK: cmd.CEK, Owner: owner, OSUser: u.OSUser, WorkdirIndex: wdi, Attach: att == domain.AttAttach}
	g, err := s.grant(ctx, h, ses, tg, s.d.Clock.Now())
	if err != nil {
		return terminal.Grant{}, err
	}
	if err := s.recordGrant(ctx, authorize.ActionTerminalAttach, h, ses, cmd.PinChanged, owner); err != nil {
		return terminal.Grant{}, err
	}
	if err := s.emitSessionChangedBy(ctx, ses, p.ID); err != nil {
		return terminal.Grant{}, err
	}
	return g, s.stage(ctx, sessionOpenSignal(h.ID, ses.ID))
}

// lockAttach locks the session of an attach and checks it (Design note 6 b and c): not closed
// (409 invalid_state_transition), for a quick reattach still detached with the device and login
// session of the request (else the step-up is required), and its session user still the person's
// valid assignment (else 403 terminal_denied no_session_user). It returns the session, the claim
// att and the assignment.
func (s *Service) lockAttach(ctx context.Context, p actor.Actor, id ids.ID, quick bool) (domain.Session, string, *domain.OwnSessionUser, error) {
	ses, found, err := s.d.Store.LockSession(ctx, id)
	if err != nil {
		return domain.Session{}, "", nil, err
	}
	if !found {
		return domain.Session{}, "", nil, invisible()
	}
	att, err := domain.AttachKind(ses.State)
	if err != nil {
		return domain.Session{}, "", nil, err
	}
	if quick && !domain.QuickReattachHolds(ses, p.DeviceID, p.SessionID) {
		return domain.Session{}, "", nil, errStepUpConsumed
	}
	u, err := s.ownAssignment(ctx, p.ID, ses.HostID)
	if err != nil {
		return domain.Session{}, "", nil, err
	}
	if u == nil || u.ID != ses.HostSessionUserID {
		return domain.Session{}, "", nil, errNoSessionUser()
	}
	return ses, att, u, nil
}

// openingWorkdir returns the working directory index of an attach that opens a claude session
// (att open): the index the relay slot of this node keeps. Without it the opening cannot be
// resumed here (409 invalid_state_transition, ruling D-M2-44-4); any other attach has none.
func (s *Service) openingWorkdir(id ids.ID, att, kind string) (*int, error) {
	if att != domain.AttOpen || kind != domain.ProfileKindClaude {
		return nil, nil
	}
	wdi, ok := s.relayControl().OpeningWorkdir(id)
	if !ok {
		return nil, errs.New(errs.CodeInvalidStateTransition, "")
	}
	return wdi, nil
}
