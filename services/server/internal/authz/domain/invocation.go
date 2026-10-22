// SPDX-License-Identifier: Apache-2.0

package domain

import "strings"

// InvocationVerdict is the verdict of runs on the invocation token of run.start and
// run.token.renew (M2-09); it mirrors kernel/authorize.TokenVerdict value for value, and the
// application layer converts it.
type InvocationVerdict uint8

// The verdicts of an invocation token, in the order of kernel/authorize.TokenVerdict.
const (
	// InvocationVerdictZero is the zero verdict; Decide treats it as an internal error.
	InvocationVerdictZero InvocationVerdict = iota
	// InvocationValid is a current token that has not expired.
	InvocationValid
	// InvocationExpired is the current token after its expiry.
	InvocationExpired
	// InvocationSuperseded is a token that a newer one replaced or that was already used.
	InvocationSuperseded
	// InvocationInvalid is a token that does not verify or names another run or agent.
	InvocationInvalid
)

// String returns the code of v ("valid", "expired", "superseded", "invalid"), "" for the zero
// verdict and a value outside the list.
func (v InvocationVerdict) String() string {
	//exhaustive:ignore // The zero verdict and values outside the list have no code ("").
	switch v {
	case InvocationValid:
		return "valid"
	case InvocationExpired:
		return "expired"
	case InvocationSuperseded:
		return "superseded"
	case InvocationInvalid:
		return "invalid"
	}
	return ""
}

// InvocationFacts are the facts of the invocation token of run.start and run.token.renew and,
// for run.start, the mode the adapter reports (codes, "" when absent; brief M2-09 design note 2).
type InvocationFacts struct {
	// Verdict is the verdict of runs on the token.
	Verdict InvocationVerdict
	// BillingModeUsed is the reported billing mode, BillingModeExpected the mode of the run.
	BillingModeUsed, BillingModeExpected string
	// BillingKeyType, BillingProvider and BillingMethod are the codes of the billing evidence.
	BillingKeyType, BillingProvider, BillingMethod string
	// ToolClassUsed is the loaded tool class, ToolClassClaim the tool_class claim of the token.
	ToolClassUsed, ToolClassClaim string
}

// keyTypeUnknown is the key_type of billing evidence the adapter could not classify; it counts as
// unequal to every configured mode (10 section 7, RK-13).
const keyTypeUnknown = "unknown"

// actionRunStart is the action with the mode guard.
const actionRunStart = "run.start"

// Token is true when the verdict of the invocation token is one of allowed (M2-09): run.start
// takes Token(InvocationValid), run.token.renew Token(InvocationValid, InvocationExpired). It
// fails with invocation_token_invalid, and Decide puts the verdict into Decision.Detail. Missing
// facts and the zero verdict are internal errors.
func Token(allowed ...InvocationVerdict) Expr {
	return Expr{op: opToken, verdicts: append([]InvocationVerdict(nil), allowed...)}
}

// tokenAtom is the atom Token.
func (v evaluator) tokenAtom(e Expr) (bool, Reason, error) {
	if err := checkInvocation(v.in); err != nil {
		return false, "", err
	}
	for _, a := range e.verdicts {
		if a == v.in.Invocation.Verdict {
			return true, "", nil
		}
	}
	return false, ReasonInvocationTokenInvalid, nil
}

// checkInvocation reports missing token facts or a verdict outside the list as an internal error.
func checkInvocation(in *Input) error {
	if in.Invocation == nil {
		return internalError(in.Action + " without its token facts")
	}
	if in.Invocation.Verdict.String() == "" {
		return internalError(in.Action + " with an unknown token verdict")
	}
	return nil
}

// tokenString renders Token for String, for example Token(valid,expired).
func tokenString(e Expr) string {
	parts := make([]string, len(e.verdicts))
	for i, v := range e.verdicts {
		parts[i] = v.String()
	}
	return "Token(" + strings.Join(parts, ",") + ")"
}

// tokenDetail is the Detail of a denial by Token: the verdict.
func tokenDetail(in *Input, d Decision) Decision {
	if d.Reason == ReasonInvocationTokenInvalid && in.Invocation != nil {
		d.Detail = in.Invocation.Verdict.String()
	}
	return d
}

// runStartGuard is the guard of run.start (07 section 4.2 step 9, brief M2-09 design note 2b),
// only when a mode is reported: an unknown key type or a mode other than the run's is
// billing_mode_mismatch; then a loaded tool class above the claim (chat < scoped < host, an
// unknown class counts as host) is tool_class_mismatch.
func runStartGuard(f *InvocationFacts) Decision {
	if f.BillingModeUsed == "" {
		return Allow
	}
	if f.BillingKeyType == keyTypeUnknown || f.BillingModeUsed != f.BillingModeExpected {
		return deny(OutcomeInvariantViolation, ReasonBillingModeMismatch)
	}
	if toolRank(f.ToolClassUsed) > toolRank(f.ToolClassClaim) {
		return deny(OutcomeInvariantViolation, ReasonToolClassMismatch)
	}
	return Allow
}
