// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The terminal windows of the test worlds (terminal.step_up_max_age and
// terminal.quick_reattach_window).
const (
	termFresh    = time.Minute
	termReattach = time.Minute
)

// termWorld is the world of the brief M2-40 over the fakes: the world of 07 section 11.1 with the
// hosts R1 (active, assigned to Alex, organization, reporting bothy-s-alex and bothy-s-sam), R2
// (active, personal with holder Robin, reporting bothy-s-robin and bothy-s-owner) and R3 (active,
// assigned to Sam, declared server host, reporting bothy-s-sam and bothy-s-owner); the
// assignments Alex and Sam on R1, Robin and the Owner on R2, Sam and the Owner on R3; grants
// terminal to Alex and Sam on R1 and to Sam on R3.
type termWorld struct {
	*world
	hosts      *fake.Hosts
	r1, r2, r3 ids.ID
}

// newTermWorld builds the world and its Authorizer with the host source and the terminal windows.
func newTermWorld(t *testing.T) *termWorld {
	t.Helper()
	cfg := app.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: 5 * time.Minute, ViewAsMaxDuration: viewAsMax,
		TerminalStepUpMaxAge: termFresh, QuickReattachWindow: termReattach}
	w := &termWorld{world: newWorldWith(t, cfg), hosts: fake.NewHosts(), r1: ids.New(), r2: ids.New(), r3: ids.New()}
	w.hosts.Put(authz.HostFacts{ID: w.r1, State: "active", AssignedUserID: w.alex, LLMCredentials: "organization", Name: "R1"})
	w.hosts.Put(authz.HostFacts{ID: w.r2, State: "active", LLMCredentials: "personal", CredentialHolderID: w.robin, Name: "R2"})
	w.hosts.Put(authz.HostFacts{ID: w.r3, State: "active", AssignedUserID: w.sam, LLMCredentials: "organization", DeclaredServerHost: true, Name: "R3"})
	w.assign(w.alex, w.r1, "bothy-s-alex", true)
	w.assign(w.sam, w.r1, "bothy-s-sam", true)
	w.assign(w.robin, w.r2, "bothy-s-robin", true)
	w.assign(w.owner, w.r2, "bothy-s-owner", true)
	w.assign(w.sam, w.r3, "bothy-s-sam", true)
	w.assign(w.owner, w.r3, "bothy-s-owner", true)
	for _, g := range []struct{ who, host ids.ID }{{w.alex, w.r1}, {w.sam, w.r1}, {w.sam, w.r3}} {
		w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: g.who}, domain.Target{Kind: domain.TargetHost, HostID: g.host},
			domain.LevelNone, time.Time{}, domain.CapTerminal)
	}
	w.rebuild(t)
	return w
}

