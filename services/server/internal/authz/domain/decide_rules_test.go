// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The tests of this file drive Decide, CheckLiveness, CheckPending, Visible and the requirement
// expressions in this package; the application tests reach the same rules through the
// Authorizer, but the coverage of a package counts only its own tests.

// channelPath is a channel path in space spaceS.
func channelPath() domain.Path {
	return domain.Path{Kind: domain.PathChannel, SpaceID: spaceS, ChannelID: chanC}
}

// userIn is a person with a session acting on a message of chanC with the rights eff.
func userIn(action string, req domain.Expr, eff domain.Rights) domain.Input {
	return domain.Input{
		Action:    action,
		Req:       req,
		Now:       t0,
		Principal: domain.Principal{ID: samID, Kind: domain.KindUser, HasSession: true},
		Resource:  domain.ResourceFacts{Kind: "message", ID: ids.New()},
		Path:      channelPath(),
		Eff:       eff,
	}
}

func mustDecide(t *testing.T, in domain.Input) domain.Decision {
	t.Helper()
	d, err := domain.Decide(in)
	if err != nil {
		t.Fatalf("Decide(%s): %v", in.Action, err)
	}
	return d
}

func wantDecision(t *testing.T, name string, got domain.Decision, o domain.Outcome, r domain.Reason) {
	t.Helper()
	if got.Outcome != o || got.Reason != r {
		t.Fatalf("%s: got %s/%s, want %s/%s", name, got.Outcome, got.Reason, o, r)
	}
}

func TestCheckLivenessReasons(t *testing.T) {
	t.Parallel()
	later := t0.Add(time.Hour)
	cases := []struct {
		name string
		l    domain.Liveness
		want domain.Reason
	}{
		{"unknown principal", domain.Liveness{}, domain.ReasonUnauthenticated},
		{"disabled", domain.Liveness{Status: "disabled"}, domain.ReasonPrincipalInactive},
		{"deleted", domain.Liveness{Status: "deleted"}, domain.ReasonPrincipalInactive},
		{"unknown status", domain.Liveness{Status: "archived"}, domain.ReasonPrincipalInactive},
		{"revoked session", domain.Liveness{Status: "active", HasSession: true, SessionExpiresAt: later}, domain.ReasonSessionRevoked},
		{"expired session", domain.Liveness{Status: "active", HasSession: true, SessionActive: true, SessionExpiresAt: t0}, domain.ReasonSessionRevoked},
		{"revoked device", domain.Liveness{Status: "active", HasDevice: true, DeviceState: domain.DeviceRevoked}, domain.ReasonDeviceInactive},
		{"unknown device", domain.Liveness{Status: "active", HasDevice: true, DeviceState: domain.DeviceUnknown}, domain.ReasonDeviceInactive},
		{"revoked token", domain.Liveness{Status: "active", HasToken: true}, domain.ReasonTokenRevoked},
		{"expired token", domain.Liveness{Status: "active", HasToken: true, TokenActive: true, TokenExpiresAt: t0}, domain.ReasonTokenRevoked},
		{"app without installation", domain.Liveness{Status: "active", IsApp: true}, domain.ReasonAppNotInstalled},
	}
	for _, c := range cases {
		wantDecision(t, c.name, domain.CheckLiveness(c.l, t0), domain.OutcomeUnauthenticated, c.want)
	}
	ok := []domain.Liveness{
		{Status: "active", HasSession: true, SessionActive: true, SessionExpiresAt: later, HasDevice: true, DeviceState: domain.DeviceActive},
		{Status: "pending", HasDevice: true, DeviceState: domain.DevicePending},
		{Status: "active", HasToken: true, TokenActive: true},
		{Status: "active", HasToken: true, TokenActive: true, TokenExpiresAt: later},
		{Status: "active", IsApp: true, HasInstallation: true},
	}
	for i, l := range ok {
		wantDecision(t, "live "+string(rune('a'+i)), domain.CheckLiveness(l, t0), domain.OutcomeAllow, "")
	}
}

