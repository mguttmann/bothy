// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// Purpose is the purpose of a server signing key (docs/spec/03-data-model.md
// server_signing_keys.purpose). The table also allows terminal, which M2-39 adds to the managed
// purposes; this version manages the four below.
type Purpose string

// The purposes this version manages.
const (
	// PurposeAccessToken signs access tokens (08 section 7.1, M1-34); rotates on schedule.
	PurposeAccessToken Purpose = "access_token"
	// PurposeInvocation signs invocation tokens (09 section 4.5, M2); rotates on schedule.
	PurposeInvocation Purpose = "invocation"
	// PurposeWebhookOut signs outgoing webhooks (11 section 7, M2); rotates on owner request.
	PurposeWebhookOut Purpose = "webhook_out"
	// PurposeAuditCheckpoint signs audit checkpoints (19 SEC-AUD-7, M1-50); rotates on owner
	// request, and its keys are never published.
	PurposeAuditCheckpoint Purpose = "audit_checkpoint"
)

// Purposes returns the managed purposes in a fixed order: EnsureActive creates one active key for
// each, the rotation job walks them.
func Purposes() []Purpose {
	return []Purpose{PurposeAccessToken, PurposeInvocation, PurposeWebhookOut, PurposeAuditCheckpoint}
}

// Valid reports whether p is a managed purpose.
func (p Purpose) Valid() bool {
	switch p {
	case PurposeAccessToken, PurposeInvocation, PurposeWebhookOut, PurposeAuditCheckpoint:
		return true
	}
	return false
}

// KeyState is the state of a server signing key: next, active, retiring or retired (19 section
// 10).
type KeyState string

// The states of a signing key.
const (
	// KeyNext is published and does not sign yet.
	KeyNext KeyState = "next"
	// KeyActive signs; exactly one per purpose.
	KeyActive KeyState = "active"
	// KeyRetiring stopped signing and stays published until its window has passed.
	KeyRetiring KeyState = "retiring"
	// KeyRetired is withdrawn; its private file is deleted.
	KeyRetired KeyState = "retired"
)

// AlgEdDSA is the one algorithm of the keys this version creates (RFC 8037, 08 section 7.1).
const AlgEdDSA = "EdDSA"

// RotateMode is SigningKeyRotate.mode (05 table 4.2).
type RotateMode string

// The rotation modes.
const (
	// ModeScheduled announces a next key; the rotation job makes it active after
	// agent.jwks_max_age.
	ModeScheduled RotateMode = "scheduled"
	// ModeEmergency retires every key of the purpose at once and creates a fresh active key.
	ModeEmergency RotateMode = "emergency"
)

// Signer signs with one fixed key. Callers put KeyID and Alg into the JWS header before they
// build the signing input; JOSE assembly stays with the token issuers (kernel/jose, ADR 0067).
type Signer interface {
	// KeyID returns the kid of the key.
	KeyID() string
	// Alg returns the JWS algorithm, always EdDSA.
	Alg() string
	// Sign returns the Ed25519 signature of signingInput (the ASCII of header.payload). Errors:
	// kind unavailable when the private key cannot be read from the key store; the context error.
	Sign(ctx context.Context, signingInput []byte) ([]byte, error)
}

// PublicKey is the public part of a signing key with its state.
type PublicKey struct {
	// KID is the key id, the RFC 7638 thumbprint of the public JWK.
	KID string
	// Purpose is the purpose of the key.
	Purpose Purpose
	// Alg is the algorithm, EdDSA.
	Alg string
	// State is the state when the key was read.
	State KeyState
	// Key is the Ed25519 public key.
	Key ed25519.PublicKey
	// NotAfter is when the key stopped signing; zero while it never did.
	NotAfter time.Time
}

// JWK is one member of the published JWKS: exactly the members of RFC 7517 and RFC 8037 for an
// Ed25519 public key, never a private member.
type JWK struct {
	// Kty is "OKP".
	Kty string `json:"kty"`
	// Crv is "Ed25519".
	Crv string `json:"crv"`
	// X is the public key, base64url without padding.
	X string `json:"x"`
	// Kid is the key id.
	Kid string `json:"kid"`
	// Alg is "EdDSA".
	Alg string `json:"alg"`
	// Use is "sig".
	Use string `json:"use"`
}

// JWKSet is the published key set (RFC 7517 section 5).
type JWKSet struct {
	// Keys are the next, active and retiring keys of access_token and invocation, ordered by
	// purpose, state (active, next, retiring) and kid.
	Keys []JWK `json:"keys"`
}

// RotateCommand is the request of signing_key.rotate.
type RotateCommand struct {
	// Purpose is the purpose to rotate.
	Purpose Purpose
	// Mode is scheduled or emergency.
	Mode RotateMode
	// Reason is the required reason of the owner; it goes into the audit entry.
	Reason string
}

// Rotation is the result of signing_key.rotate.
type Rotation struct {
	// Purpose is the rotated purpose.
	Purpose Purpose
	// NewKID is the announced next key (scheduled) or the fresh active key (emergency).
	NewKID string
	// PreviousKID is the key that was active before; empty when the purpose had none.
	PreviousKID string
	// PreviousNotAfter is when the previous key stops signing: the creation of the next key plus
	// agent.jwks_max_age for scheduled, the time of the rotation for emergency.
	PreviousNotAfter time.Time
}

// Keys is the port of the server signing keys (ADR 0066). Access tokens (M1-34), invocation
// tokens (M2), outgoing webhooks (M2) and audit checkpoints (M1-50) sign through it; no other
// module reads server_signing_keys. Implementations are safe for concurrent use.
type Keys interface {
	// Signer returns a Signer for the active key of p, read fresh from the database on every
	// call. Errors: kind internal for an unknown purpose; kind unavailable when p has no active
	// key (the start creates them) or its private key cannot be read; the database error.
	Signer(ctx context.Context, p Purpose) (Signer, error)
	// VerificationKey returns the next, active or retiring key kid of purpose p from a
	// per-process cache refreshed after signing_key.cache_ttl; an unknown kid reloads at most once
	// per signing_key.cache_ttl. A retired key, a key of another purpose and an unknown kid are
	// kind not_found.
	VerificationKey(ctx context.Context, p Purpose, kid string) (PublicKey, error)
	// PublicKey returns the key kid in any state, for verifying old audit checkpoints. Errors:
	// kind not_found; the database error.
	PublicKey(ctx context.Context, kid string) (PublicKey, error)
	// JWKS returns the published key set (getJwks) after authorizing the public action
	// instance.discovery.read for the principal of ctx (anonymous when there is none). Errors: the
	// decision of authorize; the database error.
	JWKS(ctx context.Context) (JWKSet, error)
	// Rotate runs signing_key.rotate for a (owner with fresh step-up, audited). Errors: the
	// decision of authorize (401, 403); kind validation_failed for an unknown purpose or mode or
	// an empty or forbidden reason; kind unavailable when the key store cannot be written.
	Rotate(ctx context.Context, a actor.Actor, cmd RotateCommand) (Rotation, error)
	// EnsureActive creates an active key for every managed purpose that has none. The start
	// calls it in every role (02 section 15.1), bothy setup bootstrap-owner later (M1-37).
	// Concurrent calls create exactly one active key per purpose.
	EnsureActive(ctx context.Context) error
}