// rebuild creates the Authorizer over the fakes with the host source.
func (w *termWorld) rebuild(t *testing.T) {
	t.Helper()
	az, err := app.NewAuthorizer(w.cfg, app.AuthorizerDeps{
		Catalog: app.NewCatalog(slog.New(slog.NewTextHandler(w.logs, nil))), Store: w.store, Principals: w.principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies, Hosts: w.hosts, Versions: w.versions, Recorder: w.rec,
		Clock: w.clk, Log: slog.New(slog.NewTextHandler(w.logs, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az = az
}

// assign records a valid assignment of who on host; reported says whether the host reports it.
func (w *termWorld) assign(who, host ids.ID, osUser string, reported bool) {
	w.hosts.PutSessionUser(who, authz.SessionUserFact{HostID: host, HostSessionUserID: ids.New(), OSUser: osUser, Reported: reported})
}

// person is who in a session of the app with a fresh step-up by the device key.
func (w *termWorld) person(who ids.ID) actor.Actor {
	a := w.user(who)
	a.StepUpMethod, a.ClientComponent = "device_key", "app"
	return a
}

// session is a terminal session of owner on host, opened from the device and login session of
// the person in its world session, in state with detachedAt.
func (w *termWorld) session(owner, host ids.ID, state string, detachedAt time.Time) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: ids.New(), HostID: host, OwnerID: owner,
		Terminal: &authorize.TerminalFacts{SessionState: state, SessionDeviceID: w.devices[owner], SessionAuthSessionID: w.sessions[owner], DetachedAt: detachedAt, HostConnected: true}}
}

// hostOpen is the resource of terminal.open on the host id with a connected host service and a
// known profile (M2-44).
func hostOpen(id ids.ID) authorize.Resource {
	r := host(id)
	r.Terminal = &authorize.TerminalFacts{HostConnected: true, ProfileKnown: true}
	return r
}

// actOpen is terminal.open.
var actOpen = authorize.ActionTerminalOpen

// TestAZ_M54_AlexOpensOwnHost (AZ-M54): Alex opens the assigned host R1 with a fresh step-up.
func TestAZ_M54_AlexOpensOwnHost(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.allow(t, w.person(w.alex), actOpen, hostOpen(w.r1))
}

// TestAZ_M55_SamOnForeignHost (AZ-M55, SR10): Sam with a grant and an assignment on R1 is not its
// assignee.
func TestAZ_M55_SamOnForeignHost(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.expect(t, w.person(w.sam), actOpen, hostOpen(w.r1), authorize.OutcomeForbidden, "terminal_not_host_assignee")
}

// TestAZ_M56_SecondOwnerOnPersonalHost (AZ-M56, SR11): with Robin as a second owner the Owner may
// not open Robin's personal host R2, Robin may.
func TestAZ_M56_SecondOwnerOnPersonalHost(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.principals.Put(authz.PrincipalFacts{ID: w.robin, Kind: actor.KindUser, Status: authz.PrincipalActive, IsOwner: true})
	w.versions.Bump()
	w.expect(t, w.person(w.owner), actOpen, hostOpen(w.r2), authorize.OutcomeForbidden, "i6_holder_only")
	w.allow(t, w.person(w.robin), actOpen, hostOpen(w.r2))
}

// TestAZ_M58_ConsumedStepUp (AZ-M58): a step-up consumed by another opening is not fresh.
func TestAZ_M58_ConsumedStepUp(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	a := w.person(w.alex)
	a.StepUpConsumedAt = a.StepUpAt
	w.expect(t, a, actOpen, hostOpen(w.r1), authorize.OutcomeStepUpRequired, "terminal_step_up_not_fresh")
	d := w.decide(t, context.Background(), a, actOpen, hostOpen(w.r1))
	if e, _ := errs.As(d.Err()); e == nil || e.Code != errs.CodeStepUpRequired || e.Reason != "" {
		t.Fatalf("problem %+v, want 401 step_up_required without a reason", e)
	}
}

// TestAZ_M59_AgentToken (AZ-M59, T-TM-09): the agent Lotse gets the gate's answer on R1 and on a
// random host id alike, with one deny entry each.
func TestAZ_M59_AgentToken(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	before, _ := w.rec.counts()
	for _, r := range []authorize.Resource{host(w.r1), host(ids.New())} {
		w.expect(t, w.agent(w.lotse), actOpen, r, authorize.OutcomeForbidden, "subject_kind_not_allowed")
	}
	if n, _ := w.rec.counts(); n-before != 2 {
		t.Fatalf("%d deny entries, want 2", n-before)
	}
}

// TestAZ_M60_InstanceGrantGivesNoTerminal (AZ-M60, R-AZ-24): a grant on the instance with trigger
// and every capability opens no host; R5 answers like a random id.
func TestAZ_M60_InstanceGrantGivesNoTerminal(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	r5 := ids.New()
	w.hosts.Put(authz.HostFacts{ID: r5, State: "active", LLMCredentials: "organization"})
	w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, domain.Target{Kind: domain.TargetInstance}, domain.LevelTrigger,
		time.Time{}, domain.CapApproveActions, domain.CapUpload, domain.CapExport, domain.CapManage)
	w.versions.Bump()
	w.expect(t, w.person(w.alex), actOpen, hostOpen(r5), authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.person(w.alex), actOpen, hostOpen(ids.New()), authorize.OutcomeNotFound, "no_grant")
}

// TestAZ_M61_OwnerWithoutSessionUser (AZ-M61, SR12): the Owner has no assignment on R1.
func TestAZ_M61_OwnerWithoutSessionUser(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.expect(t, w.person(w.owner), actOpen, hostOpen(w.r1), authorize.OutcomeForbidden, "terminal_no_session_user")
}

// TestTMT08_SR10 (TM-T08): the assignee rule and the server host, declared and reported.
func TestTMT08_SR10(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.expect(t, w.person(w.sam), actOpen, hostOpen(w.r1), authorize.OutcomeForbidden, "terminal_not_host_assignee")
	w.allow(t, w.person(w.alex), actOpen, hostOpen(w.r1))
	w.expect(t, w.person(w.sam), actOpen, hostOpen(w.r3), authorize.OutcomeForbidden, "terminal_server_host_owner_only")
	w.allow(t, w.person(w.owner), actOpen, hostOpen(w.r3))
	// The same with the report of the host service instead of the declaration; it raises no rights
	// version and is read fresh per call.
	w.hosts.Put(authz.HostFacts{ID: w.r3, State: "active", AssignedUserID: w.sam, LLMCredentials: "organization", ReportedIsServerHost: true})
	w.expect(t, w.person(w.sam), actOpen, hostOpen(w.r3), authorize.OutcomeForbidden, "terminal_server_host_owner_only")
	w.allow(t, w.person(w.owner), actOpen, hostOpen(w.r3))
}

// TestTMT28a_NoSessionUser (TM-T28 (a), AZ-P13): the Owner and Alex after the end of the assignment
// get terminal_no_session_user, each with a deny entry, mapped to 403 terminal_denied.
func TestTMT28a_NoSessionUser(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.hosts.RemoveSessionUser(w.alex, w.r1)
	w.versions.Bump()
	before, _ := w.rec.counts()
	for _, who := range []ids.ID{w.owner, w.alex} {
		d := w.decide(t, context.Background(), w.person(who), actOpen, hostOpen(w.r1))
		if d.Outcome != authorize.OutcomeForbidden || d.Reason != "terminal_no_session_user" {
			t.Fatalf("%s: %+v", who, d)
		}
		e, _ := errs.As(d.Err())
		if e == nil || e.Code != errs.CodeTerminalDenied || e.Reason != "no_session_user" {
			t.Fatalf("problem %+v", e)
		}
	}
	n, _ := w.rec.counts()
	if n-before != 2 {
		t.Fatalf("%d deny entries, want 2", n-before)
	}
	for _, e := range w.rec.denials[before:] {
		if string(e.Reason) != "terminal_no_session_user" || e.Action != string(actOpen) {
			t.Fatalf("deny entry %+v", e)
		}
	}
	// A name the host no longer reports counts like no assignment (07 SR12).
	w.assign(w.alex, w.r1, "bothy-s-alex", false)
	w.versions.Bump()
	w.expect(t, w.person(w.alex), actOpen, hostOpen(w.r1), authorize.OutcomeForbidden, "terminal_no_session_user")
}

// TestTerminalDecisionOrder (Design note 5): step-up before the guards, visibility before both.
func TestTerminalDecisionOrder(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	stale := func(who ids.ID) actor.Actor {
		a := w.person(who)
		a.StepUpAt = time.Time{}
		return a
	}
	w.expect(t, stale(w.sam), actOpen, hostOpen(w.r1), authorize.OutcomeStepUpRequired, "terminal_step_up_not_fresh")
	w.expect(t, w.person(w.sam), actOpen, hostOpen(w.r2), authorize.OutcomeNotFound, "no_grant")
	w.expect(t, stale(w.owner), actOpen, hostOpen(w.r2), authorize.OutcomeStepUpRequired, "terminal_step_up_not_fresh")
	// Fresh step-up ignores auth.step_up_max_age: a step-up older than terminal.step_up_max_age
	// but younger than auth.step_up_max_age is stale.
	old := w.person(w.alex)
	old.StepUpAt = w.clk.Now().Add(-termFresh - time.Second)
	w.expect(t, old, actOpen, hostOpen(w.r1), authorize.OutcomeStepUpRequired, "terminal_step_up_not_fresh")
}

// TestTerminalAttachStreamReadClose (Design notes 1, 3 and 4): attach with a fresh step-up, with
// the quick reattach and from another device; the Owner and Sam on Alex's session; stream,
// read_own and close; the internal errors of a session without its facts.
func TestTerminalAttachStreamReadClose(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	attach, stream := authorize.ActionTerminalAttach, authorize.ActionTerminalStream
	readOwn, closeAct := authorize.ActionTerminalReadOwn, authorize.ActionTerminalClose
	ses := w.session(w.alex, w.r1, "detached", w.clk.Now().Add(-termReattach))
	w.allow(t, w.person(w.alex), attach, ses)
	noStepUp := w.person(w.alex)
	noStepUp.StepUpAt = time.Time{}
	d := w.decide(t, context.Background(), noStepUp, attach, ses)
	if !d.Allowed() || !d.QuickReattach {
		t.Fatalf("quick reattach: %+v", d)
	}
	if d := w.decide(t, context.Background(), w.person(w.alex), attach, ses); !d.Allowed() || !d.QuickReattach {
		t.Fatalf("a quick reattach consumes no step-up even with a fresh one: %+v", d)
	}
	other := noStepUp
	other.DeviceID, other.SessionID = ids.New(), ids.New()
	w.creds.PutDevice(other.DeviceID, fake.Device{PrincipalID: w.alex, State: authz.DeviceActive})
	w.creds.PutSession(other.SessionID, fake.Session{PrincipalID: w.alex, DeviceID: other.DeviceID, ExpiresAt: worldStart.Add(365 * 24 * time.Hour)})
	w.expect(t, other, attach, ses, authorize.OutcomeStepUpRequired, "terminal_step_up_not_fresh")
	active := w.session(w.alex, w.r1, "active", time.Time{})
	if d := w.decide(t, context.Background(), w.person(w.alex), attach, active); !d.Allowed() || d.QuickReattach {
		t.Fatalf("attach with a fresh step-up: %+v", d)
	}
	w.expect(t, w.person(w.owner), attach, ses, authorize.OutcomeForbidden, "not_self")
	w.allow(t, w.person(w.owner), closeAct, ses)
	w.allow(t, w.person(w.owner), readOwn, ses)
	for _, act := range []authorize.Action{attach, stream, readOwn, closeAct} {
		w.expect(t, w.person(w.sam), act, ses, authorize.OutcomeNotFound, "not_self")
	}
	w.allow(t, w.person(w.alex), stream, ses)
	w.allow(t, w.person(w.alex), closeAct, ses)
	w.allow(t, w.person(w.sam), readOwn, host(w.r1))
	w.expect(t, w.person(w.sam), readOwn, host(w.r2), authorize.OutcomeNotFound, "no_grant")
	missing := ses
	missing.Terminal = nil
	if _, err := w.az.Authorize(context.Background(), w.person(w.alex), attach, missing); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("attach without Resource.Terminal: %v", err)
	}
	noHost := ses
	noHost.HostID = ids.ID{}
	if _, err := w.az.Authorize(context.Background(), w.person(w.alex), stream, noHost); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("a session without its host: %v", err)
	}
	unknownHost := ses
	unknownHost.HostID = ids.New()
	w.expect(t, w.person(w.alex), stream, unknownHost, authorize.OutcomeNotFound, "no_grant")
}

