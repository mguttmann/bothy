// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The matrix of docs/spec/07-rechte.md section 11.1, one test per id this ticket covers.

const (
	allow        = authorize.OutcomeAllow
	notFound     = authorize.OutcomeNotFound
	forbidden    = authorize.OutcomeForbidden
	unauthentic  = authorize.OutcomeUnauthenticated
	stepUpNeeded = authorize.OutcomeStepUpRequired
)

func TestAZM01(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.owner), authorize.ActionMessageRead, w.msg(w.notizen, w.assistent))
}

func TestAZM02(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.owner), authorize.ActionMessageRead, w.msg(w.dm, w.alex), notFound, "no_grant")
}

func TestAZM03(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionMessageCreate, w.thread(w.allgemein))
}

func TestAZM04(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true))
}

func TestAZM05(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
}

// TestAZM06: a search filters with Scope; Marketing never enters Alex's visible channels, so three
// hits there give zero hits and total 0 (the search module applies the set, M1-63).
func TestAZM06(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionSearchQuery, authorize.Resource{Kind: authorize.ResourceInstance})
	sc, err := w.az.Scope(context.Background(), w.user(w.alex), authorize.ActionMessageRead)
	if err != nil {
		t.Fatal(err)
	}
	hits := []ids.ID{w.kampagnen, w.kampagnen, w.intern}
	total := 0
	for _, c := range hits {
		if slices.Contains(sc.ChannelIDs, c) {
			total++
		}
	}
	if total != 0 || !slices.Equal(sc.ChannelIDs, domain.SortIDs([]ids.ID{w.allgemein, w.eskalation, w.dm})) {
		t.Fatalf("total %d, channels %v", total, sc.ChannelIDs)
	}
}

func TestAZM07(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
}

func TestAZM08(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.sam), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "level_insufficient")
}

func TestAZM09(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.clk.Set(w.membershipExpiry)
	w.expect(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), notFound, "no_grant")
}

func TestAZM10(t *testing.T) {
	w := newWorld(t)
	if _, err := w.store.RemoveMembership(context.Background(), w.samMembership, w.owner, w.clk.Now()); err != nil {
		t.Fatal(err)
	}
	w.versions.Bump()
	w.expect(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), notFound, "no_grant")
}

func TestAZM11(t *testing.T) {
	w := newWorld(t)
	w.gAlexEsk = w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelRead, time.Time{})
	w.versions.Bump()
	w.allow(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.eskalation, w.lotse, true))
}

func TestAZM12(t *testing.T) {
	w := newWorld(t)
	w.gAlexEsk = w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelRead, time.Time{})
	w.revoke(t, w.g1)
	w.expect(t, w.user(w.alex), authorize.ActionMessageCreate, w.thread(w.eskalation), forbidden, "level_insufficient")
}

func TestAZM13(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	w.revoke(t, w.g1)
	w.expect(t, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam), notFound, "no_grant")
}

func TestAZM14(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.robin), authorize.ActionMessageCreate, w.thread(w.projektA))
}

func TestAZM18(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.robin), authorize.ActionThreadRead, w.thread(w.projektA))
	w.clk.Set(w.g3Expiry.Add(time.Second))
	w.expect(t, w.user(w.robin), authorize.ActionThreadRead, w.thread(w.projektA), notFound, "no_grant")
}

func TestAZM27(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.agent(w.lotse), authorize.ActionMessageCreate, w.thread(w.allgemein))
}

func TestAZM28(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.agent(w.lotse), authorize.ActionMessageCreate, w.thread(w.kampagnen), notFound, "no_grant")
}

func TestAZM29(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.agent(w.texter), authorize.ActionMessageCreate, w.thread(w.kampagnen))
}

func TestAZM31(t *testing.T) {
	w := newWorld(t)
	r := authorize.Resource{Kind: authorize.ResourceSpace, ID: w.support, Grant: &authorize.GrantFacts{
		SubjectKind: "agent", Level: "trigger", TargetKind: "space", TargetSpaceID: w.support}}
	w.expect(t, w.user(w.owner), authorize.ActionGrantCreate, r, forbidden, "subject_kind_not_allowed")
}

func TestAZM32(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.app(), authorize.ActionMessageCreate, w.thread(w.eskalation))
}

func TestAZM33(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.app(), authorize.ActionMessageRead, w.msg(w.eskalation, w.alex), forbidden, "scope_missing")
}

func TestAZM34(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.app(), authorize.ActionMessageCreate, w.thread(w.allgemein), notFound, "no_grant")
}

func TestAZM40(t *testing.T) {
	w := newWorld(t)
	f, _, _ := w.principals.Principal(context.Background(), w.alex)
	f.Status = "disabled"
	w.principals.Put(f)
	w.versions.Bump()
	for _, c := range []struct {
		a authorize.Action
		r authorize.Resource
	}{
		{authorize.ActionMessageRead, w.msg(w.allgemein, w.alex)},
		{authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.alex, OwnerID: w.alex}},
		{authorize.ActionChannelRead, w.channel(w.kampagnen)},
	} {
		w.expect(t, w.user(w.alex), c.a, c.r, unauthentic, "principal_inactive")
	}
}

func TestAZM41(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.revokeSession(w.alex)
	w.expect(t, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), unauthentic, "session_revoked")
}

func TestAZM50(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	if _, err := w.store.SoftDeleteGroup(context.Background(), w.leser, w.clk.Now()); err != nil {
		t.Fatal(err)
	}
	w.versions.Bump()
	w.expect(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), notFound, "no_grant")
}

// helpers of the matrix

func userSubject(id ids.ID) domain.Subject {
	return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id}
}

// revoke revokes a grant and raises the rights version, as M1-29 does in one transaction.
func (w *world) revoke(t testing.TB, grant ids.ID) {
	t.Helper()
	if _, ok, err := w.store.RevokeGrant(context.Background(), grant, w.owner, "test", w.clk.Now()); err != nil || !ok {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	w.versions.Bump()
}

// revokeSession revokes the session of p and raises the rights version (04 section 8.5).
func (w *world) revokeSession(p ids.ID) {
	w.creds.PutSession(w.sessions[p], fakeSessionRevoked(p, w.devices[p]))
	w.versions.Bump()
}
