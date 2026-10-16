// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
)

// CredentialSource implements authz.CredentialSource over the device, session and API token
// stores (07 section 4.2 step 1, check_active): DeviceState of the request's device (M1-33),
// SessionActive and SessionExpiresAt of its session (M1-34), TokenActive and TokenExpiresAt of
// its API token (M1-44). Without a token store TokenActive stays false (fail closed).
type CredentialSource struct {
	devices  DeviceStore
	sessions SessionStore
	tokens   APITokenStore
}

// Compile-time check: the source port of authz.
var _ authz.CredentialSource = (*CredentialSource)(nil)

// NewCredentialSource returns the source over the device, session and API token stores.
func NewCredentialSource(devices DeviceStore, sessions SessionStore, tokens APITokenStore) *CredentialSource {
	return &CredentialSource{devices: devices, sessions: sessions, tokens: tokens}
}

// Credentials implements authz.CredentialSource. The device state is unknown for an id that does
// not exist or belongs to another principal; a zero DeviceID gives zero device facts. A session is
// active when it exists, is not revoked and belongs to PrincipalID and DeviceID; SessionExpiresAt
// is its end, which authz compares with the time of the call. A token is active when it exists,
// belongs to PrincipalID and has revoked_at IS NULL; TokenExpiresAt is its expires_at. Errors:
// those of the stores.
func (s *CredentialSource) Credentials(ctx context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	f, err := s.tokenFacts(ctx, ref)
	if err != nil {
		return authz.CredentialFacts{}, err
	}
	if !ref.SessionID.IsZero() {
		sess, found, err := s.sessions.Session(ctx, ref.SessionID)
		if err != nil {
			return authz.CredentialFacts{}, err
		}
		if found && sess.UserID == ref.PrincipalID && sess.DeviceID == ref.DeviceID {
			f.SessionActive, f.SessionExpiresAt = sess.RevokedAt.IsZero(), sess.ExpiresAt
		}
	}
	if ref.DeviceID.IsZero() {
		return f, nil
	}
	d, found, err := s.devices.Device(ctx, ref.DeviceID)
	if err != nil {
		return authz.CredentialFacts{}, err
	}
	f.DeviceState = authz.DeviceUnknown
	if found && d.UserID == ref.PrincipalID {
		f.DeviceState = authz.DeviceState(d.State)
	}
	return f, nil
}

// tokenFacts reports TokenActive and TokenExpiresAt of the request's API token: active when it
// exists, belongs to ref.PrincipalID and is not revoked. A zero TokenID or no token store gives no
// token facts (fail closed). Errors: those of the token store.
func (s *CredentialSource) tokenFacts(ctx context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	var f authz.CredentialFacts
	if ref.TokenID.IsZero() || s.tokens == nil {
		return f, nil
	}
	t, found, err := s.tokens.APIToken(ctx, ref.TokenID)
	if err != nil {
		return f, err
	}
	if found && t.PrincipalID == ref.PrincipalID {
		f.TokenActive, f.TokenExpiresAt = t.RevokedAt.IsZero(), t.ExpiresAt
	}
	return f, nil
}
