// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The people of the terminal tests (M2-40).
var (
	tOwner = ids.MustParse("019a0000-0000-7000-8000-000000000601")
	tAlex  = ids.MustParse("019a0000-0000-7000-8000-000000000602")
	tSam   = ids.MustParse("019a0000-0000-7000-8000-000000000603")
	tRobin = ids.MustParse("019a0000-0000-7000-8000-000000000604")
)

// freshAge and reattachWindow stand for terminal.step_up_max_age and
// terminal.quick_reattach_window in the tests.
const (
	freshAge       = 2 * time.Minute
	reattachWindow = 5 * time.Minute
)

var tNow = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// TestFreshStepUp (08 section 6.3 points 1, 2, 4, ID-T28): device_key and passkey are fresh,
// password_totp and oidc_reauth never; the age edge at terminal.step_up_max_age; a consumed
// step-up only counts again after a newer one; in the console only a passkey whose stored WebAuthn
// session required user verification (08 section 6.3 point 3, M6-18).
func TestFreshStepUp(t *testing.T) {
	t.Parallel()
	at := tNow.Add(-time.Minute)
	cases := map[string]struct {
		f    domain.FreshFacts
		want bool
	}{
		"device_key fresh":            {domain.FreshFacts{StepUpAt: at, StepUpMethod: "device_key", ClientComponent: "app"}, true},
		"passkey fresh":               {domain.FreshFacts{StepUpAt: at, StepUpMethod: "passkey", ClientComponent: "app"}, true},
		"password_totp fresh":         {domain.FreshFacts{StepUpAt: at, StepUpMethod: "password_totp", ClientComponent: "app"}, false},
		"oidc_reauth fresh":           {domain.FreshFacts{StepUpAt: at, StepUpMethod: "oidc_reauth", ClientComponent: "app"}, false},
		"age exactly max":             {domain.FreshFacts{StepUpAt: tNow.Add(-freshAge), StepUpMethod: "device_key"}, true},
		"one nanosecond older":        {domain.FreshFacts{StepUpAt: tNow.Add(-freshAge - time.Nanosecond), StepUpMethod: "device_key"}, false},
		"no step-up":                  {domain.FreshFacts{StepUpMethod: "device_key"}, false},
		"consumed at the step-up":     {domain.FreshFacts{StepUpAt: at, StepUpConsumedAt: at, StepUpMethod: "device_key"}, false},
		"consumed before a newer one": {domain.FreshFacts{StepUpAt: at, StepUpConsumedAt: at.Add(-time.Second), StepUpMethod: "device_key"}, true},
		"consumed after the step-up":  {domain.FreshFacts{StepUpAt: at, StepUpConsumedAt: at.Add(time.Second), StepUpMethod: "device_key"}, false},
		"console with a fresh passkey without user verification": {domain.FreshFacts{StepUpAt: at, StepUpMethod: "passkey", ClientComponent: "console"}, false},
		"console with a fresh passkey with user verification": {
			domain.FreshFacts{StepUpAt: at, StepUpMethod: "passkey", StepUpUserVerified: true, ClientComponent: "console"}, true},
		"console with a device key": {
			domain.FreshFacts{StepUpAt: at, StepUpMethod: "device_key", StepUpUserVerified: true, ClientComponent: "console"}, false},
		"console with password_totp": {domain.FreshFacts{StepUpAt: at, StepUpMethod: "password_totp", ClientComponent: "console"}, false},
		"console with an old passkey": {
			domain.FreshFacts{StepUpAt: tNow.Add(-freshAge - time.Second), StepUpMethod: "passkey", StepUpUserVerified: true, ClientComponent: "console"}, false},
		"console with a consumed passkey": {
			domain.FreshFacts{StepUpAt: at, StepUpConsumedAt: at, StepUpMethod: "passkey", StepUpUserVerified: true, ClientComponent: "console"}, false},
		"share extension with a passkey": {domain.FreshFacts{StepUpAt: at, StepUpMethod: "passkey", ClientComponent: "share_ext"}, true},
	}
	for name, c := range cases {
		if got := domain.FreshStepUp(c.f, tNow, freshAge); got != c.want {
			t.Errorf("%s: FreshStepUp = %v, want %v", name, got, c.want)
		}
	}
}

