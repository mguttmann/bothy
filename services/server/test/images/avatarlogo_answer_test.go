// SPDX-License-Identifier: Apache-2.0

package images_test

import (
	"maps"
	"net/http"
	"slices"
	"testing"
)

// TestComparedHeadersDropPerRequestValues: the headers sameAnswer compares leave out every value
// that differs from one request to the next without saying anything about the resource (the
// correlation id, the date, the rate limit counters and the DPoP nonce, which the server rotates on
// its own schedule), and keep all others.
func TestComparedHeadersDropPerRequestValues(t *testing.T) {
	a, b := http.Header{}, http.Header{}
	for _, h := range []http.Header{a, b} {
		h.Set("Content-Type", "application/problem+json")
		h.Set("Cache-Control", "no-store")
	}
	a.Set("Correlation-Id", "c-1")
	b.Set("Correlation-Id", "c-2")
	a.Set("Date", "Thu, 22 Oct 2026 20:00:00 GMT")
	b.Set("Date", "Thu, 22 Oct 2026 20:00:01 GMT")
	a.Set("Ratelimit", "limit=10, remaining=9")
	b.Set("Ratelimit", "limit=10, remaining=8")
	a.Set("Ratelimit-Policy", "10;w=60")
	a.Set("DPoP-Nonce", "rotated-nonce")
	if ha, hb := comparedHeaders(a), comparedHeaders(b); !maps.EqualFunc(ha, hb, slices.Equal) {
		t.Fatalf("per-request values compared: %v and %v", ha, hb)
	}
	b.Set("Cache-Control", "private")
	if ha, hb := comparedHeaders(a), comparedHeaders(b); maps.EqualFunc(ha, hb, slices.Equal) {
		t.Fatal("a differing Cache-Control is not compared")
	}
}
