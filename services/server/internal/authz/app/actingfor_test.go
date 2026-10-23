// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// actingRun is the run an agent acts in for the tests of SR9 (brief M2-14): its id and its
// thread, the main thread of Support/Allgemein.
type actingRun struct {
	id, thread ids.ID
}

// newActingRun returns a run in the main thread of Support/Allgemein.
func (w *world) newActingRun() actingRun {
	return actingRun{id: ids.New(), thread: w.mainThread[w.allgemein]}
}

// agentFor returns the principal of agent with its token, bound by the Invocation-Token header to
// the run r of person with the given status (M2-14 design note 2).
func (w *world) agentFor(agent, person ids.ID, r actingRun, status actor.ActingForStatus) actor.Actor {
	a := w.agent(agent)
	a.ActingFor = actor.ActingFor{Status: status, UserID: person, RunID: r.id, ThreadID: r.thread, JTI: "jti-test"}
	if status == actor.ActingForInvalid {
		a.ActingFor = actor.ActingFor{Status: status}
	}
	return a
}

// grantLotse gives Lotse level read on the space and raises the rights version.
func (w *world) grantLotse(t testing.TB, space ids.ID) {
	t.Helper()
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetSpace, SpaceID: space}, domain.LevelRead, time.Time{})
	w.versions.Bump()
}

// runRes is a run resource as runs builds it (OwnerID the run's agent, M2-09) in thread.
func runRes(id, thread, agent ids.ID, tok *authorize.TokenFacts) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceRun, ID: id, ThreadID: thread, OwnerID: agent, Token: tok}
}

// engineeringIDs are the channels and the space of Engineering.
func (w *world) engineeringIDs() []ids.ID { return []ids.ID{w.engineering, w.projektA, w.werkzeuge} }

// scopeHasAny reports whether sc names one of these ids in a list.
func scopeHasAny(sc authorize.Scope, these []ids.ID) bool {
	for _, id := range these {
		if slices.Contains(sc.ChannelIDs, id) || slices.Contains(sc.SpaceIDs, id) || slices.Contains(sc.FullyVisibleSpaceIDs, id) {
			return true
		}
	}
	return false
}

// TestAZM44_ActingForInsufficient (AZ-M44): Lotse in Alex's run, with read on Engineering of its
// own, reads nothing of Engineering: the message is not_found acting_for_insufficient and Scope
// names no Engineering channel.
func TestAZM44_ActingForInsufficient(t *testing.T) {
	w := newWorld(t)
	w.grantLotse(t, w.engineering)
	lotse := w.agentFor(w.lotse, w.alex, w.newActingRun(), actor.ActingForBound)
	w.expect(t, lotse, authorize.ActionMessageRead, w.msg(w.projektA, w.robin), notFound, "acting_for_insufficient")
	sc, err := w.az.Scope(context.Background(), lotse, authorize.ActionMessageRead)
	if err != nil {
		t.Fatal(err)
	}
	if scopeHasAny(sc, w.engineeringIDs()) || !slices.Contains(sc.ChannelIDs, w.allgemein) {
		t.Fatalf("scope %+v", sc)
	}
}

// TestAZM45_WithoutRun (AZ-M45): the same request without the header acts with Lotse's own
// rights (hint H1).
func TestAZM45_WithoutRun(t *testing.T) {
	w := newWorld(t)
	w.grantLotse(t, w.engineering)
	w.allow(t, w.agent(w.lotse), authorize.ActionMessageRead, w.msg(w.projektA, w.robin))
	sc, err := w.az.Scope(context.Background(), w.agent(w.lotse), authorize.ActionMessageRead)
	if err != nil || !scopeHasAny(sc, w.engineeringIDs()) {
		t.Fatalf("scope %+v %v", sc, err)
	}
}

