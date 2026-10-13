// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister: identity declares exactly the durable events user.updated (M1-31) and
// session.revoked (M1-34).
func TestRegister(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	identity.Register(r)
	want := []authorize.EventRegistration{
		{Module: "identity", Type: authorize.EventSessionRevoked}, {Module: "identity", Type: authorize.EventUserUpdated},
	}
	if got := r.Events(); !slices.Equal(got, want) {
		t.Fatalf("Events() = %v, want %v", got, want)
	}
}
