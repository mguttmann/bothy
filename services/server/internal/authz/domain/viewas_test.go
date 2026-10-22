// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestViewAsExempt: while a view is stored, access.view_as.end, auth.logout and auth.step_up on
// the own session and self.read on oneself are decided without view_as (R-AZ-32); the same
// actions on another session or person, and every other action, are not exempt (M6-01).
func TestViewAsExempt(t *testing.T) {
	t.Parallel()
	owner, session, other := ids.New(), ids.New(), ids.New()
	for _, act := range []string{"access.view_as.end", "auth.logout", "auth.step_up"} {
		t.Run(act, func(t *testing.T) {
			t.Parallel()
			if !domain.ViewAsExempt(act, domain.ExemptTarget{Kind: "auth_session", ID: session, OwnerID: owner}, owner, session) {
				t.Fatal("the own session must be exempt")
			}
			if domain.ViewAsExempt(act, domain.ExemptTarget{Kind: "auth_session", ID: other, OwnerID: owner}, owner, session) {
				t.Fatal("another session must not be exempt")
			}
			if domain.ViewAsExempt(act, domain.ExemptTarget{Kind: "user", ID: owner, OwnerID: owner}, owner, session) {
				t.Fatal("another resource kind must not be exempt")
			}
		})
	}
	t.Run("self.read", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []string{"principal", "user"} {
			if !domain.ViewAsExempt("self.read", domain.ExemptTarget{Kind: kind, ID: owner, OwnerID: owner}, owner, session) {
				t.Fatalf("self.read on oneself (%s) must be exempt", kind)
			}
			if domain.ViewAsExempt("self.read", domain.ExemptTarget{Kind: kind, ID: other, OwnerID: other}, owner, session) {
				t.Fatalf("self.read on another principal (%s) must not be exempt", kind)
			}
		}
	})
	for _, act := range []string{"access.view_as", "message.create", "grant.create", "session.revoke"} {
		if domain.ViewAsExempt(act, domain.ExemptTarget{Kind: "auth_session", ID: session, OwnerID: owner}, owner, session) {
			t.Fatalf("%s must not be exempt", act)
		}
	}
	if domain.ViewAsExempt("auth.logout", domain.ExemptTarget{Kind: "auth_session", ID: session, OwnerID: owner}, owner, ids.ID{}) {
		t.Fatal("a principal without a session is never exempt")
	}
}