// TestActingFor_ForbiddenReason (M2-14 design note 4f): a requirement that holds for the agent but
// not for the intersection is forbidden acting_for_insufficient; one the agent fails alone keeps
// the agent's reason.
func TestActingFor_ForbiddenReason(t *testing.T) {
	w := newWorld(t)
	tg := domain.Target{Kind: domain.TargetChannel, ChannelID: w.werkzeuge}
	alexRead := w.grant(t, userSubject(w.alex), tg, domain.LevelRead, time.Time{})
	lotseWrite := w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, tg, domain.LevelWrite, time.Time{})
	w.versions.Bump()
	lotse := w.agentFor(w.lotse, w.alex, w.newActingRun(), actor.ActingForBound)
	w.expect(t, lotse, authorize.ActionMessageCreate, w.thread(w.werkzeuge), forbidden, "acting_for_insufficient")
	w.revoke(t, lotseWrite)
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, tg, domain.LevelRead, time.Time{})
	w.versions.Bump()
	w.expect(t, lotse, authorize.ActionMessageCreate, w.thread(w.werkzeuge), forbidden, "level_insufficient")
	_ = alexRead
	if e := w.lastDenial(t); e.Reason != auditing.ReasonLevelInsufficient {
		t.Fatalf("audit %+v", e)
	}
}

// exceptionCase is one SR9 exception (07 "SR9") with a resource on a thread and run.
type exceptionCase struct {
	act authorize.Action
	res func(thread, run ids.ID) authorize.Resource
}

// exceptionCases are the five exceptions of the mode table.
func (w *world) exceptionCases() []exceptionCase {
	return []exceptionCase{
		{authorize.ActionMessageCreate, func(thread, _ ids.ID) authorize.Resource {
			return authorize.Resource{Kind: authorize.ResourceThread, ID: thread}
		}},
		{authorize.ActionRunUpdate, func(thread, run ids.ID) authorize.Resource { return runRes(run, thread, w.lotse, nil) }},
		{authorize.ActionRunFinish, func(thread, run ids.ID) authorize.Resource { return runRes(run, thread, w.lotse, nil) }},
		{authorize.ActionInteractionRespond, func(thread, _ ids.ID) authorize.Resource {
			return authorize.Resource{Kind: authorize.ResourceInteraction, ID: ids.New(), ThreadID: thread, OwnerID: w.lotse}
		}},
		{authorize.ActionAgentSessionReport, func(thread, _ ids.ID) authorize.Resource {
			return authorize.Resource{Kind: authorize.ResourceAgentSession, ID: ids.New(), ThreadID: thread, OwnerID: w.lotse}
		}},
	}
}

// TestActingFor_Exceptions (07 "SR9", M2-14 design note 5): on the own thread and run only Lotse's
// rights count, also after Alex lost every grant; on a thread Alex cannot read the exception is
// bound; run.finish of an earlier run of the same thread is bound.
func TestActingFor_Exceptions(t *testing.T) {
	w := newWorld(t)
	w.grantLotse(t, w.engineering)
	run := w.newActingRun()
	lotse := w.agentFor(w.lotse, w.alex, run, actor.ActingForBound)
	w.revoke(t, w.g1)
	for _, c := range w.exceptionCases() {
		w.allow(t, lotse, c.act, c.res(run.thread, run.id))
		w.expect(t, lotse, c.act, c.res(w.mainThread[w.projektA], run.id), notFound, "acting_for_insufficient")
	}
	w.expect(t, lotse, authorize.ActionRunFinish, runRes(ids.New(), run.thread, w.lotse, nil), notFound, "acting_for_insufficient")
	// The same calls without the header are decided on Lotse's rights alone.
	w.allow(t, w.agent(w.lotse), authorize.ActionRunFinish, runRes(ids.New(), run.thread, w.lotse, nil))
}

