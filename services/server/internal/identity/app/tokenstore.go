// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// APITokenRecord is a row of api_tokens (03 section 4.2). Hash is the HMAC of the token; the
// secret itself is never stored.
type APITokenRecord struct {
	ID, PrincipalID ids.ID
	Name            string
	Prefix          string
	Hash            []byte
	Scopes          []string
	// DPoPJKT is "" for an unbound token.
	DPoPJKT   string
	ExpiresAt time.Time
	CreatedBy ids.ID
	CreatedAt time.Time
	// LastUsedAt, RevokedAt and RevokedBy are zero for none.
	LastUsedAt, RevokedAt time.Time
	RevokedBy             ids.ID
}

// Active reports a token that is not revoked and has not expired at now.
func (r APITokenRecord) Active(now time.Time) bool {
	return r.RevokedAt.IsZero() && now.Before(r.ExpiresAt)
}

// APITokenAuthRecord is a token with the kind of its principal, what the authenticator reads in
// one query per request.
type APITokenAuthRecord struct {
	APITokenRecord
	// PrincipalKind is principals.kind of the holder.
	PrincipalKind domain.Kind
}

// APITokenStore is the outgoing port of the API tokens over api_tokens (implemented by
// adapters/postgres). Reads join the ambient transaction of ctx or use the pool; writes need a
// unit of work, except TouchAPIToken. Errors are kernel/errs errors without SQL, hashes or
// values.
type APITokenStore interface {
	// InsertAPIToken inserts r.
	InsertAPIToken(ctx context.Context, r APITokenRecord) error
	// APIToken reads the token id; found is false for none.
	APIToken(ctx context.Context, id ids.ID) (APITokenRecord, bool, error)
	// LockAPIToken reads the token id FOR UPDATE inside the unit of work.
	LockAPIToken(ctx context.Context, id ids.ID) (APITokenRecord, bool, error)
	// APITokenByHash reads the token with token_hash hash and the kind of its principal.
	APITokenByHash(ctx context.Context, hash []byte) (APITokenAuthRecord, bool, error)
	// ActiveAPITokens reads up to limit tokens of principalID with revoked_at IS NULL ordered by
	// created_at descending, then id descending, strictly after the position (afterCreated,
	// afterID); a zero afterID starts at the newest.
	ActiveAPITokens(ctx context.Context, principalID ids.ID, afterCreated time.Time, afterID ids.ID, limit int) ([]APITokenRecord, error)
	// RevokeAPIToken sets revoked_at and revoked_by of the token id when it is active; revoked
	// reports a change.
	RevokeAPIToken(ctx context.Context, id, by ids.ID, at time.Time) (revoked bool, err error)
	// RevokeAPITokensOfPrincipal revokes every active token of principalID and returns their
	// number.
	RevokeAPITokensOfPrincipal(ctx context.Context, principalID, by ids.ID, at time.Time) (int, error)
	// TouchAPIToken sets last_used_at = at when it is NULL or before threshold. It runs on the
	// pool, never in the request's unit of work.
	TouchAPIToken(ctx context.Context, id ids.ID, at, threshold time.Time) error
}