func TestCheckPendingRules(t *testing.T) {
	t.Parallel()
	pending := domain.Liveness{Status: "pending"}
	wantDecision(t, "pending person", domain.CheckPending(pending, "message.create"), domain.OutcomeForbidden, domain.ReasonPrincipalPending)
	wantDecision(t, "pending person self.read", domain.CheckPending(pending, "self.read"), domain.OutcomeAllow, "")
	device := domain.Liveness{Status: "active", HasDevice: true, DeviceState: domain.DevicePending}
	wantDecision(t, "pending device", domain.CheckPending(device, "message.create"), domain.OutcomeForbidden, domain.ReasonDevicePending)
	wantDecision(t, "pending device list", domain.CheckPending(device, "device.list"), domain.OutcomeAllow, "")
	wantDecision(t, "active", domain.CheckPending(domain.Liveness{Status: "active"}, "message.create"), domain.OutcomeAllow, "")
}

func TestDecideVisibilityOfChannelKinds(t *testing.T) {
	t.Parallel()
	read := domain.Rights{Level: domain.LevelRead}
	wantDecision(t, "no level", mustDecide(t, userIn("message.read", domain.AtLeast(domain.LevelRead), domain.Rights{})), domain.OutcomeNotFound, domain.ReasonNoGrant)
	wantDecision(t, "read", mustDecide(t, userIn("message.read", domain.AtLeast(domain.LevelRead), read)), domain.OutcomeAllow, "")
	// A dm is visible to participants only.
	in := userIn("message.read", domain.AtLeast(domain.LevelRead), read)
	in.Path = domain.Path{Kind: domain.PathChannel, ChannelID: chanD, DM: true, DMParticipants: []ids.ID{alexID}}
	wantDecision(t, "foreign dm", mustDecide(t, in), domain.OutcomeNotFound, domain.ReasonNoGrant)
	in.Path.DMParticipants = append(in.Path.DMParticipants, samID)
	wantDecision(t, "own dm", mustDecide(t, in), domain.OutcomeAllow, "")
	// Acting for a person whose rights do not reach: the base rights would have seen it.
	in = userIn("message.read", domain.AtLeast(domain.LevelRead), domain.Rights{})
	in.Principal.ActingFor, in.BaseVisible = true, true
	wantDecision(t, "acting for", mustDecide(t, in), domain.OutcomeNotFound, domain.ReasonActingForInsufficient)
	// A second target that is invisible hides the call; a move without its target is an error.
	in = userIn("thread.move", domain.TargetHasCap(domain.CapManage), read)
	in.Target = &domain.TargetFacts{}
	wantDecision(t, "invisible target", mustDecide(t, in), domain.OutcomeNotFound, domain.ReasonNoGrant)
	in.Target = nil
	if _, err := domain.Decide(in); err == nil {
		t.Fatal("thread.move without its target decided")
	}
}

