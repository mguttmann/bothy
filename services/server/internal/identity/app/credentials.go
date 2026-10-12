// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
)

// CredentialSource implements authz.CredentialSource over the device store (07 section 4.2 step
// 1, check_active). M1-33 builds the device part: DeviceState of the request's device. The session
// part (M1-34) and the token part (M1-44) join this type; until then SessionActive and TokenActive
// stay false, so a request that names a session or a token is unauthenticated (fail closed).
type CredentialSource struct {
	devices DeviceStore
}

// Compile-time check: the source port of authz.
var _ authz.CredentialSource = (*CredentialSource)(nil)

// NewCredentialSource returns the source over the device store.
func NewCredentialSource(devices DeviceStore) *CredentialSource {
	return &CredentialSource{devices: devices}
}

// Credentials implements authz.CredentialSource. The device state is unknown for an id that does
// not exist or belongs to another principal; a zero DeviceID gives zero facts. Errors: those of
// the store.
func (s *CredentialSource) Credentials(ctx context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	var f authz.CredentialFacts
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
