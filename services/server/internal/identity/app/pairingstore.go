// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PairingRecord is a row of device_pairings. Zero values stand for NULL. It never holds the
// pairing code, only its HMAC.
type PairingRecord struct {
	ID ids.ID
	// Hash is pairing_code_hash.
	Hash []byte
	// Registration is device_registration (schema device_registration.v1).
	Registration domain.DeviceRegistrationV1
	Status       domain.PairingStatus
	// UserID is the approving person, ApprovedByDeviceID the approving device, DeviceID the
	// device created at the approval.
	UserID, ApprovedByDeviceID, DeviceID ids.ID
	CreatedAt, ExpiresAt, CompletedAt    time.Time
}

// PairingStore is the outgoing port over device_pairings (implemented by adapters/postgres, M1-38).
// Writes and locks run only inside a unit of work. Errors are kernel/errs errors without SQL,
// codes or hashes.
type PairingStore interface {
	// InsertPairing inserts rec with status requested; inserted is false when another requested
	// pairing holds the same code hash (the caller draws a new code).
	InsertPairing(ctx context.Context, rec PairingRecord) (inserted bool, err error)
	// LockRequestedPairingByHash returns the requested pairing with hash, locked FOR UPDATE until
	// the end of the unit of work; found is false when none has it.
	LockRequestedPairingByHash(ctx context.Context, hash []byte) (rec PairingRecord, found bool, err error)
	// LockPairing returns the pairing id locked FOR UPDATE; found is false for an unknown id.
	LockPairing(ctx context.Context, id ids.ID) (rec PairingRecord, found bool, err error)
	// UpdatePairing writes status, user_id, approved_by_device_id, device_id, expires_at and
	// completed_at of rec.
	UpdatePairing(ctx context.Context, rec PairingRecord) error
	// DuePairings returns up to limit requested or approved pairings with expires_at <= now,
	// oldest end first (the expiry of the job identity.auth_cleanup).
	DuePairings(ctx context.Context, now time.Time, limit int) ([]PairingRecord, error)
}