// TestSubjectGate_TerminalActions (T-TM-09, R-AZ-23): agents, apps and personal tokens never reach
// a terminal action, for known and unknown ids alike.
func TestSubjectGate_TerminalActions(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	tok := ids.New()
	w.creds.PutToken(tok, fake.Token{PrincipalID: w.alex})
	personal := actor.Actor{ID: w.alex, Kind: actor.KindUser, TokenID: tok}
	ses := w.session(w.alex, w.r1, "detached", w.clk.Now())
	unknown := w.session(w.alex, ids.New(), "active", time.Time{})
	cases := []struct {
		act authorize.Action
		res []authorize.Resource
	}{
		{actOpen, []authorize.Resource{host(w.r1), host(ids.New())}},
		{authorize.ActionTerminalAttach, []authorize.Resource{ses, unknown}},
		{authorize.ActionTerminalStream, []authorize.Resource{ses, unknown}},
		{authorize.ActionTerminalReadOwn, []authorize.Resource{ses, host(w.r1), host(ids.New())}},
		{authorize.ActionTerminalClose, []authorize.Resource{ses, unknown}},
	}
	for name, p := range map[string]actor.Actor{"agent": w.agent(w.lotse), "app": w.app(), "personal token": personal} {
		for _, c := range cases {
			for _, r := range c.res {
				d := w.decide(t, context.Background(), p, c.act, r)
				if d.Outcome != authorize.OutcomeForbidden || d.Reason != "subject_kind_not_allowed" {
					t.Errorf("%s %s: %+v", name, c.act, d)
				}
			}
		}
	}
}

