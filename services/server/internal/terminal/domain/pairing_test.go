// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// testKeys returns a fresh X25519 key and Ed25519 JWK as the host service sends them.
func testKeys(t testing.TB) (string, map[string]any, []byte) {
	t.Helper()
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(x.PublicKey().Bytes()), map[string]any{"kty": "OKP", "crv": "Ed25519", "x": enc.EncodeToString(pub)}, pub
}

// hasField reports a field error with pointer and code.
func hasField(list []errs.FieldError, pointer, code string) bool {
	for _, f := range list {
		if f.Pointer == pointer && f.Code == code {
			return true
		}
	}
	return false
}

// TestParseHostKeys (Design note 4a and 4b).
func TestParseHostKeys(t *testing.T) {
	t.Parallel()
	noise, jwk, raw := testKeys(t)
	k, findings := domain.ParseHostKeys(noise, jwk)
	if len(findings) != 0 || len(k.Noise) != 32 || string(k.AuthRaw) != string(raw) {
		t.Fatalf("valid keys: %v", findings)
	}
	thumb := sha256.Sum256([]byte(`{"crv":"Ed25519","kty":"OKP","x":"` + jwk["x"].(string) + `"}`))
	if k.JKT != base64.RawURLEncoding.EncodeToString(thumb[:]) {
		t.Fatalf("auth_jkt %s is not the RFC 7638 thumbprint", k.JKT)
	}
	if string(k.JWK) != `{"crv":"Ed25519","kty":"OKP","x":"`+jwk["x"].(string)+`"}` {
		t.Fatalf("stored JWK %s", k.JWK)
	}
	enc := base64.RawURLEncoding
	padded := base64.URLEncoding.EncodeToString(make([]byte, 32))
	for name, key := range map[string]string{
		"padded": padded, "31 bytes": enc.EncodeToString(seq(1, 31)), "33 bytes": enc.EncodeToString(seq(1, 33)),
		"low order zero": enc.EncodeToString(make([]byte, 32)), "not base64": "%%%", "empty": "",
	} {
		if _, f := domain.ParseHostKeys(key, jwk); !hasField(f, "/noise_public_key", "invalid_key") {
			t.Errorf("noise key %s: %v", name, f)
		}
	}
	with := func(k, v string) map[string]any {
		m := map[string]any{}
		for a, b := range jwk {
			m[a] = b
		}
		if v == "" {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name          string
		jwk           map[string]any
		pointer, code string
	}{
		{"private d", with("d", "secret"), "/auth_public_jwk/d", "unknown_member"},
		{"kid", with("kid", "k1"), "/auth_public_jwk/kid", "unknown_member"},
		{"kty EC", with("kty", "EC"), "/auth_public_jwk/kty", "invalid_key"},
		{"crv X25519", with("crv", "X25519"), "/auth_public_jwk/crv", "invalid_key"},
		{"x of 31 bytes", with("x", enc.EncodeToString(seq(1, 31))), "/auth_public_jwk/x", "invalid_key"},
		{"x padded", with("x", base64.URLEncoding.EncodeToString(seq(1, 32))), "/auth_public_jwk/x", "invalid_key"},
		{"x missing", with("x", ""), "/auth_public_jwk/x", "invalid_key"},
		{"x not a string", map[string]any{"kty": "OKP", "crv": "Ed25519", "x": 7.0}, "/auth_public_jwk/x", "invalid_key"},
	}
	for _, c := range cases {
		if _, f := domain.ParseHostKeys(noise, c.jwk); !hasField(f, c.pointer, c.code) {
			t.Errorf("%s: %v", c.name, f)
		}
	}
	if _, f := domain.ParseHostKeys(noise, with("d", "secret")); strings.Contains(f[0].Detail, "secret") {
		t.Fatal("the detail repeats the value")
	}
}

// limits are small report limits.
var limits = domain.ReportLimits{OSUserMaxChars: 12, LabelMaxChars: 10, MaxEntries: 2}

// goodReport is a report within limits.
func goodReport() domain.PairReport {
	return domain.PairReport{
		Platform: "linux", OSVersion: "Debian 13", HostVersion: "1.0.0",
		Profiles:     []domain.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}},
		SessionUsers: []domain.ReportedSessionUser{{OSUser: "bothy-s-sam", Workdirs: []string{"Project A"}}},
	}
}