func TestVisibleByResourceKind(t *testing.T) {
	t.Parallel()
	owner := domain.Rights{Owner: true}
	base := func(kind string, id, ownerID ids.ID) domain.Input {
		return domain.Input{
			Req:       domain.Auth,
			Principal: domain.Principal{ID: samID, Kind: domain.KindUser},
			Resource:  domain.ResourceFacts{Kind: kind, ID: id, OwnerID: ownerID},
		}
	}
	type vcase struct {
		name   string
		in     domain.Input
		want   bool
		reason domain.Reason
	}
	cases := []vcase{
		{"own device", base("device", ids.New(), samID), true, ""},
		{"foreign device", base("device", ids.New(), alexID), false, domain.ReasonNotSelf},
		{"instance", base("instance", ids.ID{}, ids.ID{}), true, ""},
		{"space without level", base("space", spaceS, ids.ID{}), false, domain.ReasonNoGrant},
		{"app without owner", base("app", ids.New(), ids.ID{}), false, domain.ReasonNoGrant},
		{"person in the directory", base("user", alexID, ids.ID{}), true, ""},
		{"principal itself", base("principal", samID, ids.ID{}), true, ""},
		{"agent without facts", base("agent", agentID, ids.ID{}), false, domain.ReasonNoGrant},
		{"group of a non-member", base("group", groupG, ids.ID{}), false, domain.ReasonNoGrant},
	}
	foreignOwned := base("device", ids.New(), alexID)
	foreignOwned.Eff, foreignOwned.Req = owner, domain.Any(domain.Self, domain.Owner)
	cases = append(cases, vcase{"owner on a device by requirement", foreignOwned, true, ""})
	anon := base("instance", ids.ID{}, ids.ID{})
	anon.Principal.Kind = domain.KindAnonymous
	cases = append(cases, vcase{"anonymous instance", anon, false, domain.ReasonNoGrant})
	anon.Req = domain.Public
	cases = append(cases, vcase{"anonymous public instance", anon, true, ""})
	space := base("space", spaceS, ids.ID{})
	space.SpaceHasVisibleChannel = true
	cases = append(cases, vcase{"space with a visible channel", space, true, ""})
	app := base("app", ids.New(), ids.ID{})
	app.Eff = owner
	cases = append(cases, vcase{"app for an owner", app, true, ""})
	agentSelf := base("agent", agentID, ids.ID{})
	agentSelf.Principal = domain.Principal{ID: agentID, Kind: domain.KindAgent}
	cases = append(cases, vcase{"agent itself", agentSelf, true, ""})
	operated := base("agent", agentID, ids.ID{})
	operated.ReadOnOperatedChannel = true
	cases = append(cases, vcase{"agent of a read channel", operated, true, ""})
	foreignPerson := base("user", alexID, ids.ID{})
	foreignPerson.Principal = domain.Principal{ID: agentID, Kind: domain.KindAgent}
	cases = append(cases, vcase{"agent on a person", foreignPerson, false, domain.ReasonNoGrant})
	member := base("group", groupG, ids.ID{})
	member.GroupMember = true
	cases = append(cases, vcase{"group member", member, true, ""})
	for _, tk := range []struct {
		target domain.TargetKind
		eff    domain.Rights
		want   bool
	}{
		{domain.TargetInstance, domain.Rights{Level: domain.LevelTrigger}, false},
		{domain.TargetInstance, owner, true},
		{domain.TargetHost, owner, true},
		{domain.TargetSpace, domain.Rights{Level: domain.LevelRead}, false},
		{domain.TargetSpace, domain.Rights{Level: domain.LevelRead, Caps: domain.CapsOf(domain.CapManage)}, true},
		{domain.TargetChannel, domain.Rights{}, false},
	} {
		g := base("grant", ids.New(), ids.ID{})
		g.GrantTarget, g.Eff = tk.target, tk.eff
		reason := domain.ReasonNoGrant
		if tk.want {
			reason = ""
		}
		cases = append(cases, vcase{"grant on " + string(tk.target), g, tk.want, reason})
	}
	for _, c := range cases {
		got, reason, err := domain.Visible(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want || (!got && reason != c.reason) {
			t.Fatalf("%s: visible %v reason %q, want %v %q", c.name, got, reason, c.want, c.reason)
		}
	}
	for _, kind := range []string{"no_such_kind"} {
		if _, _, err := domain.Visible(base(kind, ids.New(), ids.ID{})); err == nil {
			t.Fatalf("kind %s has a visibility rule", kind)
		}
	}
	if _, _, err := domain.Visible(base("grant", ids.New(), ids.ID{})); err == nil {
		t.Fatal("grant without a target kind is visible")
	}
	if !domain.IsChannelPathKind("thread") || domain.IsChannelPathKind("space") {
		t.Fatal("channel path kinds")
	}
}

func TestDecideRequirementAtoms(t *testing.T) {
	t.Parallel()
	write := domain.Rights{Level: domain.LevelWrite}
	cases := []struct {
		name   string
		mutate func(in *domain.Input)
		want   domain.Outcome
		reason domain.Reason
	}{
		{"level", func(in *domain.Input) { in.Req = domain.AtLeast(domain.LevelTrigger) }, domain.OutcomeForbidden, domain.ReasonLevelInsufficient},
		{"none level", func(in *domain.Input) { in.Req = domain.AtLeast(domain.LevelNone) }, domain.OutcomeForbidden, domain.ReasonLevelInsufficient},
		{"cap", func(in *domain.Input) { in.Req = domain.HasCap(domain.CapUpload) }, domain.OutcomeForbidden, domain.ReasonCapabilityMissing},
		{"cap if without attachments", func(in *domain.Input) {
			in.Req = domain.HasCapIf(domain.CapUpload, domain.FactHasAttachments)
		}, domain.OutcomeAllow, ""},
		{"cap if with attachments", func(in *domain.Input) {
			in.Req = domain.HasCapIf(domain.CapUpload, domain.FactHasAttachments)
			in.Resource.HasAttachments = true
		}, domain.OutcomeForbidden, domain.ReasonCapabilityMissing},
		{"cap if restricted tags", func(in *domain.Input) {
			in.Req = domain.HasCapIf(domain.CapManage, domain.FactRestrictedTags)
			in.Resource.RestrictedTags = true
		}, domain.OutcomeForbidden, domain.ReasonCapabilityMissing},
		{"human", func(in *domain.Input) { in.Req = domain.Human }, domain.OutcomeAllow, ""},
		{"app kind", func(in *domain.Input) { in.Req = domain.AppKind }, domain.OutcomeForbidden, domain.ReasonSubjectKindNotAllowed},
		{"self", func(in *domain.Input) { in.Req = domain.Self }, domain.OutcomeForbidden, domain.ReasonNotSelf},
		{"self owned", func(in *domain.Input) { in.Req = domain.Self; in.Resource.OwnerID = samID }, domain.OutcomeAllow, ""},
		{"agent self for a person", func(in *domain.Input) { in.Req = domain.AgentSelf }, domain.OutcomeForbidden, domain.ReasonSubjectKindNotAllowed},
		{"app self for a person", func(in *domain.Input) { in.Req = domain.AppSelf }, domain.OutcomeForbidden, domain.ReasonSubjectKindNotAllowed},
		{"owner", func(in *domain.Input) { in.Req = domain.Owner }, domain.OutcomeForbidden, domain.ReasonOwnerOnly},
		{"participant", func(in *domain.Input) { in.Req = domain.Participant }, domain.OutcomeForbidden, domain.ReasonNoGrant},
		{"thread operator", func(in *domain.Input) { in.Req = domain.ThreadOperator }, domain.OutcomeForbidden, domain.ReasonSubjectKindNotAllowed},
		{"channel operator", func(in *domain.Input) { in.Req = domain.ChannelOperator }, domain.OutcomeForbidden, domain.ReasonSubjectKindNotAllowed},
		{"read on operated channel", func(in *domain.Input) { in.Req = domain.ReadOnOperatedChannel }, domain.OutcomeForbidden, domain.ReasonNoGrant},
		{"group member", func(in *domain.Input) { in.Req = domain.GroupMember }, domain.OutcomeForbidden, domain.ReasonNotSelf},
		{"all", func(in *domain.Input) {
			in.Req = domain.All(domain.AtLeast(domain.LevelRead), domain.HasCap(domain.CapManage))
		}, domain.OutcomeForbidden, domain.ReasonCapabilityMissing},
		{"any reports its first alternative", func(in *domain.Input) {
			in.Req = domain.Any(domain.Owner, domain.Self)
		}, domain.OutcomeForbidden, domain.ReasonOwnerOnly},
		{"any", func(in *domain.Input) { in.Req = domain.Any(domain.Owner, domain.AtLeast(domain.LevelWrite)) }, domain.OutcomeAllow, ""},
		{"by kind", func(in *domain.Input) {
			in.Req = domain.ByKind(map[string]domain.Expr{"message": domain.AtLeast(domain.LevelWrite)})
		}, domain.OutcomeAllow, ""},
		{"target cap", func(in *domain.Input) {
			in.Req = domain.TargetHasCap(domain.CapManage)
			in.Target = &domain.TargetFacts{Visible: true, Eff: domain.Rights{Caps: domain.CapsOf(domain.CapManage)}}
		}, domain.OutcomeAllow, ""},
		{"target cap missing", func(in *domain.Input) {
			in.Req = domain.TargetHasCap(domain.CapManage)
			in.Target = &domain.TargetFacts{Visible: true}
		}, domain.OutcomeForbidden, domain.ReasonCapabilityMissing},
		{"acting for", func(in *domain.Input) {
			in.Req = domain.AtLeast(domain.LevelTrigger)
			in.Principal.ActingFor, in.BaseVisible = true, true
			in.BaseEff = domain.Rights{Level: domain.LevelTrigger}
		}, domain.OutcomeForbidden, domain.ReasonActingForInsufficient},
		{"read-only channel", func(in *domain.Input) {
			in.Req = domain.AtLeast(domain.LevelWrite)
			in.Path.ReadOnly = true
		}, domain.OutcomeForbidden, domain.ReasonChannelReadOnly},
		{"read-only channel with an owner path", func(in *domain.Input) {
			in.Req = domain.Any(domain.AtLeast(domain.LevelWrite), domain.Owner)
			in.Path.ReadOnly = true
			in.Eff.Owner = true
		}, domain.OutcomeAllow, ""},
	}
	for _, c := range cases {
		in := userIn("message.create", domain.Auth, write)
		c.mutate(&in)
		wantDecision(t, c.name, mustDecide(t, in), c.want, c.reason)
	}
	// The operator atoms hold for the agent that operates the channel or the thread.
	agent := userIn("message.create", domain.ChannelOperator, write)
	agent.Principal = domain.Principal{ID: agentID, Kind: domain.KindAgent}
	agent.Path.ChannelOperatorID = agentID
	wantDecision(t, "channel operator agent", mustDecide(t, agent), domain.OutcomeAllow, "")
	agent.Req = domain.ThreadOperator
	wantDecision(t, "thread operator agent", mustDecide(t, agent), domain.OutcomeAllow, "")
	agent.Req = domain.AgentSelf
	agent.Resource.OwnerID = agentID
	wantDecision(t, "agent self", mustDecide(t, agent), domain.OutcomeAllow, "")
	// A space reads with a visible channel; a dm participant reads.
	space := userIn("space.read", domain.AtLeast(domain.LevelRead), domain.Rights{})
	space.Resource = domain.ResourceFacts{Kind: "space", ID: spaceS}
	space.Path = domain.Path{Kind: domain.PathSpace, SpaceID: spaceS}
	space.SpaceHasVisibleChannel = true
	wantDecision(t, "space via channel", mustDecide(t, space), domain.OutcomeAllow, "")
	dm := userIn("message.read", domain.Participant, domain.Rights{Level: domain.LevelWrite})
	dm.Path = domain.Path{Kind: domain.PathChannel, ChannelID: chanD, DM: true, DMParticipants: []ids.ID{samID}}
	wantDecision(t, "participant", mustDecide(t, dm), domain.OutcomeAllow, "")
}

func TestDecideRequirementErrors(t *testing.T) {
	t.Parallel()
	write := domain.Rights{Level: domain.LevelWrite}
	for name, req := range map[string]domain.Expr{
		"invalid":      {},
		"deferred":     domain.Deferred("M2"),
		"system":       domain.System,
		"unknown kind": domain.ByKind(map[string]domain.Expr{"thread": domain.Auth}),
		"target cap":   domain.All(domain.Auth, domain.TargetHasCap(domain.CapUpload)),
		"preview":      domain.Preview,
	} {
		in := userIn("message.create", req, write)
		if _, err := domain.Decide(in); err == nil {
			t.Fatalf("%s: decided without an error", name)
		}
	}
	// access.preview evaluates the previewed action, which must change rights.
	in := userIn("access.preview", domain.Preview, write)
	in.Preview = &domain.PreviewFacts{Action: "message.create", Req: domain.Auth}
	if _, err := domain.Decide(in); err == nil {
		t.Fatal("preview of an action without rights decided")
	}
	in.Preview = &domain.PreviewFacts{Action: "grant.create", Spec: domain.ActionSpec{Rights: true}, Req: domain.Preview}
	if _, err := domain.Decide(in); err == nil {
		t.Fatal("preview of a preview decided")
	}
	in.Preview = &domain.PreviewFacts{Action: "grant.create", Spec: domain.ActionSpec{Rights: true}, Req: domain.Owner}
	wantDecision(t, "preview", mustDecide(t, in), domain.OutcomeForbidden, domain.ReasonOwnerOnly)
}

func TestDecidePrincipalRules(t *testing.T) {
	t.Parallel()
	write := domain.Rights{Level: domain.LevelWrite}
	app := userIn("message.create", domain.Auth, write)
	app.Principal = domain.Principal{ID: ids.New(), Kind: domain.KindApp}
	if r := domain.CheckAppScope("message.create", nil, nil, false); r != "" {
		wantDecision(t, "app scope", mustDecide(t, app), domain.OutcomeForbidden, r)
	}
	if _, listed := domain.AppScopeOf("no.such.action"); listed || len(domain.ScopeTableActions()) == 0 {
		t.Fatal("scope table")
	}
	if r := domain.CheckAppScope("no.such.action", nil, nil, false); r != domain.ReasonSubjectKindNotAllowed {
		t.Fatalf("unlisted action: %q", r)
	}
	view := userIn("message.create", domain.Auth, write)
	view.Principal.ViewAs = true
	wantDecision(t, "view as", mustDecide(t, view), domain.OutcomeForbidden, domain.ReasonViewAsReadOnly)
	step := userIn("message.create", domain.Auth, write)
	step.Spec.StepUp, step.StepUpMaxAge = true, 5*time.Minute
	wantDecision(t, "no step-up", mustDecide(t, step), domain.OutcomeStepUpRequired, domain.ReasonStepUpRequired)
	step.Principal.StepUpAt = t0.Add(-10 * time.Minute)
	wantDecision(t, "old step-up", mustDecide(t, step), domain.OutcomeStepUpRequired, domain.ReasonStepUpRequired)
	step.Principal.StepUpAt = t0.Add(-time.Minute)
	wantDecision(t, "fresh step-up", mustDecide(t, step), domain.OutcomeAllow, "")
	step.Principal.HasSession = false
	wantDecision(t, "step-up without a session", mustDecide(t, step), domain.OutcomeForbidden, domain.ReasonStepUpNotPossible)
	// A pending device lists only itself.
	pend := userIn("device.list", domain.Auth, write)
	pend.Principal.Pending, pend.Principal.DeviceID = domain.ReasonDevicePending, ids.New()
	pend.Resource = domain.ResourceFacts{Kind: "device", ID: ids.New(), OwnerID: samID}
	wantDecision(t, "pending device on another", mustDecide(t, pend), domain.OutcomeForbidden, domain.ReasonDevicePending)
	pend.Resource.ID = pend.Principal.DeviceID
	wantDecision(t, "pending device on itself", mustDecide(t, pend), domain.OutcomeAllow, "")
}

func TestDecideGrantGuards(t *testing.T) {
	t.Parallel()
	shape := domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelTrigger}
	in := userIn("grant.create", domain.Auth, domain.Rights{Level: domain.LevelWrite})
	if _, err := domain.Decide(in); err == nil {
		t.Fatal("grant.create without the grant decided")
	}
	in.Grant = &domain.GrantFacts{Shape: shape}
	wantDecision(t, "above own level", mustDecide(t, in), domain.OutcomeForbidden, domain.ReasonDelegationExceedsOwnLevel)
	in.Eff.Level = domain.LevelTrigger
	wantDecision(t, "own level", mustDecide(t, in), domain.OutcomeAllow, "")
	in.Action = "grant.revoke"
	wantDecision(t, "revoke", mustDecide(t, in), domain.OutcomeAllow, "")
	in.Action, in.Grant.Shape.TargetKind = "grant.revoke", domain.TargetInstance
	wantDecision(t, "revoke on the instance", mustDecide(t, in), domain.OutcomeForbidden, domain.ReasonOwnerOnly)
}

