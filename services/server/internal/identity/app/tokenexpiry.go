// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ExpiringTokenRecord is an API token of an agent or an app that expires soon: its id, holder and
// expiry, never the hash, the prefix or the name (M6-07).
type ExpiringTokenRecord struct {
	ID, PrincipalID ids.ID
	ExpiresAt       time.Time
}

// TokenExpiryStore reads the API tokens that expire soon (adapters/postgres implements it).
type TokenExpiryStore interface {
	// ExpiringBotTokens returns the tokens with revoked_at IS NULL of agents and apps whose
	// expires_at lies in (at, until], ordered by expires_at, then id.
	ExpiringBotTokens(ctx context.Context, at, until time.Time) ([]ExpiringTokenRecord, error)
}

// TokenExpiry implements authz.TokenExpirySource over a TokenExpiryStore, so the list "Expiring
// soon" of authz learns the tokens of agents and apps without reading api_tokens (07 section 9.4,
// brief M6-07); token values never leave identity. It is safe for concurrent use when its store
// is.
//
// Example:
//
//	src := identityapp.NewTokenExpiry(identitypg.NewRepository(pool))
//	toks, err := src.ExpiringTokens(ctx, now, now.Add(lead))
type TokenExpiry struct {
	store TokenExpiryStore
}

// Compile-time check: TokenExpiry is the source port of authz.
var _ authz.TokenExpirySource = (*TokenExpiry)(nil)

// NewTokenExpiry returns the source over store. Errors: a nil store.
func NewTokenExpiry(store TokenExpiryStore) (*TokenExpiry, error) {
	if store == nil {
		return nil, errors.New("identity: NewTokenExpiry needs a store")
	}
	return &TokenExpiry{store: store}, nil
}

// ExpiringTokens implements authz.TokenExpirySource: the tokens of agents and apps that are not
// revoked and expire in (t, until], ordered by expiry, then id; personal tokens never appear.
// Errors: those of the store.
func (s *TokenExpiry) ExpiringTokens(ctx context.Context, t, until time.Time) ([]authz.ExpiringToken, error) {
	rows, err := s.store.ExpiringBotTokens(ctx, t, until)
	if err != nil {
		return nil, err
	}
	out := make([]authz.ExpiringToken, 0, len(rows))
	for _, r := range rows {
		out = append(out, authz.ExpiringToken{ID: r.ID, PrincipalID: r.PrincipalID, ExpiresAt: r.ExpiresAt})
	}
	return out, nil
}
