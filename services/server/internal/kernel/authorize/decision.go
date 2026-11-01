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
	// PasskeyRequired marks a step_up_required of terminal.open or terminal.attach from the web
	// console whose step-up was no passkey with user verification (08 section 6.3 point 3, M6-18):
	// the problem body carries reason passkey_required; the audit reason stays
	// terminal_step_up_not_fresh of 07 section 4.1.
	PasskeyRequired bool
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
	// reasonComponentInactive is a component that is not active, has expired or belongs to an
	// ended run (M2-15).
	reasonComponentInactive Reason = "component_inactive"
)

// ReasonPasskeyRequired is the problem reason of a console terminal action without a passkey
// step-up with user verification (08 section 6.3, 24 section 13; M6-18). It is a reason of the
// problem body only, not of the closed audit list of 07 section 4.1.
const ReasonPasskeyRequired = "passkey_required"

// terminalReasons are the reasons of forbidden that answer 403 terminal_denied, with the reason of
// the problem body (07 section 4.1, 24 section 13; M2-40).
var terminalReasons = map[Reason]string{
	"terminal_not_host_assignee":        "not_host_assignee",
	"terminal_server_host_owner_only":   "server_host_owner_only",
	"i6_holder_only":                    "i6_holder_only",
	"terminal_no_session_user":          "no_session_user",
	"terminal_owner_account_owner_only": "owner_account_owner_only",
	"terminal_profile_unknown":          "profile_unknown",
	"terminal_session_limit":            "session_limit",
}

// Err returns nil for allow and otherwise a kernel/errs error that the problem writer renders
// (06 section 5; mapping table of the brief M1-28):
//
//   - not_found: 404 not_found without the reason, which never leaves the server (R-AZ-01);
//   - forbidden: 403 scope_missing, device_pending (also for principal_pending, 07 section 4.1),
//     channel_read_only (also for channel_archived, 07 section 4.5) or forbidden, each with the
//     reason in the extension reason;
//   - step_up_required: 401 step_up_required, with reason passkey_required when PasskeyRequired
//     marks the console rule of the fresh terminal step-up (M6-18);
//   - unauthenticated: 401 session_revoked for principal_inactive, session_revoked and
//     device_inactive, 401 unauthenticated otherwise; no reason;
//   - forbidden with a terminal guard reason (M2-40): 403 terminal_denied with the reason of 24
//     section 13 (not_host_assignee, server_host_owner_only, i6_holder_only, no_session_user,
//     profile_unknown, session_limit); host_not_active gives 409 host_unavailable without one;
//   - forbidden with invocation_token_invalid (M2-09): 403 invocation_token_invalid with Detail,
//     the token verdict, as reason;
//   - forbidden with component_inactive (M2-15): 409 component_used without the reason, which
//     stays in the audit, so a lost race answers the same whether authorize or the single-use
//     lock decided (brief M2-15 design note 8 d);
//   - forbidden with a reason of the agent.invoke guard (M2-15): origin_not_human,
//     agent_not_reachable, billing_mode_personal_not_holder, tool_class_owner_only,
//     project_not_ready, run_active and run_active_other_agent give 403 agent_invocation_denied
//     with the reason; every other reason of interaction.create, component.update and
//     interaction.respond (level_insufficient, capability_missing, component_restricted,
//     subject_kind_not_allowed, not_self, view_as_read_only) stays 403 forbidden with the reason;
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
		return d.forbiddenErr()
	case OutcomeStepUpRequired:
		if d.PasskeyRequired {
			return errs.New(errs.CodeStepUpRequired, "").WithReason(ReasonPasskeyRequired)
		}
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

// forbiddenErr is the problem of a forbidden decision, as listed at Err.
func (d Decision) forbiddenErr() error {
	if r, ok := terminalReasons[d.Reason]; ok {
		return errs.New(errs.CodeTerminalDenied, "").WithReason(r)
	}
	if d.Reason == reasonHostNotActive {
		return errs.New(errs.CodeHostUnavailable, "")
	}
	if d.Reason == reasonTokenInvalid {
		return errs.New(errs.CodeInvocationTokenInvalid, "").WithReason(d.Detail)
	}
	if d.Reason == reasonComponentInactive {
		return errs.New(errs.CodeComponentUsed, "")
	}
	code, ok := forbiddenCodes[d.Reason]
	if !ok {
		code = errs.CodeForbidden
	}
	return errs.New(code, "").WithReason(string(d.Reason))
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
	// The guard of agent.invoke (05 section 5 points 1 to 5, 7 and 8), also on interaction.create
	// (brief M2-15 design note 8 d): the person may use the channel, but the agent may not be
	// triggered now.
	"origin_not_human":                 errs.CodeAgentInvocationDenied,
	"agent_not_reachable":              errs.CodeAgentInvocationDenied,
	"billing_mode_personal_not_holder": errs.CodeAgentInvocationDenied,
	"tool_class_owner_only":            errs.CodeAgentInvocationDenied,
	"project_not_ready":                errs.CodeAgentInvocationDenied,
	"dev_environment_not_ready":        errs.CodeAgentInvocationDenied,
	"run_active":                       errs.CodeAgentInvocationDenied,
	"run_active_other_agent":           errs.CodeAgentInvocationDenied,
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