// TestConsolePasskeyRequired (08 section 6.3 point 3, ID-T28, TM-T12, M6-18): in the console every
// refused fresh step-up answers with reason passkey_required (missing, not a UV passkey, old or
// consumed: the only way on is a new UV passkey); outside the console never.
func TestConsolePasskeyRequired(t *testing.T) {
	t.Parallel()
	at := tNow.Add(-time.Minute)
	cases := map[string]struct {
		f    domain.FreshFacts
		want bool
	}{
		"console without a step-up":         {domain.FreshFacts{ClientComponent: "console"}, true},
		"console with password_totp":        {domain.FreshFacts{StepUpAt: at, StepUpMethod: "password_totp", ClientComponent: "console"}, true},
		"console with oidc_reauth":          {domain.FreshFacts{StepUpAt: at, StepUpMethod: "oidc_reauth", ClientComponent: "console"}, true},
		"console with a passkey without uv": {domain.FreshFacts{StepUpAt: at, StepUpMethod: "passkey", ClientComponent: "console"}, true},
		"console with a device key":         {domain.FreshFacts{StepUpAt: at, StepUpMethod: "device_key", StepUpUserVerified: true, ClientComponent: "console"}, true},
		"console with an old uv passkey": {
			domain.FreshFacts{StepUpAt: tNow.Add(-time.Hour), StepUpMethod: "passkey", StepUpUserVerified: true, ClientComponent: "console"}, true},
		"console with a consumed uv passkey": {
			domain.FreshFacts{StepUpAt: at, StepUpConsumedAt: at, StepUpMethod: "passkey", StepUpUserVerified: true, ClientComponent: "console"}, true},
		"app with password_totp": {domain.FreshFacts{StepUpAt: at, StepUpMethod: "password_totp", ClientComponent: "app"}, false},
		"app without a step-up":  {domain.FreshFacts{ClientComponent: "app"}, false},
	}
	for name, c := range cases {
		if got := domain.ConsolePasskeyRequired(c.f); got != c.want {
			t.Errorf("%s: ConsolePasskeyRequired = %v, want %v", name, got, c.want)
		}
	}
}

// TestQuickReattach (08 section 6.3 point 5, 24 section 6.2): only terminal.attach of a detached
// session from the same device and login session within terminal.quick_reattach_window, never in
// the console.
func TestQuickReattach(t *testing.T) {
	t.Parallel()
	dev, sess := ids.New(), ids.New()
	base := domain.ReattachFacts{
		Action: "terminal.attach", PrincipalDeviceID: dev, PrincipalSessionID: sess, ClientComponent: "app",
		Session: domain.TerminalSessionFacts{SessionState: "detached", SessionDeviceID: dev, SessionAuthSessionID: sess, DetachedAt: tNow.Add(-reattachWindow)},
	}
	cases := map[string]struct {
		edit func(*domain.ReattachFacts)
		want bool
	}{
		"exactly the window":    {func(*domain.ReattachFacts) {}, true},
		"one nanosecond more":   {func(f *domain.ReattachFacts) { f.Session.DetachedAt = f.Session.DetachedAt.Add(-time.Nanosecond) }, false},
		"another device":        {func(f *domain.ReattachFacts) { f.PrincipalDeviceID = ids.New() }, false},
		"another login session": {func(f *domain.ReattachFacts) { f.PrincipalSessionID = ids.New() }, false},
		"state active":          {func(f *domain.ReattachFacts) { f.Session.SessionState = "active" }, false},
		"terminal.open":         {func(f *domain.ReattachFacts) { f.Action = "terminal.open" }, false},
		"console":               {func(f *domain.ReattachFacts) { f.ClientComponent = "console" }, false},
		"no detachment time":    {func(f *domain.ReattachFacts) { f.Session.DetachedAt = time.Time{} }, false},
	}
	for name, c := range cases {
		f := base
		c.edit(&f)
		if got := domain.QuickReattach(f, tNow, reattachWindow); got != c.want {
			t.Errorf("%s: QuickReattach = %v, want %v", name, got, c.want)
		}
	}
}

// guardHosts are the hosts of the guard test: R1 assigned to Alex (organization), R2 personal
// with holder Robin, R3 assigned to Sam and declared as the server host.
var guardHosts = map[string]domain.HostFacts{
	"R1": {ID: hostR1, AssignedUserID: tAlex, LLMCredentials: "organization"},
	"R2": {ID: hostR2, LLMCredentials: "personal", CredentialHolderID: tRobin},
	"R3": {ID: hostR3, AssignedUserID: tSam, LLMCredentials: "organization", DeclaredServerHost: true},
}

// su is a reported session user of the host.
func su(host ids.ID, reported bool) *domain.SessionUserFact {
	return &domain.SessionUserFact{HostID: host, HostSessionUserID: ids.New(), OSUser: "bothy-s-x", Reported: reported}
}

