// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestOwnerAccount_OpenOnlyForOwners (ADR 0073, D-OA-7): on R4 the session user of Robin is the
// owner account. As an active owner Robin opens it; after losing the owner role the same
// assignment gives 403 terminal_denied with reason owner_account_owner_only and a deny entry.
func TestOwnerAccount_OpenOnlyForOwners(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	r4 := ids.New()
	w.hosts.Put(authz.HostFacts{ID: r4, State: "active", LLMCredentials: "none", DeclaredServerHost: true, Name: "R4"})
	w.hosts.PutSessionUser(w.robin, authz.SessionUserFact{HostID: r4, HostSessionUserID: ids.New(), OSUser: "owner", Reported: true, OwnerAccount: true})
	w.principals.Put(authz.PrincipalFacts{ID: w.robin, Kind: actor.KindUser, Status: authz.PrincipalActive, IsOwner: true})
	w.versions.Bump()
	w.allow(t, w.person(w.robin), actOpen, hostOpen(r4))
	w.principals.Put(authz.PrincipalFacts{ID: w.robin, Kind: actor.KindUser, Status: authz.PrincipalActive, IsOwner: false})
	w.versions.Bump()
	// R4 is a server host, so SR10 refuses first; on a host assigned to Robin the owner account
	// rule decides.
	w.hosts.Put(authz.HostFacts{ID: r4, State: "active", LLMCredentials: "none", AssignedUserID: w.robin, Name: "R4"})
	w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin}, domain.Target{Kind: domain.TargetHost, HostID: r4},
		domain.LevelNone, time.Time{}, domain.CapTerminal)
	w.rebuild(t)
	d := w.decide(t, context.Background(), w.person(w.robin), actOpen, hostOpen(r4))
	if d.Outcome != authorize.OutcomeForbidden || d.Reason != "terminal_owner_account_owner_only" {
		t.Fatalf("demoted owner: %+v", d)
	}
	if e, _ := errs.As(d.Err()); e == nil || e.Code != errs.CodeTerminalDenied || e.Reason != "owner_account_owner_only" {
		t.Fatalf("problem %+v", e)
	}
}
