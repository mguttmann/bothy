// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The hosts of the host tests (M2-39).
var (
	hostR1 = ids.MustParse("019a0000-0000-7000-8000-000000000301")
	hostR2 = ids.MustParse("019a0000-0000-7000-8000-000000000302")
	hostR3 = ids.MustParse("019a0000-0000-7000-8000-000000000303")
	hostR4 = ids.MustParse("019a0000-0000-7000-8000-000000000304")
)

// TestSnapshot_HostGrants (M2-39, 07 section 2.12): grants with target host fill the map Hosts and
// never the tree; a direct grant and one of a group in effect count, an expired and a revoked one
// do not, and the expiry of a host grant enters ValidUntil.
func TestSnapshot_HostGrants(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	sam := ids.MustParse("019a0000-0000-7000-8000-000000000401")
	leser := domain.Group{ID: ids.MustParse("019a0000-0000-7000-8000-000000000501"), Name: "Leser", Source: domain.GroupSourceLocal}
	user := domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam}
	terminal := domain.CapsOf(domain.CapTerminal)
	on := func(h ids.ID) domain.Target { return domain.Target{Kind: domain.TargetHost, HostID: h} }
	soon := at.Add(time.Hour)
	in := domain.SnapshotInput{
		PrincipalID: sam, Version: 7, At: at, Active: true,
		Grants: []domain.Grant{
			{ID: ids.New(), Subject: user, Target: on(hostR1), Caps: terminal, ExpiresAt: soon},
			{ID: ids.New(), Subject: user, Target: on(hostR3), Caps: terminal, ExpiresAt: at},
			{ID: ids.New(), Subject: user, Target: on(hostR4), Caps: terminal, RevokedAt: at.Add(-time.Minute)},
		},
		Memberships: []domain.MembershipWithGroup{{Membership: domain.Membership{ID: ids.New(), GroupID: leser.ID, UserID: sam, AddedAt: at}, Group: leser}},
		GroupGrants: []domain.Grant{{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectGroup, GroupID: leser.ID}, Target: on(hostR2), Caps: terminal}},
	}
	s := domain.BuildSnapshot(in)
	want := map[ids.ID]domain.Caps{hostR1: terminal, hostR2: terminal}
	if !reflect.DeepEqual(s.Hosts, want) {
		t.Fatalf("Hosts = %v, want %v", s.Hosts, want)
	}
	if s.Instance != (domain.Rights{}) || len(s.Spaces) != 0 || len(s.Channels) != 0 {
		t.Fatalf("a host grant entered the tree: %+v", s)
	}
	if !s.ValidUntil.Equal(soon) {
		t.Fatalf("ValidUntil = %v, want the expiry of the host grant %v", s.ValidUntil, soon)
	}
	for h, want := range map[ids.ID]bool{hostR1: true, hostR2: true, hostR3: false, hostR4: false} {
		if got := domain.TerminalOn(s, h); got != want {
			t.Errorf("TerminalOn(%s) = %v, want %v", h, got, want)
		}
	}
	owner := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: sam, At: at, Active: true, Owner: true})
	if !domain.TerminalOn(owner, hostR3) {
		t.Fatal("an effective owner holds terminal on every host (07 section 2.12 point 3)")
	}
	if inactive := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: sam, At: at, Grants: in.Grants}); domain.TerminalOn(inactive, hostR1) {
		t.Fatal("a principal that is not active holds terminal")
	}
}