// TestTerminalGuard (07 SR10 to SR12, AZ-M55, AZ-M56, AZ-M61, TM-T08): each rule alone and the
// order SR10 server host, SR10 assignee, SR11, SR12.
func TestTerminalGuard(t *testing.T) {
	t.Parallel()
	r3Reported := guardHosts["R3"]
	r3Reported.DeclaredServerHost, r3Reported.ReportedIsServerHost = false, true
	cases := map[string]struct {
		f    domain.TerminalGuardFacts
		want domain.Reason
	}{
		"Sam on the server host assigned to him": {domain.TerminalGuardFacts{PrincipalID: tSam, Host: guardHosts["R3"], SessionUser: su(hostR3, true)},
			domain.ReasonTerminalServerHostOwnerOnly},
		"Sam on a reported server host": {domain.TerminalGuardFacts{PrincipalID: tSam, Host: r3Reported, SessionUser: su(hostR3, true)},
			domain.ReasonTerminalServerHostOwnerOnly},
		"Sam on R1, not assigned": {domain.TerminalGuardFacts{PrincipalID: tSam, Host: guardHosts["R1"], SessionUser: su(hostR1, true)},
			domain.ReasonTerminalNotHostAssignee},
		"Alex on R1": {domain.TerminalGuardFacts{PrincipalID: tAlex, Host: guardHosts["R1"], SessionUser: su(hostR1, true)}, ""},
		"owner on R3 passes SR10": {domain.TerminalGuardFacts{PrincipalID: tOwner, Owner: true, Host: guardHosts["R3"], SessionUser: su(hostR3, true)},
			""},
		"owner not the holder on R2": {domain.TerminalGuardFacts{PrincipalID: tOwner, Owner: true, Host: guardHosts["R2"], SessionUser: su(hostR2, true)},
			domain.ReasonI6HolderOnly},
		"holder Robin on R2 as owner": {domain.TerminalGuardFacts{PrincipalID: tRobin, Owner: true, Host: guardHosts["R2"], SessionUser: su(hostR2, true)},
			""},
		"Robin on R2 without assignment of the host": {domain.TerminalGuardFacts{PrincipalID: tRobin, Host: guardHosts["R2"], SessionUser: su(hostR2, true)},
			domain.ReasonTerminalNotHostAssignee},
		"owner without assignment": {domain.TerminalGuardFacts{PrincipalID: tOwner, Owner: true, Host: guardHosts["R1"]},
			domain.ReasonTerminalNoSessionUser},
		"owner with an unreported assignment": {domain.TerminalGuardFacts{PrincipalID: tOwner, Owner: true, Host: guardHosts["R1"], SessionUser: su(hostR1, false)},
			domain.ReasonTerminalNoSessionUser},
		"assignee with an unreported assignment": {domain.TerminalGuardFacts{PrincipalID: tAlex, Host: guardHosts["R1"], SessionUser: su(hostR1, false)},
			domain.ReasonTerminalNoSessionUser},
		"assignee with an assignment of another host": {domain.TerminalGuardFacts{PrincipalID: tAlex, Host: guardHosts["R1"], SessionUser: su(hostR2, true)},
			domain.ReasonTerminalNoSessionUser},
		"server host before the assignee": {domain.TerminalGuardFacts{PrincipalID: tAlex, Host: guardHosts["R3"]},
			domain.ReasonTerminalServerHostOwnerOnly},
		"no assignee: owners only": {domain.TerminalGuardFacts{PrincipalID: tAlex, Host: domain.HostFacts{ID: hostR4, LLMCredentials: "none"}, SessionUser: su(hostR4, true)},
			domain.ReasonTerminalNotHostAssignee},
	}
	for name, c := range cases {
		reason, ok := domain.TerminalGuard(c.f)
		if reason != c.want || ok != (c.want == "") {
			t.Errorf("%s: TerminalGuard = %q, %v; want %q", name, reason, ok, c.want)
		}
	}
}

