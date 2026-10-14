// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// LifecycleDeps are the collaborators of the revocation use cases (M1-45). The use cases of
// People, Sessions and Tokens bring the person, session and token steps, so every revocation
// writes rows, events and counts the same way as logout and token.revoke.
type LifecycleDeps struct {
	// Store locks and revokes devices, keys and sessions and disables principals.
	Store LifecycleStore
	// People reads and locks the person and appends user.updated.
	People *People
	// Sessions revokes sessions and families and appends session.revoked.
	Sessions *Sessions
	// Tokens revokes the personal API tokens of a disabled person.
	Tokens *Tokens
	// Clock is the time of every revocation; nil means clock.System().
	Clock clock.Clock
}

// Lifecycle implements identity.Lifecycle and the inactivity sweep of identity.auth_cleanup
// (brief M1-45). It is safe for concurrent use.
type Lifecycle struct {
	cfg Config
	d   LifecycleDeps
}

// Compile-time check: Lifecycle is the revocation port of identity.
var _ identity.Lifecycle = (*Lifecycle)(nil)

// NewLifecycle returns the revocation use cases; auth.device_inactive_revoke_after of cfg must
// be above 0. Errors: a missing collaborator or a duration that is not positive.
func NewLifecycle(cfg Config, d LifecycleDeps) (*Lifecycle, error) {
	if d.Store == nil || d.People == nil || d.Sessions == nil || d.Tokens == nil {
		return nil, errors.New("identity: NewLifecycle needs the store and the people, session and token use cases")
	}
	if cfg.DeviceInactiveRevokeAfter <= 0 {
		return nil, errors.New("identity: auth.device_inactive_revoke_after must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Lifecycle{cfg: cfg, d: d}, nil
}

// Audit attributes of the revocations (counts and codes, never token values; SEC-AUD-5).
// attrRevokeReason and resourceDevice are shared with the approval of M1-38 (approval.go).
const (
	attrDevicesRevoked = "devices_revoked"
	attrTokensRevoked  = "tokens_revoked"
	attrUserID         = "user_id"
)

// now is the time of the clock at the precision of PostgreSQL, so answers equal stored values.
func (l *Lifecycle) now() time.Time { return l.d.Clock.Now().Truncate(time.Microsecond) }

// authorizeOn authorizes act on r for a and returns the decision as an error.
func (l *Lifecycle) authorizeOn(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := l.d.People.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// record writes an allow entry of act inside the unit of work.
func (l *Lifecycle) record(ctx context.Context, act authorize.Action, kind string, id ids.ID, attrs auditing.Attrs) error {
	return l.d.People.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: kind, ResourceID: id, Attrs: attrs,
	})
}

// change runs work in a unit of work declared as the rights change act on the resource
// (kind, id) for the principal affected: phase 1 locks and raises permission_version, the hook of
// phase 3.1 verifies I1 and SR8 and appends access.changed (02 section 8.1).
func (l *Lifecycle) change(ctx context.Context, act authorize.Action, kind string, id, affected ids.ID, work func(context.Context) error) error {
	return l.d.People.d.UoW.Do(ctx, work, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(act), Resource: txn.Ref{Kind: kind, ID: id}, Principals: []ids.ID{affected},
	}))
}

// read runs work in a unit of work that changes no rights.
func (l *Lifecycle) read(ctx context.Context, work func(context.Context) error) error {
	return l.d.People.d.UoW.Do(ctx, work)
}

// noChange are the attributes of a call without effect (06 section 2 "Ohne Wirkung").
func noChange(attrs auditing.Attrs) auditing.Attrs {
	attrs[attrNoChange] = auditing.Bool(true)
	return attrs
}

