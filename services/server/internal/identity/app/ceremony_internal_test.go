// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// lockedPasskeys is a PasskeyStore whose LockPasskey returns pk and records UpdatePasskeyUse.
type lockedPasskeys struct {
	PasskeyStore
	pk      StoredPasskey
	updated bool
}

func (s *lockedPasskeys) LockPasskey(context.Context, ids.ID) (StoredPasskey, bool, error) {
	return s.pk, true, nil
}

func (s *lockedPasskeys) UpdatePasskeyUse(context.Context, ids.ID, PasskeyUse) error {
	s.updated = true
	return nil
}

// TestRecordUseRefusesPasskeyRevokedSinceLookup: a passkey revoked between the unlocked lookup of
// the verifier and the row lock of recordUse ends the ceremony as revoked and records no use
// (time of check against time of use).
func TestRecordUseRefusesPasskeyRevokedSinceLookup(t *testing.T) {
	id := ids.New()
	store := &lockedPasskeys{pk: StoredPasskey{ID: id, UserID: ids.New(), RevokedAt: time.Unix(1, 0)}}
	c := &ceremonies{passkeys: store}
	err := c.recordUse(context.Background(), AssertionResult{Passkey: StoredPasskey{ID: id}}, authorize.ActionAuthLoginComplete, time.Unix(2, 0))
	if cause, ok := refusalCause(err); !ok || cause != causeCredentialRevoked {
		t.Fatalf("recordUse of a revoked passkey = %v, want the refusal %s", err, causeCredentialRevoked)
	}
	if store.updated {
		t.Error("the use of a revoked passkey was recorded")
	}
}
