// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// runInput is action on a run of the agent agentID in channel chanC, decided for p with write on
// the channel and the invocation facts f.
func runInput(t *testing.T, action string, p domain.Principal, f *domain.InvocationFacts) domain.Input {
	t.Helper()
	in := input(t, action, "run", p, domain.Rights{Level: domain.LevelWrite})
	in.Path = domain.Path{Kind: domain.PathChannel, SpaceID: spaceS, ChannelID: chanC}
	in.Resource.OwnerID = agentID
	in.Invocation = f
	return in
}

// TestRunTokenRequirements: run.start and run.token.renew are real requirements now (M2-09
// design note 2a), with the 05 texts of the catalogue.
func TestRunTokenRequirements(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{
		"run.start":       "All(AgentSelf,Token(valid))",
		"run.token.renew": "All(AgentSelf,Token(valid,expired))",
	} {
		r, ok := domain.RequirementOf(id)
		if !ok {
			t.Fatalf("%s: no entry", id)
		}
		if _, deferred := r.Expr.DeferredTo(); deferred {
			t.Fatalf("%s: still deferred", id)
		}
		if got := r.Expr.String(); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
}

// TestDecide_InvocationToken: the atom Token on the verdict of runs; a verdict outside the allowed
// set fails forbidden invocation_token_invalid with the verdict as Detail.
func TestDecide_InvocationToken(t *testing.T) {
	t.Parallel()
	agent := domain.Principal{ID: agentID, Kind: domain.KindAgent}
	cases := []struct {
		action  string
		verdict domain.InvocationVerdict
		want    domain.Decision
	}{
		{"run.start", domain.InvocationValid, domain.Allow},
		{"run.start", domain.InvocationExpired, tokenDenied("expired")},
		{"run.start", domain.InvocationSuperseded, tokenDenied("superseded")},
		{"run.start", domain.InvocationInvalid, tokenDenied("invalid")},
		{"run.token.renew", domain.InvocationValid, domain.Allow},
		{"run.token.renew", domain.InvocationExpired, domain.Allow},
		{"run.token.renew", domain.InvocationSuperseded, tokenDenied("superseded")},
		{"run.token.renew", domain.InvocationInvalid, tokenDenied("invalid")},
	}
	for _, c := range cases {
		got, err := domain.Decide(runInput(t, c.action, agent, &domain.InvocationFacts{Verdict: c.verdict}))
		if err != nil || got != c.want {
			t.Errorf("%s %s: %+v, %v; want %+v", c.action, c.verdict, got, err, c.want)
		}
	}
	// AgentSelf decides before the token: another agent is not_self, a person is refused by kind.
	other := domain.Principal{ID: ids.New(), Kind: domain.KindAgent}
	if got, err := domain.Decide(runInput(t, "run.start", other, &domain.InvocationFacts{Verdict: domain.InvocationInvalid})); err != nil ||
		got != (domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonNotSelf}) {
		t.Errorf("other agent: %+v, %v", got, err)
	}
	person := domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true}
	if got, err := domain.Decide(runInput(t, "run.start", person, &domain.InvocationFacts{Verdict: domain.InvocationValid})); err != nil ||
		got != (domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}) {
		t.Errorf("person: %+v, %v", got, err)
	}
}

func tokenDenied(detail string) domain.Decision {
	return domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonInvocationTokenInvalid, Detail: detail}
}

