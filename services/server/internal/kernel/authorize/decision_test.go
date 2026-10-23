// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestDecisionErrMapping covers every row of the mapping table of the brief M1-28 (06 section 5).
func TestDecisionErrMapping(t *testing.T) {
	cases := []struct {
		outcome    authorize.Outcome
		reason     string
		kind       errs.Kind
		code       errs.Code
		wantReason string
	}{
		{authorize.OutcomeNotFound, "no_grant", errs.KindNotFound, errs.CodeNotFound, ""},
		{authorize.OutcomeNotFound, "acting_for_insufficient", errs.KindNotFound, errs.CodeNotFound, ""},
		{authorize.OutcomeForbidden, "scope_missing", errs.KindForbidden, errs.CodeScopeMissing, "scope_missing"},
		{authorize.OutcomeForbidden, "device_pending", errs.KindForbidden, errs.CodeDevicePending, "device_pending"},
		{authorize.OutcomeForbidden, "principal_pending", errs.KindForbidden, errs.CodeDevicePending, "principal_pending"},
		{authorize.OutcomeForbidden, "channel_read_only", errs.KindForbidden, errs.CodeChannelReadOnly, "channel_read_only"},
		// 07 section 4.5: an archived channel answers like a read-only one, with its own reason (M1-53).
		{authorize.OutcomeForbidden, "channel_archived", errs.KindForbidden, errs.CodeChannelReadOnly, "channel_archived"},
		{authorize.OutcomeForbidden, "level_insufficient", errs.KindForbidden, errs.CodeForbidden, "level_insufficient"},
		{authorize.OutcomeForbidden, "owner_only", errs.KindForbidden, errs.CodeForbidden, "owner_only"},
		{authorize.OutcomeStepUpRequired, "step_up_required", errs.KindStepUpRequired, errs.CodeStepUpRequired, ""},
		{authorize.OutcomeUnauthenticated, "principal_inactive", errs.KindUnauthenticated, errs.CodeSessionRevoked, ""},
		{authorize.OutcomeUnauthenticated, "session_revoked", errs.KindUnauthenticated, errs.CodeSessionRevoked, ""},
		{authorize.OutcomeUnauthenticated, "device_inactive", errs.KindUnauthenticated, errs.CodeSessionRevoked, ""},
		{authorize.OutcomeUnauthenticated, "unauthenticated", errs.KindUnauthenticated, errs.CodeUnauthenticated, ""},
		{authorize.OutcomeUnauthenticated, "token_revoked", errs.KindUnauthenticated, errs.CodeUnauthenticated, ""},
		{authorize.OutcomeUnauthenticated, "app_not_installed", errs.KindUnauthenticated, errs.CodeUnauthenticated, ""},
		{authorize.OutcomeInvariantViolation, "i1_violation", errs.KindInvariantViolation, errs.CodeI1Violation, "i1_violation"},
		{authorize.OutcomeInvariantViolation, "last_owner", errs.KindInvariantViolation, errs.CodeLastOwner, "last_owner"},
		{authorize.OutcomeInvariantViolation, "billing_mode_mismatch", errs.KindInvariantViolation, errs.CodeBillingModeMismatch, "billing_mode_mismatch"},
		{authorize.OutcomeInvariantViolation, "tool_class_mismatch", errs.KindInvariantViolation, errs.CodeToolClassMismatch, "tool_class_mismatch"},
	}
	for _, c := range cases {
		d := authorize.Decision{Outcome: c.outcome, Reason: authorize.Reason(c.reason)}
		if d.Allowed() {
			t.Fatalf("%v allowed", d)
		}
		e, ok := errs.As(d.Err())
		if !ok {
			t.Fatalf("%s/%s: not an *errs.Error: %v", c.outcome, c.reason, d.Err())
		}
		if e.Kind() != c.kind || e.Code != c.code || e.Reason != c.wantReason {
			t.Errorf("%s/%s: got kind %s code %s reason %q, want %s %s %q", c.outcome, c.reason, e.Kind(), e.Code, e.Reason, c.kind, c.code, c.wantReason)
		}
	}
	allow := authorize.Decision{Outcome: authorize.OutcomeAllow}
	if !allow.Allowed() || allow.Err() != nil {
		t.Fatalf("allow: %v %v", allow.Allowed(), allow.Err())
	}
	// An outcome outside the list fails closed as an internal error, never nil.
	if errs.KindOf(authorize.Decision{Outcome: "bogus"}.Err()) != errs.KindInternal {
		t.Fatal("unknown outcome must be internal")
	}
	// The zero Decision is no allow.
	if (authorize.Decision{}).Allowed() || (authorize.Decision{}).Err() == nil {
		t.Fatal("zero decision must deny")
	}
}

func TestPermissionVersionPin(t *testing.T) {
	ctx := context.Background()
	if _, ok := authorize.PinnedPermissionVersion(ctx); ok {
		t.Fatal("no pin expected")
	}
	ctx = authorize.WithPermissionVersion(ctx, 7)
	if v, ok := authorize.PinnedPermissionVersion(ctx); !ok || v != 7 {
		t.Fatalf("got %d %v", v, ok)
	}
}

