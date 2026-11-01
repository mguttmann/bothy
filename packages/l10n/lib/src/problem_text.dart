// SPDX-License-Identifier: Apache-2.0

/// The text of a server problem code (docs/spec/01-product.md section 9, docs/spec/06-api.md
/// section 5).
library;

import 'generated/app_localizations.dart';

/// The interface text for the problem [code] of a server answer (RFC 9457 `code`), in the
/// language of [l].
///
/// Every code of `x-problem-codes` in docs/api/openapi.yaml has its own message
/// (`problem<Code>`; test/problem_text_test.dart holds the list complete). A known [reason]
/// selects a more specific message where 07 or 06 names one, for example `forbidden` with
/// `capability_missing`; an unknown reason keeps the message of the code. Any other code gives
/// [AppLocalizations.problemFallback], which shows the code, so a code added on the server
/// after this app was built is still visible to support.
///
/// Example: `problemText(l, 'forbidden', reason: 'capability_missing')`.
String problemText(AppLocalizations l, String code, {String? reason}) {
  final specific = switch ((code, reason)) {
    ('forbidden', 'capability_missing') => l.problemForbiddenCapabilityMissing,
    ('forbidden', 'level_insufficient') => l.problemForbiddenLevelInsufficient,
    ('forbidden', 'owner_only') => l.problemForbiddenOwnerOnly,
    ('forbidden', 'view_as_read_only') => l.problemForbiddenViewAsReadOnly,
    ('forbidden', 'delegation_exceeds_own_level') =>
      l.problemForbiddenDelegationLevel,
    ('forbidden', 'delegation_exceeds_own_capabilities') =>
      l.problemForbiddenDelegationCapabilities,
    ('forbidden', 'subject_kind_not_allowed') => l.problemForbiddenSubjectKind,
    ('forbidden', 'step_up_not_possible') =>
      l.problemForbiddenStepUpNotPossible,
    // The password step-up refused by the login rules, counted nothing (08 6.2 and 13).
    ('step_up_required', 'password_locked') =>
      l.problemStepUpRequiredPasswordLocked,
    ('step_up_required', 'password_changed') =>
      l.problemStepUpRequiredPasswordChanged,

    ('channel_read_only', 'channel_archived') =>
      l.problemChannelReadOnlyChannelArchived,
    ('agent_invocation_denied', 'project_not_ready') =>
      l.problemAgentInvocationDeniedProjectNotReady,
    // The reasons of `terminal_denied` (24 13; brief M4-36: denials show the server's reason).
    ('terminal_denied', 'not_host_assignee') =>
      l.problemTerminalDeniedNotHostAssignee,
    ('terminal_denied', 'server_host_owner_only') =>
      l.problemTerminalDeniedServerHostOwnerOnly,
    ('terminal_denied', 'i6_holder_only') =>
      l.problemTerminalDeniedI6HolderOnly,
    ('terminal_denied', 'no_session_user') =>
      l.problemTerminalDeniedNoSessionUser,
    ('terminal_denied', 'session_limit') => l.problemTerminalDeniedSessionLimit,
    ('terminal_denied', 'profile_unknown') =>
      l.problemTerminalDeniedProfileUnknown,
    _ => null,
  };
  if (specific != null) {
    return specific;
  }
  return switch (code) {
    'validation_failed' => l.problemValidationFailed,
    'malformed_request' => l.problemMalformedRequest,
    'cursor_invalid' => l.problemCursorInvalid,
    'webhook_host_not_allowed' => l.problemWebhookHostNotAllowed,
    'unauthenticated' => l.problemUnauthenticated,
    'token_expired' => l.problemTokenExpired,
    'session_revoked' => l.problemSessionRevoked,
    'step_up_required' => l.problemStepUpRequired,
    'use_dpop_nonce' => l.problemUseDpopNonce,
    'invalid_dpop_proof' => l.problemInvalidDpopProof,
    'forbidden' => l.problemForbidden,
    'scope_missing' => l.problemScopeMissing,
    'device_pending' => l.problemDevicePending,
    'channel_read_only' => l.problemChannelReadOnly,
    'feature_disabled' => l.problemFeatureDisabled,
    'agent_invocation_denied' => l.problemAgentInvocationDenied,
    'terminal_denied' => l.problemTerminalDenied,
    'invocation_token_invalid' => l.problemInvocationTokenInvalid,
    'not_found' => l.problemNotFound,
    'method_not_allowed' => l.problemMethodNotAllowed,
    'conflict' => l.problemConflict,
    'request_in_progress' => l.problemRequestInProgress,
    'i1_violation' => l.problemI1Violation,
    'i6_violation' => l.problemI6Violation,
    'last_owner' => l.problemLastOwner,
    'already_started' => l.problemAlreadyStarted,
    'billing_mode_mismatch' => l.problemBillingModeMismatch,
    'tool_class_mismatch' => l.problemToolClassMismatch,
    'host_unavailable' => l.problemHostUnavailable,
    'fingerprint_mismatch' => l.problemFingerprintMismatch,
    'invalid_state_transition' => l.problemInvalidStateTransition,
    'base_version_conflict' => l.problemBaseVersionConflict,
    'component_used' => l.problemComponentUsed,
    'operator_mismatch' => l.problemOperatorMismatch,
    'file_not_ready' => l.problemFileNotReady,
    'file_blocked' => l.problemFileBlocked,
    'title_manual' => l.problemTitleManual,
    'resync_required' => l.problemResyncRequired,
    'message_deleted' => l.problemMessageDeleted,
    'gone' => l.problemGone,
    'version_conflict' => l.problemVersionConflict,
    'payload_too_large' => l.problemPayloadTooLarge,
    'unsupported_media_type' => l.problemUnsupportedMediaType,
    'range_not_satisfiable' => l.problemRangeNotSatisfiable,
    'idempotency_key_reuse' => l.problemIdempotencyKeyReuse,
    'precondition_required' => l.problemPreconditionRequired,
    'rate_limited' => l.problemRateLimited,
    'internal_error' => l.problemInternalError,
    'unavailable' => l.problemUnavailable,
    'maintenance' => l.problemMaintenance,
    'setup_required' => l.problemSetupRequired,
    _ => l.problemFallback(code),
  };
}
