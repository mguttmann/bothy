// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The last_error codes of webhook_deliveries (docs/DECISIONS.md D-M2-26-6): a closed list, never a
// response body (03 webhook_deliveries.last_error; webhooks.md section 4).
const (
	LastErrorTimeout          = "timeout"
	LastErrorConnectionFailed = "connection_failed"
	LastErrorTLSFailed        = "tls_failed"
	LastErrorAddressBlocked   = "address_blocked"
	LastErrorHostNotAllowed   = "host_not_allowed"
	LastErrorHTTPStatus       = "http_status"
	LastErrorThrottled        = "throttled"
	LastErrorGone             = "gone"
	LastErrorNotAuthorized    = "not_authorized"
	LastErrorNotSubscribed    = "not_subscribed"
	LastErrorEventExpired     = "event_expired"
)

// LastErrorCodes returns the closed list of last_error codes in a fresh slice.
func LastErrorCodes() []string {
	return []string{LastErrorTimeout, LastErrorConnectionFailed, LastErrorTLSFailed, LastErrorAddressBlocked,
		LastErrorHostNotAllowed, LastErrorHTTPStatus, LastErrorThrottled, LastErrorGone, LastErrorNotAuthorized,
		LastErrorNotSubscribed, LastErrorEventExpired}
}

// OutcomeKind is the class of one delivery attempt (webhooks.md section 4 table).
type OutcomeKind int

// The classes of an attempt.
const (
	// OutcomeSucceeded is a 2xx answer.
	OutcomeSucceeded OutcomeKind = iota + 1
	// OutcomeGone is 410: the delivery fails and the subscription is disabled.
	OutcomeGone
	// OutcomeThrottled is 429, 502, 503 or 504: retried, Retry-After honoured, not counted.
	OutcomeThrottled
	// OutcomeFailed is every other status, a timeout, a refused or failed connection: retried and
	// counted in consecutive_failures.
	OutcomeFailed
)

// Outcome is the classified result of one attempt: its kind and the last_error code ("" for a
// success).
type Outcome struct {
	Kind  OutcomeKind
	Error string
}

// Counted reports whether the attempt raises consecutive_failures (webhooks.md section 4 point 2:
// only failures count, throttles do not).
func (o Outcome) Counted() bool { return o.Kind == OutcomeFailed }

// Retry reports whether the delivery is tried again (throttle or failure).
func (o Outcome) Retry() bool { return o.Kind == OutcomeThrottled || o.Kind == OutcomeFailed }

// BlockedError is a refusal of the egress client in the terms of this package: the sender adapter
// converts egress.BlockedError into it, so the pure classification needs no platform import.
// Reason is the egress reason (address_blocked, host_not_allowed, scheme_not_allowed,
// url_not_allowed, redirect_not_allowed).
type BlockedError struct {
	Reason string
}

// Error names the reason only.
func (e *BlockedError) Error() string { return "webhook target blocked: " + e.Reason }

// The egress reason that maps to address_blocked; every other refusal is host_not_allowed.
const egressAddressBlocked = "address_blocked"

// Classify maps the answer of one attempt to its outcome (docs/DECISIONS.md D-M2-26-6): with a
// transport error the error decides (a refusal of the egress client, a deadline, a TLS failure,
// any other connection failure); otherwise 2xx succeeds, 410 is gone, 429, 502, 503 and 504
// throttle, and every other status, 3xx included (redirects are never followed), is a counted
// http_status failure.
//
// Example:
//
//	o := domain.Classify(resp.StatusCode, nil) // 503 -> OutcomeThrottled, "throttled"
func Classify(statusCode int, transportErr error) Outcome {
	if transportErr != nil {
		return Outcome{Kind: OutcomeFailed, Error: transportCode(transportErr)}
	}
	switch {
	case statusCode >= 200 && statusCode <= 299:
		return Outcome{Kind: OutcomeSucceeded}
	case statusCode == http.StatusGone:
		return Outcome{Kind: OutcomeGone, Error: LastErrorGone}
	case statusCode == http.StatusTooManyRequests, statusCode == http.StatusBadGateway,
		statusCode == http.StatusServiceUnavailable, statusCode == http.StatusGatewayTimeout:
		return Outcome{Kind: OutcomeThrottled, Error: LastErrorThrottled}
	default:
		return Outcome{Kind: OutcomeFailed, Error: LastErrorHTTPStatus}
	}
}

