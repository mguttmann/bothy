// SPDX-License-Identifier: Apache-2.0

package errs

// Code is a stable public error code from the closed list of docs/spec/06-api.md (the generated
// block "problems", which follows x-problem-codes of the OpenAPI document). Clients branch on it,
// so a code never changes its meaning; TestCodesMatchChapter06 holds this list against 06.
type Code string

// The public codes in the order of the 06 block. A code added to 06 gets a constant and a row in
// codeTable in the same change.
const (
	// CodeValidationFailed means the request violates the schema or a rule; errors[] names the fields.
	CodeValidationFailed Code = "validation_failed"
	// CodeMalformedRequest means the body is not valid JSON, has duplicate members, invalid UTF-8 or
	// nests too deep.
	CodeMalformedRequest Code = "malformed_request"
	// CodeCursorInvalid means the cursor is unknown or does not match the filters.
	CodeCursorInvalid Code = "cursor_invalid"
	// CodeWebhookHostNotAllowed means the webhook target host is not in webhook.allowed_hosts.
	CodeWebhookHostNotAllowed Code = "webhook_host_not_allowed"
	// CodeUnauthenticated means there is no valid principal.
	CodeUnauthenticated Code = "unauthenticated"
	// CodeTokenExpired means the access token has expired.
	CodeTokenExpired Code = "token_expired"
	// CodeSessionRevoked means the session, device or principal was revoked.
	CodeSessionRevoked Code = "session_revoked"
	// CodeStepUpRequired means the action needs a fresh confirmation, or a step-up failed.
	CodeStepUpRequired Code = "step_up_required"
	// CodeUseDPoPNonce means the DPoP proof must carry the fresh nonce from the DPoP-Nonce header.
	CodeUseDPoPNonce Code = "use_dpop_nonce"
	// CodeInvalidDPoPProof means the DPoP proof is missing, wrongly signed, expired or bound to
	// another key.
	CodeInvalidDPoPProof Code = "invalid_dpop_proof"
	// CodeForbidden means read access exists, but the level or a capability is missing.
	CodeForbidden Code = "forbidden"
	// CodeScopeMissing means the app lacks the required scope.
	CodeScopeMissing Code = "scope_missing"
	// CodeDevicePending means the device or provisioned account waits for approval.
	CodeDevicePending Code = "device_pending"
	// CodeChannelReadOnly means the channel is read-only or archived.
	CodeChannelReadOnly Code = "channel_read_only"
	// CodeFeatureDisabled means the feature is switched off.
	CodeFeatureDisabled Code = "feature_disabled"
	// CodeAgentInvocationDenied means agent.invoke denies; reason names why.
	CodeAgentInvocationDenied Code = "agent_invocation_denied"
	// CodeTerminalDenied means the terminal right exists, but a guard rule or limit blocks.
	CodeTerminalDenied Code = "terminal_denied"
	// CodeInvocationTokenInvalid means the invocation token is invalid, expired or for another agent.
	CodeInvocationTokenInvalid Code = "invocation_token_invalid"
	// CodeNotFound means the resource does not exist or is not visible.
	CodeNotFound Code = "not_found"
	// CodeMethodNotAllowed means the method is not defined on this path.
	CodeMethodNotAllowed Code = "method_not_allowed"
	// CodeConflict means the state prevents the change.
	CodeConflict Code = "conflict"
	// CodeRequestInProgress means the first request with this Idempotency-Key is still running.
	CodeRequestInProgress Code = "request_in_progress"
	// CodeI1Violation means the change would violate invariant I1; hint names the way out.
	CodeI1Violation Code = "i1_violation"
	// CodeI6Violation means the change would violate invariant I6; hint names the way out.
	CodeI6Violation Code = "i6_violation"
	// CodeLastOwner means at least one active owner must remain.
	CodeLastOwner Code = "last_owner"
	// CodeAlreadyStarted means the run is no longer queued.
	CodeAlreadyStarted Code = "already_started"
	// CodeBillingModeMismatch means the reported billing mode differs from the target.
	CodeBillingModeMismatch Code = "billing_mode_mismatch"
	// CodeToolClassMismatch means the reported tool class is above the one in the invocation token.
	CodeToolClassMismatch Code = "tool_class_mismatch"
	// CodeHostUnavailable means the host is not active or its host service is not connected.
	CodeHostUnavailable Code = "host_unavailable"
	// CodeFingerprintMismatch means the typed fingerprint prefix does not match the host's keys.
	CodeFingerprintMismatch Code = "fingerprint_mismatch"
	// CodeInvalidStateTransition means the state machine does not allow the transition.
	CodeInvalidStateTransition Code = "invalid_state_transition"
	// CodeBaseVersionConflict means base_version of a run patch is outdated; current holds the document.
	CodeBaseVersionConflict Code = "base_version_conflict"
	// CodeComponentUsed means the component is used, expired or disabled.
	CodeComponentUsed Code = "component_used"
	// CodeOperatorMismatch means the target channel has another effective operator.
	CodeOperatorMismatch Code = "operator_mismatch"
	// CodeFileNotReady means the upload is incomplete or the scan is still running.
	CodeFileNotReady Code = "file_not_ready"
	// CodeFileBlocked means the file is never delivered (malware, scan error or failed upload).
	CodeFileBlocked Code = "file_blocked"
	// CodeTitleManual means the thread title was set manually, so the agent keeps it.
	CodeTitleManual Code = "title_manual"
	// CodeResyncRequired means catch-up is not possible; load a snapshot.
	CodeResyncRequired Code = "resync_required"
	// CodeMessageDeleted means the message was deleted in the meantime.
	CodeMessageDeleted Code = "message_deleted"
	// CodeGone means the object has expired.
	CodeGone Code = "gone"
	// CodeVersionConflict means If-Match does not match; current holds the current version.
	CodeVersionConflict Code = "version_conflict"
	// CodePayloadTooLarge means the body or file is above the limit.
	CodePayloadTooLarge Code = "payload_too_large"
	// CodeUnsupportedMediaType means the media type is not allowed.
	CodeUnsupportedMediaType Code = "unsupported_media_type"
	// CodeRangeNotSatisfiable means Range lies outside the file.
	CodeRangeNotSatisfiable Code = "range_not_satisfiable"
	// CodeIdempotencyKeyReuse means the same client ID or Idempotency-Key came with different content.
	CodeIdempotencyKeyReuse Code = "idempotency_key_reuse"
	// CodePreconditionRequired means If-Match is missing.
	CodePreconditionRequired Code = "precondition_required"
	// CodeRateLimited means the rate-limit class is exhausted.
	CodeRateLimited Code = "rate_limited"
	// CodeInternalError means an unexpected error; correlation_id leads to the log entry.
	CodeInternalError Code = "internal_error"
	// CodeUnavailable means the server is not ready.
	CodeUnavailable Code = "unavailable"
	// CodeMaintenance means maintenance mode blocks writing actions.
	CodeMaintenance Code = "maintenance"
	// CodeSetupRequired means the instance is not set up yet.
	CodeSetupRequired Code = "setup_required"
)