// TestTerminalSnapshotVersion (Design note 6): the snapshot caches the session users and the
// personal hosts per rights version; a new version rebuilds them.
func TestTerminalSnapshotVersion(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	owner := w.person(w.owner)
	w.allow(t, owner, actOpen, hostOpen(w.r3))
	p0, u0 := w.hosts.Calls()
	w.allow(t, owner, actOpen, hostOpen(w.r3))
	if p, u := w.hosts.Calls(); p != p0 || u != u0 {
		t.Fatalf("the same version read the sources again: %d/%d after %d/%d", p, u, p0, u0)
	}
	w.hosts.RemoveSessionUser(w.owner, w.r3)
	w.allow(t, owner, actOpen, hostOpen(w.r3))
	w.versions.Bump()
	w.expect(t, owner, actOpen, hostOpen(w.r3), authorize.OutcomeForbidden, "terminal_no_session_user")
	if p, u := w.hosts.Calls(); p != p0+1 || u != u0+1 {
		t.Fatalf("a new version did not rebuild once: %d/%d after %d/%d", p, u, p0, u0)
	}
}

// stubCounters and stubExpiry let a RightsHook over the fakes run VerifyInvariants, which uses
// neither.
type stubCounters struct{ instance.Counters }

type stubExpiry struct{ app.ExpiryStore }