// invokeIn is agent.invoke by sam on the main thread of chanC operated by agentID.
func invokeIn() domain.Input {
	in := userIn("agent.invoke", domain.Auth, domain.Rights{Level: domain.LevelTrigger})
	in.Path.ChannelOperatorID = agentID
	in.Invoke = &domain.InvokeFacts{
		AgentID: agentID, TriggerOrigin: domain.OriginHuman, AgentFound: true, AgentKind: domain.KindAgent,
		AgentStatus: "active", PolicyFound: true, BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat,
	}
	return in
}

func TestDecideInvokeGuard(t *testing.T) {
	t.Parallel()
	wantDecision(t, "operator", mustDecide(t, invokeIn()), domain.OutcomeAllow, "")
	missing := invokeIn()
	missing.Invoke = nil
	if _, err := domain.Decide(missing); err == nil {
		t.Fatal("agent.invoke without its trigger decided")
	}
	cases := []struct {
		name   string
		mutate func(in *domain.Input)
		reason domain.Reason
	}{
		{"agent origin", func(in *domain.Input) { in.Invoke.TriggerOrigin = "agent" }, domain.ReasonOriginNotHuman},
		{"dm", func(in *domain.Input) {
			in.Path = domain.Path{Kind: domain.PathChannel, ChannelID: chanD, DM: true, DMParticipants: []ids.ID{samID}}
		}, domain.ReasonAgentNotReachable},
		{"inactive agent", func(in *domain.Input) { in.Invoke.AgentStatus = "disabled" }, domain.ReasonAgentNotReachable},
		{"mention required", func(in *domain.Input) { in.Invoke.RequireMention = true }, domain.ReasonAgentNotReachable},
		{"personal billing", func(in *domain.Input) { in.Invoke.BillingMode = domain.BillingPersonal }, domain.ReasonBillingModePersonalNotHolder},
		{"host tool class", func(in *domain.Input) { in.Invoke.ReportedToolClass = domain.ToolHost }, domain.ReasonToolClassOwnerOnly},
		{"unknown tool class", func(in *domain.Input) { in.Invoke.ToolClass = "other" }, domain.ReasonToolClassOwnerOnly},
		{"project not ready", func(in *domain.Input) { in.Invoke.ProjectBound = true }, domain.ReasonProjectNotReady},
		{"dev environment", func(in *domain.Input) { in.Invoke.ProjectBound, in.Invoke.ProjectReady = true, true }, domain.ReasonDevEnvironmentNotReady},
		{"other agent running", func(in *domain.Input) { in.Invoke.ActiveRunAgentID = ids.New() }, domain.ReasonRunActiveOtherAgent},
	}
	for _, c := range cases {
		in := invokeIn()
		c.mutate(&in)
		wantDecision(t, c.name, mustDecide(t, in), domain.OutcomeForbidden, c.reason)
	}
	// A scoped tool class, a holder of personal billing, a ready dev environment and the own run
	// pass; a guest needs a mention, allowed mentioned agents, write and organization billing.
	ok := invokeIn()
	ok.Invoke.ToolClass, ok.Invoke.BillingMode, ok.Invoke.CredentialHolderID = domain.ToolScoped, domain.BillingPersonal, samID
	ok.Invoke.ActiveRunAgentID = agentID
	wantDecision(t, "holder", mustDecide(t, ok), domain.OutcomeAllow, "")
	owner := invokeIn()
	owner.Eff.Owner, owner.Invoke.ToolClass, owner.Invoke.ProjectBound, owner.Invoke.ProjectReady = true, domain.ToolHost, true, true
	wantDecision(t, "owner with a host tool", mustDecide(t, owner), domain.OutcomeAllow, "")
	guest := invokeIn()
	guest.Path.ChannelOperatorID = ids.New()
	wantDecision(t, "guest without mention", mustDecide(t, guest), domain.OutcomeForbidden, domain.ReasonAgentNotReachable)
	guest.Invoke.MentionsAgent, guest.Invoke.AllowMentionedAgents, guest.Invoke.AgentLevel = true, true, domain.LevelWrite
	wantDecision(t, "guest", mustDecide(t, guest), domain.OutcomeAllow, "")
	thread := invokeIn()
	thread.Path.ThreadID, thread.Path.ThreadOperatorID, thread.Invoke.ThreadRequireMention = ids.New(), agentID, true
	wantDecision(t, "thread needs a mention", mustDecide(t, thread), domain.OutcomeForbidden, domain.ReasonAgentNotReachable)
	thread.Invoke.FreeResponse = true
	wantDecision(t, "free response", mustDecide(t, thread), domain.OutcomeAllow, "")
}

