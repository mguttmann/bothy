// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// input returns the decision input of action on a resource of kind with the principal p and the
// rights eff, at t0 with a step-up age of five minutes.
func input(t *testing.T, action, kind string, p domain.Principal, eff domain.Rights) domain.Input {
	t.Helper()
	spec, ok := domain.LookupAction(action)
	req, ok2 := domain.RequirementOf(action)
	if !ok || !ok2 {
		t.Fatalf("unknown action %s", action)
	}
	path := domain.Path{Kind: domain.PathNone}
	if kind == "channel" || kind == "message" {
		path = domain.Path{Kind: domain.PathChannel, SpaceID: spaceS, ChannelID: chanC}
	}
	if kind == "instance" {
		path = domain.Path{Kind: domain.PathInstance}
	}
	return domain.Input{Action: action, Spec: spec, Req: req.Expr, Now: t0, StepUpMaxAge: 5 * time.Minute, Principal: p,
		Resource: domain.ResourceFacts{Kind: kind, ID: ids.New()}, Path: path, Eff: eff, BaseEff: eff}
}

// TestDecide_Hosts (M2-39, 07 sections 2.12 and 4.2): host.read and host.manage decided from the
// host facts: visibility (owner, holder, terminal), the atoms Assignee and Terminal, owner_only,
// step-up, "view as" read-only, and a host resource without its facts is an internal error.
func TestDecide_Hosts(t *testing.T) {
	t.Parallel()
	person := domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true, StepUpAt: t0}
	owner := domain.Rights{Owner: true}
	cases := []struct {
		name   string
		action string
		p      domain.Principal
		eff    domain.Rights
		host   *domain.HostFacts
		want   domain.Decision
	}{
		{"owner reads", "host.read", person, owner, &domain.HostFacts{Visible: true}, domain.Allow},
		{"holder reads", "host.read", person, domain.Rights{}, &domain.HostFacts{Visible: true, Assignee: true}, domain.Allow},
		{"terminal reads", "host.read", person, domain.Rights{}, &domain.HostFacts{Visible: true, Terminal: true}, domain.Allow},
		{"stranger", "host.read", person, domain.Rights{}, &domain.HostFacts{}, domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNoGrant}},
		{"visible without an atom", "host.read", person, domain.Rights{}, &domain.HostFacts{Visible: true},
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonOwnerOnly}},
		{"holder manages", "host.manage", person, domain.Rights{}, &domain.HostFacts{Visible: true, Assignee: true},
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonOwnerOnly}},
		{"owner manages", "host.manage", person, owner, &domain.HostFacts{Visible: true}, domain.Allow},
		{"owner with a stale step-up", "host.manage", domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true, StepUpAt: t0.Add(-time.Hour)},
			owner, &domain.HostFacts{Visible: true}, domain.Decision{Outcome: domain.OutcomeStepUpRequired, Reason: domain.ReasonStepUpRequired}},
		{"owner without a step-up", "host.manage", domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true}, owner,
			&domain.HostFacts{Visible: true}, domain.Decision{Outcome: domain.OutcomeStepUpRequired, Reason: domain.ReasonStepUpRequired}},
		{"view as writes", "host.manage", domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true, StepUpAt: t0, ViewAs: true}, owner,
			&domain.HostFacts{Visible: true}, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonViewAsReadOnly}},
		{"personal token", "host.read", domain.Principal{ID: alexID, Kind: domain.KindUser}, owner, &domain.HostFacts{Visible: true},
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}},
	}
	for _, c := range cases {
		in := input(t, c.action, "host", c.p, c.eff)
		in.Host = c.host
		got, err := domain.Decide(in)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	in := input(t, "host.read", "host", person, owner)
	if _, err := domain.Decide(in); err == nil {
		t.Fatal("a host resource without its facts decided")
	}
	// grant.create on a host: only the owner, the holder gets owner_only.
	g := input(t, "grant.create", "host", person, domain.Rights{})
	g.Host = &domain.HostFacts{Visible: true, Assignee: true}
	g.Grant = &domain.GrantFacts{Shape: domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetHost, Caps: domain.CapsOf(domain.CapTerminal)}}
	if got, err := domain.Decide(g); err != nil || got.Reason != domain.ReasonOwnerOnly {
		t.Fatalf("grant.create by the holder: %+v, %v", got, err)
	}
}

