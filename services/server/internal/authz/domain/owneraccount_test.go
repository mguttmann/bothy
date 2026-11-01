// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestTerminalGuard_OwnerAccount (ADR 0073, D-OA-7): an assignment of the owner account passes SR12
// only for an active owner; a demoted owner and every non-owner get
// terminal_owner_account_owner_only; an assignment whose flag the host no longer reports
// (Reported false) stays terminal_no_session_user; a separate session user is unchanged.
func TestTerminalGuard_OwnerAccount(t *testing.T) {
	t.Parallel()
	me, host := ids.New(), ids.New()
	h := domain.HostFacts{ID: host, State: "active", AssignedUserID: me}
	su := func(ownerAccount, reported bool) *domain.SessionUserFact {
		return &domain.SessionUserFact{HostID: host, HostSessionUserID: ids.New(), OSUser: "owner", Reported: reported, OwnerAccount: ownerAccount}
	}
	for _, tc := range []struct {
		name  string
		owner bool
		su    *domain.SessionUserFact
		want  domain.Reason
	}{
		{"active owner", true, su(true, true), ""},
		{"demoted owner", false, su(true, true), domain.ReasonTerminalOwnerAccountOwnerOnly},
		{"flag no longer reported", true, su(true, false), domain.ReasonTerminalNoSessionUser},
		{"separate session user of a non-owner", false, su(false, true), ""},
	} {
		got, ok := domain.TerminalGuard(domain.TerminalGuardFacts{PrincipalID: me, Owner: tc.owner, Host: h, SessionUser: tc.su})
		if got != tc.want || ok != (tc.want == "") {
			t.Errorf("%s: %q %v", tc.name, got, ok)
		}
	}
}
