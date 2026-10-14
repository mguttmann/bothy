// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DeviceRevokeReason is devices.revoke_reason (docs/spec/03-datenmodell.md, 08 section 5.4).
type DeviceRevokeReason string

// The revocation reasons of an unconfirmed device (M1-38); M1-45 adds the reasons of active
// devices.
const (
	// DeviceRevokeVerificationFailed: the approving person typed a wrong verification code.
	DeviceRevokeVerificationFailed DeviceRevokeReason = "verification_failed"
	// DeviceRevokePendingExpired: the device waited longer than auth.device_pending_ttl.
	DeviceRevokePendingExpired DeviceRevokeReason = "pending_expired"
	// DeviceRevokePairingExpired: the paired device never collected its tokens.
	DeviceRevokePairingExpired DeviceRevokeReason = "pairing_expired"
)

// DeviceRevoked is the in-transaction event of a device revocation (docs/spec/02-architektur.md
// section 4.3): the device, its person and the reason. Its sessions end in the same unit of work
// with SessionEnded.
type DeviceRevoked struct {
	// DeviceID is the revoked device; UserID its person.
	DeviceID, UserID ids.ID
	// Reason is the code of devices.revoke_reason.
	Reason DeviceRevokeReason
}

// PairingStarted is the answer of beginPairing (schema PairingStarted): Code is the display form
// behind the redacting Token; it leaves the server once, in that answer, and never reaches a log.
type PairingStarted struct {
	// ID is device_pairings.id, in the API pairing_id.
	ID ids.ID
	// Code is the pairing code in display form, for example 7KQ2-M9XD.
	Code Token
	// ExpiresAt is device_pairings.expires_at.
	ExpiresAt time.Time
}

// PairingOutcome is the answer of completePairing: Completed with the tokens of the new session,
// or still requested with the end of the pairing (schema PairingStatus).
type PairingOutcome struct {
	// Completed reports an approved pairing whose tokens were issued now.
	Completed bool
	// PairingID and ExpiresAt describe a pairing that still waits.
	PairingID ids.ID
	ExpiresAt time.Time
	// Tokens are the token pair of the new session when Completed.
	Tokens TokenSet
}

// DeviceApproval is the incoming port of device approval and pairing (PLAN M1-38; 08 sections 5.2
// and 5.3). The calling principal comes from ctx (kernel/actor); the REST handlers verify the DPoP
// proof of the public pairing endpoints and pass its thumbprint. Every error is a kernel/errs
// error without codes, hashes or SQL.
type DeviceApproval interface {
	// ApproveDevice approves the pending device deviceID with the verification code it shows
	// (device.approve, owner or self, step-up). A correct code makes it active; a wrong code
	// revokes it (verification_failed) and still answers the device, then revoked. An active
	// device is answered unchanged. Errors: the decision of authorize, not_found,
	// invalid_state_transition for a revoked or expired pending device.
	ApproveDevice(ctx context.Context, deviceID ids.ID, verificationCode string) (Device, error)
	// BeginPairing starts a pairing without login (auth.login.begin) for the registration reg
	// whose dpop key signed the DPoP proof of thumbprint proofJKT. Errors: validation_failed for
	// the registration, invalid_dpop_proof for another key, conflict for a device id in use.
	BeginPairing(ctx context.Context, reg RegisterDevice, proofJKT string, clientIP netip.Addr) (PairingStarted, error)
	// ApprovePairing creates the device of the open pairing with pairingCode as an active device
	// of the caller (device.approve, step-up). Errors: the decision of authorize,
	// validation_failed for a malformed code, not_found for a code without an open pairing (which
	// ends the step-up of the calling session), conflict for a device id or key in use.
	ApprovePairing(ctx context.Context, pairingCode string) (Device, error)
	// CompletePairing collects the tokens of the pairing pairingID with a DPoP proof of the
	// registered key (auth.login.complete): not completed while requested, the tokens once
	// approved. Errors: gone for an unknown, expired, rejected or completed pairing,
	// invalid_dpop_proof for another key (which rejects a requested pairing).
	CompletePairing(ctx context.Context, pairingID ids.ID, proofJKT string, clientIP netip.Addr) (PairingOutcome, error)
}
