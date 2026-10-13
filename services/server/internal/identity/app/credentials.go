// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
)

// CredentialSource implements authz.CredentialSource over the device and session stores (07
// section 4.2 step 1, check_active): DeviceState of the request's device (M1-33), SessionActive and
// SessionExpiresAt of its session (M1-34). The token part follows with M1-44; until then
// TokenActive stays false, so a request that names a token is unauthenticated (fail closed).
type CredentialSource struct {
	devices  DeviceStore
	sessions SessionStore
}

// Compile-time check: the source port of authz.
var _ authz.CredentialSource = (*CredentialSource)(nil)

// NewCredentialSource returns the source over the device and session stores.
func NewCredentialSource(devices DeviceStore, sessions SessionStore) *CredentialSource {
	return &CredentialSource{devices: devices, sessions: sessions}
}

// Credentials implements authz.CredentialSource. The device state is unknown for an id that does
// not exist or belongs to another principal; a zero DeviceID gives zero device facts. A session is
// active when it exists, is not revoked and belongs to PrincipalID and DeviceID; SessionExpiresAt
// is its end, which authz compares with the time of the call. Errors: those of the stores.
func (s *CredentialSource) Credentials(ctx context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	var f authz.CredentialFacts
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