// DisableUser implements identity.Lifecycle (08 section 11). A first unit loads the person,
// authorizes user.disable, checks the reason and decides the transition; a disabled person ends
// there as a call without effect, so a no-op never invalidates the rights caches. A second unit,
// declared as the rights change, locks the person after permission_version, authorizes again (a
// concurrent disable of the caller is seen here) and runs the steps of 08 section 11 in order;
// SR8 is verified by the rights hook before the commit, so the last owner gets 409 last_owner and
// nothing of the unit stays.
func (l *Lifecycle) DisableUser(ctx context.Context, cmd identity.DisableUserCmd) (identity.UserView, error) {
	a := cmd.Actor
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.UserView{}, err
	}
	var (
		out    identity.UserView
		change bool
	)
	err = l.read(ctx, func(ctx context.Context) error {
		rec, found, err := l.d.People.d.Store.User(ctx, cmd.UserID)
		if err != nil {
			return err
		}
		if err := l.authorizeOn(ctx, a, authorize.ActionUserDisable, userResource(cmd.UserID)); err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		if err := domain.CheckReason(cmd.Reason); err != nil {
			return err
		}
		change, err = domain.DisableTransition(rec.User.Principal.Status)
		if err != nil || change {
			return err
		}
		out = userView(rec)
		return l.record(ctx, authorize.ActionUserDisable, resourceUser, cmd.UserID, noChange(auditing.Attrs{attrReason: auditing.ReasonText(cmd.Reason)}))
	})
	if err != nil || !change {
		return out, err
	}
	err = l.change(ctx, authorize.ActionUserDisable, resourceUser, cmd.UserID, cmd.UserID, func(ctx context.Context) error {
		var err error
		out, err = l.disable(ctx, a, cmd.UserID, cmd.Reason)
		return err
	})
	return out, err
}