// TestValidatePairReport (Design note 4c to 4f).
func TestValidatePairReport(t *testing.T) {
	t.Parallel()
	if f := domain.ValidatePairReport(goodReport(), limits); len(f) != 0 {
		t.Fatalf("good report: %v", f)
	}
	empty := goodReport()
	empty.Profiles = nil
	if f := domain.ValidatePairReport(empty, limits); len(f) != 0 {
		t.Fatalf("an empty profile list is refused: %v", f)
	}
	cases := []struct {
		name          string
		edit          func(r *domain.PairReport)
		pointer, code string
	}{
		{"no session users", func(r *domain.PairReport) { r.SessionUsers = nil }, "/session_users", "empty"},
		{"case duplicate", func(r *domain.PairReport) {
			r.SessionUsers = append(r.SessionUsers, domain.ReportedSessionUser{OSUser: "BOTHY-S-SAM", Workdirs: []string{}})
		}, "/session_users/1/os_user", "duplicate"},
		{"name too long", func(r *domain.PairReport) { r.SessionUsers[0].OSUser = strings.Repeat("a", 13) }, "/session_users/0/os_user", "too_long"},
		{"name with slash", func(r *domain.PairReport) { r.SessionUsers[0].OSUser = "a/b" }, "/session_users/0/os_user", "pattern_mismatch"},
		{"workdir bidi", func(r *domain.PairReport) { r.SessionUsers[0].Workdirs = []string{"a\u202eb"} }, "/session_users/0/workdirs/0", "bidi_control"},
		{"workdir empty", func(r *domain.PairReport) { r.SessionUsers[0].Workdirs = []string{""} }, "/session_users/0/workdirs/0", "required"},
		{"workdir too long", func(r *domain.PairReport) { r.SessionUsers[0].Workdirs = []string{strings.Repeat("w", 11)} }, "/session_users/0/workdirs/0", "too_long"},
		{"too many workdirs", func(r *domain.PairReport) { r.SessionUsers[0].Workdirs = []string{"a", "b", "c"} }, "/session_users/0/workdirs", "limit_reached"},
		{"too many session users", func(r *domain.PairReport) {
			r.SessionUsers = []domain.ReportedSessionUser{{OSUser: "a", Workdirs: []string{}}, {OSUser: "b", Workdirs: []string{}}, {OSUser: "c", Workdirs: []string{}}}
		}, "/session_users", "limit_reached"},
		{"too many profiles", func(r *domain.PairReport) {
			r.Profiles = []domain.Profile{{Name: "a", DisplayName: "A", Kind: "shell"}, {Name: "b", DisplayName: "B", Kind: "shell"}, {Name: "c", DisplayName: "C", Kind: "shell"}}
		}, "/profiles", "limit_reached"},
		{"duplicate profile", func(r *domain.PairReport) {
			r.Profiles = append(r.Profiles, domain.Profile{Name: "shell", DisplayName: "Other", Kind: "shell"})
		}, "/profiles/1/name", "duplicate"},
		{"profile name too long", func(r *domain.PairReport) { r.Profiles[0].Name = strings.Repeat("p", 11) }, "/profiles/0/name", "too_long"},
		{"profile name pattern", func(r *domain.PairReport) { r.Profiles[0].Name = "Shell" }, "/profiles/0/name", "pattern_mismatch"},
		{"profile display bidi", func(r *domain.PairReport) { r.Profiles[0].DisplayName = "a\u202eb" }, "/profiles/0/display_name", "bidi_control"},
		{"profile display too long", func(r *domain.PairReport) { r.Profiles[0].DisplayName = strings.Repeat("d", 11) }, "/profiles/0/display_name", "too_long"},
		{"host version with v", func(r *domain.PairReport) { r.HostVersion = "v1.0.0" }, "/host_version", "invalid_format"},
		{"host version short", func(r *domain.PairReport) { r.HostVersion = "1.0" }, "/host_version", "invalid_format"},
		{"os version too long", func(r *domain.PairReport) { r.OSVersion = strings.Repeat("o", 11) }, "/os_version", "too_long"},
	}
	for _, c := range cases {
		r := goodReport()
		c.edit(&r)
		if f := domain.ValidatePairReport(r, limits); !hasField(f, c.pointer, c.code) {
			t.Errorf("%s: %v, want %s at %s", c.name, f, c.code, c.pointer)
		}
	}
}