// TestDiffVisible_Hosts (M2-39, 04 section 8.2): a host the principal gains terminal on is added
// with capabilities [terminal] and without level, a lost one is removed with kind and id only;
// an owner sees every host; a host never changes (one capability).
func TestDiffVisible_Hosts(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	sam := ids.MustParse("019a0000-0000-7000-8000-000000000401")
	all := []ids.ID{hostR1, hostR2, hostR3}
	user := domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam}
	grant := func(h ids.ID) domain.Grant {
		return domain.Grant{ID: ids.New(), Subject: user, Target: domain.Target{Kind: domain.TargetHost, HostID: h}, Caps: domain.CapsOf(domain.CapTerminal)}
	}
	viewOf := func(owner bool, grants ...domain.Grant) domain.AccessView {
		s := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: sam, At: at, Active: true, Owner: owner, Grants: grants})
		return domain.AccessViewOf(s, domain.KindUser, sam, nil, nil, all)
	}
	host := func(id ids.ID) domain.AccessEntry {
		return domain.AccessEntry{Kind: domain.AccessKindHost, ID: id, Caps: domain.CapsOf(domain.CapTerminal)}
	}
	gone := func(id ids.ID) domain.AccessEntry { return domain.AccessEntry{Kind: domain.AccessKindHost, ID: id} }
	cases := []struct {
		name          string
		before, after domain.AccessView
		want          domain.PrincipalDiff
	}{
		{"grant added", viewOf(false), viewOf(false, grant(hostR2)), domain.PrincipalDiff{Added: []domain.AccessEntry{host(hostR2)}}},
		{"grant revoked", viewOf(false, grant(hostR2)), viewOf(false), domain.PrincipalDiff{Removed: []domain.AccessEntry{gone(hostR2)}}},
		{"owner gains every host", viewOf(false, grant(hostR2)), viewOf(true, grant(hostR2)),
			domain.PrincipalDiff{Added: []domain.AccessEntry{host(hostR1), host(hostR3)}}},
		{"owner loses the role", viewOf(true), viewOf(false, grant(hostR3)),
			domain.PrincipalDiff{Removed: []domain.AccessEntry{gone(hostR1), gone(hostR2)}}},
		{"no change", viewOf(false, grant(hostR1)), viewOf(false, grant(hostR1)), domain.PrincipalDiff{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.DiffVisible(c.before, c.after); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("DiffVisible = %+v, want %+v", got, c.want)
			}
		})
	}
	// An agent never sees a host, also with a grant row whose kind does not fit.
	s := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: sam, At: at, Active: true, Grants: []domain.Grant{grant(hostR1)}})
	if v := domain.AccessViewOf(s, domain.KindAgent, sam, nil, nil, all); len(v.Hosts) != 0 {
		t.Fatalf("an agent sees hosts %v", v.Hosts)
	}
}

// TestHostRequirementEntries (M2-39): host.read and host.manage are real requirements with the 05
// text of their column "Recht", gated by HumanSession; the other host and terminal actions stay
// deferred, terminal.terminate stays a system action.
func TestHostRequirementEntries(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{
		"host.read":   "All(HumanSession,Any(Owner,Assignee,Terminal))",
		"host.manage": "All(HumanSession,Owner)",
	} {
		req, ok := domain.RequirementOf(id)
		if !ok || req.Expr.String() != want {
			t.Errorf("%s = %v %v, want %s", id, req.Expr, ok, want)
		}
		if !domain.HumanGated(req.Expr) {
			t.Errorf("%s is not gated by HumanSession", id)
		}
	}
	// host.pair is Public since M2-41 (TestHostPairRequirement); host.connect is HostSelf since M2-43.
	if req, _ := domain.RequirementOf("host.connect"); !req.Expr.IsHostSelf() || req.Source != "host-self" {
		t.Errorf("host.connect = %v from %q, want HostSelf from host-self", req.Expr, req.Source)
	}
	if req, _ := domain.RequirementOf("terminal.terminate"); !req.Expr.IsSystem() {
		t.Error("terminal.terminate must stay a system action")
	}
	if req, _ := domain.RequirementOf("typing.start"); domain.HumanGated(req.Expr) {
		t.Error("typing.start is not gated: Human is not HumanSession")
	}
}

// TestGateHumanSession (M2-39, 07 section 4.2): a requirement gated by HumanSession refuses
// agents, apps and a person with a personal token with subject_kind_not_allowed, from the
// principal alone; a person in a session passes.
func TestGateHumanSession(t *testing.T) {
	t.Parallel()
	req, _ := domain.RequirementOf("host.read")
	cases := []struct {
		name string
		p    domain.Principal
		want domain.Decision
	}{
		{"person in a session", domain.Principal{Kind: domain.KindUser, HasSession: true}, domain.Allow},
		{"personal token", domain.Principal{Kind: domain.KindUser}, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}},
		{"agent", domain.Principal{Kind: domain.KindAgent}, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}},
		{"app", domain.Principal{Kind: domain.KindApp}, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}},
	}
	for _, c := range cases {
		if got := domain.GateHumanSession(req.Expr, c.p); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	other, _ := domain.RequirementOf("message.read")
	if got := domain.GateHumanSession(other.Expr, domain.Principal{Kind: domain.KindAgent}); got != domain.Allow {
		t.Errorf("an ungated requirement is gated: %+v", got)
	}
}
