// SPDX-License-Identifier: Apache-2.0

package authorize

// TokenVerdict is the verdict of runs on the invocation token of run.start and run.token.renew
// (M2-09): runs verifies the token, authz decides on the verdict (05 rule 1.1, 07 section 4.1).
type TokenVerdict uint8

// The verdicts of an invocation token.
const (
	// TokenVerdictZero is the zero verdict; authz treats it as an internal error.
	TokenVerdictZero TokenVerdict = iota
	// TokenValid is a current token of the run that has not expired.
	TokenValid
	// TokenExpired is the current token of the run after its expiry.
	TokenExpired
	// TokenSuperseded is a token of the run that a newer one replaced or that was already used.
	TokenSuperseded
	// TokenInvalid is a token that does not verify, names another run or agent, or is malformed.
	TokenInvalid
)

// String returns the code of v as the problem body names it in reason ("valid", "expired",
// "superseded", "invalid"), and "" for the zero verdict and a value outside the list.
func (v TokenVerdict) String() string {
	//exhaustive:ignore // The zero verdict and values outside the list have no code ("").
	switch v {
	case TokenValid:
		return "valid"
	case TokenExpired:
		return "expired"
	case TokenSuperseded:
		return "superseded"
	case TokenInvalid:
		return "invalid"
	}
	return ""
}

// TokenFacts describes the invocation token of run.start and run.token.renew (M2-09) and, for
// run.start, the mode the adapter reports. The mode facts are codes, "" when absent; runs sets
// them only when the run would transition, and the guard of run.start (07 section 4.2 step 9)
// checks them only when BillingModeUsed is set.
//
// Example:
//
//	r := authorize.Resource{Kind: authorize.ResourceRun, ID: run.ID, ThreadID: run.ThreadID, OwnerID: run.AgentID,
//		Token: &authorize.TokenFacts{Verdict: authorize.TokenValid, BillingModeUsed: "organization",
//			BillingModeExpected: run.BillingModeExpected, BillingKeyType: "api_key", ToolClassUsed: "chat", ToolClassClaim: "scoped"}}
type TokenFacts struct {
	// Verdict is the verdict of runs on the token.
	Verdict TokenVerdict
	// BillingModeUsed is the billing mode the adapter reports; BillingModeExpected the mode of the
	// run (runs.billing_mode_expected).
	BillingModeUsed, BillingModeExpected string
	// BillingKeyType, BillingProvider and BillingMethod are the codes of the reported billing
	// evidence (key_type, provider, method); never a key or a value.
	BillingKeyType, BillingProvider, BillingMethod string
	// ToolClassUsed is the tool class the adapter loaded; ToolClassClaim the tool_class claim of
	// the token.
	ToolClassUsed, ToolClassClaim string
}
