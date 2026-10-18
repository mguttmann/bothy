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
