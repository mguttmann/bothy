// SPDX-License-Identifier: Apache-2.0

package errs

// titles holds the English title of every public code exactly as x-problem-codes of the schema
// Problem in docs/api/openapi.yaml writes it, the one source of titles (06 R-API-23);
// TestTitlesMatchOpenAPI holds the two equal. The brief of M1-18 proposed a title rule (the code
// with spaces and a capital letter), but openapi.yaml deviates from that rule for several codes
// ("Invalid cursor", "Use DPoP nonce", "Too many requests"), and the contract wins
// (docs/DECISIONS.md M1-18/5).
var titles = map[Code]string{
	CodeValidationFailed:       "Validation failed",
	CodeMalformedRequest:       "Malformed request",
	CodeCursorInvalid:          "Invalid cursor",
	CodeWebhookHostNotAllowed:  "Webhook host not allowed",
	CodeUnauthenticated:        "Unauthenticated",
	CodeTokenExpired:           "Token expired",
	CodeSessionRevoked:         "Session revoked",
	CodeStepUpRequired:         "Step-up required",
	CodeUseDPoPNonce:           "Use DPoP nonce",
	CodeInvalidDPoPProof:       "Invalid DPoP proof",
	CodeForbidden:              "Forbidden",
	CodeScopeMissing:           "Scope missing",
	CodeDevicePending:          "Device pending",
	CodeChannelReadOnly:        "Channel read-only",
	CodeFeatureDisabled:        "Feature disabled",
	CodeAgentInvocationDenied:  "Agent invocation denied",
	CodeTerminalDenied:         "Terminal denied",
	CodeInvocationTokenInvalid: "Invocation token invalid",
	CodeNotFound:               "Not found",
	CodeMethodNotAllowed:       "Method not allowed",
	CodeConflict:               "Conflict",
	CodeRequestInProgress:      "Request in progress",
	CodeI1Violation:            "I1 violation",
	CodeI6Violation:            "I6 violation",
	CodeLastOwner:              "Last owner",
	CodeAlreadyStarted:         "Already started",
	CodeBillingModeMismatch:    "Billing mode mismatch",
	CodeToolClassMismatch:      "Tool class mismatch",
	CodeHostUnavailable:        "Host unavailable",
	CodeFingerprintMismatch:    "Fingerprint mismatch",
	CodeInvalidStateTransition: "Invalid state transition",
	CodeBaseVersionConflict:    "Base version conflict",
	CodeComponentUsed:          "Component used",
	CodeOperatorMismatch:       "Operator mismatch",
	CodeFileNotReady:           "File not ready",
	CodeFileBlocked:            "File blocked",
	CodeTitleManual:            "Title set manually",
	CodeResyncRequired:         "Resync required",
	CodeMessageDeleted:         "Message deleted",
	CodeGone:                   "Gone",
	CodeVersionConflict:        "Version conflict",
	CodePayloadTooLarge:        "Payload too large",
	CodeUnsupportedMediaType:   "Unsupported media type",
	CodeRangeNotSatisfiable:    "Range not satisfiable",
	CodeIdempotencyKeyReuse:    "Idempotency key reused",
	CodePreconditionRequired:   "Precondition required",
	CodeRateLimited:            "Too many requests",
	CodeInternalError:          "Internal error",
	CodeUnavailable:            "Service unavailable",
	CodeMaintenance:            "Maintenance",
	CodeSetupRequired:          "Setup required",
}

// Title returns the English title of the code for the problem body (RFC 9457 "title"), or "" for
// a string that is not a public code.
func (c Code) Title() string {
	return titles[c]
}