// tokenRes returns run.start and run.token.renew resources of Lotse's own run r.
func (w *world) tokenRes(r actingRun) map[authorize.Action]authorize.Resource {
	return map[authorize.Action]authorize.Resource{
		authorize.ActionRunStart:      runRes(r.id, r.thread, w.lotse, validToken()),
		authorize.ActionRunTokenRenew: runRes(r.id, r.thread, w.lotse, &authorize.TokenFacts{Verdict: authorize.TokenExpired}),
	}
}

// TestActingFor_Unbound (M2-14 design note 5): run.start and run.token.renew answer with the
// header exactly as without, with the statuses bound and run_inactive, also when the person has
// lost every grant.
func TestActingFor_Unbound(t *testing.T) {
	w := newWorld(t)
	run := w.newActingRun()
	check := func(stage string) {
		t.Helper()
		for act, res := range w.tokenRes(run) {
			want := w.decide(t, context.Background(), w.agent(w.lotse), act, res)
			for _, st := range []actor.ActingForStatus{actor.ActingForBound, actor.ActingForRunInactive} {
				got := w.decide(t, context.Background(), w.agentFor(w.lotse, w.alex, run, st), act, res)
				if got != want {
					t.Errorf("%s %s %s: %+v, want %+v", stage, act, st, got, want)
				}
			}
		}
	}
	check("granted")
	w.revoke(t, w.g1)
	check("revoked")
	// A run outside Lotse's sight stays not_found for both.
	res := runRes(ids.New(), w.mainThread[w.werkzeuge], w.lotse, validToken())
	w.expect(t, w.agentFor(w.lotse, w.alex, run, actor.ActingForBound), authorize.ActionRunStart, res, notFound, "no_grant")
}

// wantTokenInvalid checks a decision of forbidden invocation_token_invalid with the detail.
func wantTokenInvalid(t *testing.T, what string, d authorize.Decision, detail string) {
	t.Helper()
	if d.Outcome != forbidden || d.Reason != "invocation_token_invalid" || d.Detail != detail {
		t.Fatalf("%s: %+v, want invocation_token_invalid %s", what, d, detail)
	}
}

// wantScopeTokenInvalid checks that Scope fails with the problem invocation_token_invalid.
func wantScopeTokenInvalid(t *testing.T, w *world, p actor.Actor, detail string) {
	t.Helper()
	_, err := w.az.Scope(context.Background(), p, authorize.ActionMessageRead)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != detail {
		t.Fatalf("scope: %v, want invocation_token_invalid %s", err, detail)
	}
}

// TestActingFor_StatusInvalid (M2-14 design note 4b): an invalid binding denies a bound, an
// exception and an unbound action and Scope before the path is resolved, so an unknown resource
// answers like a known one; the denial carries token_verdict and no person or run.
func TestActingFor_StatusInvalid(t *testing.T) {
	w := newWorld(t)
	run := w.newActingRun()
	lotse := w.agentFor(w.lotse, w.alex, run, actor.ActingForInvalid)
	ctx := context.Background()
	wantTokenInvalid(t, "message.read", w.decide(t, ctx, lotse, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex)), "invalid")
	unknown := authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: ids.New()}
	wantTokenInvalid(t, "unknown message", w.decide(t, ctx, lotse, authorize.ActionMessageRead, unknown), "invalid")
	wantTokenInvalid(t, "message.create", w.decide(t, ctx, lotse, authorize.ActionMessageCreate, w.thread(w.allgemein)), "invalid")
	wantTokenInvalid(t, "run.start", w.decide(t, ctx, lotse, authorize.ActionRunStart, w.tokenRes(run)[authorize.ActionRunStart]), "invalid")
	wantScopeTokenInvalid(t, w, lotse, "invalid")
	wantCodes(t, w.lastDenial(t), map[string]string{"token_verdict": "invalid"})
}

