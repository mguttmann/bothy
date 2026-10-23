// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"math"
	"time"
)

// ResultClass is the class of a provider answer of 13 sections 7.1 to 7.3; the provider adapters
// (M3-20, M3-21) map their status codes onto it.
type ResultClass string

// The result classes of the table in 13 7.3 (docs/DECISIONS.md D-M3-19-12).
const (
	// ResultAccepted: the provider took the push.
	ResultAccepted ResultClass = "accepted"
	// ResultDisable: the token is invalid for good; SendResult.DisableReason says why.
	ResultDisable ResultClass = "disable"
	// ResultRetry: a transient failure of the provider or the network; counts towards
	// push.max_consecutive_failures.
	ResultRetry ResultClass = "retry"
	// ResultThrottled: the provider limits the server, not the token; retried without counting.
	ResultThrottled ResultClass = "throttled"
	// ResultConfigError: the server's provider configuration is wrong; the adapter raises its
	// alert, the hint is dropped.
	ResultConfigError ResultClass = "config_error"
	// ResultProgramError: the request was malformed; the hint is dropped.
	ResultProgramError ResultClass = "program_error"
)

// The values of push_registrations.disabled_reason (03).
const (
	// DisableTokenInvalid: the provider reports the token as unknown or expired.
	DisableTokenInvalid = "token_invalid"
	// DisableDeviceRevoked: the device was revoked (M3-22).
	DisableDeviceRevoked = "device_revoked"
	// DisableLogout: the session of the registration ended (M3-22).
	DisableLogout = "logout"
	// DisableTopicMismatch: the token belongs to another app topic (APNs).
	DisableTopicMismatch = "topic_mismatch"
	// DisableSenderMismatch: the token belongs to another sender (FCM).
	DisableSenderMismatch = "sender_mismatch"
	// DisableRepeatedFailures: push.max_consecutive_failures transient failures in a row.
	DisableRepeatedFailures = "repeated_failures"
)

// The values of the attribute bothy.outcome of bothy.push.sent (docs/observability/signals.md).
const (
	// OutcomeAccepted: the provider took the push.
	OutcomeAccepted = "accepted"
	// OutcomeRejectedToken: the token was refused.
	OutcomeRejectedToken = "rejected_token"
	// OutcomeProviderError: a transient failure.
	OutcomeProviderError = "provider_error"
	// OutcomeThrottled: the provider limited the server.
	OutcomeThrottled = "throttled"
	// OutcomeRejectedOther: a configuration or program error.
	OutcomeRejectedOther = "rejected_other"
)

// SendResult is the answer of one provider request (PushProvider.Send).
type SendResult struct {
	// Class is the result class.
	Class ResultClass
	// DisableReason is token_invalid, topic_mismatch or sender_mismatch for ResultDisable.
	DisableReason string
	// InvalidSince is the time since which the provider reports the token invalid (the timestamp
	// of an APNs 410); zero when the provider gives none.
	InvalidSince time.Time
	// RetryAfter is the provider's Retry-After; zero for none.
	RetryAfter time.Duration
}

// RegistrationState holds the delivery columns of one push_registrations row (03); zero times
// stand for NULL.
type RegistrationState struct {
	// CreatedAt is created_at.
	CreatedAt time.Time
	// LastSuccessAt is last_success_at.
	LastSuccessAt time.Time
	// FailureCount is failure_count.
	FailureCount int
	// DisabledAt is disabled_at.
	DisabledAt time.Time
	// DisabledReason is disabled_reason, "" for NULL.
	DisabledReason string
}

// RegistrationChange is what ApplyResult decides for the registration and the hint.
type RegistrationChange struct {
	// State is the new state of the row; equal to the input when Changed is false.
	State RegistrationState
	// Changed reports that the row must be written.
	Changed bool
	// Retry reports that the hint is tried again after RetryDelay, unless it is past push.max_age.
	Retry bool
	// Outcome is the value of bothy.outcome.
	Outcome string
}

// ApplyResult applies the table of 13 7.3 to the registration state of one send (brief M3-19
// design note 13, D-M3-19-12):
//
//   - accepted: last_success_at = now and failure_count = 0; the hint is done;
//   - disable: disabled_at = now with the reason, unless InvalidSince is set and the row was
//     created after it (13 7.1 row 410: a newer registration of the same token stays); the hint
//     is done;
//   - retry: failure_count + 1 and, at maxFailures, disabled_at = now with repeated_failures; the
//     hint is retried unless the row was deactivated;
//   - throttled: the row stays (the limit is the server's, not the token's); the hint is retried;
//   - config_error and program_error: the row stays; the hint is dropped.
//
// A disable with another reason and an unknown class count as a program error of the adapter.
// ApplyResult never mutates state.
func ApplyResult(state RegistrationState, res SendResult, now time.Time, maxFailures int) RegistrationChange {
	out := RegistrationChange{State: state, Outcome: OutcomeRejectedOther}
	switch res.Class {
	case ResultAccepted:
		out.State.LastSuccessAt, out.State.FailureCount = now, 0
		out.Changed, out.Outcome = true, OutcomeAccepted
	case ResultDisable:
		if !providerDisableReason(res.DisableReason) {
			return out
		}
		out.Outcome = OutcomeRejectedToken
		if !res.InvalidSince.IsZero() && state.CreatedAt.After(res.InvalidSince) {
			return out
		}
		out.State.DisabledAt, out.State.DisabledReason = now, res.DisableReason
		out.Changed = true
	case ResultRetry:
		out.State.FailureCount++
		out.Changed, out.Outcome = true, OutcomeProviderError
		if out.State.FailureCount >= maxFailures {
			out.State.DisabledAt, out.State.DisabledReason = now, DisableRepeatedFailures
			return out
		}
		out.Retry = true
	case ResultThrottled:
		out.Retry, out.Outcome = true, OutcomeThrottled
	case ResultConfigError, ResultProgramError:
	}
	return out
}

// providerDisableReason reports the reasons a provider answer may set.
func providerDisableReason(r string) bool {
	return r == DisableTokenInvalid || r == DisableTopicMismatch || r == DisableSenderMismatch
}

// RetryDelay returns the pause before the attempt after attempt failed: min(maxDelay, base *
// 2^(attempt-1)) scaled by jitter, and at least retryAfter (13 7.2; base push.retry_base, maxDelay
// push.retry_cap). The caller draws jitter; values outside [0.5, 1] are clamped into it. An
// attempt below 1 counts as 1.
//
// Example:
//
//	d := domain.RetryDelay(3, base, maxDelay, 0, 1) // base * 4, unless that is above maxDelay
func RetryDelay(attempt int64, base, maxDelay, retryAfter time.Duration, jitter float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := maxDelay
	if attempt-1 < 62 {
		if f := float64(base) * math.Pow(2, float64(attempt-1)); f < float64(maxDelay) {
			d = time.Duration(f)
		}
	}
	jitter = min(1, max(0.5, jitter))
	d = time.Duration(float64(d) * jitter)
	return max(d, retryAfter)
}
