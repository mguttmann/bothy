// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StoredPasskey is a row of passkey_credentials (docs/spec/03-datenmodell.md section 4.2). Zero
// values stand for NULL. It holds only the public key and counters, no secret.
type StoredPasskey struct {
	ID, UserID ids.ID
	// CredentialID is the WebAuthn credential ID; PublicKeyCOSE the public key in COSE form.
	CredentialID, PublicKeyCOSE []byte
	// SignCount is the stored signature counter.
	SignCount uint32
	// AAGUID is the authenticator model; zero for none.
	AAGUID ids.ID
	// Transports are the transports the client reported at registration.
	Transports []string
	// BackupEligible and BackupState are the backup flags of the authenticator data.
	BackupEligible, BackupState bool
	// Name is the name the person gave the passkey.
	Name                             string
	CreatedAt, LastUsedAt, RevokedAt time.Time
}

// PasskeyUse is the update of a passkey after a successful assertion: the kept counter, the
// backup state of the authenticator data and the time of use.
type PasskeyUse struct {
	SignCount   uint32
	BackupState bool
	At          time.Time
}

// PasskeyStore is the outgoing port over passkey_credentials (implemented by adapters/postgres,
// M1-40). Writes and the lock run only inside a unit of work. Errors are kernel/errs errors
// without SQL; a duplicate credential ID is 409 conflict.
type PasskeyStore interface {
	// InsertPasskey inserts p and returns the stored row.
	InsertPasskey(ctx context.Context, p StoredPasskey) (StoredPasskey, error)
	// ActivePasskeysOfUser returns the passkeys of userID with revoked_at IS NULL, oldest first.
	ActivePasskeysOfUser(ctx context.Context, userID ids.ID) ([]StoredPasskey, error)
	// PasskeyByCredentialID returns the passkey with credentialID, revoked or not; found is false
	// for an unknown ID.
	PasskeyByCredentialID(ctx context.Context, credentialID []byte) (p StoredPasskey, found bool, err error)
	// LockPasskey locks the passkey id FOR UPDATE and returns it; found is false for an unknown id.
	LockPasskey(ctx context.Context, id ids.ID) (p StoredPasskey, found bool, err error)
	// UpdatePasskeyUse writes u on the passkey id.
	UpdatePasskeyUse(ctx context.Context, id ids.ID, u PasskeyUse) error
	// RevokePasskey sets revoked_at = at on the passkey id while it is active; revoked is false
	// when it was revoked already.
	RevokePasskey(ctx context.Context, id ids.ID, at time.Time) (revoked bool, err error)
	// CountActivePasskeys counts the active passkeys of userID.
	CountActivePasskeys(ctx context.Context, userID ids.ID) (int, error)
}
