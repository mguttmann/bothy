// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: Service is the port of device approval and pairing (M1-38).
var _ identity.DeviceApproval = (*Service)(nil)

// Audit values of device.approve and device.expire (codes and ids only, 19 SEC-AUD-5).
const (
	resourceDevice       = string(authorize.ResourceDevice)
	attrResult           = "result"
	attrRevokeReason     = "revoke_reason"
	attrPairingID        = "pairing_id"
	resultApproved       = "approved"
	resultRejectedWrong  = "rejected_wrong_code"
	resultPairingUnknown = "pairing_code_unknown"
	fieldPairingCode     = "/pairing_code"
)

// pendingExpired reports a pending device whose window auth.device_pending_ttl has passed at now:
// it counts as revoked everywhere, also before the job revokes it (08 section 5.2).
func pendingExpired(d identity.Device, now time.Time, ttl time.Duration) bool {
	return d.State == identity.DeviceStatePending && !now.Before(d.CreatedAt.Add(ttl))
}

// verificationCode returns the code d shows while it is pending, "" otherwise (08 section 5.2).
// Callers set it only on answers to the device itself.
func (s *Service) verificationCode(d identity.Device) string {
	if d.State != identity.DeviceStatePending || d.KeyThumbprint == "" {
		return ""
	}
	return domain.DeriveVerificationCode(s.d.Pepper, d.ID, d.KeyThumbprint, s.cfg.DeviceVerificationCodeDigits)
}

// caller returns the principal of the request; outside the chain the anonymous principal, which
// authorize refuses.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// committed returns ctx outside any unit of work, so a unit run on it commits on its own. An
// effect that must stay although the request then answers an error (an ended step-up, an expiry
// found on access, a rejected pairing) runs on it: a keyed request runs the use case inside the
// unit of work of the Idempotency-Key stage, which rolls back every error answer (red team
// M1-38 round 1).
func committed(ctx context.Context) context.Context { return txn.Detach(ctx) }

// system returns ctx with the anonymous principal, so an expiry inside a request is audited and
// evented like the job: as a system action without actor (device.expire).
func system(ctx context.Context) context.Context { return actor.WithActor(ctx, actor.Anonymous()) }

// ApproveDevice implements identity.DeviceApproval (brief M1-38 "ApproveDevice order"): load the
// device, authorize device.approve (owner or self with step-up; a pending caller gets
// device_pending), answer an active device unchanged and a revoked one with
// invalid_state_transition, revoke an expired pending device (pending_expired) and answer
// invalid_state_transition, then compare the code in one unit of work declared as the rights change
// of the device's person: a correct code activates the device, a wrong one revokes it
// (verification_failed). Both are audited as allow with their result and answer the device.
func (s *Service) ApproveDevice(ctx context.Context, deviceID ids.ID, verificationCode string) (identity.Device, error) {
	a := caller(ctx)
	dev, found, err := s.d.Devices.Device(ctx, deviceID)
	if err != nil {
		return identity.Device{}, err
	}
	if err := s.authorize(ctx, a, authorize.ActionDeviceApprove, authorize.Resource{Kind: authorize.ResourceDevice, ID: deviceID, OwnerID: dev.UserID}); err != nil {
		return identity.Device{}, err
	}
	if !found {
		return identity.Device{}, invisible()
	}
	switch {
	case dev.State == identity.DeviceStateActive:
		// A call without effect (06 section 2): the device is answered unchanged.
		return dev, nil
	case dev.State == identity.DeviceStateRevoked:
		return identity.Device{}, errs.New(errs.CodeInvalidStateTransition, "")
	case pendingExpired(dev, s.d.Clock.Now(), s.cfg.DevicePendingTTL):
		if err := s.expirePendingDevice(system(committed(ctx)), dev.ID, dev.UserID); err != nil {
			return identity.Device{}, err
		}
		return identity.Device{}, errs.New(errs.CodeInvalidStateTransition, "")
	}
	var out identity.Device
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.approveWrite(ctx, a, deviceID, verificationCode)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionDeviceApprove), Resource: txn.Ref{Kind: resourceDevice, ID: deviceID}, Principals: []ids.ID{dev.UserID},
	}))
	return out, err
}

