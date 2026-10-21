// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestTokenVerdictString: the verdicts render as the codes of the problem reason (M2-09).
func TestTokenVerdictString(t *testing.T) {
	for v, want := range map[authorize.TokenVerdict]string{
		authorize.TokenVerdictZero: "", authorize.TokenValid: "valid", authorize.TokenExpired: "expired",
		authorize.TokenSuperseded: "superseded", authorize.TokenInvalid: "invalid", authorize.TokenVerdict(99): "",
	} {
		if got := v.String(); got != want {
			t.Errorf("%d: got %q, want %q", v, got, want)
		}
	}
}

// TestDecisionErr_InvocationTokenInvalid: forbidden invocation_token_invalid answers the problem
// code invocation_token_invalid with the verdict as reason, while the decision keeps the audit
// reason invocation_token_invalid (brief M2-09 design note 2c).
func TestDecisionErr_InvocationTokenInvalid(t *testing.T) {
	for _, detail := range []string{"expired", "superseded", "invalid"} {
		d := authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "invocation_token_invalid", Detail: detail}
		e, ok := errs.As(d.Err())
		if !ok {
			t.Fatalf("%s: not an *errs.Error: %v", detail, d.Err())
		}
		if e.Kind() != errs.KindForbidden || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != detail {
			t.Errorf("%s: got kind %s code %s reason %q", detail, e.Kind(), e.Code, e.Reason)
		}
		if d.Reason != "invocation_token_invalid" {
			t.Errorf("%s: audit reason changed to %q", detail, d.Reason)
		}
	}
}
