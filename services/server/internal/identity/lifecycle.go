// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DisableUserCmd is the input of Lifecycle.DisableUser (operation disableUser, schema ReasonBody).
type DisableUserCmd struct {
	// Actor is the calling principal.
	Actor actor.Actor
	// UserID is the person to disable.
	UserID ids.ID
	// Reason is the free text of the request; it goes into the audit entry only.
	Reason string
}

// RevokeDeviceCmd is the input of Lifecycle.RevokeDevice (operation revokeDevice, schema
// ReasonBody).
type RevokeDeviceCmd struct {
	// Actor is the calling principal.
	Actor actor.Actor
	// DeviceID is the device to revoke.
	DeviceID ids.ID
	// Reason is the free text of the request; it goes into the audit entry only, the stored
	// devices.revoke_reason is a machine code (docs/DECISIONS.md M1-45).
	Reason string
}

// RevokeSessionCmd is the input of Lifecycle.RevokeSession (operation revokeSession).
type RevokeSessionCmd struct {
	// Actor is the calling principal.
	Actor actor.Actor
	// SessionID is the session to end; its child sessions end with it.
	SessionID ids.ID
}

// Lifecycle is the incoming port of the revocation use cases (PLAN M1-45; 08 sections 10 and 11):
// each runs in one unit of work declared as a rights change of its action, raises
// permission_version, writes session.revoked per ended session, publishes SessionEnded and
// DeviceRevoked and writes the audit entry. From the commit on every request with a token of the
// disabled person, the revoked device or the ended session fails with 401 session_revoked. Calls
// on something already disabled or revoked succeed without effect (audit no_change, no rights
// version). Errors are kernel/errs errors; a missing and an invisible resource give the same
// not_found.
type Lifecycle interface {
	// DisableUser disables the person (user.disable, owner with step-up): status disabled, all
	// sessions and their families, all devices and their keys and the personal API tokens
	// revoked; grants, memberships and the owners row stay. Errors: validation_failed on
	// /reason, invalid_state_transition for a deleted person, last_owner (409) when no active
	// owner would remain, the decision of authorize.
	DisableUser(ctx context.Context, cmd DisableUserCmd) (UserView, error)
	// RevokeDevice revokes the device with its keys, sessions and their families (device.revoke,
	// self or owner with step-up). Errors: validation_failed on /reason, the decision of
	// authorize (not_found for the device of another person and a non-owner).
	RevokeDevice(ctx context.Context, cmd RevokeDeviceCmd) (Device, error)
	// RevokeSession ends the session and its child sessions with their families (session.revoke,
	// self or owner, no step-up); the reason is logout for the own session and admin otherwise.
	// Errors: the decision of authorize (not_found for the session of another person and a
	// non-owner).
	RevokeSession(ctx context.Context, cmd RevokeSessionCmd) error
}