// codeTable assigns each code its one kind: the mapping of the brief of M1-05, plus
// docs/DECISIONS.md M1-05/4 for terminal_denied, i6_violation, host_unavailable and
// fingerprint_mismatch, and docs/DECISIONS.md M1-05/19 for billing_mode_mismatch and
// tool_class_mismatch, which docs/spec/07-permissions.md section 4.1 maps to invariant_violation. Its
// order is the order of the 06 block and of AllCodes.
var codeTable = [...]struct {
	code Code
	kind Kind
}{
	{CodeValidationFailed, KindBadRequest},
	{CodeMalformedRequest, KindBadRequest},
	{CodeCursorInvalid, KindBadRequest},
	{CodeWebhookHostNotAllowed, KindBadRequest},
	{CodeUnauthenticated, KindUnauthenticated},
	{CodeTokenExpired, KindUnauthenticated},
	{CodeSessionRevoked, KindUnauthenticated},
	{CodeStepUpRequired, KindStepUpRequired},
	{CodeUseDPoPNonce, KindUnauthenticated},
	{CodeInvalidDPoPProof, KindUnauthenticated},
	{CodeForbidden, KindForbidden},
	{CodeScopeMissing, KindForbidden},
	{CodeDevicePending, KindForbidden},
	{CodeChannelReadOnly, KindForbidden},
	{CodeFeatureDisabled, KindForbidden},
	{CodeAgentInvocationDenied, KindForbidden},
	{CodeTerminalDenied, KindForbidden},
	{CodeInvocationTokenInvalid, KindForbidden},
	{CodeNotFound, KindNotFound},
	{CodeMethodNotAllowed, KindMethodNotAllowed},
	{CodeConflict, KindConflict},
	{CodeRequestInProgress, KindConflict},
	{CodeI1Violation, KindInvariantViolation},
	{CodeI6Violation, KindInvariantViolation},
	{CodeLastOwner, KindInvariantViolation},
	{CodeAlreadyStarted, KindConflict},
	{CodeBillingModeMismatch, KindInvariantViolation},
	{CodeToolClassMismatch, KindInvariantViolation},
	{CodeHostUnavailable, KindConflict},
	{CodeFingerprintMismatch, KindConflict},
	{CodeInvalidStateTransition, KindConflict},
	{CodeBaseVersionConflict, KindConflict},
	{CodeComponentUsed, KindConflict},
	{CodeOperatorMismatch, KindConflict},
	{CodeFileNotReady, KindConflict},
	{CodeFileBlocked, KindConflict},
	{CodeTitleManual, KindConflict},
	{CodeResyncRequired, KindGone},
	{CodeMessageDeleted, KindGone},
	{CodeGone, KindGone},
	{CodeVersionConflict, KindPreconditionFailed},
	{CodePayloadTooLarge, KindTooLarge},
	{CodeUnsupportedMediaType, KindUnsupportedMediaType},
	{CodeRangeNotSatisfiable, KindRangeNotSatisfiable},
	{CodeIdempotencyKeyReuse, KindUnprocessable},
	{CodePreconditionRequired, KindPreconditionRequired},
	{CodeRateLimited, KindRateLimited},
	{CodeInternalError, KindInternal},
	{CodeUnavailable, KindUnavailable},
	{CodeMaintenance, KindUnavailable},
	{CodeSetupRequired, KindUnavailable},
}

// kindByCode indexes codeTable for Code.Kind.
var kindByCode = func() map[Code]Kind {
	m := make(map[Code]Kind, len(codeTable))
	for _, row := range codeTable {
		m[row.code] = row.kind
	}
	return m
}()

// Kind returns the kind of the code, or KindNone for a string that is not a public code.
func (c Code) Kind() Kind {
	return kindByCode[c]
}

// AllCodes returns every public code in the order of the 06 block. The slice is a fresh copy, so
// callers may modify it.
func AllCodes() []Code {
	codes := make([]Code, len(codeTable))
	for i, row := range codeTable {
		codes[i] = row.code
	}
	return codes
}
