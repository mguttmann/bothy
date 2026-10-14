// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestDecisionErr_TerminalReasons covers the rows of the decision mapping of M2-40 (Design note
// 12; 07 section 4.1, 24 section 13): the terminal guards give 403 terminal_denied with the
// reason of 24, host_not_active 409 host_unavailable, a stale fresh step-up 401 step_up_required
// without a reason and i6_violation 409 i6_violation; the audit reason stays the code of 07.
func TestDecisionErr_TerminalReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		outcome    authorize.Outcome
		reason     string
		code       errs.Code
		status     int
		wantReason string
	}{
		{authorize.OutcomeForbidden, "terminal_not_host_assignee", errs.CodeTerminalDenied, http.StatusForbidden, "not_host_assignee"},
		{authorize.OutcomeForbidden, "terminal_server_host_owner_only", errs.CodeTerminalDenied, http.StatusForbidden, "server_host_owner_only"},
		{authorize.OutcomeForbidden, "i6_holder_only", errs.CodeTerminalDenied, http.StatusForbidden, "i6_holder_only"},
		{authorize.OutcomeForbidden, "terminal_no_session_user", errs.CodeTerminalDenied, http.StatusForbidden, "no_session_user"},
		{authorize.OutcomeForbidden, "terminal_profile_unknown", errs.CodeTerminalDenied, http.StatusForbidden, "profile_unknown"},
		{authorize.OutcomeForbidden, "terminal_session_limit", errs.CodeTerminalDenied, http.StatusForbidden, "session_limit"},
		{authorize.OutcomeForbidden, "host_not_active", errs.CodeHostUnavailable, http.StatusConflict, ""},
		{authorize.OutcomeStepUpRequired, "terminal_step_up_not_fresh", errs.CodeStepUpRequired, http.StatusUnauthorized, ""},
		{authorize.OutcomeInvariantViolation, "i6_violation", errs.CodeI6Violation, http.StatusConflict, "i6_violation"},
	}
	for _, c := range cases {
		d := authorize.Decision{Outcome: c.outcome, Reason: authorize.Reason(c.reason)}
		e, ok := errs.As(d.Err())
		if !ok {
			t.Fatalf("%s/%s: not an *errs.Error", c.outcome, c.reason)
		}
		if e.Code != c.code || e.Kind().Status() != c.status || e.Reason != c.wantReason {
			t.Errorf("%s/%s: code %s status %d reason %q, want %s %d %q", c.outcome, c.reason, e.Code, e.Kind().Status(), e.Reason, c.code, c.status, c.wantReason)
		}
		if string(d.Reason) != c.reason {
			t.Errorf("%s: the audit reason changed to %q", c.reason, d.Reason)
		}
	}
}