// TestDecide_Rules (M2-39 regression of the common steps): requirement reasons, the read-only and
// archived channel, step-up not possible for an agent, the pending device list and an evaluable
// requirement only.
func TestDecide_Rules(t *testing.T) {
	t.Parallel()
	person := domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true, StepUpAt: t0}
	read := domain.Rights{Level: domain.LevelRead}
	write := domain.Rights{Level: domain.LevelWrite, Caps: domain.CapsOf(domain.CapUpload)}
	cases := []struct {
		name   string
		action string
		kind   string
		p      domain.Principal
		eff    domain.Rights
		mod    func(*domain.Input)
		want   domain.Decision
	}{
		{"read allowed", "channel.read", "channel", person, read, nil, domain.Allow},
		{"no read", "channel.read", "channel", person, domain.Rights{}, nil, domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNoGrant}},
		{"write missing", "thread.create", "channel", person, read, nil, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonLevelInsufficient}},
		{"manage missing", "channel.update", "channel", person, write, nil, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonCapabilityMissing}},
		{"read-only channel", "thread.create", "channel", person, write, func(in *domain.Input) { in.Path.ReadOnly = true },
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonChannelReadOnly}},
		{"archived channel", "thread.create", "channel", person, write, func(in *domain.Input) { in.Path.Archived = true },
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonChannelArchived}},
		{"agent cannot step up", "export.channel", "channel", domain.Principal{ID: agentID, Kind: domain.KindAgent},
			domain.Rights{Level: domain.LevelRead, Caps: domain.CapsOf(domain.CapExport)}, nil,
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonStepUpNotPossible}},
		{"human only", "dm.create", "instance", domain.Principal{ID: agentID, Kind: domain.KindAgent}, domain.Rights{}, nil,
			domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}},
		{"public", "instance.discovery.read", "instance", domain.Principal{Kind: domain.KindAnonymous}, domain.Rights{}, nil, domain.Allow},
	}
	for _, c := range cases {
		in := input(t, c.action, c.kind, c.p, c.eff)
		if c.mod != nil {
			c.mod(&in)
		}
		got, err := domain.Decide(in)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	// A deferred requirement never decides.
	in := input(t, "host.connect", "host", person, domain.Rights{Owner: true})
	in.Host = &domain.HostFacts{Visible: true}
	if _, err := domain.Decide(in); err == nil {
		t.Fatal("a deferred requirement decided")
	}
}

// TestHostPairRequirement (M2-41, Design note 10): host.pair is Public with the source text of 05;
// an anonymous principal and an authenticated person on a host they cannot see are allowed for
// host.pair, while the same host stays invisible to them for host.read.
func TestHostPairRequirement(t *testing.T) {
	t.Parallel()
	req, ok := domain.RequirementOf("host.pair")
	if !ok || req.Expr.String() != domain.Public.String() || req.Source != "public (gültiger Kopplungs-Code)" {
		t.Fatalf("host.pair: %v %q", req.Expr, req.Source)
	}
	for _, p := range []domain.Principal{
		{Kind: domain.KindAnonymous},
		{ID: alexID, Kind: domain.KindUser, HasSession: true, StepUpAt: t0},
	} {
		in := input(t, "host.pair", "host", p, domain.Rights{})
		in.Host = &domain.HostFacts{}
		if got, err := domain.Decide(in); err != nil || got != domain.Allow {
			t.Errorf("%s: host.pair %+v, %v", p.Kind, got, err)
		}
		if p.Kind == domain.KindAnonymous {
			continue
		}
		read := input(t, "host.read", "host", p, domain.Rights{})
		read.Host = &domain.HostFacts{}
		if got, _ := domain.Decide(read); got.Outcome != domain.OutcomeNotFound {
			t.Errorf("%s: host.read %+v, want not_found", p.Kind, got)
		}
	}
}