// transportCode is the last_error code of a transport error.
func transportCode(err error) string {
	var blocked *BlockedError
	if errors.As(err, &blocked) {
		if blocked.Reason == egressAddressBlocked {
			return LastErrorAddressBlocked
		}
		return LastErrorHostNotAllowed
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return LastErrorTimeout
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return LastErrorTimeout
	}
	if tlsFailure(err) {
		return LastErrorTLSFailed
	}
	return LastErrorConnectionFailed
}

// tlsFailure reports a failed TLS handshake or certificate check.
func tlsFailure(err error) bool {
	var (
		verify   *tls.CertificateVerificationError
		alert    tls.AlertError
		header   tls.RecordHeaderError
		unknown  x509.UnknownAuthorityError
		hostname x509.HostnameError
		invalid  x509.CertificateInvalidError
	)
	return errors.As(err, &verify) || errors.As(err, &alert) || errors.As(err, &header) || errors.As(err, &unknown) ||
		errors.As(err, &hostname) || errors.As(err, &invalid)
}

// Backoff returns the pause after the failedAttempts-th throttled or counted failure in a row of
// one delivery: a uniform random duration in [base, min(cap, base * 2^(failedAttempts-1))],
// exponential with full jitter between base and cap (webhooks.md section 4 point 1). rnd(n)
// returns a value in [0, n); tests inject it. failedAttempts below 1 counts as 1, so the first
// retry waits exactly base; a cap below base gives base.
//
// Example:
//
//	d := domain.Backoff(10*time.Second, time.Hour, 3, rand.Int64N) // in [10s, 40s]
func Backoff(base, ceiling time.Duration, failedAttempts int, rnd func(n int64) int64) time.Duration {
	if failedAttempts < 1 {
		failedAttempts = 1
	}
	upper := ceiling
	if shift := failedAttempts - 1; shift < 62 && base <= math.MaxInt64>>shift {
		upper = min(ceiling, base<<shift)
	}
	if upper <= base {
		return base
	}
	return base + time.Duration(rnd(int64(upper-base)+1))
}

// maxRetryAfterSeconds bounds delta-seconds so the duration does not overflow.
const maxRetryAfterSeconds = math.MaxInt64 / int64(time.Second)

// ParseRetryAfter reads a Retry-After header (RFC 9110 section 10.2.3): delta-seconds or an
// HTTP-date relative to now; a date in the past counts as zero. ok is false for an absent, negative
// or malformed value or a number too large for a duration, which the caller ignores.
func ParseRetryAfter(header string, now time.Time) (time.Duration, bool) {
	h := strings.TrimSpace(header)
	if h == "" {
		return 0, false
	}
	if h[0] >= '0' && h[0] <= '9' {
		n, err := strconv.ParseInt(h, 10, 64)
		if err != nil || n > maxRetryAfterSeconds {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	}
	t, err := http.ParseTime(h)
	if err != nil {
		return 0, false
	}
	return max(t.Sub(now), 0), true
}

// RetryConfig holds the timing keys of the deliveries: webhook.retry_backoff_base,
// webhook.retry_backoff_cap and webhook.retry_max_age.
type RetryConfig struct {
	BackoffBase time.Duration
	BackoffCap  time.Duration
	MaxAge      time.Duration
}

// NextAttempt returns the time of the next attempt after a retryable outcome o (throttle or counted
// failure) of the attempt-th attempt, or dead when the retry deadline createdAt +
// webhook.retry_max_age is reached at now (docs/DECISIONS.md D-M2-26-7). The delay is the backoff,
// for a throttle with a valid Retry-After the larger of backoff and Retry-After; a time after the
// deadline is cut to it. The deadline is checked only here, after a failed attempt, so a head that
// waited in a paused subscription still gets its attempt.
func NextAttempt(o Outcome, attempt int, retryAfter time.Duration, hasRetryAfter bool, createdAt, now time.Time, cfg RetryConfig, rnd func(n int64) int64) (time.Time, bool) {
	deadline := createdAt.Add(cfg.MaxAge)
	if !now.Before(deadline) {
		return time.Time{}, true
	}
	delay := Backoff(cfg.BackoffBase, cfg.BackoffCap, attempt, rnd)
	if o.Kind == OutcomeThrottled && hasRetryAfter && retryAfter > delay {
		delay = retryAfter
	}
	next := now.Add(delay)
	if next.After(deadline) || next.Before(now) {
		next = deadline
	}
	return next, false
}