func TestExprRendering(t *testing.T) {
	t.Parallel()
	e := domain.All(domain.AtLeast(domain.LevelWrite), domain.Any(domain.Self, domain.Owner),
		domain.HasCap(domain.CapUpload), domain.HasCapIf(domain.CapUpload, domain.FactHasAttachments),
		domain.TargetHasCap(domain.CapManage), domain.ByKind(map[string]domain.Expr{"user": domain.Human}),
		domain.Deferred("M2"))
	want := "All(Level(write),Any(Self,Owner),Cap(upload),CapIf(upload),TargetCap(manage),ByKind(user:Human),Deferred(M2))"
	if got := e.String(); got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
	if (domain.Expr{}).String() != "Invalid" {
		t.Fatal("zero Expr renders")
	}
	if !e.Contains(domain.Human) || !e.Contains(domain.Owner) || e.Contains(domain.GroupMember) {
		t.Fatal("Contains")
	}
	if !domain.System.IsSystem() || domain.Auth.IsSystem() {
		t.Fatal("IsSystem")
	}
	if m, ok := domain.Deferred("M3").DeferredTo(); !ok || m != "M3" {
		t.Fatal("DeferredTo")
	}
	if !domain.AtLeast(domain.LevelRead).IsReadLevel() || domain.AtLeast(domain.LevelWrite).IsReadLevel() {
		t.Fatal("IsReadLevel")
	}
	for _, id := range domain.DeferredActions() {
		if r, ok := domain.RequirementOf(id); !ok {
			t.Fatalf("deferred %s without an entry", id)
		} else if _, deferred := r.Expr.DeferredTo(); !deferred {
			t.Fatalf("%s is not deferred", id)
		}
	}
	if !domain.Writes("no.such.action") || domain.Writes("audit.read") {
		t.Fatal("Writes")
	}
}