// TestAZ_M57_GroupGrantOnPersonalHost (AZ-M57, Design note 9): a grant terminal to the group Leser
// on R2 violates I6 with the principal ids [Sam]; revoked grants, grants that expire exactly at t,
// grants of a disabled person and of a deleted group are ignored.
func TestAZ_M57_GroupGrantOnPersonalHost(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	hook, err := app.NewRightsHook(app.RightsHookDeps{Counters: stubCounters{}, Store: w.store, Principals: w.principals,
		Structure: w.structure, Policies: w.policies, Hosts: w.hosts, Expiry: stubExpiry{}, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t0 := w.clk.Now()
	verify := func() app.Violations {
		t.Helper()
		v, err := hook.VerifyInvariants(ctx, t0)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := verify(); len(v.I6) != 0 {
		t.Fatalf("the world violates I6: %+v", v.I6)
	}
	term := domain.CapTerminal
	onR2 := domain.Target{Kind: domain.TargetHost, HostID: w.r2}
	revoked := w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, onR2, domain.LevelNone, time.Time{}, term)
	if _, _, err := w.store.RevokeGrant(ctx, revoked, w.owner, "", t0); err != nil {
		t.Fatal(err)
	}
	w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, onR2, domain.LevelNone, t0, term)
	disabled := ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: disabled, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: disabled}, onR2, domain.LevelNone, time.Time{}, term)
	gone := ids.New()
	mustNoErr(t, w.store.InsertGroup(ctx, domain.Group{ID: gone, Name: "Alt", Source: domain.GroupSourceLocal, CreatedAt: worldStart}))
	if _, err := w.store.SoftDeleteGroup(ctx, gone, worldStart); err != nil {
		t.Fatal(err)
	}
	w.grant(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: gone}, onR2, domain.LevelNone, time.Time{}, term)
	if v := verify(); len(v.I6) != 0 {
		t.Fatalf("invalid grants count for I6: %+v", v.I6)
	}
	w.grant(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: w.leser}, onR2, domain.LevelNone, time.Time{}, term)
	v := verify()
	if len(v.I6) != 1 || v.I6[0].HostID != w.r2 || v.I6[0].CredentialHolderID != w.robin || !slices.Equal(v.I6[0].PrincipalIDs, []ids.ID{w.sam}) {
		t.Fatalf("I6 = %+v, want R2 with [Sam]", v.I6)
	}
	if e, _ := errs.As(v.Err()); e == nil || e.Code != errs.CodeI6Violation || e.Hint != authz.HintI6 {
		t.Fatalf("Err = %v", v.Err())
	}
}

