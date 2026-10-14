// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/base32"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
)

// rfc6238Secret is the SHA-1 seed of RFC 6238 Appendix B: the ASCII string "1234567890" twice.
var rfc6238Secret = []byte(strings.Repeat("1234567890", 2))

func TestIDT27_TOTPVectorsRFC6238(t *testing.T) {
	t.Parallel()
	// RFC 6238 Appendix B, mode SHA1, eight digits.
	vectors := []struct {
		unix int64
		code string
	}{
		{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"},
		{1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"},
	}
	for _, v := range vectors {
		step := domain.TOTPStep(time.Unix(v.unix, 0))
		if step != v.unix/30 {
			t.Fatalf("step of %d = %d, want %d (30-second step, T0 = 0)", v.unix, step, v.unix/30)
		}
		if got := domain.TOTPCode(rfc6238Secret, step, 8); got != v.code {
			t.Errorf("T=%d: code %s, want %s", v.unix, got, v.code)
		}
	}
	// The fixed parameters of 08 section 2.5 and the brief: 30 seconds, one step tolerance,
	// six digits, a secret of 160 bits.
	if domain.TOTPPeriod != 30*time.Second || domain.TOTPTolerance != 1 || domain.TOTPDigits != 6 || domain.TOTPSecretBytes != 20 {
		t.Fatalf("period %v tolerance %d digits %d secret %d", domain.TOTPPeriod, domain.TOTPTolerance, domain.TOTPDigits, domain.TOTPSecretBytes)
	}
	now := time.Unix(1111111111, 0)
	cur := domain.TOTPStep(now)
	for delta, want := range map[int64]bool{-2: false, -1: true, 0: true, 1: true, 2: false} {
		_, ok := domain.VerifyTOTP(rfc6238Secret, domain.TOTPCode(rfc6238Secret, cur+delta, 6), now, nil)
		if ok != want {
			t.Errorf("step %+d accepted %v, want %v", delta, ok, want)
		}
	}
}

func TestIDT20_TOTPCodeOnlyOnce(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0)
	cur := domain.TOTPStep(now)
	code := domain.TOTPCode(rfc6238Secret, cur, 6)
	step, ok := domain.VerifyTOTP(rfc6238Secret, code, now, nil)
	if !ok || step != cur {
		t.Fatalf("first use: step %d ok %v", step, ok)
	}
	// The same code a second time in the same step (RFC 6238 section 5.2).
	if _, ok := domain.VerifyTOTP(rfc6238Secret, code, now.Add(5*time.Second), &step); ok {
		t.Fatal("the same code was accepted twice")
	}
	// A newer step was used; the code of the previous step is never accepted again.
	later := now.Add(30 * time.Second)
	newer, ok := domain.VerifyTOTP(rfc6238Secret, domain.TOTPCode(rfc6238Secret, cur+1, 6), later, &step)
	if !ok || newer != cur+1 {
		t.Fatalf("newer step: %d %v", newer, ok)
	}
	if _, ok := domain.VerifyTOTP(rfc6238Secret, code, later, &newer); ok {
		t.Fatal("a code of an older step was accepted after a newer one")
	}
}

func TestTOTPWindowEdges(t *testing.T) {
	t.Parallel()
	now := time.Unix(1800000015, 0)
	cur := domain.TOTPStep(now)
	cases := []struct {
		delta int64
		want  bool
	}{{-2, false}, {-1, true}, {0, true}, {1, true}, {2, false}}
	for _, c := range cases {
		step, ok := domain.VerifyTOTP(rfc6238Secret, domain.TOTPCode(rfc6238Secret, cur+c.delta, 6), now, nil)
		if ok != c.want || (ok && step != cur+c.delta) {
			t.Errorf("delta %+d: step %d ok %v, want ok %v", c.delta, step, ok, c.want)
		}
	}
	// Malformed codes are rejected without panicking.
	for _, bad := range []string{"", "12345", "1234567", "12345a", " 123456", "１２３４５６"} {
		if _, ok := domain.VerifyTOTP(rfc6238Secret, bad, now, nil); ok {
			t.Errorf("malformed code %q accepted", bad)
		}
	}
}

func TestTOTPSecretAndURI(t *testing.T) {
	t.Parallel()
	s, err := domain.NewTOTPSecret(bytes.NewReader(bytes.Repeat([]byte{7}, 32)))
	if err != nil || len(s) != 20 {
		t.Fatalf("secret %d bytes, err %v", len(s), err)
	}
	if _, err := domain.NewTOTPSecret(bytes.NewReader(make([]byte, 5))); err == nil {
		t.Fatal("a short random source gave a secret")
	}
	uri := domain.TOTPURI("chat.example.com", "alex", rfc6238Secret)
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	want := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rfc6238Secret)
	if u.Scheme != "otpauth" || u.Host != "totp" || q.Get("secret") != want || q.Get("issuer") != "chat.example.com" ||
		q.Get("algorithm") != "SHA1" || q.Get("digits") != "6" || q.Get("period") != "30" {
		t.Fatalf("uri %s", uri)
	}
	if !strings.HasPrefix(uri, "otpauth://totp/chat.example.com:alex?") {
		t.Fatalf("label of %s", uri)
	}
	// Every part is escaped.
	if got := domain.TOTPURI("a b", "x/y?", rfc6238Secret); !strings.HasPrefix(got, "otpauth://totp/a%20b:x%2Fy%3F?") {
		t.Fatalf("escaping: %s", got)
	}
}

