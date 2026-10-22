// SPDX-License-Identifier: Apache-2.0

package authorize

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// Outcome is the decision of Authorize (docs/spec/05-actions.md section 2.5).
type Outcome string

// The outcomes of 05 section 2.5.
const (
	// OutcomeAllow allows the action.
	OutcomeAllow Outcome = "allow"
	// OutcomeNotFound means no read right on the resource, or the resource is missing (404).
	OutcomeNotFound Outcome = "not_found"
	// OutcomeForbidden means read right exists but a level, capability, scope or guard rule
	// refuses (403).
	OutcomeForbidden Outcome = "forbidden"
	// OutcomeStepUpRequired asks for a fresh confirmation of the person (401).
	OutcomeStepUpRequired Outcome = "step_up_required"
	// OutcomeInvariantViolation means the change would break an invariant (409). Authorize
	// itself never returns it; the invariant check of M1-29 does.
	OutcomeInvariantViolation Outcome = "invariant_violation"
	// OutcomeUnauthenticated means there is no valid principal (401).
	OutcomeUnauthenticated Outcome = "unauthenticated"
)

// Reason is the reason code of a decision. The one closed list of docs/spec/07-permissions.md section
// 4.1 is kernel/auditing.Reason (R-DM-28); kernel ports never import each other, so this type
// carries the same codes as plain text and authz converts its constants. The empty Reason belongs
// to allow.
type Reason string

// Decision is the result of one Authorize call: the outcome and, for every outcome other than
// allow, the reason code of 07 section 4.1.
type Decision struct {
	// Outcome is the decision.
	Outcome Outcome
	// Reason is the reason code; empty for allow.
	Reason Reason
	// QuickReattach is true exactly when the step-up rule of an action with fresh step-up (terminal.attach)
	// was waived by the quick reattach of 08 section 6.3 point 5 (M2-40): the use case then
	// consumes no step-up.
	QuickReattach bool
	// Detail is the sub-reason of a decision whose problem reason differs from the audit reason;
	// today the token verdict ("expired", "superseded", "invalid") of invocation_token_invalid
	// (M2-09). Empty otherwise.
	Detail string
}

// Allowed reports whether d allows the action. The zero Decision is no allow.
func (d Decision) Allowed() bool { return d.Outcome == OutcomeAllow }

// The reasons that the problem body names with their own code (06 section 5).
const (
	reasonScopeMissing    Reason = "scope_missing"
	reasonDevicePending   Reason = "device_pending"
	reasonPrincipalPend   Reason = "principal_pending"
	reasonChannelReadOnly Reason = "channel_read_only"
	reasonChannelArchived Reason = "channel_archived"
	reasonPrincipalInact  Reason = "principal_inactive"
	reasonSessionRevoked  Reason = "session_revoked"
	reasonDeviceInactive  Reason = "device_inactive"
	reasonI1Violation     Reason = "i1_violation"
	reasonLastOwner       Reason = "last_owner"
	reasonBillingMismatch Reason = "billing_mode_mismatch"
	reasonToolMismatch    Reason = "tool_class_mismatch"
	reasonI6Violation     Reason = "i6_violation"
	reasonHostNotActive   Reason = "host_not_active"
	reasonTokenInvalid    Reason = "invocation_token_invalid"
)

// terminalReasons are the reasons of forbidden that answer 403 terminal_denied, with the reason of
// the problem body (07 section 4.1, 24 section 13; M2-40).
var terminalReasons = map[Reason]string{
	"terminal_not_host_assignee":      "not_host_assignee",
	"terminal_server_host_owner_only": "server_host_owner_only",
	"i6_holder_only":                  "i6_holder_only",
	"terminal_no_session_user":        "no_session_user",
	"terminal_profile_unknown":        "profile_unknown",
	"terminal_session_limit":          "session_limit",
}