// TestNormalizeFingerprintPrefix (Design note 6).
func TestNormalizeFingerprintPrefix(t *testing.T) {
	t.Parallel()
	if got, err := domain.NormalizeFingerprintPrefix("f0cc6qs6", 8); err != nil || got != "F0CC6QS6" {
		t.Fatalf("lower case: %q, %v", got, err)
	}
	for _, in := range []string{"F0CC6QS", "F0CC6QS6R", "F0CC-6QS", "F0CC6QSI", "F0CC6QSL", "F0CC6QSO", "F0CC6QSU", ""} {
		_, err := domain.NormalizeFingerprintPrefix(in, 8)
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "/fingerprint_prefix" || e.Fields[0].Code != "invalid_format" {
			t.Errorf("%q: %v", in, err)
		}
	}
}

// TestPrefixMatches: the prefix against the compact fingerprint.
func TestPrefixMatches(t *testing.T) {
	t.Parallel()
	fp, _ := domain.ComputeFingerprint(seq(0x00, 32), seq(0x20, 32))
	if !domain.PrefixMatches(fp, "F0CC6QS6") || !domain.PrefixMatches(fp, "F0CC6QS6RF5D") {
		t.Fatal("the right prefix does not match")
	}
	for _, p := range []string{"F0CC6QS7", "0CC6QS6R", "", strings.Repeat("F", 53)} {
		if domain.PrefixMatches(fp, p) {
			t.Errorf("%q matches", p)
		}
	}
}

// TestNextAttempt (TM-T19, Design note 7e).
func TestNextAttempt(t *testing.T) {
	t.Parallel()
	const maxAttempts = 3
	for before := 0; before < maxAttempts; before++ {
		o := domain.NextAttempt(before, maxAttempts, false)
		wantRemaining := maxAttempts - before - 1
		if o.Remaining != wantRemaining || o.Fallback != (wantRemaining == 0) {
			t.Errorf("before %d: %+v", before, o)
		}
		if o.Fallback && o.Failures != 0 || !o.Fallback && o.Failures != before+1 {
			t.Errorf("before %d: failures %d", before, o.Failures)
		}
		if m := domain.NextAttempt(before, maxAttempts, true); m.Failures != 0 || m.Fallback || !m.Match {
			t.Errorf("match after %d: %+v", before, m)
		}
	}
	if o := domain.NextAttempt(maxAttempts, maxAttempts, false); !o.Fallback || o.Remaining != 0 {
		t.Errorf("at the maximum: %+v", o)
	}
}

// TestRepairEffects (Design note 11, 24 sections 3.1, 3.3 and 3.4).
func TestRepairEffects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		state                domain.State
		reason               string
		drop, end, clear, ok bool
	}{
		{domain.StatePairing, "", false, false, false, true},
		{domain.StateConfirming, "", true, false, false, true},
		{domain.StateActive, "", true, true, false, true},
		{domain.StateDisabled, domain.DisabledOwner, true, false, true, true},
		{domain.StateDisabled, domain.DisabledDuplicateConnection, true, false, true, true},
		{domain.StateRetired, "", false, false, false, false},
	}
	for _, c := range cases {
		p, err := domain.RepairEffects(c.state, c.reason)
		if !c.ok {
			if !domain.IsInvalidTransition(err) {
				t.Errorf("%s: %v, want invalid_state_transition", c.state, err)
			}
			continue
		}
		if err != nil || p.DropKeys != c.drop || p.EndSessions != c.end || p.ClearDisabled != c.clear || p.Result.To != domain.StatePairing {
			t.Errorf("%s/%s: %+v, %v", c.state, c.reason, p, err)
		}
	}
}

// TestHost_HasKeys: the representation shows the fingerprint only with stored keys (Design note 9).
func TestHost_HasKeys(t *testing.T) {
	t.Parallel()
	if (domain.Host{KeyFingerprint: "F0CC"}).HasKeys() || !(domain.Host{NoisePublicKey: make([]byte, 32)}).HasKeys() {
		t.Fatal("HasKeys")
	}
	if _, err := domain.CheckReason("  Host reinstalled  ", 40); err != nil {
		t.Fatal(err)
	}
	if !(domain.Declaration{Credentials: domain.LLMNone}).Same(domain.Declaration{Credentials: domain.LLMNone}) {
		t.Fatal("Same")
	}
}
