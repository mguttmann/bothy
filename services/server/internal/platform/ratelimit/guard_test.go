// SPDX-License-Identifier: Apache-2.0

package ratelimit_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// TestFailedCredentialGuard (M2-27 design note 2c): the guard reserves one token of rl.public_auth
// of the client IP; settle(true) keeps it, settle(false) gives it back; while the bucket is empty
// it answers 429 rate_limited with Retry-After and the RateLimit headers and no settle function.
// Other client IPs keep their own bucket.
func TestFailedCredentialGuard(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 2}}))
	r := keyRequest("192.0.2.20", nil, "")
	guard := func() (func(bool), error) {
		t.Helper()
		settle, err := l.FailedCredentialGuard(r)
		if err == nil && settle == nil {
			t.Fatal("no error and no settle function")
		}
		return settle, err
	}

	// A valid credential costs nothing, however often.
	for range 5 {
		settle, err := guard()
		if err != nil {
			t.Fatalf("valid credential: %v", err)
		}
		settle(false)
	}
	// Two failures empty the bucket of the burst.
	for i := range 2 {
		settle, err := guard()
		if err != nil {
			t.Fatalf("failure %d: %v", i+1, err)
		}
		settle(true)
	}
	settle, err := l.FailedCredentialGuard(r)
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != errs.CodeRateLimited || settle != nil {
		t.Fatalf("empty bucket: err %v, settle set %t; want 429 rate_limited without settle", err, settle != nil)
	}
	if e.RetryAfter <= 0 || e.Headers.Get(ratelimit.HeaderPolicy) == "" || e.Headers.Get(ratelimit.HeaderState) == "" {
		t.Fatalf("429 without Retry-After or RateLimit headers: %+v", e)
	}
	if _, err := l.FailedCredentialGuard(keyRequest("192.0.2.21", nil, "")); err != nil {
		t.Fatalf("another client IP: %v", err)
	}
}
