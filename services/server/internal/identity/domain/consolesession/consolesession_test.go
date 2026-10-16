// SPDX-License-Identifier: Apache-2.0

package consolesession_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/consolesession"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// pepper is the pepper port of the tests.
var pepper = consolesession.StaticPepper("0123456789abcdef0123456789abcdef")

// TestCookieValueIs256BitsAndHashed: 32 random bytes, base64url without padding; two values
// differ; the stored hash is the HMAC with the pepper of the port, never the value (ID-T27).
func TestCookieValueIs256BitsAndHashed(t *testing.T) {
	t.Parallel()
	a, err := consolesession.NewCookieValue(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b, err := consolesession.NewCookieValue(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two cookie values are equal")
	}
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil || len(raw) != 32 || strings.ContainsAny(a, "=+/") {
		t.Fatalf("value %q: %d bytes, %v", a, len(raw), err)
	}
	h := consolesession.CookieHash(pepper, a)
	m := hmac.New(sha256.New, []byte(pepper))
	m.Write([]byte(a))
	if !bytes.Equal(h, m.Sum(nil)) {
		t.Fatal("the hash is not HMAC-SHA-256 with the pepper")
	}
	if bytes.Contains(h, []byte(a)) || bytes.Equal(h, consolesession.CookieHash(pepper, b)) {
		t.Fatal("the hash carries the value or does not tell values apart")
	}
	if _, err := consolesession.NewCookieValue(bytes.NewReader(make([]byte, 3))); err == nil {
		t.Fatal("a short random source must fail")
	}
}

// TestCookieAttributes: the cookie carries exactly the attributes of D-M6-01-2; the clearing
// form has Max-Age=0.
func TestCookieAttributes(t *testing.T) {
	t.Parallel()
	got := consolesession.SetCookie("v1", 90*time.Second+500*time.Millisecond)
	if got != "__Host-console_session=v1; Path=/; Max-Age=90; Secure; HttpOnly; SameSite=Strict" {
		t.Fatalf("SetCookie = %q", got)
	}
	if got := consolesession.SetCookie("v1", -time.Second); !strings.Contains(got, "Max-Age=0;") {
		t.Fatalf("a past end = %q", got)
	}
	if got := consolesession.ClearCookie(); got != "__Host-console_session=; Path=/; Max-Age=0; Secure; HttpOnly; SameSite=Strict" {
		t.Fatalf("ClearCookie = %q", got)
	}
	if strings.Contains(strings.ToLower(consolesession.SetCookie("v", time.Hour)), "domain") {
		t.Fatal("the cookie must not name a domain")
	}
}

// TestExpiry: absolute expiry; idle exactly at idle timeout plus one hundredth after last_seen_at
// and not one tick before; the write bound of last_seen_at is one hundredth of the timeout.
func TestExpiry(t *testing.T) {
	t.Parallel()
	tm := consolesession.Timing{MaxAge: 12 * time.Hour, IdleTimeout: 30 * time.Minute}
	created := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	end := tm.ExpiresAt(created)
	if !end.Equal(created.Add(12 * time.Hour)) {
		t.Fatalf("ExpiresAt = %v", end)
	}
	if tm.Expired(end, end.Add(-time.Nanosecond)) || !tm.Expired(end, end) {
		t.Fatal("the absolute end is the first instant of expiry")
	}
	bound := tm.WriteBound()
	if bound != 18*time.Second {
		t.Fatalf("WriteBound = %v", bound)
	}
	idleAt := created.Add(tm.IdleTimeout + bound)
	if tm.Idle(created, idleAt.Add(-time.Nanosecond)) || !tm.Idle(created, idleAt) {
		t.Fatal("idle exactly at timeout plus the write bound")
	}
	if tm.NeedsTouch(created, created.Add(bound)) || !tm.NeedsTouch(created, created.Add(bound+time.Nanosecond)) {
		t.Fatal("last_seen_at is written only when older than the write bound")
	}
	if !tm.NeedsTouch(time.Time{}, created) {
		t.Fatal("a session without last_seen_at is touched")
	}
	if err := (consolesession.Timing{MaxAge: time.Hour}).Validate(); err == nil {
		t.Fatal("an idle timeout below 100ns cannot have a write bound")
	}
	if err := tm.Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestPreStepUpGate: self.read, auth.step_up and auth.logout pass, grant.create and audit.read do
// not; after step_up_at is set everything passes the gate.
func TestPreStepUpGate(t *testing.T) {
	t.Parallel()
	for _, act := range []string{"self.read", "auth.step_up", "auth.logout", "instance.discovery.read", "auth.login.begin", "auth.login.complete"} {
		if !consolesession.PassesGate(act, time.Time{}) {
			t.Errorf("%s must pass before the step-up", act)
		}
	}
	for _, act := range []string{"grant.create", "audit.read", "access.view_as", "user.read", "access.view_as.end", ""} {
		if consolesession.PassesGate(act, time.Time{}) {
			t.Errorf("%s must not pass before the step-up", act)
		}
	}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for _, act := range []string{"grant.create", "audit.read", "access.view_as"} {
		if !consolesession.PassesGate(act, at) {
			t.Errorf("%s must pass after the step-up", act)
		}
	}
}

// TestViewAsExpiry: a stored view is active before authz.view_as_max_duration and ended at it.
func TestViewAsExpiry(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	const max = 15 * time.Minute
	if !consolesession.ViewActive(start, start.Add(max-time.Nanosecond), max) {
		t.Fatal("active one tick before the end")
	}
	if consolesession.ViewActive(start, start.Add(max), max) {
		t.Fatal("ended at the end")
	}
	if consolesession.ViewActive(time.Time{}, start, max) {
		t.Fatal("a view without a start is ended (fail closed)")
	}
	if got := consolesession.ViewEndsAt(start, max); !got.Equal(start.Add(max)) {
		t.Fatalf("ViewEndsAt = %v", got)
	}
}

// TestWebDevice: the device of a console login is an active web device without keys, approved by
// the person, with the fixed English name.
func TestWebDevice(t *testing.T) {
	t.Parallel()
	user, id := ids.New(), ids.New()
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	d := consolesession.WebDevice(id, user, now)
	if d.ID != id || d.UserID != user || d.Platform != device.PlatformWeb || d.State != device.StateActive || d.ApprovedBy != user ||
		len(d.Keys) != 0 || d.Name != "Web console" || !d.CreatedAt.Equal(now) {
		t.Fatalf("WebDevice = %+v", d)
	}
}
