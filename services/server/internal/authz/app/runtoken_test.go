// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// run is a run of agent in the main thread of channel, as runs builds it for the agent-self
// actions (brief M2-09 design note 1: OwnerID is runs.agent_id), with the token facts f.
func (w *world) run(channel, agent ids.ID, f *authorize.TokenFacts) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceRun, ID: ids.New(), ThreadID: w.mainThread[channel], OwnerID: agent, Token: f}
}

// validToken is the verdict of a valid token without mode facts.
func validToken() *authorize.TokenFacts { return &authorize.TokenFacts{Verdict: authorize.TokenValid} }

// lastDenial returns the last denial handed to the recorder.
func (w *world) lastDenial(t *testing.T) auditing.Entry {
	t.Helper()
	w.rec.mu.Lock()
	defer w.rec.mu.Unlock()
	if len(w.rec.denials) == 0 {
		t.Fatal("no denial recorded")
	}
	return w.rec.denials[len(w.rec.denials)-1]
}

// wantCodes checks that the details of e are exactly the codes want.
func wantCodes(t *testing.T, e auditing.Entry, want map[string]string) {
	t.Helper()
	if len(e.Attrs) != len(want) {
		t.Fatalf("details %v, want %v", e.Attrs, want)
	}
	for k, v := range want {
		got, ok := e.Attrs[k]
		if !ok || got.Kind() != auditing.KindCode || got.Data() != v {
			t.Fatalf("detail %s: %v, want code %q (all: %v)", k, got, v, e.Attrs)
		}
	}
}

// TestRunStartRequirement (brief M2-09 "Tests"): run.start on the token verdict and the mode
// facts of runs, with the audit of the denials.
func TestRunStartRequirement(t *testing.T) {
	w := newWorld(t)
	// Texter may read Support, so its refusal is about the run, not about visibility.
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{})
	w.allow(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, validToken()))
	w.expect(t, w.agent(w.texter), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, validToken()), forbidden, "not_self")
	w.expect(t, w.user(w.alex), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, validToken()), forbidden, "subject_kind_not_allowed")
	// A run in a channel the agent cannot read is invisible.
	w.expect(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.werkzeuge, w.lotse, validToken()), authorize.OutcomeNotFound, "no_grant")

	for _, c := range []struct {
		verdict authorize.TokenVerdict
		detail  string
	}{{authorize.TokenExpired, "expired"}, {authorize.TokenSuperseded, "superseded"}, {authorize.TokenInvalid, "invalid"}} {
		d := w.decide(t, context.Background(), w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, &authorize.TokenFacts{Verdict: c.verdict}))
		if d.Outcome != forbidden || d.Reason != "invocation_token_invalid" || d.Detail != c.detail {
			t.Fatalf("%s: %+v", c.detail, d)
		}
		if e, ok := errs.As(d.Err()); !ok || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != c.detail {
			t.Fatalf("%s: problem %v", c.detail, d.Err())
		}
		e := w.lastDenial(t)
		if e.Decision != auditing.DecisionDeny || e.Reason != auditing.ReasonInvocationTokenInvalid || e.Action != "run.start" || e.ResourceKind != "run" {
			t.Fatalf("%s: audit %+v", c.detail, e)
		}
		wantCodes(t, e, map[string]string{"token_verdict": c.detail})
	}

	org := func() *authorize.TokenFacts {
		return &authorize.TokenFacts{Verdict: authorize.TokenValid, BillingModeUsed: "organization", BillingModeExpected: "organization",
			BillingKeyType: "api_key", BillingProvider: "anthropic", BillingMethod: "env", ToolClassUsed: "scoped", ToolClassClaim: "scoped"}
	}
	w.allow(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, org()))

	differ := org()
	differ.BillingModeUsed = "personal"
	w.expect(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, differ), authorize.OutcomeInvariantViolation, "billing_mode_mismatch")
	wantCodes(t, w.lastDenial(t), map[string]string{"billing_mode_expected": "organization", "billing_mode_used": "personal",
		"billing_evidence_provider": "anthropic", "billing_evidence_key_type": "api_key", "billing_evidence_method": "env"})

	unknown := org()
	unknown.BillingKeyType, unknown.BillingProvider, unknown.BillingMethod = "unknown", "", ""
	w.expect(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, unknown), authorize.OutcomeInvariantViolation, "billing_mode_mismatch")
	wantCodes(t, w.lastDenial(t), map[string]string{"billing_mode_expected": "organization", "billing_mode_used": "organization",
		"billing_evidence_key_type": "unknown"})

	host := org()
	host.ToolClassUsed = "host"
	w.expect(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, host), authorize.OutcomeInvariantViolation, "tool_class_mismatch")
	wantCodes(t, w.lastDenial(t), map[string]string{"tool_class": "scoped", "tool_class_used": "host"})

	chat := org()
	chat.ToolClassUsed = "chat"
	w.allow(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, chat))

	// A detail that is no valid code is left out instead of losing the denial.
	odd := org()
	odd.BillingModeUsed, odd.BillingProvider = "personal", "Not A Code"
	w.expect(t, w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, odd), authorize.OutcomeInvariantViolation, "billing_mode_mismatch")
	wantCodes(t, w.lastDenial(t), map[string]string{"billing_mode_expected": "organization", "billing_mode_used": "personal",
		"billing_evidence_key_type": "api_key", "billing_evidence_method": "env"})

	// Other denials carry no details.
	w.expect(t, w.agent(w.texter), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, &authorize.TokenFacts{Verdict: authorize.TokenExpired}), forbidden, "not_self")
	wantCodes(t, w.lastDenial(t), map[string]string{})

	// Without the token facts the call is an internal error.
	if _, err := w.az.Authorize(context.Background(), w.agent(w.lotse), authorize.ActionRunStart, w.run(w.allgemein, w.lotse, nil)); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("no token facts: %v, want internal", err)
	}
}

// TestRunTokenRenewRequirement (brief M2-09 "Tests"): run.token.renew allows a valid and an
// expired token and refuses a superseded and an invalid one.
func TestRunTokenRenewRequirement(t *testing.T) {
	w := newWorld(t)
	for _, v := range []authorize.TokenVerdict{authorize.TokenValid, authorize.TokenExpired} {
		w.allow(t, w.agent(w.lotse), authorize.ActionRunTokenRenew, w.run(w.allgemein, w.lotse, &authorize.TokenFacts{Verdict: v}))
	}
	for _, c := range []struct {
		verdict authorize.TokenVerdict
		detail  string
	}{{authorize.TokenSuperseded, "superseded"}, {authorize.TokenInvalid, "invalid"}} {
		d := w.decide(t, context.Background(), w.agent(w.lotse), authorize.ActionRunTokenRenew, w.run(w.allgemein, w.lotse, &authorize.TokenFacts{Verdict: c.verdict}))
		if d.Outcome != forbidden || d.Reason != "invocation_token_invalid" || d.Detail != c.detail {
			t.Fatalf("%s: %+v", c.detail, d)
		}
		wantCodes(t, w.lastDenial(t), map[string]string{"token_verdict": c.detail})
	}
	if _, err := w.az.Authorize(context.Background(), w.agent(w.lotse), authorize.ActionRunTokenRenew,
		w.run(w.allgemein, w.lotse, &authorize.TokenFacts{})); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("zero verdict: %v, want internal", err)
	}
}