// Err returns nil for allow and otherwise a kernel/errs error that the problem writer renders
// (06 section 5; mapping table of the brief M1-28):
//
//   - not_found: 404 not_found without the reason, which never leaves the server (R-AZ-01);
//   - forbidden: 403 scope_missing, device_pending (also for principal_pending, 07 section 4.1),
//     channel_read_only (also for channel_archived, 07 section 4.5) or forbidden, each with the
//     reason in the extension reason;
//   - step_up_required: 401 step_up_required;
//   - unauthenticated: 401 session_revoked for principal_inactive, session_revoked and
//     device_inactive, 401 unauthenticated otherwise; no reason;
//   - forbidden with a terminal guard reason (M2-40): 403 terminal_denied with the reason of 24
//     section 13 (not_host_assignee, server_host_owner_only, i6_holder_only, no_session_user,
//     profile_unknown, session_limit); host_not_active gives 409 host_unavailable without one;
//   - forbidden with invocation_token_invalid (M2-09): 403 invocation_token_invalid with Detail,
//     the token verdict, as reason;
//   - invariant_violation: 409 with the reason as code (i1_violation, i6_violation, last_owner,
//     billing_mode_mismatch, tool_class_mismatch) and the reason extension.
//
// An outcome outside the list, and the zero Decision, give 500 internal_error: fail closed.
func (d Decision) Err() error {
	switch d.Outcome {
	case OutcomeAllow:
		return nil
	case OutcomeNotFound:
		return errs.New(errs.CodeNotFound, "")
	case OutcomeForbidden:
		if r, ok := terminalReasons[d.Reason]; ok {
			return errs.New(errs.CodeTerminalDenied, "").WithReason(r)
		}
		if d.Reason == reasonHostNotActive {
			return errs.New(errs.CodeHostUnavailable, "")
		}
		if d.Reason == reasonTokenInvalid {
			return errs.New(errs.CodeInvocationTokenInvalid, "").WithReason(d.Detail)
		}
		code, ok := forbiddenCodes[d.Reason]
		if !ok {
			code = errs.CodeForbidden
		}
		return errs.New(code, "").WithReason(string(d.Reason))
	case OutcomeStepUpRequired:
		return errs.New(errs.CodeStepUpRequired, "")
	case OutcomeUnauthenticated:
		if revokedReasons[d.Reason] {
			return errs.New(errs.CodeSessionRevoked, "")
		}
		return errs.New(errs.CodeUnauthenticated, "")
	case OutcomeInvariantViolation:
		if code, ok := invariantCodes[d.Reason]; ok {
			return errs.New(code, "").WithReason(string(d.Reason))
		}
	}
	return errs.New(errs.CodeInternalError, "authorize: decision without a mapping")
}

// forbiddenCodes are the reasons of forbidden with an own problem code; every other reason is
// forbidden.
var forbiddenCodes = map[Reason]errs.Code{
	reasonScopeMissing:    errs.CodeScopeMissing,
	reasonDevicePending:   errs.CodeDevicePending,
	reasonPrincipalPend:   errs.CodeDevicePending,
	reasonChannelReadOnly: errs.CodeChannelReadOnly,
	// An archived channel or space is read-only for everyone (07 section 4.5, M1-53).
	reasonChannelArchived: errs.CodeChannelReadOnly,
}

// revokedReasons are the reasons of unauthenticated that answer session_revoked.
var revokedReasons = map[Reason]bool{reasonPrincipalInact: true, reasonSessionRevoked: true, reasonDeviceInactive: true}

// invariantCodes are the reasons of invariant_violation and their codes.
var invariantCodes = map[Reason]errs.Code{
	reasonI1Violation:     errs.CodeI1Violation,
	reasonI6Violation:     errs.CodeI6Violation,
	reasonLastOwner:       errs.CodeLastOwner,
	reasonBillingMismatch: errs.CodeBillingModeMismatch,
	reasonToolMismatch:    errs.CodeToolClassMismatch,
}
