// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestFinishDecision covers every transition of brief M2-09 note 6b and every reason rule of 6c.
func TestFinishDecision(t *testing.T) {
	t.Parallel()
	transition := func(reason, original string) domain.FinishOutcome {
		return domain.FinishOutcome{Kind: domain.FinishTransition, Reason: reason, Original: original}
	}
	noChange := domain.FinishOutcome{Kind: domain.FinishNoChange}
	q, r, w := domain.StateQueued, domain.StateRunning, domain.StateWaitingInput
	c, f, x := domain.StateCompleted, domain.StateFailed, domain.StateCancelled
	rows := []struct {
		name    string
		current domain.State
		req     domain.FinishRequest
		want    domain.FinishOutcome
		wantErr error
		code    errs.Code
	}{
		// Transitions of 6b.
		{name: "completed from running", current: r, req: domain.FinishRequest{Target: c}, want: transition("", "")},
		{name: "completed drops a reason", current: r, req: domain.FinishRequest{Target: c, Reason: "runtime_error"}, want: transition("", "")},
		{name: "completed from queued", current: q, req: domain.FinishRequest{Target: c}, code: errs.CodeInvalidStateTransition},
		{name: "completed from waiting_input", current: w, req: domain.FinishRequest{Target: c}, code: errs.CodeInvalidStateTransition},
		{name: "failed from queued", current: q, req: domain.FinishRequest{Target: f, Reason: "runtime_error"}, want: transition("runtime_error", "")},
		{name: "failed from running", current: r, req: domain.FinishRequest{Target: f, Reason: "billing_mode_mismatch"}, want: transition("billing_mode_mismatch", "")},
		{name: "failed from waiting_input", current: w, req: domain.FinishRequest{Target: f, Reason: "tool_class_mismatch"}, want: transition("tool_class_mismatch", "")},
		{name: "cancelled from queued", current: q, req: domain.FinishRequest{Target: x, Reason: "user_cancelled"}, want: transition("user_cancelled", "")},
		{name: "cancelled from running", current: r, req: domain.FinishRequest{Target: x, Reason: "access_revoked"}, want: transition("access_revoked", "")},
		{name: "cancelled from waiting_input", current: w, req: domain.FinishRequest{Target: x, Reason: "agent_disabled"}, want: transition("agent_disabled", "")},
		// The same final state again: no change, whatever the reason (rules of 6c still apply).
		{name: "completed again", current: c, req: domain.FinishRequest{Target: c}, want: noChange},
		{name: "failed again", current: f, req: domain.FinishRequest{Target: f, Reason: "runtime_error"}, want: noChange},
		{name: "cancelled again", current: x, req: domain.FinishRequest{Target: x, Reason: "user_cancelled"}, want: noChange},
		// Another target on an ended run.
		{name: "completed on failed", current: f, req: domain.FinishRequest{Target: c}, code: errs.CodeInvalidStateTransition},
		{name: "completed on cancelled", current: x, req: domain.FinishRequest{Target: c}, code: errs.CodeInvalidStateTransition},
		{name: "failed on completed", current: c, req: domain.FinishRequest{Target: f, Reason: "runtime_error"}, code: errs.CodeInvalidStateTransition},
		{name: "failed on cancelled", current: x, req: domain.FinishRequest{Target: f, Reason: "runtime_error"}, code: errs.CodeInvalidStateTransition},
		{name: "cancelled on completed", current: c, req: domain.FinishRequest{Target: x, Reason: "user_cancelled"}, code: errs.CodeInvalidStateTransition},
		{name: "cancelled on failed", current: f, req: domain.FinishRequest{Target: x, Reason: "user_cancelled"}, code: errs.CodeInvalidStateTransition},
		// Reasons of 6c: every adapter reason of failed is kept.
		{name: "failed invalid_invocation_token", current: r, req: domain.FinishRequest{Target: f, Reason: "invalid_invocation_token"}, want: transition("invalid_invocation_token", "")},
		{name: "failed adapter_error", current: r, req: domain.FinishRequest{Target: f, Reason: "adapter_error"}, want: transition("adapter_error", "")},
		{name: "failed runtime_not_authenticated", current: r, req: domain.FinishRequest{Target: f, Reason: "runtime_not_authenticated"}, want: transition("runtime_not_authenticated", "")},
		// A server-only or unknown reason of failed becomes adapter_error with the original.
		{name: "failed stale from the adapter", current: r, req: domain.FinishRequest{Target: f, Reason: "stale"}, want: transition("adapter_error", "stale")},
		{name: "failed cancel reason from the adapter", current: r, req: domain.FinishRequest{Target: f, Reason: "user_cancelled"}, want: transition("adapter_error", "user_cancelled")},
		{name: "failed unknown code", current: r, req: domain.FinishRequest{Target: f, Reason: "quota_exhausted"}, want: transition("adapter_error", "quota_exhausted")},
		// Missing reasons.
		{name: "failed without reason", current: r, req: domain.FinishRequest{Target: f}, wantErr: domain.ErrReasonRequired},
		{name: "cancelled without reason", current: r, req: domain.FinishRequest{Target: x}, wantErr: domain.ErrReasonRequired},
		{name: "missing reason before no change", current: f, req: domain.FinishRequest{Target: f}, wantErr: domain.ErrReasonRequired},
		// Cancel reasons: only the five of a cancel request.
		{name: "cancelled channel_read_only", current: r, req: domain.FinishRequest{Target: x, Reason: "channel_read_only"}, want: transition("channel_read_only", "")},
		{name: "cancelled user_disabled", current: r, req: domain.FinishRequest{Target: x, Reason: "user_disabled"}, want: transition("user_disabled", "")},
		{name: "cancelled cancel_timeout", current: r, req: domain.FinishRequest{Target: x, Reason: "cancel_timeout"}, wantErr: domain.ErrCancelReason},
		{name: "cancelled runtime_error", current: r, req: domain.FinishRequest{Target: x, Reason: "runtime_error"}, wantErr: domain.ErrCancelReason},
		{name: "cancelled unknown", current: x, req: domain.FinishRequest{Target: x, Reason: "nope"}, wantErr: domain.ErrCancelReason},
		// Costs and targets come first.
		{name: "costs", current: r, req: domain.FinishRequest{Target: c, CostsPresent: true}, wantErr: domain.ErrCostsNotEnabled},
		{name: "costs before the target", current: r, req: domain.FinishRequest{Target: q, CostsPresent: true}, wantErr: domain.ErrCostsNotEnabled},
		{name: "target queued", current: r, req: domain.FinishRequest{Target: q}, wantErr: domain.ErrFinishTarget},
		{name: "target running", current: q, req: domain.FinishRequest{Target: r}, wantErr: domain.ErrFinishTarget},
		{name: "target waiting_input", current: r, req: domain.FinishRequest{Target: w}, wantErr: domain.ErrFinishTarget},
		{name: "target unknown", current: r, req: domain.FinishRequest{Target: "done"}, wantErr: domain.ErrFinishTarget},
		{name: "target before the reason", current: r, req: domain.FinishRequest{Target: ""}, wantErr: domain.ErrFinishTarget},
	}
	for _, row := range rows {
		got, err := domain.FinishDecision(row.current, row.req)
		switch {
		case row.wantErr != nil:
			if !errors.Is(err, row.wantErr) {
				t.Errorf("%s: err %v, want %v", row.name, err, row.wantErr)
			}
		case row.code != "":
			var e *errs.Error
			if !errors.As(err, &e) || e.Code != row.code {
				t.Errorf("%s: err %v, want code %s", row.name, err, row.code)
			}
		default:
			if err != nil || got != row.want {
				t.Errorf("%s: got %+v, %v, want %+v", row.name, got, err, row.want)
			}
		}
	}
}

// TestCancelReasons: the five reasons of a cancel request, each a cancelled row of the reason
// table, in the order of the brief; the result is a copy.
func TestCancelReasons(t *testing.T) {
	t.Parallel()
	want := []string{"user_cancelled", "access_revoked", "agent_disabled", "channel_read_only", "user_disabled"}
	got := domain.CancelReasons()
	if !slices.Equal(got, want) {
		t.Fatalf("CancelReasons() = %v, want %v", got, want)
	}
	for _, code := range got {
		if r, ok := domain.ReasonOf(code); !ok || r.State != domain.StateCancelled {
			t.Errorf("%s is no cancelled reason of the table", code)
		}
	}
	got[0] = "changed"
	if domain.CancelReasons()[0] != "user_cancelled" {
		t.Fatal("CancelReasons returned a shared slice")
	}
}
