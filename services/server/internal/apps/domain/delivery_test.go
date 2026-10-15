// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
)

// timeoutErr is a transport error that reports a timeout, like a net.Error of a dial.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestClassify(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		status  int
		err     error
		kind    domain.OutcomeKind
		code    string
		counted bool
	}{
		{name: "200", status: 200, kind: domain.OutcomeSucceeded},
		{name: "204", status: 204, kind: domain.OutcomeSucceeded},
		{name: "299", status: 299, kind: domain.OutcomeSucceeded},
		{name: "410", status: 410, kind: domain.OutcomeGone, code: domain.LastErrorGone},
		{name: "429", status: 429, kind: domain.OutcomeThrottled, code: domain.LastErrorThrottled},
		{name: "502", status: 502, kind: domain.OutcomeThrottled, code: domain.LastErrorThrottled},
		{name: "503", status: 503, kind: domain.OutcomeThrottled, code: domain.LastErrorThrottled},
		{name: "504", status: 504, kind: domain.OutcomeThrottled, code: domain.LastErrorThrottled},
		{name: "300", status: 300, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "302", status: 302, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "307", status: 307, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "400", status: 400, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "401", status: 401, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "404", status: 404, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "500", status: 500, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "501", status: 501, kind: domain.OutcomeFailed, code: domain.LastErrorHTTPStatus, counted: true},
		{name: "address blocked", err: &domain.BlockedError{Reason: "address_blocked"}, kind: domain.OutcomeFailed,
			code: domain.LastErrorAddressBlocked, counted: true},
		{name: "host not allowed", err: &domain.BlockedError{Reason: "host_not_allowed"}, kind: domain.OutcomeFailed,
			code: domain.LastErrorHostNotAllowed, counted: true},
		{name: "scheme not allowed", err: &domain.BlockedError{Reason: "scheme_not_allowed"}, kind: domain.OutcomeFailed,
			code: domain.LastErrorHostNotAllowed, counted: true},
		{name: "url not allowed", err: &domain.BlockedError{Reason: "url_not_allowed"}, kind: domain.OutcomeFailed,
			code: domain.LastErrorHostNotAllowed, counted: true},
		{name: "wrapped block", err: fmt.Errorf("post: %w", &domain.BlockedError{Reason: "address_blocked"}),
			kind: domain.OutcomeFailed, code: domain.LastErrorAddressBlocked, counted: true},
		{name: "deadline", err: context.DeadlineExceeded, kind: domain.OutcomeFailed, code: domain.LastErrorTimeout, counted: true},
		{name: "net timeout", err: timeoutErr{}, kind: domain.OutcomeFailed, code: domain.LastErrorTimeout, counted: true},
		{name: "unknown authority", err: fmt.Errorf("tls: %w", x509.UnknownAuthorityError{}), kind: domain.OutcomeFailed,
			code: domain.LastErrorTLSFailed, counted: true},
		{name: "hostname", err: x509.HostnameError{Host: "hooks.example.com"}, kind: domain.OutcomeFailed,
			code: domain.LastErrorTLSFailed, counted: true},
		{name: "verification", err: &tls.CertificateVerificationError{Err: errors.New("bad")}, kind: domain.OutcomeFailed,
			code: domain.LastErrorTLSFailed, counted: true},
		{name: "alert", err: tls.AlertError(40), kind: domain.OutcomeFailed, code: domain.LastErrorTLSFailed, counted: true},
		{name: "record header", err: tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"},
			kind: domain.OutcomeFailed, code: domain.LastErrorTLSFailed, counted: true},
		{name: "reset", err: &net.OpError{Op: "read", Err: syscall.ECONNRESET}, kind: domain.OutcomeFailed,
			code: domain.LastErrorConnectionFailed, counted: true},
		{name: "refused", err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, kind: domain.OutcomeFailed,
			code: domain.LastErrorConnectionFailed, counted: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			o := domain.Classify(c.status, c.err)
			if o.Kind != c.kind || o.Error != c.code || o.Counted() != c.counted {
				t.Fatalf("Classify(%d, %v) = %+v counted %v, want %v %q counted %v", c.status, c.err, o, o.Counted(), c.kind, c.code, c.counted)
			}
			if retry := c.kind == domain.OutcomeThrottled || c.kind == domain.OutcomeFailed; o.Retry() != retry {
				t.Fatalf("Retry() = %v, want %v", o.Retry(), retry)
			}
		})
	}
}

// fixedRand returns a random source that always returns n-1 (the largest value) or 0.
func fixedRand(max bool) func(n int64) int64 {
	return func(n int64) int64 {
		if max {
			return n - 1
		}
		return 0
	}
}

