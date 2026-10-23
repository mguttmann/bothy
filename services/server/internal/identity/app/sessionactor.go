// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SessionActor implements identity.Auth (M3-05 design note 6). It reads the session per call
// with the same query as the Authenticator, so a revocation takes effect on the next download in
// every process (04 section 8.5). The key counts as active only when the device's active dpop key
// is the key of the session's active refresh family, the binding a DPoP request proves with
// cnf.jkt; KeyThumbprint is the dpop_jkt of that active family (M3-22). The flags follow
// Authenticator.describe and person, so authorize refuses and audits an inactive session with its
// reason.
func (s *Sessions) SessionActor(ctx context.Context, userID, sessionID ids.ID) (actor.Actor, error) {
	out := actor.Actor{ID: userID, Kind: actor.KindUser, SessionID: sessionID, DeviceState: actor.DeviceUnknown, CredentialInactive: true}
	st, found, err := s.d.Store.AuthState(ctx, sessionID)
	if err != nil {
		return actor.Actor{}, err
	}
	if !found || st.Session.UserID != userID {
		return out, nil
	}
	sess, now := st.Session, s.d.Clock.Now()
	out.DeviceID, out.SessionActive = sess.DeviceID, sess.Active(now)
	out.AuthMethod, out.ClientComponent = string(sess.AuthMethod), string(sess.Component)
	out.DeviceState = string(st.DeviceState)
	if st.DeviceState == identity.DeviceStatePending && !now.Before(st.DeviceCreatedAt.Add(s.cfg.DevicePendingTTL)) {
		out.DeviceState = actor.DeviceRevoked
	}
	f, found, err := s.d.Store.ActiveFamily(ctx, sessionID)
	if err != nil {
		return actor.Actor{}, err
	}
	if found {
		// The key of the session's family is the key a DPoP request of this session proves (cnf.jkt);
		// the preview fetch checks its proof against it (M3-22 design note 8 d).
		out.KeyThumbprint = f.DPoPJKT
	}
	out.DeviceKeyActive = found && st.DeviceKeyJKT != "" && st.DeviceKeyJKT == f.DPoPJKT && !s.inactive(st, now)
	out.CredentialInactive = !out.SessionActive || !out.DeviceKeyActive ||
		out.DeviceState != actor.DeviceActive && out.DeviceState != actor.DevicePending
	return out, nil
}

// inactive reports an active device without contact for auth.device_inactive_revoke_after, as
// Authenticator.inactive does, so a ticket of an inactive device is refused like a request of it
// before identity.auth_cleanup revokes the device (M1-45). Zero switches the check off.
func (s *Sessions) inactive(st AuthState, now time.Time) bool {
	if s.cfg.DeviceInactiveRevokeAfter <= 0 || st.DeviceState != identity.DeviceStateActive {
		return false
	}
	return inactiveSince(st.DeviceLastSeenAt, st.DeviceApprovedAt, time.Time{}, now.Add(-s.cfg.DeviceInactiveRevokeAfter))
}