// TestDecide_InvocationTokenMissing: a run.start or run.token.renew without the token facts, or
// with the zero verdict, is an internal error, never an allow; also for another agent, so a caller
// that forgets the facts learns it on every path.
func TestDecide_InvocationTokenMissing(t *testing.T) {
	t.Parallel()
	agent := domain.Principal{ID: agentID, Kind: domain.KindAgent}
	other := domain.Principal{ID: ids.New(), Kind: domain.KindAgent}
	for _, action := range []string{"run.start", "run.token.renew"} {
		for name, in := range map[string]domain.Input{
			"nil":        runInput(t, action, agent, nil),
			"nil other":  runInput(t, action, other, nil),
			"zero":       runInput(t, action, agent, &domain.InvocationFacts{}),
			"out of set": runInput(t, action, agent, &domain.InvocationFacts{Verdict: domain.InvocationVerdict(42)}),
		} {
			got, err := domain.Decide(in)
			if errs.KindOf(err) != errs.KindInternal {
				t.Errorf("%s %s: %+v, %v; want an internal error", action, name, got, err)
			}
		}
	}
}

// TestDecide_RunStartGuard: the guard of run.start (07 section 4.2 step 9) on the mode facts;
// without a reported mode it passes, and a token verdict that is not valid never reaches it.
func TestDecide_RunStartGuard(t *testing.T) {
	t.Parallel()
	agent := domain.Principal{ID: agentID, Kind: domain.KindAgent}
	mismatch := domain.Decision{Outcome: domain.OutcomeInvariantViolation, Reason: domain.ReasonBillingModeMismatch}
	toolMismatch := domain.Decision{Outcome: domain.OutcomeInvariantViolation, Reason: domain.ReasonToolClassMismatch}
	org := domain.InvocationFacts{Verdict: domain.InvocationValid, BillingModeUsed: "organization", BillingModeExpected: "organization",
		BillingKeyType: "api_key", ToolClassUsed: "scoped", ToolClassClaim: "scoped"}
	cases := []struct {
		name string
		mod  func(*domain.InvocationFacts)
		want domain.Decision
	}{
		{"equal", func(*domain.InvocationFacts) {}, domain.Allow},
		{"no reported mode", func(f *domain.InvocationFacts) {
			*f = domain.InvocationFacts{Verdict: domain.InvocationValid}
		}, domain.Allow},
		{"modes differ", func(f *domain.InvocationFacts) { f.BillingModeUsed = "personal" }, mismatch},
		{"key type unknown", func(f *domain.InvocationFacts) { f.BillingKeyType = "unknown" }, mismatch},
		{"mode before tool class", func(f *domain.InvocationFacts) { f.BillingModeUsed, f.ToolClassUsed = "personal", "host" }, mismatch},
		{"host against scoped", func(f *domain.InvocationFacts) { f.ToolClassUsed = "host" }, toolMismatch},
		{"unknown class against scoped", func(f *domain.InvocationFacts) { f.ToolClassUsed = "shell" }, toolMismatch},
		{"chat against scoped", func(f *domain.InvocationFacts) { f.ToolClassUsed = "chat" }, domain.Allow},
		{"expired token never reaches the guard", func(f *domain.InvocationFacts) {
			f.Verdict, f.BillingModeUsed = domain.InvocationExpired, "personal"
		}, tokenDenied("expired")},
	}
	for _, c := range cases {
		f := org
		c.mod(&f)
		got, err := domain.Decide(runInput(t, "run.start", agent, &f))
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	// run.token.renew has no mode guard.
	f := org
	f.BillingModeUsed = "personal"
	if got, err := domain.Decide(runInput(t, "run.token.renew", agent, &f)); err != nil || got != domain.Allow {
		t.Errorf("renew with a mode mismatch: %+v, %v", got, err)
	}
}

// TestInvocationVerdictString: the verdict codes match kernel/authorize.TokenVerdict.String.
func TestInvocationVerdictString(t *testing.T) {
	t.Parallel()
	for v, want := range map[domain.InvocationVerdict]string{
		domain.InvocationVerdictZero: "", domain.InvocationValid: "valid", domain.InvocationExpired: "expired",
		domain.InvocationSuperseded: "superseded", domain.InvocationInvalid: "invalid",
	} {
		if got := v.String(); got != want {
			t.Errorf("%d: %q, want %q", v, got, want)
		}
	}
}
