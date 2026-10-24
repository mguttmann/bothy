// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

var t0 = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// TestLastChange: the latest of created_at, completed_at, scanned_at, and expires_at only for an
// expired session (brief M3-08 design note 7).
func TestLastChange(t *testing.T) {
	for name, tc := range map[string]struct {
		f    domain.CleanupFacts
		want time.Time
	}{
		"created only": {domain.CleanupFacts{CreatedAt: t0}, t0},
		"completed":    {domain.CleanupFacts{CreatedAt: t0, CompletedAt: t0.Add(time.Hour)}, t0.Add(time.Hour)},
		"scanned":      {domain.CleanupFacts{CreatedAt: t0, CompletedAt: t0.Add(time.Hour), ScannedAt: t0.Add(2 * time.Hour)}, t0.Add(2 * time.Hour)},
		"expired":      {domain.CleanupFacts{CreatedAt: t0, SessionState: domain.SessionExpired, ExpiresAt: t0.Add(3 * time.Hour)}, t0.Add(3 * time.Hour)},
		"aborted":      {domain.CleanupFacts{CreatedAt: t0, SessionState: domain.SessionAborted, ExpiresAt: t0.Add(3 * time.Hour)}, t0},
		"open":         {domain.CleanupFacts{CreatedAt: t0, SessionState: domain.SessionOpen, ExpiresAt: t0.Add(3 * time.Hour)}, t0},
	} {
		if got := domain.LastChange(tc.f); !got.Equal(tc.want) {
			t.Fatalf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

// TestUnattachedExpired and TestFailedRowExpired: exactly ttl after the last change is expired,
// one nanosecond earlier is not.
func TestUnattachedExpired(t *testing.T) {
	expiryEdge(t, domain.UnattachedExpired)
}

func TestFailedRowExpired(t *testing.T) {
	expiryEdge(t, domain.FailedRowExpired)
}

func expiryEdge(t *testing.T, fn func(domain.CleanupFacts, time.Time, time.Duration) bool) {
	t.Helper()
	ttl := 24 * time.Hour
	for _, f := range []domain.CleanupFacts{
		{CreatedAt: t0},
		{CreatedAt: t0, CompletedAt: t0.Add(time.Hour), ScannedAt: t0.Add(90 * time.Minute)},
		{CreatedAt: t0, SessionState: domain.SessionExpired, ExpiresAt: t0.Add(4 * time.Hour)},
		{CreatedAt: t0, SessionState: domain.SessionAborted, ExpiresAt: t0.Add(4 * time.Hour), ScannedAt: t0.Add(time.Minute)},
	} {
		last := domain.LastChange(f)
		if !fn(f, last.Add(ttl), ttl) {
			t.Fatalf("%+v: not expired exactly ttl after %v", f, last)
		}
		if fn(f, last.Add(ttl-time.Nanosecond), ttl) {
			t.Fatalf("%+v: expired one nanosecond early", f)
		}
	}
}

// TestSessionPastExpiry: open at and after expires_at, never before and never in another state.
func TestSessionPastExpiry(t *testing.T) {
	exp := t0.Add(time.Hour)
	if !domain.SessionPastExpiry(domain.SessionOpen, exp, exp) || !domain.SessionPastExpiry(domain.SessionOpen, exp, exp.Add(time.Second)) {
		t.Fatal("an open session at or after expires_at is expired")
	}
	if domain.SessionPastExpiry(domain.SessionOpen, exp, exp.Add(-time.Nanosecond)) {
		t.Fatal("an open session before expires_at is not expired")
	}
	for _, st := range []domain.SessionState{domain.SessionCompleted, domain.SessionAborted, domain.SessionExpired} {
		if domain.SessionPastExpiry(st, exp, exp.Add(time.Hour)) {
			t.Fatalf("%s expired", st)
		}
	}
}