// disable is the rights change of DisableUser: the steps 2 to 6 and 8 of 08 section 11 (step 1 is
// SR8 in the rights hook; steps 5 for push registrations and 7 for runs and dev environments are
// subscribers of DeviceRevoked, SessionEnded and AccessChanged in later stages).
func (l *Lifecycle) disable(ctx context.Context, a actor.Actor, userID ids.ID, reason string) (identity.UserView, error) {
	rec, found, err := l.d.People.d.Store.LockUser(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	if err := l.authorizeOn(ctx, a, authorize.ActionUserDisable, userResource(userID)); err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, invisible()
	}
	change, err := domain.DisableTransition(rec.User.Principal.Status)
	if err != nil {
		return identity.UserView{}, err
	}
	if !change {
		// A concurrent disable won the race: answered as without effect (the one extra rights
		// version is harmless and rare).
		return userView(rec), l.record(ctx, authorize.ActionUserDisable, resourceUser, userID, noChange(auditing.Attrs{attrReason: auditing.ReasonText(reason)}))
	}
	now := l.now()
	// Step 2.
	if err := l.d.Store.SetPrincipalDisabled(ctx, userID, now); err != nil {
		return identity.UserView{}, err
	}
	// Devices are locked before sessions (queries/lifecycle.sql), the writes follow 08.
	devices, err := l.d.Store.LockLiveDevicesOfUser(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	sessions, err := l.d.Store.LockLiveSessionsOfUser(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	// Step 3.
	by := a.ID
	nSessions, err := l.d.Sessions.revokeCounted(ctx, sessions, identity.RevokeUserDisabled, &by)
	if err != nil {
		return identity.UserView{}, err
	}
	// Step 4.
	list := make([]ids.ID, 0, len(devices))
	for _, d := range devices {
		list = append(list, d.ID)
	}
	nDevices, err := l.revokeDevices(ctx, list, userID, by, domain.DeviceRevokeUserDisabled, now)
	if err != nil {
		return identity.UserView{}, err
	}
	// Step 5: personal API tokens.
	nTokens, err := l.d.Tokens.RevokeAllForPrincipal(ctx, userID, by)
	if err != nil {
		return identity.UserView{}, err
	}
	// Step 6: the rights hook raises the version and appends access.changed; session.revoked is
	// appended above; user.updated carries the new state.
	rec.User.Principal.Status, rec.User.Principal.DisabledAt = domain.StatusDisabled, now
	out := userView(rec)
	if err := l.d.People.emitUpdated(ctx, out); err != nil {
		return identity.UserView{}, err
	}
	// Step 8.
	return out, l.record(ctx, authorize.ActionUserDisable, resourceUser, userID, auditing.Attrs{
		attrReason: auditing.ReasonText(reason), attrSessionsRevoked: auditing.Int(int64(nSessions)),
		attrDevicesRevoked: auditing.Int(int64(nDevices)), attrTokensRevoked: auditing.Int(int64(nTokens)),
	})
}

// revokeDevices revokes the devices of list (already locked) of the person userID with reason, by
// the principal by (zero for none), revokes their keys and publishes one DeviceRevoked per device;
// it returns the number of devices it revoked.
func (l *Lifecycle) revokeDevices(ctx context.Context, list []ids.ID, userID, by ids.ID, reason domain.DeviceRevokeReason, at time.Time) (int, error) {
	if len(list) == 0 {
		return 0, nil
	}
	revoked, err := l.d.Store.RevokeDevices(ctx, list, by, reason, at)
	if err != nil || len(revoked) == 0 {
		return 0, err
	}
	if _, err := l.d.Store.RevokeKeysOfDevices(ctx, revoked, at); err != nil {
		return 0, err
	}
	for _, id := range revoked {
		if err := txn.Publish(ctx, identity.DeviceRevoked{DeviceID: id, UserID: userID, Reason: identity.DeviceRevokeReason(reason), At: at}); err != nil {
			return 0, err
		}
	}
	return len(revoked), nil
}

// deviceResource describes the device id of the person owner (zero for a missing device, which
// only an owner sees; the use case answers not_found then).
func deviceResource(id, owner ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceDevice, ID: id, OwnerID: owner}
}

// RevokeDevice implements identity.Lifecycle (08 section 10). As DisableUser: a first unit
// authorizes device.revoke on the device (self or owner, step-up) and ends an already revoked
// device without effect; a second unit, declared as the rights change for the device's person,
// locks the device, authorizes again and revokes device, keys, sessions and families.
func (l *Lifecycle) RevokeDevice(ctx context.Context, cmd identity.RevokeDeviceCmd) (identity.Device, error) {
	a := cmd.Actor
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.Device{}, err
	}
	var (
		out    identity.Device
		owner  ids.ID
		change bool
	)
	err = l.read(ctx, func(ctx context.Context) error {
		d, found, err := l.d.Sessions.d.Devices.Device(ctx, cmd.DeviceID)
		if err != nil {
			return err
		}
		if err := l.authorizeOn(ctx, a, authorize.ActionDeviceRevoke, deviceResource(cmd.DeviceID, d.UserID)); err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		if err := domain.CheckReason(cmd.Reason); err != nil {
			return err
		}
		owner = d.UserID
		if change = domain.DeviceRevokeTransition(string(d.State)); change {
			return nil
		}
		out = d
		return l.record(ctx, authorize.ActionDeviceRevoke, resourceDevice, cmd.DeviceID, noChange(auditing.Attrs{
			attrReason: auditing.ReasonText(cmd.Reason), attrUserID: auditing.ID(d.UserID),
		}))
	})
	if err != nil || !change {
		return out, err
	}
	err = l.change(ctx, authorize.ActionDeviceRevoke, resourceDevice, cmd.DeviceID, owner, func(ctx context.Context) error {
		var err error
		out, err = l.revokeDevice(ctx, a, cmd.DeviceID, cmd.Reason)
		return err
	})
	return out, err
}

