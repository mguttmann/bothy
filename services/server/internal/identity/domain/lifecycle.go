// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SessionRevokeReason is a value of auth_sessions.revoke_reason that the lifecycle use cases
// store (docs/spec/03-datenmodell.md section 4.2, docs/spec/08-identitaet.md sections 10 and 11).
type SessionRevokeReason string

// The session reasons of the lifecycle use cases (M1-45).
const (
	// SessionRevokeLogout: the person of the session ended it (session.revoke on an own session).
	SessionRevokeLogout SessionRevokeReason = "logout"
	// SessionRevokeAdmin: someone else ended it (session.revoke by an owner).
	SessionRevokeAdmin SessionRevokeReason = "admin"
	// SessionRevokeDeviceRevoked: its device was revoked (device.revoke).
	SessionRevokeDeviceRevoked SessionRevokeReason = "device_revoked"
	// SessionRevokeUserDisabled: its person was disabled (user.disable).
	SessionRevokeUserDisabled SessionRevokeReason = "user_disabled"
	// SessionRevokeInactive: its device had no contact for auth.device_inactive_revoke_after
	// (device.expire).
	SessionRevokeInactive SessionRevokeReason = "inactive"
)

// Valid reports whether r is one of the five lifecycle reasons.
func (r SessionRevokeReason) Valid() bool {
	switch r {
	case SessionRevokeLogout, SessionRevokeAdmin, SessionRevokeDeviceRevoked, SessionRevokeUserDisabled, SessionRevokeInactive:
		return true
	}
	return false
}

// DeviceRevokeReason is a value of devices.revoke_reason that the lifecycle use cases store; the
// column holds a machine code with a CHECK, the free text of a request goes only into the audit
// entry (docs/DECISIONS.md M1-45).
type DeviceRevokeReason string

// The device reasons of the lifecycle use cases (M1-45).
const (
	// DeviceRevokedBySelf: the person of the device revoked it.
	DeviceRevokedBySelf DeviceRevokeReason = "revoked_by_self"
	// DeviceRevokedByOwner: an owner revoked the device of another person.
	DeviceRevokedByOwner DeviceRevokeReason = "revoked_by_owner"
	// DeviceRevokeUserDisabled: the person of the device was disabled.
	DeviceRevokeUserDisabled DeviceRevokeReason = "user_disabled"
	// DeviceRevokeInactive: the device had no contact for auth.device_inactive_revoke_after.
	DeviceRevokeInactive DeviceRevokeReason = "inactive"
)

// Valid reports whether r is one of the four lifecycle reasons.
func (r DeviceRevokeReason) Valid() bool {
	switch r {
	case DeviceRevokedBySelf, DeviceRevokedByOwner, DeviceRevokeUserDisabled, DeviceRevokeInactive:
		return true
	}
	return false
}

// SessionRevokeReasonFor returns the reason of session.revoke: logout when actorID is the person
// sessionUserID of the session, admin otherwise. 03 has no own code for "ended remotely by the
// person"; logout is what the person did, admin what someone else did, and the audit entry names
// the actor anyway.
//
// Example:
//
//	domain.SessionRevokeReasonFor(alex, alex) // logout
func SessionRevokeReasonFor(actorID, sessionUserID ids.ID) SessionRevokeReason {
	if !actorID.IsZero() && actorID == sessionUserID {
		return SessionRevokeLogout
	}
	return SessionRevokeAdmin
}

// DeviceRevokeReasonFor returns the reason of device.revoke: revoked_by_self when actorID is the
// person deviceUserID of the device, revoked_by_owner otherwise (only an owner may revoke the
// device of another person, 05 section 4.3).
func DeviceRevokeReasonFor(actorID, deviceUserID ids.ID) DeviceRevokeReason {
	if !actorID.IsZero() && actorID == deviceUserID {
		return DeviceRevokedBySelf
	}
	return DeviceRevokedByOwner
}

// DisableTransition decides user.disable for a principal in status s (08 section 11): active and
// pending become disabled (change true); a disabled principal stays as it is (change false, a call
// without effect, 06 section 2 "Ohne Wirkung").
//
// Errors: invalid_state_transition for deleted and any other status.
func DisableTransition(s Status) (change bool, err error) {
	switch s {
	case StatusActive, StatusPending:
		return true, nil
	case StatusDisabled:
		return false, nil
	case StatusDeleted:
	}
	return false, errs.New(errs.CodeInvalidStateTransition, "a deleted person cannot be disabled")
}

// DeviceRevokeTransition decides device.revoke for a device in state (devices.state, 08 section
// 10): pending and active become revoked (change true); a revoked device stays (change false, a
// call without effect). An unknown state is never changed.
func DeviceRevokeTransition(state string) (change bool) {
	switch state {
	case "pending", "active":
		return true
	}
	return false
}