func TestRequestScopeMemo(t *testing.T) {
	ctx := context.Background()
	if _, ok := authorize.RequestMemoFrom(ctx); ok {
		t.Fatal("no memo expected")
	}
	ctx = authorize.WithRequestScope(ctx)
	m, ok := authorize.RequestMemoFrom(ctx)
	if !ok {
		t.Fatal("memo expected")
	}
	type key struct{}
	if _, ok := m.Load(key{}); ok {
		t.Fatal("empty memo")
	}
	m.Store(key{}, 3)
	m2, _ := authorize.RequestMemoFrom(ctx)
	if v, ok := m2.Load(key{}); !ok || v.(int) != 3 {
		t.Fatalf("got %v %v", v, ok)
	}
	// A second WithRequestScope inside the same request keeps the existing memo.
	m3, _ := authorize.RequestMemoFrom(authorize.WithRequestScope(ctx))
	if v, ok := m3.Load(key{}); !ok || v.(int) != 3 {
		t.Fatal("nested scope must reuse the memo")
	}
}

func TestFilterMode(t *testing.T) {
	ctx := context.Background()
	if authorize.IsFilter(ctx) {
		t.Fatal("not a filter")
	}
	if !authorize.IsFilter(authorize.Filter(ctx)) {
		t.Fatal("filter expected")
	}
}

// TestDecisionMapping_Interactions covers every row of the decision mapping of interaction.create,
// component.update and interaction.respond (brief M2-15 design note 8 d; 06 sections 4 and 5).
func TestDecisionMapping_Interactions(t *testing.T) {
	cases := []struct {
		outcome    authorize.Outcome
		reason     string
		kind       errs.Kind
		code       errs.Code
		wantReason string
	}{
		{authorize.OutcomeNotFound, "no_grant", errs.KindNotFound, errs.CodeNotFound, ""},
		{authorize.OutcomeNotFound, "not_self", errs.KindNotFound, errs.CodeNotFound, ""},
		{authorize.OutcomeForbidden, "level_insufficient", errs.KindForbidden, errs.CodeForbidden, "level_insufficient"},
		{authorize.OutcomeForbidden, "capability_missing", errs.KindForbidden, errs.CodeForbidden, "capability_missing"},
		{authorize.OutcomeForbidden, "component_restricted", errs.KindForbidden, errs.CodeForbidden, "component_restricted"},
		{authorize.OutcomeForbidden, "subject_kind_not_allowed", errs.KindForbidden, errs.CodeForbidden, "subject_kind_not_allowed"},
		{authorize.OutcomeForbidden, "not_self", errs.KindForbidden, errs.CodeForbidden, "not_self"},
		{authorize.OutcomeForbidden, "view_as_read_only", errs.KindForbidden, errs.CodeForbidden, "view_as_read_only"},
		{authorize.OutcomeForbidden, "channel_read_only", errs.KindForbidden, errs.CodeChannelReadOnly, "channel_read_only"},
		{authorize.OutcomeForbidden, "channel_archived", errs.KindForbidden, errs.CodeChannelReadOnly, "channel_archived"},
		// A lost race answers the same whether authorize or the single-use lock decided; the
		// reason stays in the audit (ruling of design note 8 d).
		{authorize.OutcomeForbidden, "component_inactive", errs.KindConflict, errs.CodeComponentUsed, ""},
		{authorize.OutcomeForbidden, "origin_not_human", errs.KindForbidden, errs.CodeAgentInvocationDenied, "origin_not_human"},
		{authorize.OutcomeForbidden, "agent_not_reachable", errs.KindForbidden, errs.CodeAgentInvocationDenied, "agent_not_reachable"},
		{authorize.OutcomeForbidden, "billing_mode_personal_not_holder", errs.KindForbidden, errs.CodeAgentInvocationDenied, "billing_mode_personal_not_holder"},
		{authorize.OutcomeForbidden, "tool_class_owner_only", errs.KindForbidden, errs.CodeAgentInvocationDenied, "tool_class_owner_only"},
		{authorize.OutcomeForbidden, "project_not_ready", errs.KindForbidden, errs.CodeAgentInvocationDenied, "project_not_ready"},
		{authorize.OutcomeForbidden, "dev_environment_not_ready", errs.KindForbidden, errs.CodeAgentInvocationDenied, "dev_environment_not_ready"},
		{authorize.OutcomeForbidden, "run_active", errs.KindForbidden, errs.CodeAgentInvocationDenied, "run_active"},
		{authorize.OutcomeForbidden, "run_active_other_agent", errs.KindForbidden, errs.CodeAgentInvocationDenied, "run_active_other_agent"},
	}
	for _, c := range cases {
		e, ok := errs.As(authorize.Decision{Outcome: c.outcome, Reason: authorize.Reason(c.reason)}.Err())
		if !ok {
			t.Fatalf("%s/%s: not an *errs.Error", c.outcome, c.reason)
		}
		if e.Kind() != c.kind || e.Code != c.code || e.Reason != c.wantReason {
			t.Errorf("%s/%s: got kind %s code %s reason %q, want %s %s %q", c.outcome, c.reason, e.Kind(), e.Code, e.Reason, c.kind, c.code, c.wantReason)
		}
	}
}
