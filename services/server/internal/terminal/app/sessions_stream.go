// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The decisions of the two stream endpoints (Design notes 8 and 9 of M2-44). The adapter runs the
// connections; these use cases read and authorize.

// revokedReasons are the reasons of unauthenticated that close the device stream with 4012.
var revokedReasons = map[authorize.Reason]bool{"principal_inactive": true, "session_revoked": true, "device_inactive": true}

// StreamAccess decides the identify of a device stream of the authenticated principal a for the
// session id (Design note 8): authorize terminal.stream with the descriptor of the session (the
// device of the current token, Design note 3 d), whose denials authz records. It returns 0 for a
// device that may stream, else the close code: 4100 for not_found (unknown and foreign alike) and
// for an own closed session (with its close_reason as reason), 4103 for not_self, 4012 for a
// revoked principal, session or device, 4004 for every other denial. Errors: those of the store
// and of authorize.
func (s *Service) StreamAccess(ctx context.Context, a actor.Actor, id ids.ID) (domain.StreamCode, string, error) {
	ses, found, err := s.d.Store.Session(ctx, id)
	if err != nil {
		return 0, "", err
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionTerminalStream, sessionResource(id, ses, found, true))
	if err != nil {
		return 0, "", err
	}
	switch {
	case d.Allowed() && ses.State == domain.SessionClosed:
		return domain.StreamTerminalClosed, string(ses.CloseReason), nil
	case d.Allowed():
		return 0, "", nil
	case d.Outcome == authorize.OutcomeNotFound:
		return domain.StreamTerminalClosed, "", nil
	case d.Outcome == authorize.OutcomeForbidden && d.Reason == "not_self":
		return domain.StreamReplaced, "", nil
	case d.Outcome == authorize.OutcomeUnauthenticated && revokedReasons[d.Reason]:
		return domain.StreamRevoked, "", nil
	}
	return domain.StreamAuthFailed, "", nil
}

// StreamAuthFailed records the denial of a device stream whose identify did not authenticate (no
// identify within terminal.open_timeout, an invalid token, proof, nonce, htu or device_id):
// authorize terminal.stream as the anonymous principal on the session, which authz records as a
// denial, aggregated (AZ-P13). The connection closes with 4004.
func (s *Service) StreamAuthFailed(ctx context.Context, id ids.ID) error {
	ses, found, err := s.d.Store.Session(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.d.Authorizer.Authorize(ctx, actor.Anonymous(), authorize.ActionTerminalStream, sessionResource(id, ses, found, true))
	return err
}

// DataSession checks the session of a data connection of the host service of l (Design note 9):
// true for a session of that host that is not closed; an unknown session, one of another host and
// a closed one are false (the connection closes with 4100). The login itself authorized
// host.connect. Errors: those of the store.
func (s *Service) DataSession(ctx context.Context, l Login, id ids.ID) (bool, error) {
	ses, found, err := s.d.Store.Session(ctx, id)
	if err != nil {
		return false, err
	}
	return found && ses.HostID == l.HostID && ses.State != domain.SessionClosed, nil
}