// TestTerminalGuard_HostNotActive (M2-44, 24 section 6.1 step 2): a host that is not active, or
// active without a reported control connection, gives host_not_active after SR12, mapped to 409
// host_unavailable; a nil descriptor counts as not connected.
func TestTerminalGuard_HostNotActive(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	attach := authorize.ActionTerminalAttach
	for _, state := range []string{"disabled", "retired", "pairing"} {
		w.hosts.Put(authz.HostFacts{ID: w.r1, State: state, AssignedUserID: w.alex, LLMCredentials: "organization", Name: "R1"})
		w.expect(t, w.person(w.alex), actOpen, hostOpen(w.r1), authorize.OutcomeForbidden, "host_not_active")
		w.expect(t, w.person(w.alex), attach, w.session(w.alex, w.r1, "detached", w.clk.Now()), authorize.OutcomeForbidden, "host_not_active")
	}
	w.hosts.Put(authz.HostFacts{ID: w.r1, State: "active", AssignedUserID: w.alex, LLMCredentials: "organization", Name: "R1"})
	off := hostOpen(w.r1)
	off.Terminal.HostConnected = false
	d := w.decide(t, context.Background(), w.person(w.alex), actOpen, off)
	if d.Outcome != authorize.OutcomeForbidden || d.Reason != "host_not_active" {
		t.Fatalf("not connected: %+v", d)
	}
	if e, _ := errs.As(d.Err()); e == nil || e.Code != errs.CodeHostUnavailable {
		t.Fatalf("problem %+v, want 409 host_unavailable", e)
	}
	w.expect(t, w.person(w.alex), actOpen, host(w.r1), authorize.OutcomeForbidden, "host_not_active")
	ses := w.session(w.alex, w.r1, "detached", w.clk.Now())
	ses.Terminal.HostConnected = false
	w.expect(t, w.person(w.alex), attach, ses, authorize.OutcomeForbidden, "host_not_active")
	// The guard runs after SR12: Sam is still refused by SR10 first.
	w.expect(t, w.person(w.sam), actOpen, off, authorize.OutcomeForbidden, "terminal_not_host_assignee")
}