// TestActingFor_StatusRunInactive (M2-14 design note 4c): after the run ended bound and exception
// actions and Scope are invocation_token_invalid run_inactive; unbound actions answer as without
// the header. The denial names the person and the run.
func TestActingFor_StatusRunInactive(t *testing.T) {
	w := newWorld(t)
	run := w.newActingRun()
	lotse := w.agentFor(w.lotse, w.alex, run, actor.ActingForRunInactive)
	ctx := context.Background()
	wantTokenInvalid(t, "message.read", w.decide(t, ctx, lotse, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex)), "run_inactive")
	for _, c := range w.exceptionCases() {
		wantTokenInvalid(t, string(c.act), w.decide(t, ctx, lotse, c.act, c.res(run.thread, run.id)), "run_inactive")
	}
	e := w.lastDenial(t)
	if e.Attrs["acting_for_user_id"].Data() != w.alex.String() || e.Attrs["run_id"].Data() != run.id.String() ||
		e.Attrs["acting_for_user_id"].Kind() != auditing.KindID || e.Attrs["token_verdict"].Data() != "run_inactive" {
		t.Fatalf("audit details %v", e.Attrs)
	}
	wantScopeTokenInvalid(t, w, lotse, "run_inactive")
	for act, res := range w.tokenRes(run) {
		if got, want := w.decide(t, ctx, lotse, act, res), w.decide(t, ctx, w.agent(w.lotse), act, res); got != want {
			t.Errorf("%s: %+v, want %+v", act, got, want)
		}
	}
}