// TestTerminalEffective (07 section 2.12 point 3): an owner on an organization host and the holder
// on a personal host hold terminal, another owner on a personal host does not; a person through a
// grant only.
func TestTerminalEffective(t *testing.T) {
	t.Parallel()
	owner := domain.Snapshot{PrincipalID: tOwner, Owner: true, Hosts: map[ids.ID]domain.Caps{}, PersonalHosts: map[ids.ID]ids.ID{hostR2: tRobin}}
	holder := domain.Snapshot{PrincipalID: tRobin, Owner: true, Hosts: map[ids.ID]domain.Caps{}, PersonalHosts: map[ids.ID]ids.ID{hostR2: tRobin}}
	alex := domain.Snapshot{PrincipalID: tAlex, Hosts: map[ids.ID]domain.Caps{hostR1: domain.CapsOf(domain.CapTerminal)}}
	cases := map[string]struct {
		s    domain.Snapshot
		host ids.ID
		want bool
	}{
		"owner on organization":        {owner, hostR1, true},
		"holder on personal":           {holder, hostR2, true},
		"owner not holder on personal": {owner, hostR2, false},
		"person with a grant":          {alex, hostR1, true},
		"person without a grant":       {alex, hostR2, false},
	}
	for name, c := range cases {
		if got := domain.TerminalEffective(c.s, c.host); got != c.want {
			t.Errorf("%s: TerminalEffective = %v, want %v", name, got, c.want)
		}
	}
	if !domain.TerminalOn(owner, hostR2) {
		t.Error("TerminalOn keeps the owner part on a personal host (SR11 denies, Design note 1)")
	}
}

// TestCheckI6 (03 section 6.8, AZ-M57): an organization host and a personal host with only the
// holder hold; another user, an owner who is not the holder and a group, with and without
// members, violate with the principal ids of Design note 9.
func TestCheckI6(t *testing.T) {
	t.Parallel()
	leser, empty := ids.New(), ids.New()
	personal := guardHosts["R2"]
	members := map[ids.ID][]ids.ID{leser: {tSam, tRobin}}
	user := func(id ids.ID) domain.HostGrant { return domain.HostGrant{SubjectKind: "user", SubjectID: id} }
	group := func(id ids.ID) domain.HostGrant { return domain.HostGrant{SubjectKind: "group", SubjectID: id} }
	cases := map[string]struct {
		host   domain.HostFacts
		grants []domain.HostGrant
		want   []ids.ID
		bad    bool
	}{
		"organization host":            {guardHosts["R1"], []domain.HostGrant{user(tSam), group(leser)}, nil, false},
		"personal without grants":      {personal, nil, nil, false},
		"grant to the holder":          {personal, []domain.HostGrant{user(tRobin)}, nil, false},
		"grant to another user":        {personal, []domain.HostGrant{user(tSam), user(tRobin)}, []ids.ID{tSam}, true},
		"grant to an owner not holder": {personal, []domain.HostGrant{user(tOwner)}, []ids.ID{tOwner}, true},
		"group with members":           {personal, []domain.HostGrant{group(leser)}, []ids.ID{tSam}, true},
		"group without members":        {personal, []domain.HostGrant{group(empty)}, []ids.ID{}, true},
		"user and group, unique":       {personal, []domain.HostGrant{user(tSam), group(leser)}, []ids.ID{tSam}, true},
	}
	for name, c := range cases {
		got, bad := domain.CheckI6(c.host, c.grants, members)
		if bad != c.bad {
			t.Errorf("%s: violated = %v, want %v", name, bad, c.bad)
			continue
		}
		if !bad {
			continue
		}
		if got.HostID != c.host.ID || got.CredentialHolderID != c.host.CredentialHolderID || !slices.Equal(got.PrincipalIDs, domain.SortIDs(c.want)) {
			t.Errorf("%s: conflict %+v, want principals %v", name, got, c.want)
		}
		if got.PrincipalIDs == nil {
			t.Errorf("%s: principal ids must be a list, not nil", name)
		}
	}
}

// TestRequirementSources_Terminal (Design note 1 of M2-40, AZ-P3): the five terminal actions are
// real requirements behind the gate of HumanSession, with the 05 texts as their sources.
func TestRequirementSources_Terminal(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]struct{ source, expr string }{
		"terminal.open":     {"human and +terminal", "All(HumanSession,Terminal)"},
		"terminal.attach":   {"self and +terminal", "All(HumanSession,Self,Terminal)"},
		"terminal.stream":   {"self", "All(HumanSession,Self)"},
		"terminal.close":    {"self or owner", "All(HumanSession,Any(Self,Owner))"},
		"terminal.read_own": {"self or owner", "All(HumanSession,ByKind(host:Auth,terminal_session:Any(Self,Owner)))"},
	} {
		req, ok := domain.RequirementOf(id)
		if !ok || req.Source != want.source || req.Expr.String() != want.expr {
			t.Errorf("%s = %q %s, want %q %s", id, req.Source, req.Expr, want.source, want.expr)
		}
		if !domain.HumanGated(req.Expr) {
			t.Errorf("%s is not behind the gate of HumanSession", id)
		}
		if spec, _ := domain.LookupAction(id); spec.Right != req.Source {
			t.Errorf("%s: source %q differs from 05 %q", id, req.Source, spec.Right)
		}
	}
}