// approveWrite is the unit of work of ApproveDevice: it locks the device, checks its state again
// (a concurrent approval or revocation may have won), compares the code in constant time and
// activates or revokes the device with its audit entry.
func (s *Service) approveWrite(ctx context.Context, a actor.Actor, deviceID ids.ID, code string) (identity.Device, error) {
	cur, found, err := s.d.Devices.LockDevice(ctx, deviceID)
	if err != nil || !found {
		return identity.Device{}, orInvisible(err)
	}
	now := s.d.Clock.Now()
	switch {
	case cur.State == identity.DeviceStateActive:
		return cur, nil
	case cur.State != identity.DeviceStatePending || pendingExpired(cur, now, s.cfg.DevicePendingTTL):
		return identity.Device{}, errs.New(errs.CodeInvalidStateTransition, "")
	}
	expected := domain.DeriveVerificationCode(s.d.Pepper, cur.ID, cur.KeyThumbprint, s.cfg.DeviceVerificationCodeDigits)
	result := resultApproved
	if cur.KeyThumbprint != "" && domain.VerificationCodeEqual(expected, code) {
		if _, err := s.d.Devices.ActivateDevice(ctx, deviceID, a.ID, now); err != nil {
			return identity.Device{}, err
		}
	} else {
		// A wrong code rejects the device: a blind or guessed approval never activates a foreign
		// device, and the answer is 200 because the request had an effect (08 section 5.2).
		result = resultRejectedWrong
		by := a.ID
		if err := s.revokeUnconfirmedDevice(ctx, deviceID, identity.DeviceRevokeVerificationFailed, &by); err != nil {
			return identity.Device{}, err
		}
	}
	err = s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionDeviceApprove), Decision: auditing.DecisionAllow, ResourceKind: resourceDevice, ResourceID: deviceID,
		Attrs: auditing.Attrs{attrDeviceID: auditing.ID(deviceID), attrResult: auditing.Code(result)},
	})
	if err != nil {
		return identity.Device{}, err
	}
	out, found, err := s.d.Devices.Device(ctx, deviceID)
	if err != nil || !found {
		return identity.Device{}, orInvisible(err)
	}
	return out, nil
}

// authorize calls the Authorizer and turns a denial into its error.
func (s *Service) authorize(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// revokeUnconfirmedDevice revokes a device that never became usable (pending, or created by a
// pairing that was never collected) inside the ambient unit of work: the device with reason and
// the principal by (nil for the system), all its keys, every open session of the device with
// device_revoked (their families session_revoked, one session.revoked each, SessionEnded) through
// the revocation primitive of identity.Auth, and the in-transaction event DeviceRevoked. This
// subset is complete because such a device never had push registrations or gateway connections;
// M1-45 owns the revocation of active devices. A revoked device stays unchanged.
func (s *Service) revokeUnconfirmedDevice(ctx context.Context, deviceID ids.ID, reason identity.DeviceRevokeReason, by *ids.ID) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "identity: revoking a device needs a unit of work")
	}
	var byID ids.ID
	if by != nil {
		byID = *by
	}
	at := s.d.Clock.Now()
	userID, revoked, err := s.d.Devices.RevokeDevice(ctx, deviceID, reason, byID, at)
	if err != nil || !revoked {
		return err
	}
	sessions, err := s.d.Sessions.OpenSessionsOfDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	for _, sid := range sessions {
		if err := s.d.Auth.RevokeSession(ctx, sid, identity.RevokeDeviceRevoked, by); err != nil {
			return err
		}
	}
	return txn.Publish(ctx, identity.DeviceRevoked{DeviceID: deviceID, UserID: userID, Reason: reason, At: at})
}

// expireChange declares the rights change of the system action device.expire for the person of a
// device.
func expireChange(deviceID, userID ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionDeviceExpire), Resource: txn.Ref{Kind: resourceDevice, ID: deviceID}, Principals: []ids.ID{userID},
	})
}

// expirePendingDevice revokes the device deviceID of userID with pending_expired in an own unit
// of work when it is still pending past auth.device_pending_ttl, audited as the system action
// device.expire. A device that changed meanwhile stays unchanged.
func (s *Service) expirePendingDevice(ctx context.Context, deviceID, userID ids.ID) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Devices.LockDevice(ctx, deviceID)
		if err != nil || !found || !pendingExpired(cur, s.d.Clock.Now(), s.cfg.DevicePendingTTL) {
			return err
		}
		if err := s.revokeUnconfirmedDevice(ctx, deviceID, identity.DeviceRevokePendingExpired, nil); err != nil {
			return err
		}
		return s.recordExpire(ctx, deviceID, identity.DeviceRevokePendingExpired)
	}, expireChange(deviceID, userID))
}

// recordExpire is the allow entry of the system action device.expire for deviceID with reason.
func (s *Service) recordExpire(ctx context.Context, deviceID ids.ID, reason identity.DeviceRevokeReason) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionDeviceExpire), Decision: auditing.DecisionAllow, ResourceKind: resourceDevice, ResourceID: deviceID,
		Attrs: auditing.Attrs{attrDeviceID: auditing.ID(deviceID), attrRevokeReason: auditing.Code(string(reason))},
	})
}

// invalidCode is the validation_failed of a malformed code at pointer.
func invalidCode(pointer string) error {
	return errs.New(errs.CodeValidationFailed, "").
		WithFields(errs.FieldError{Pointer: pointer, Code: "invalid", Detail: "The code is not well-formed."})
}