// TestTerminalGuard_ProfileUnknown (M2-44): the profile verdict decides only terminal.open, after
// host_not_active.
func TestTerminalGuard_ProfileUnknown(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	r := hostOpen(w.r1)
	r.Terminal.ProfileKnown = false
	d := w.decide(t, context.Background(), w.person(w.alex), actOpen, r)
	if d.Outcome != authorize.OutcomeForbidden || d.Reason != "terminal_profile_unknown" {
		t.Fatalf("%+v", d)
	}
	if e, _ := errs.As(d.Err()); e == nil || e.Code != errs.CodeTerminalDenied || e.Reason != "profile_unknown" {
		t.Fatalf("problem %+v", e)
	}
	r.Terminal.HostConnected = false
	w.expect(t, w.person(w.alex), actOpen, r, authorize.OutcomeForbidden, "host_not_active")
	ses := w.session(w.alex, w.r1, "detached", w.clk.Now())
	ses.Terminal.ProfileKnown = false
	w.allow(t, w.person(w.alex), authorize.ActionTerminalAttach, ses)
}

// TestTerminalStream_OtherDeviceNotSelf (M2-44, 05 terminal.stream): only the device of the
// current session token passes; another device of the person is forbidden not_self.
func TestTerminalStream_OtherDeviceNotSelf(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	stream := authorize.ActionTerminalStream
	ses := w.session(w.alex, w.r1, "active", time.Time{})
	w.allow(t, w.person(w.alex), stream, ses)
	other := ses
	other.Terminal = &authorize.TerminalFacts{SessionState: "active", SessionDeviceID: ids.New(), SessionAuthSessionID: w.sessions[w.alex], HostConnected: true}
	w.expect(t, w.person(w.alex), stream, other, authorize.OutcomeForbidden, "not_self")
	bare := ses
	bare.Terminal = nil
	if _, err := w.az.Authorize(context.Background(), w.person(w.alex), stream, bare); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("stream without its facts: %v", err)
	}
}

// TestRecheck_PassesStepUpOnly (M2-44, Design note 12): under authorize.Recheck a consumed
// step-up still allows, every other denial still denies, and no denial is recorded.
func TestRecheck_PassesStepUpOnly(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	ctx := authorize.Recheck(context.Background())
	consumed := w.person(w.alex)
	consumed.StepUpConsumedAt = consumed.StepUpAt
	before, _ := w.rec.counts()
	if d := w.decide(t, ctx, consumed, actOpen, hostOpen(w.r1)); !d.Allowed() {
		t.Fatalf("recheck with a consumed step-up: %+v", d)
	}
	none := w.person(w.alex)
	none.StepUpAt = time.Time{}
	if d := w.decide(t, ctx, none, authorize.ActionTerminalAttach, w.session(w.alex, w.r1, "active", time.Time{})); !d.Allowed() {
		t.Fatalf("recheck of an attach: %+v", d)
	}
	for _, c := range []struct {
		who    actor.Actor
		r      authorize.Resource
		reason string
	}{
		{w.person(w.sam), hostOpen(w.r1), "terminal_not_host_assignee"},
		{w.person(w.owner), hostOpen(w.r1), "terminal_no_session_user"},
		{w.person(w.owner), hostOpen(w.r2), "i6_holder_only"},
		{w.person(w.sam), hostOpen(w.r3), "terminal_server_host_owner_only"},
		{w.agent(w.lotse), hostOpen(w.r1), "subject_kind_not_allowed"},
		{w.person(w.alex), host(w.r1), "host_not_active"},
	} {
		d := w.decide(t, ctx, c.who, actOpen, c.r)
		if d.Allowed() || string(d.Reason) != c.reason {
			t.Errorf("recheck %s: %+v, want %s", c.reason, d, c.reason)
		}
	}
	w.expect(t, w.person(w.alex), actOpen, hostOpen(ids.New()), authorize.OutcomeNotFound, "no_grant")
	after, _ := w.rec.counts()
	if after-before != 1 {
		t.Fatalf("%d deny entries, want only the one outside the recheck", after-before)
	}
}