// revokeDevice is the rights change of RevokeDevice.
func (l *Lifecycle) revokeDevice(ctx context.Context, a actor.Actor, deviceID ids.ID, reason string) (identity.Device, error) {
	d, found, err := l.d.Store.LockDevice(ctx, deviceID)
	if err != nil {
		return identity.Device{}, err
	}
	if err := l.authorizeOn(ctx, a, authorize.ActionDeviceRevoke, deviceResource(deviceID, d.UserID)); err != nil {
		return identity.Device{}, err
	}
	if !found {
		return identity.Device{}, invisible()
	}
	attrs := auditing.Attrs{attrReason: auditing.ReasonText(reason), attrUserID: auditing.ID(d.UserID)}
	if !domain.DeviceRevokeTransition(string(d.State)) {
		return d, l.record(ctx, authorize.ActionDeviceRevoke, resourceDevice, deviceID, noChange(attrs))
	}
	code := domain.DeviceRevokeReasonFor(a.ID, d.UserID)
	n, err := l.endDevice(ctx, d, a.ID, code, identity.RevokeDeviceRevoked)
	if err != nil {
		return identity.Device{}, err
	}
	out, found, err := l.d.Sessions.d.Devices.Device(ctx, deviceID)
	if err != nil {
		return identity.Device{}, err
	}
	if !found {
		return identity.Device{}, errs.New(errs.CodeInternalError, "identity: a revoked device is gone")
	}
	attrs[attrRevokeReason], attrs[attrSessionsRevoked] = auditing.Code(string(code)), auditing.Int(int64(n))
	return out, l.record(ctx, authorize.ActionDeviceRevoke, resourceDevice, deviceID, attrs)
}

// endDevice runs the steps of 08 section 10 on the locked device d: its live sessions end with
// sessionReason (children included, their families session_revoked), then the device and its keys
// are revoked with code by the principal by (zero for the system). It returns the number of ended
// sessions.
func (l *Lifecycle) endDevice(ctx context.Context, d identity.Device, by ids.ID, code domain.DeviceRevokeReason, sessionReason identity.SessionRevokeReason) (int, error) {
	sessions, err := l.d.Store.LockLiveSessionsOfDevice(ctx, d.ID)
	if err != nil {
		return 0, err
	}
	var actorRef *ids.ID
	if !by.IsZero() {
		actorRef = &by
	}
	n, err := l.d.Sessions.revokeCounted(ctx, sessions, sessionReason, actorRef)
	if err != nil {
		return 0, err
	}
	if _, err := l.revokeDevices(ctx, []ids.ID{d.ID}, d.UserID, by, code, l.now()); err != nil {
		return 0, err
	}
	return n, nil
}

// sessionResource describes the session id of the person owner (zero for a missing session).
func sessionResource(id, owner ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceAuthSession, ID: id, OwnerID: owner}
}

// RevokeSession implements identity.Lifecycle (05 section 4.3 session.revoke, self or owner,
// no step-up): the session and its live children end with logout when they are the caller's and
// admin otherwise, in a unit declared as the rights change for the session's person; an already
// revoked session ends after the first unit without effect.
func (l *Lifecycle) RevokeSession(ctx context.Context, cmd identity.RevokeSessionCmd) error {
	a := cmd.Actor
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	var (
		owner  ids.ID
		change bool
	)
	err = l.read(ctx, func(ctx context.Context) error {
		s, found, err := l.d.Sessions.d.Store.Session(ctx, cmd.SessionID)
		if err != nil {
			return err
		}
		if err := l.authorizeOn(ctx, a, authorize.ActionSessionRevoke, sessionResource(cmd.SessionID, s.UserID)); err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		owner = s.UserID
		if change = s.RevokedAt.IsZero(); change {
			return nil
		}
		return l.record(ctx, authorize.ActionSessionRevoke, resourceAuthSession, cmd.SessionID, noChange(auditing.Attrs{attrUserID: auditing.ID(s.UserID)}))
	})
	if err != nil || !change {
		return err
	}
	return l.change(ctx, authorize.ActionSessionRevoke, resourceAuthSession, cmd.SessionID, owner, func(ctx context.Context) error {
		return l.revokeSession(ctx, a, cmd.SessionID)
	})
}

