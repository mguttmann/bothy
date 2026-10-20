// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The subscribers of the terminal module (M2-46 Design notes 4 and 5, 02 section 4.3): they end
// sessions in the transaction of a revocation that comes from outside the module, synchronously in
// its unit of work, through the one end function. Each locks only terminal_sessions rows (after
// permission_version of phase 1 and before event_seq of phase 3.2, 03 section 6.3) and never calls
// authorize, because the snapshot cache does not see uncommitted rights (M1-29). All three are
// idempotent through EndSessions, so their order within one unit (user.disable publishes all of
// them) does not matter.

// OnAccessChanged ends the sessions a rights change outside the module made unusable (phase 3.1 of
// the revoking transaction, 04 section 8.2): for every principal of ev, each removed target of kind
// host ends that person's sessions on that host (a revoked or expired grant, a group left or
// expired, a lost owner role, an I6 switch; authz computes the set with TerminalEffective); then,
// when the person is no longer an active owner (People.IsActiveOwner, read in the transaction),
// every remaining session that SR10 no longer allows ends (07 SR10 "on every change of the owner
// status": not the assignee, or a server host). The reasons come from domain.ReasonForLostHost.
// Errors: those of the store, People and EndSessions; any error rolls the revocation back.
func (s *Service) OnAccessChanged(ctx context.Context, ev authorize.AccessChanged) error {
	for _, c := range ev.Changes {
		for _, t := range c.Removed {
			if t.Kind != authorize.AccessHost {
				continue
			}
			if err := s.endLostHost(ctx, ev.Action, t.ID, c.PrincipalID); err != nil {
				return err
			}
		}
		if err := s.endSR10Lost(ctx, ev.Action, c.PrincipalID); err != nil {
			return err
		}
	}
	return nil
}

// endLostHost ends the sessions of userID on hostID with the reason of the lost host.
func (s *Service) endLostHost(ctx context.Context, action authorize.Action, hostID, userID ids.ID) error {
	h, found, err := s.d.Store.Host(ctx, hostID)
	if err != nil || !found {
		return err
	}
	_, err = s.endSessions(ctx, SessionSelector{HostID: hostID, UserID: userID}, domain.ServerEnd(domain.ReasonForLostHost(string(action), h, userID)))
	return err
}

// endSR10Lost ends the open sessions of userID that SR10 no longer allows when the person is no
// longer an active owner (Design note 4 b): a person who never was an owner holds no such session.
func (s *Service) endSR10Lost(ctx context.Context, action authorize.Action, userID ids.ID) error {
	open, err := s.d.Store.LockOpenSessionsOfUsers(ctx, []ids.ID{userID})
	if err != nil || len(open) == 0 {
		return err
	}
	owner, err := s.d.People.IsActiveOwner(ctx, userID)
	if err != nil || owner {
		return err
	}
	for _, ses := range open {
		if err := s.endIfSR10Lost(ctx, action, ses); err != nil {
			return err
		}
	}
	return nil
}

// endIfSR10Lost ends ses of a person who is no longer an active owner when SR10 no longer allows
// it on its host, read fresh in the transaction (domain.SR10Lost).
func (s *Service) endIfSR10Lost(ctx context.Context, action authorize.Action, ses domain.Session) error {
	h, found, err := s.d.Store.Host(ctx, ses.HostID)
	if err != nil || !found || !domain.SR10Lost(false, ses.UserID, h) {
		return err
	}
	_, err = s.endSessions(ctx, SessionSelector{SessionID: ses.ID}, domain.ServerEnd(domain.ReasonForLostHost(string(action), h, ses.UserID)))
	return err
}

// OnDeviceRevoked ends the open sessions whose current device (terminal_sessions.device_id, the
// device of the last opening or attaching) is the revoked device, with user_disabled for a device
// revoked by user.disable and device_revoked otherwise (Design note 5 a; ruling D-M2-46-5).
// Errors: those of EndSessions.
func (s *Service) OnDeviceRevoked(ctx context.Context, ev identity.DeviceRevoked) error {
	_, err := s.endSessions(ctx, SessionSelector{DeviceID: ev.DeviceID}, domain.ServerEnd(domain.ReasonForDeviceRevoke(string(ev.Reason))))
	return err
}

// OnSessionEnded ends the open sessions of the ended login sessions with the exact reason (Design
// note 5 b): logout, user_disabled, device_revoked, else session_revoked. Errors: those of
// EndSessions.
func (s *Service) OnSessionEnded(ctx context.Context, ev identity.SessionEnded) error {
	if len(ev.SessionIDs) == 0 {
		return nil
	}
	_, err := s.endSessions(ctx, SessionSelector{AuthSessionIDs: ev.SessionIDs}, domain.ServerEnd(domain.ReasonForSessionEnd(string(ev.Reason))))
	return err
}
