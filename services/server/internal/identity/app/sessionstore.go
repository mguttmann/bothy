// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SessionRecord is a row of auth_sessions. Zero values stand for NULL.
type SessionRecord struct {
	ID, UserID, DeviceID ids.ID
	Component            identity.ClientComponent
	// ParentID is parent_session_id; IdentityID is identity_id.
	ParentID, IdentityID ids.ID
	AuthMethod           identity.AuthMethod
	CreatedAt, ExpiresAt time.Time
	StepUpAt             time.Time
	StepUpMethod         string
	LastSeenAt           time.Time
	RevokedAt            time.Time
	RevokeReason         identity.SessionRevokeReason
}

// Active reports a session that is not revoked and has not reached its end at now.
func (s SessionRecord) Active(now time.Time) bool {
	return s.RevokedAt.IsZero() && now.Before(s.ExpiresAt)
}

// FamilyRecord is a row of refresh_token_families.
type FamilyRecord struct {
	ID, SessionID     ids.ID
	DPoPJKT           string
	CurrentGeneration int
	RotatedAt         time.Time
	CreatedAt         time.Time
	RevokedAt         time.Time
}

// RefreshRecord is a row of refresh_tokens.
type RefreshRecord struct {
	ID, FamilyID        ids.ID
	Generation          int
	Hash                []byte
	IssuedAt, ExpiresAt time.Time
	UsedAt              time.Time
}

// AuthState is the result of the one authentication query: the session with the state and last
// activity of its device and the thumbprint of the device's active dpop key ("" for none).
type AuthState struct {
	Session          SessionRecord
	DeviceState      identity.DeviceState
	DeviceLastSeenAt time.Time
	DeviceKeyJKT     string
}

// FamilyRevokeReason is refresh_token_families.revoke_reason.
type FamilyRevokeReason string

// The reasons of a family.
const (
	FamilyReuseDetected  FamilyRevokeReason = "reuse_detected"
	FamilyLogout         FamilyRevokeReason = "logout"
	FamilySessionRevoked FamilyRevokeReason = "session_revoked"
	FamilyExpired        FamilyRevokeReason = "expired"
)

// SessionStore is the outgoing port of the session use cases over auth_sessions,
// refresh_token_families and refresh_tokens (implemented by adapters/postgres). Reads join the
// ambient transaction of ctx when there is one; writes and locks run only inside a unit of work.
// Errors are kernel/errs errors without SQL, tokens or hashes.
type SessionStore interface {
	// InsertSession inserts s (created_at and expires_at as given).
	InsertSession(ctx context.Context, s SessionRecord) error
	// InsertFamily inserts f.
	InsertFamily(ctx context.Context, f FamilyRecord) error
	// InsertRefreshToken inserts t.
	InsertRefreshToken(ctx context.Context, t RefreshRecord) error
	// Session returns the session id; found is false for an unknown id.
	Session(ctx context.Context, id ids.ID) (s SessionRecord, found bool, err error)
	// LockSessionTree locks the session id, then its children in ascending id, and returns them
	// (the session first); empty for an unknown id. A child that a concurrent holder of the
	// session lock inserted is among them.
	LockSessionTree(ctx context.Context, id ids.ID) ([]SessionRecord, error)
	// RevokeSessions sets revoked_at and revoke_reason on the sessions among list that are not
	// revoked and returns those, ascending.
	RevokeSessions(ctx context.Context, list []ids.ID, reason identity.SessionRevokeReason, at time.Time) ([]ids.ID, error)
	// ActiveSessionsOfUser returns up to limit sessions of userID that are neither revoked nor
	// expired at now, by id descending, with ids below before (zero for the first page).
	ActiveSessionsOfUser(ctx context.Context, userID, before ids.ID, now time.Time, limit int) ([]SessionRecord, error)
	// AuthState returns the session id with its device and active dpop key; found is false for an
	// unknown session.
	AuthState(ctx context.Context, id ids.ID) (st AuthState, found bool, err error)
	// Touch sets last_seen_at = at (and last_ip on the session, when valid) on the session and its
	// device, each only where last_seen_at is unset or older than threshold; one statement
	// outside any unit of work.
	Touch(ctx context.Context, sessionID, deviceID ids.ID, ip netip.Addr, at, threshold time.Time) error

	// ActiveFamily returns the active family of sessionID; found is false when there is none.
	ActiveFamily(ctx context.Context, sessionID ids.ID) (f FamilyRecord, found bool, err error)
	// LockFamily locks the family id and returns it; found is false for an unknown id.
	LockFamily(ctx context.Context, id ids.ID) (f FamilyRecord, found bool, err error)
	// RefreshTokenByHash returns the token with hash and the session of its family; found is false
	// when no token has it.
	RefreshTokenByHash(ctx context.Context, hash []byte) (t RefreshRecord, sessionID ids.ID, found bool, err error)
	// MarkRefreshUsed sets used_at = at on generation of familyID when it is unset.
	MarkRefreshUsed(ctx context.Context, familyID ids.ID, generation int, at time.Time) error
	// AdvanceFamily sets current_generation and rotated_at of familyID.
	AdvanceFamily(ctx context.Context, familyID ids.ID, generation int, at time.Time) error
	// RevokeFamily revokes familyID with reason when it is active.
	RevokeFamily(ctx context.Context, familyID ids.ID, reason FamilyRevokeReason, at time.Time) error
	// RevokeActiveFamilies revokes the active families of the sessions in list with reason.
	RevokeActiveFamilies(ctx context.Context, list []ids.ID, reason FamilyRevokeReason, at time.Time) error
}