// revokeSession is the rights change of RevokeSession.
func (l *Lifecycle) revokeSession(ctx context.Context, a actor.Actor, sessionID ids.ID) error {
	tree, err := l.d.Sessions.d.Store.LockSessionTree(ctx, sessionID)
	if err != nil {
		return err
	}
	root, found := find(tree, sessionID)
	if err := l.authorizeOn(ctx, a, authorize.ActionSessionRevoke, sessionResource(sessionID, root.UserID)); err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	attrs := auditing.Attrs{attrUserID: auditing.ID(root.UserID)}
	if !root.RevokedAt.IsZero() {
		return l.record(ctx, authorize.ActionSessionRevoke, resourceAuthSession, sessionID, noChange(attrs))
	}
	reason := identity.SessionRevokeReason(domain.SessionRevokeReasonFor(a.ID, root.UserID))
	by := a.ID
	n, err := l.d.Sessions.revokeCounted(ctx, tree, reason, &by)
	if err != nil {
		return err
	}
	attrs[attrRevokeReason], attrs[attrSessionsRevoked] = auditing.Code(string(reason)), auditing.Int(int64(n))
	return l.record(ctx, authorize.ActionSessionRevoke, resourceAuthSession, sessionID, attrs)
}

// inactiveBatch is the number of inactive devices one sweep step selects.
const inactiveBatch = 100

// ExpireInactiveDevices revokes every active device without contact for
// auth.device_inactive_revoke_after (last_seen_at, or approved_at when never seen; 08 section 5.4)
// with the steps of 08 section 10: one unit per device, declared as the rights change
// device.expire for its person, which locks the device, checks it again, ends its sessions with
// reason inactive, revokes device and keys with reason inactive and no actor and records the
// system action device.expire. Selecting reads without a rights change, so a run without such a
// device never raises the rights version. It returns the number of revoked devices. Errors: the
// first failing unit; the next run selects the device again.
func (l *Lifecycle) ExpireInactiveDevices(ctx context.Context) (int, error) {
	cutoff := l.now().Add(-l.cfg.DeviceInactiveRevokeAfter)
	var (
		after ids.ID
		total int
	)
	for {
		list, err := l.d.Store.InactiveDevices(ctx, cutoff, after, inactiveBatch)
		if err != nil {
			return total, err
		}
		for _, ref := range list {
			ok, err := l.expireDevice(ctx, ref, cutoff)
			if err != nil {
				return total, err
			}
			if ok {
				total++
			}
		}
		if len(list) < inactiveBatch {
			return total, nil
		}
		after = list[len(list)-1].ID
	}
}

// errStillActive rolls back the unit of a device that had contact since the selection.
var errStillActive = errors.New("identity: device no longer inactive")

// expireDevice revokes the device of ref in its own rights change when it is still active and
// inactive since before cutoff; ok reports the revocation.
func (l *Lifecycle) expireDevice(ctx context.Context, ref DeviceRef, cutoff time.Time) (bool, error) {
	err := l.change(ctx, authorize.ActionDeviceExpire, resourceDevice, ref.ID, ref.UserID, func(ctx context.Context) error {
		d, found, err := l.d.Store.LockDevice(ctx, ref.ID)
		if err != nil {
			return err
		}
		if !found || d.State != identity.DeviceStateActive || !inactiveSince(d.LastSeenAt, d.ApprovedAt, d.CreatedAt, cutoff) {
			return errStillActive
		}
		n, err := l.endDevice(ctx, d, ids.ID{}, domain.DeviceRevokeInactive, identity.RevokeInactive)
		if err != nil {
			return err
		}
		return l.record(ctx, authorize.ActionDeviceExpire, resourceDevice, d.ID, auditing.Attrs{
			attrRevokeReason: auditing.Code(string(domain.DeviceRevokeInactive)), attrUserID: auditing.ID(d.UserID),
			attrSessionsRevoked: auditing.Int(int64(n)),
		})
	})
	if errors.Is(err, errStillActive) {
		return false, nil
	}
	return err == nil, err
}

// inactiveSince reports a device whose last contact (lastSeen, else approved, else created) lies
// before cutoff.
func inactiveSince(lastSeen, approved, created, cutoff time.Time) bool {
	last := lastSeen
	if last.IsZero() {
		last = approved
	}
	if last.IsZero() {
		last = created
	}
	return !last.IsZero() && last.Before(cutoff)
}