func TestLockoutWindow(t *testing.T) {
	t.Parallel()
	p := domain.LockoutPolicy{Limit: 3, Window: 15 * time.Minute, LockDuration: 10 * time.Minute}
	t0 := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		in         domain.Lockout
		now        time.Time
		want       domain.Lockout
		wantLocked bool
	}{
		{"first failure starts the window", domain.Lockout{}, t0,
			domain.Lockout{FailedCount: 1, WindowStartedAt: t0}, false},
		{"failure inside the window counts", domain.Lockout{FailedCount: 1, WindowStartedAt: t0}, t0.Add(14 * time.Minute),
			domain.Lockout{FailedCount: 2, WindowStartedAt: t0}, false},
		{"failure at the window end starts a new window", domain.Lockout{FailedCount: 2, WindowStartedAt: t0}, t0.Add(15 * time.Minute),
			domain.Lockout{FailedCount: 1, WindowStartedAt: t0.Add(15 * time.Minute)}, false},
		{"limit reached locks and resets the counter", domain.Lockout{FailedCount: 2, WindowStartedAt: t0}, t0.Add(time.Minute),
			domain.Lockout{LockedUntil: t0.Add(11 * time.Minute)}, true},
	}
	for _, c := range cases {
		got, locked := domain.RecordFailure(c.in, p, c.now)
		if got != c.want || locked != c.wantLocked {
			t.Errorf("%s: got %+v locked %v, want %+v locked %v", c.name, got, locked, c.want, c.wantLocked)
		}
	}
	// A limit of one locks at the first failure.
	if _, locked := domain.RecordFailure(domain.Lockout{}, domain.LockoutPolicy{Limit: 1, Window: time.Minute, LockDuration: time.Minute}, t0); !locked {
		t.Fatal("limit 1 did not lock")
	}
	// Lock expiry: locked strictly before locked_until.
	l := domain.Lockout{LockedUntil: t0.Add(10 * time.Minute)}
	if !l.Locked(t0.Add(10*time.Minute-time.Nanosecond)) || l.Locked(t0.Add(10*time.Minute)) || (domain.Lockout{}).Locked(t0) {
		t.Fatal("lock expiry")
	}
}

func TestPasswordRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pw     string
		listed bool
		want   string
	}{
		{"EXAMPLE-pw-1", false, ""},
		{"EXAMPLE-pw-", false, domain.PasswordTooShort},
		// Twelve code points, more than twelve bytes: counted in code points, not bytes.
		{"ÄÖÜäöüßÄÖÜäö", false, ""},
		{"ÄÖÜäöüßÄÖÜä", false, domain.PasswordTooShort},
		{"EXAMPLE-listed", true, domain.PasswordCommon},
		// Too short wins over listed.
		{"EXAMPLE", true, domain.PasswordTooShort},
	}
	for _, c := range cases {
		if got := domain.CheckPassword(c.pw, 12, c.listed); got != c.want {
			t.Errorf("%q listed %v: %q, want %q", c.pw, c.listed, got, c.want)
		}
	}
}
