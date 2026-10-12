// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// KeyRow is one row of server_signing_keys as the use cases read and write it; the public JWK is
// stored from Public and KID, the private key only behind Ref.
type KeyRow struct {
	// KID is the RFC 7638 thumbprint of the public key.
	KID string
	// Purpose is the purpose.
	Purpose instance.Purpose
	// Alg is EdDSA.
	Alg string
	// Public is the Ed25519 public key.
	Public ed25519.PublicKey
	// Ref is private_key_ref, the secret reference of the private key in the key store.
	Ref string
	// State is the state.
	State instance.KeyState
	// CreatedAt is created_at.
	CreatedAt time.Time
	// NotAfter is not_after; zero for NULL.
	NotAfter time.Time
}

// KeyRepository is the outgoing port to server_signing_keys; adapters/postgres implements it.
// Statements run in the ambient transaction of ctx when there is one.
type KeyRepository interface {
	// LockPurpose takes the transaction-scoped advisory lock of purpose p
	// (pg_advisory_xact_lock(hashtext('bothy.server_signing_keys'), hashtext(p))). It requires a
	// unit of work.
	LockPurpose(ctx context.Context, p instance.Purpose) error
	// ListKeys returns the next, active and retiring keys of p, oldest first.
	ListKeys(ctx context.Context, p instance.Purpose) ([]KeyRow, error)
	// ActiveKey returns the active key of p; found is false when p has none.
	ActiveKey(ctx context.Context, p instance.Purpose) (row KeyRow, found bool, err error)
	// PublishedKeys returns the next, active and retiring keys of every purpose.
	PublishedKeys(ctx context.Context) ([]KeyRow, error)
	// KeyByKID returns the key kid in any state; found is false when it does not exist.
	KeyByKID(ctx context.Context, kid string) (row KeyRow, found bool, err error)
	// InsertKey inserts row with its public JWK.
	InsertKey(ctx context.Context, row KeyRow) error
	// TransitionKey moves the key kid from state from to state to and sets not_after when
	// notAfter is not zero. It reports false when the key was not in state from.
	TransitionKey(ctx context.Context, kid string, from, to instance.KeyState, notAfter time.Time) (bool, error)
	// PurposesWithoutActive returns the purposes of ps without an active key, ascending.
	PurposesWithoutActive(ctx context.Context, ps []instance.Purpose) ([]instance.Purpose, error)
}

// PrivateKey is a loaded private signing key. Implementations keep the key material out of every
// output path (the redacting secret type of platform/secrets) and never expose it.
type PrivateKey interface {
	// Public returns the public key that belongs to the private key.
	Public() ed25519.PublicKey
	// Sign returns the Ed25519 signature of msg.
	Sign(msg []byte) []byte
}

// ErrKeyMissing is wrapped by KeyStore.Load when the private file behind a reference does not
// exist; EnsureActive retires such keys (domain.LostTransitions).
var ErrKeyMissing = errors.New("instance: the private key file of a signing key is missing")

// KeyStore is the outgoing port to the key store secrets.signing_key_dir (ADR 0066);
// adapters/keystore implements it over the secret provider.
type KeyStore interface {
	// Generate creates a new Ed25519 key for purpose p, writes the private key to the key store
	// and returns its kid (RFC 7638 thumbprint), public key and secret reference. It is called
	// outside every transaction, before the row is inserted. Errors: the write error of the key
	// store (never containing key material).
	Generate(ctx context.Context, p instance.Purpose) (kid string, pub ed25519.PublicKey, ref string, err error)
	// Load reads the private key behind ref. Errors: ErrKeyMissing (wrapped) when the file does
	// not exist; the read or parse error otherwise (never containing key material).
	Load(ctx context.Context, ref string) (PrivateKey, error)
	// Remove deletes the private key behind ref; a missing file is no error.
	Remove(ctx context.Context, ref string) error
}