func TestBackoff(t *testing.T) {
	t.Parallel()
	base, limit := 10*time.Second, time.Hour
	if got := domain.Backoff(base, limit, 1, fixedRand(true)); got != base {
		t.Fatalf("first retry %v, want exactly base %v", got, base)
	}
	for n, want := range map[int]time.Duration{2: 20 * time.Second, 3: 40 * time.Second, 4: 80 * time.Second, 10: limit, 70: limit} {
		if got := domain.Backoff(base, limit, n, fixedRand(true)); got != want {
			t.Fatalf("upper bound after %d failures %v, want %v", n, got, want)
		}
		if got := domain.Backoff(base, limit, n, fixedRand(false)); got != base {
			t.Fatalf("lower bound after %d failures %v, want %v", n, got, base)
		}
	}
	var seen []int64
	rnd := func(n int64) int64 {
		seen = append(seen, n)
		return n / 2
	}
	if got := domain.Backoff(base, limit, 3, rnd); got != base+15*time.Second {
		t.Fatalf("jitter from the source: %v", got)
	}
	if len(seen) != 1 || seen[0] != int64(30*time.Second)+1 {
		t.Fatalf("random source asked for %v, want the width of [base, 4 base]", seen)
	}
}

func FuzzBackoff(f *testing.F) {
	f.Add(int64(time.Second), int64(time.Hour), 1, int64(0))
	f.Add(int64(10*time.Second), int64(time.Hour), 7, int64(12345))
	f.Add(int64(time.Millisecond), int64(time.Millisecond), 100, int64(-3))
	f.Add(int64(1), int64(1<<62), 63, int64(1<<40))
	f.Fuzz(func(t *testing.T, base, limit int64, n int, r int64) {
		if base <= 0 || limit < base || n < 1 {
			return
		}
		rnd := func(width int64) int64 {
			if width <= 0 {
				t.Fatalf("width %d", width)
			}
			v := r % width
			if v < 0 {
				v += width
			}
			return v
		}
		got := domain.Backoff(time.Duration(base), time.Duration(limit), n, rnd)
		if got < time.Duration(base) || got > time.Duration(limit) {
			t.Fatalf("Backoff(%d, %d, %d) = %d outside [base, cap]", base, limit, n, got)
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"120", 120 * time.Second, true},
		{"0", 0, true},
		{" 7 ", 7 * time.Second, true},
		{now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second, true},
		{"Tue, 13 Oct 2026 09:05:00 GMT", 5 * time.Minute, true},
		{"Tue, 13 Oct 2026 08:00:00 GMT", 0, true},
		{"-5", 0, false},
		{"", 0, false},
		{"soon", 0, false},
		{"1.5", 0, false},
		{"99999999999999999999999", 0, false},
	}
	for _, c := range cases {
		got, ok := domain.ParseRetryAfter(c.in, now)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseRetryAfter(%q) = %v %v, want %v %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func FuzzParseRetryAfter(f *testing.F) {
	for _, s := range []string{"120", "", "-1", "Tue, 13 Oct 2026 09:05:00 GMT", "9223372036854775807", "x"} {
		f.Add(s)
	}
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, s string) {
		d, ok := domain.ParseRetryAfter(s, now)
		if d < 0 || (!ok && d != 0) {
			t.Fatalf("ParseRetryAfter(%q) = %v %v", s, d, ok)
		}
	})
}

func TestNextAttempt(t *testing.T) {
	t.Parallel()
	cfg := domain.RetryConfig{BackoffBase: 10 * time.Second, BackoffCap: time.Hour, MaxAge: 72 * time.Hour}
	created := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	now := created.Add(time.Minute)
	failed := domain.Outcome{Kind: domain.OutcomeFailed, Error: domain.LastErrorHTTPStatus}
	throttled := domain.Outcome{Kind: domain.OutcomeThrottled, Error: domain.LastErrorThrottled}

	next, dead := domain.NextAttempt(failed, 1, 0, false, created, now, cfg, fixedRand(true))
	if dead || !next.Equal(now.Add(10*time.Second)) {
		t.Fatalf("first failure: %v %v", next, dead)
	}
	// Retry-After larger than the backoff wins for a throttle.
	next, dead = domain.NextAttempt(throttled, 1, 5*time.Minute, true, created, now, cfg, fixedRand(true))
	if dead || !next.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("retry-after: %v %v", next, dead)
	}
	// A smaller Retry-After keeps the backoff.
	next, _ = domain.NextAttempt(throttled, 3, time.Second, true, created, now, cfg, fixedRand(true))
	if !next.Equal(now.Add(40 * time.Second)) {
		t.Fatalf("small retry-after: %v", next)
	}
	// Retry-After is ignored for a counted failure.
	next, _ = domain.NextAttempt(failed, 1, 5*time.Minute, true, created, now, cfg, fixedRand(true))
	if !next.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("retry-after on a failure: %v", next)
	}
	// A retry beyond the deadline is cut to the deadline.
	late := created.Add(cfg.MaxAge - time.Minute)
	next, dead = domain.NextAttempt(throttled, 1, time.Hour, true, created, late, cfg, fixedRand(true))
	if dead || !next.Equal(created.Add(cfg.MaxAge)) {
		t.Fatalf("cut at the deadline: %v %v", next, dead)
	}
	// At and after the deadline the delivery is dead.
	for _, at := range []time.Time{created.Add(cfg.MaxAge), created.Add(cfg.MaxAge + time.Hour)} {
		if _, dead := domain.NextAttempt(failed, 9, 0, false, created, at, cfg, fixedRand(true)); !dead {
			t.Fatalf("not dead at %v", at)
		}
		if _, dead := domain.NextAttempt(throttled, 9, 0, false, created, at, cfg, fixedRand(true)); !dead {
			t.Fatalf("throttle not dead at %v", at)
		}
	}
}