// TestActingFor_DeniedDetails (M2-14 design note 7): a denial of a bound request names the person
// and the run as IDs; an allow writes nothing.
func TestActingFor_DeniedDetails(t *testing.T) {
	w := newWorld(t)
	w.grantLotse(t, w.engineering)
	run := w.newActingRun()
	lotse := w.agentFor(w.lotse, w.alex, run, actor.ActingForBound)
	before, _ := w.rec.counts()
	w.allow(t, lotse, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	if after, _ := w.rec.counts(); after != before {
		t.Fatal("an allow wrote an audit entry")
	}
	w.expect(t, lotse, authorize.ActionMessageRead, w.msg(w.projektA, w.robin), notFound, "acting_for_insufficient")
	e := w.lastDenial(t)
	if e.Reason != auditing.ReasonActingForInsufficient || len(e.Attrs) != 2 || e.Attrs["acting_for_user_id"].Data() != w.alex.String() ||
		e.Attrs["run_id"].Data() != run.id.String() {
		t.Fatalf("audit %+v", e)
	}
}

// TestActingFor_DeliveryModeIgnores (07 section 8.1, M2-14 design note 6): in delivery mode the
// decision and Scope of an agent consumer ignore the binding; in filter mode they stay bound.
func TestActingFor_DeliveryModeIgnores(t *testing.T) {
	w := newWorld(t)
	w.grantLotse(t, w.engineering)
	run := w.newActingRun()
	bound := w.agentFor(w.lotse, w.alex, run, actor.ActingForBound)
	del := authorize.Delivery(context.Background())
	res := w.msg(w.projektA, w.robin)
	if got, want := w.decide(t, del, bound, authorize.ActionMessageRead, res), w.decide(t, del, w.agent(w.lotse), authorize.ActionMessageRead, res); got != want || !got.Allowed() {
		t.Fatalf("delivery: %+v, want %+v", got, want)
	}
	for _, st := range []actor.ActingForStatus{actor.ActingForInvalid, actor.ActingForRunInactive} {
		if d := w.decide(t, del, w.agentFor(w.lotse, w.alex, run, st), authorize.ActionMessageRead, res); !d.Allowed() {
			t.Fatalf("delivery with %s: %+v", st, d)
		}
	}
	sb, err := w.az.Scope(del, bound, authorize.ActionMessageRead)
	if err != nil {
		t.Fatal(err)
	}
	sa, _ := w.az.Scope(del, w.agent(w.lotse), authorize.ActionMessageRead)
	if !slices.Equal(sb.ChannelIDs, sa.ChannelIDs) || !scopeHasAny(sb, w.engineeringIDs()) {
		t.Fatalf("delivery scope %+v, want %+v", sb, sa)
	}
	filter := authorize.Filter(context.Background())
	if d := w.decide(t, filter, bound, authorize.ActionMessageRead, res); d.Outcome != notFound || d.Reason != "acting_for_insufficient" {
		t.Fatalf("filter: %+v", d)
	}
}

// TestActingFor_OwnerTriggerer (M2-14 design note 4e): an owner as the triggering person has the
// owner rights without a session, so the intersection equals Lotse's own rights; after
// owner.remove only the person's grants count; the owner's personal token never shares that
// snapshot.
func TestActingFor_OwnerTriggerer(t *testing.T) {
	w := newWorld(t)
	w.grantLotse(t, w.engineering)
	w.addToken(w.owner)
	personal := actor.Actor{ID: w.owner, Kind: actor.KindUser, TokenID: w.tokens[w.owner]}
	lotse := w.agentFor(w.lotse, w.owner, w.newActingRun(), actor.ActingForBound)
	res := w.msg(w.projektA, w.robin)
	// The personal token first, so its snapshot is cached before the acting-for one.
	w.expect(t, personal, authorize.ActionMessageRead, res, notFound, "no_grant")
	w.allow(t, lotse, authorize.ActionMessageRead, res)
	w.expect(t, personal, authorize.ActionMessageRead, res, notFound, "no_grant")
	w.expect(t, lotse, authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
	f, _, _ := w.principals.Principal(context.Background(), w.owner)
	f.IsOwner = false
	w.principals.Put(f)
	w.versions.Bump()
	w.expect(t, lotse, authorize.ActionMessageRead, res, notFound, "acting_for_insufficient")
}

// TestActingFor_TriggererInactive (SR7, M2-14 design note 4e): a disabled or pending person has no
// rights, so every bound action of the agent ends acting_for_insufficient.
func TestActingFor_TriggererInactive(t *testing.T) {
	for _, st := range []authz.PrincipalStatus{authz.PrincipalDisabled, authz.PrincipalPending, authz.PrincipalDeleted} {
		w := newWorld(t)
		f, _, _ := w.principals.Principal(context.Background(), w.alex)
		f.Status = st
		w.principals.Put(f)
		w.versions.Bump()
		lotse := w.agentFor(w.lotse, w.alex, w.newActingRun(), actor.ActingForBound)
		w.expect(t, lotse, authorize.ActionMessageRead, w.msg(w.eskalation, w.robin), notFound, "acting_for_insufficient")
		w.expect(t, lotse, authorize.ActionThreadRead, w.thread(w.eskalation), notFound, "acting_for_insufficient")
	}
}

// TestActingFor_FreshRights (07 section 8.2 point 4): a revocation of G-1 makes the next decision
// deny without any change of the binding.
func TestActingFor_FreshRights(t *testing.T) {
	w := newWorld(t)
	lotse := w.agentFor(w.lotse, w.alex, w.newActingRun(), actor.ActingForBound)
	res := w.msg(w.eskalation, w.robin)
	w.allow(t, lotse, authorize.ActionMessageRead, res)
	w.revoke(t, w.g1)
	w.expect(t, lotse, authorize.ActionMessageRead, res, notFound, "acting_for_insufficient")
}

// TestActingFor_PathlessActionsUnchanged (M2-14 design note 5): actions without a path in the
// channel tree are bound, but the intersection changes nothing for them.
func TestActingFor_PathlessActionsUnchanged(t *testing.T) {
	w := newWorld(t)
	self := authorize.Resource{Kind: authorize.ResourceAgent, ID: w.lotse}
	cases := map[authorize.Action]authorize.Resource{
		authorize.ActionAgentHeartbeat:      self,
		authorize.ActionAgentEventsConsume:  self,
		authorize.ActionAgentTriggerersRead: self,
		authorize.ActionSelfRead:            {Kind: authorize.ResourcePrincipal, ID: w.lotse},
		authorize.ActionInstanceRead:        {Kind: authorize.ResourceInstance},
	}
	run := w.newActingRun()
	for _, stage := range []string{"granted", "revoked"} {
		for act, res := range cases {
			want := w.decide(t, context.Background(), w.agent(w.lotse), act, res)
			got := w.decide(t, context.Background(), w.agentFor(w.lotse, w.alex, run, actor.ActingForBound), act, res)
			if got != want || !want.Allowed() {
				t.Errorf("%s %s: %+v, want %+v (allowed)", stage, act, got, want)
			}
		}
		if stage == "granted" {
			w.revoke(t, w.g1)
		}
	}
}

// TestScope_ActingForIntersection (07 section 7.4 rule 1, M2-14 design note 4h): every list of
// Scope is the intersection of Lotse's and Alex's lists, ascending; a channel created without a
// new rights version appears only where both read.
func TestScope_ActingForIntersection(t *testing.T) {
	w := newWorld(t)
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetInstance}, domain.LevelRead, time.Time{})
	w.versions.Bump()
	lotse := w.agentFor(w.lotse, w.alex, w.newActingRun(), actor.ActingForBound)
	own, err := w.az.Scope(context.Background(), w.agent(w.lotse), authorize.ActionMessageRead)
	if err != nil || !own.InstanceWide || len(own.FullyVisibleSpaceIDs) != 4 {
		t.Fatalf("own scope %+v %v", own, err)
	}
	newSupport, newEng := ids.New(), ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: newSupport, SpaceID: w.support, Kind: authz.ChannelText})
	w.structure.PutChannel(authz.ChannelFacts{ID: newEng, SpaceID: w.engineering, Kind: authz.ChannelText})
	sc, err := w.az.Scope(context.Background(), lotse, authorize.ActionMessageRead)
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.SortIDs([]ids.ID{w.allgemein, w.eskalation, newSupport}); !slices.Equal(sc.ChannelIDs, want) {
		t.Errorf("channels %v, want %v", sc.ChannelIDs, want)
	}
	if want := []ids.ID{w.support}; !slices.Equal(sc.SpaceIDs, want) || !slices.Equal(sc.FullyVisibleSpaceIDs, want) {
		t.Errorf("spaces %v fully %v, want %v", sc.SpaceIDs, sc.FullyVisibleSpaceIDs, want)
	}
	if sc.InstanceWide {
		t.Error("instance wide although Alex has no read on the instance")
	}
	if slices.Contains(sc.ChannelIDs, newEng) {
		t.Error("the new Engineering channel is visible")
	}
}

// TestDecisionErr_ActingFor (M2-14 design note 4g): both details of invocation_token_invalid map
// to the problem code with the detail as reason; acting_for_insufficient is 403 forbidden with the
// reason; not_found never carries a reason.
func TestDecisionErr_ActingFor(t *testing.T) {
	t.Parallel()
	for _, detail := range []string{"invalid", "run_inactive"} {
		d := authorize.Decision{Outcome: forbidden, Reason: "invocation_token_invalid", Detail: detail}
		if e, ok := errs.As(d.Err()); !ok || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != detail {
			t.Errorf("%s: %v", detail, d.Err())
		}
	}
	d := authorize.Decision{Outcome: forbidden, Reason: "acting_for_insufficient"}
	if e, ok := errs.As(d.Err()); !ok || e.Code != errs.CodeForbidden || e.Reason != "acting_for_insufficient" {
		t.Errorf("forbidden: %v", d.Err())
	}
	d = authorize.Decision{Outcome: notFound, Reason: "acting_for_insufficient"}
	if e, ok := errs.As(d.Err()); !ok || e.Code != errs.CodeNotFound || e.Reason != "" {
		t.Errorf("not_found: %v", d.Err())
	}
}
