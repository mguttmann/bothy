// SPDX-License-Identifier: Apache-2.0

package dpop_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
)

func TestNormalizeHTU(t *testing.T) {
	t.Parallel()
	ok := map[string]string{
		"https://chat.example.com/api/v1/me":          "https://chat.example.com/api/v1/me",
		"HTTPS://Chat.Example.COM/api/v1/me":          "https://chat.example.com/api/v1/me",
		"https://chat.example.com:443/api/v1/me":      "https://chat.example.com/api/v1/me",
		"http://chat.example.com:80/x":                "http://chat.example.com/x",
		"http://chat.example.com:8443/x":              "http://chat.example.com:8443/x",
		"https://chat.example.com":                    "https://chat.example.com/",
		"https://chat.example.com/a?b=c#d":            "https://chat.example.com/a",
		"https://chat.example.com/%7euser/%2f%c3%a4":  "https://chat.example.com/~user/%2F%C3%A4",
		"https://chat.example.com/A%2Db":              "https://chat.example.com/A-b",
		"https://[2001:db8::1]:443/x":                 "https://[2001:db8::1]/x",
		"https://[2001:db8::1]:8443/x":                "https://[2001:db8::1]:8443/x",
		"https://192.0.2.1/api/v1/devices/0192f4a0-x": "https://192.0.2.1/api/v1/devices/0192f4a0-x",
	}
	for in, want := range ok {
		got, err := dpop.NormalizeHTU(in)
		if err != nil || got != want {
			t.Errorf("NormalizeHTU(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "/api/v1/me", "ftp://example.com/x", "https://user@example.com/x",
		"https://example.com/%zz", "https://example.com/%4", "mailto:a@example.com", "https:///x", "https://example.com:abc/x"} {
		if got, err := dpop.NormalizeHTU(bad); err == nil {
			t.Errorf("NormalizeHTU(%q) = %q, want an error", bad, got)
		}
	}
	// The path stays case sensitive and is compared exactly otherwise.
	a, _ := dpop.NormalizeHTU("https://example.com/A")
	b, _ := dpop.NormalizeHTU("https://example.com/a")
	if a == b {
		t.Error("path case was folded")
	}
}

func code(t *testing.T, err error) string {
	t.Helper()
	var f *dpop.ProofError
	if !errors.As(err, &f) {
		t.Fatalf("error %v is no *dpop.ProofError", err)
	}
	if f.Reason == "" {
		t.Errorf("failure %q without an internal reason", f.Code)
	}
	return f.Code
}

func TestParseClaims(t *testing.T) {
	t.Parallel()
	good := `{"jti":"j1","htm":"GET","htu":"https://example.com/x","iat":1760000000,"nonce":"n"}`
	c, err := dpop.ParseClaims([]byte(good), false)
	if err != nil {
		t.Fatal(err)
	}
	if c.JTI != "j1" || c.HTM != "GET" || c.HTU != "https://example.com/x" || c.Nonce != "n" || !c.IAT.Equal(time.Unix(1760000000, 0)) || c.HasATH {
		t.Fatalf("claims %+v", c)
	}
	if _, err := dpop.ParseClaims([]byte(`{"jti":"j1","htm":"GET","htu":"https://example.com/x","iat":1760000000,"nonce":"n","ath":"a"}`), true); err != nil {
		t.Fatalf("with ath and token: %v", err)
	}
	cases := []struct {
		name, payload string
		token         bool
		want          string
	}{
		{"missing jti", `{"htm":"GET","htu":"u","iat":1,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"empty jti", `{"jti":"","htm":"GET","htu":"u","iat":1,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"jti number", `{"jti":1,"htm":"GET","htu":"u","iat":1,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"htm missing", `{"jti":"j","htu":"u","iat":1,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"htu number", `{"jti":"j","htm":"GET","htu":2,"iat":1,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"iat string", `{"jti":"j","htm":"GET","htu":"u","iat":"1","nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"iat fraction", `{"jti":"j","htm":"GET","htu":"u","iat":1.5,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"iat exponent", `{"jti":"j","htm":"GET","htu":"u","iat":1e9,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"iat missing", `{"jti":"j","htm":"GET","htu":"u","nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"nonce missing", `{"jti":"j","htm":"GET","htu":"u","iat":1}`, false, dpop.CodeUseNonce},
		{"nonce number", `{"jti":"j","htm":"GET","htu":"u","iat":1,"nonce":5}`, false, dpop.CodeInvalidProof},
		{"nonce null", `{"jti":"j","htm":"GET","htu":"u","iat":1,"nonce":null}`, false, dpop.CodeInvalidProof},
		{"htm null", `{"jti":"j","htm":null,"htu":"u","iat":1,"nonce":"n"}`, false, dpop.CodeInvalidProof},
		{"ath null", `{"jti":"j","htm":"GET","htu":"u","iat":1,"nonce":"n","ath":null}`, true, dpop.CodeInvalidProof},
		{"ath without token", `{"jti":"j","htm":"GET","htu":"u","iat":1,"nonce":"n","ath":"a"}`, false, dpop.CodeInvalidProof},
		{"no ath with token", `{"jti":"j","htm":"GET","htu":"u","iat":1,"nonce":"n"}`, true, dpop.CodeInvalidProof},
		{"ath number", `{"jti":"j","htm":"GET","htu":"u","iat":1,"nonce":"n","ath":1}`, true, dpop.CodeInvalidProof},
		{"not JSON", `nope`, false, dpop.CodeInvalidProof},
	}
	for _, tc := range cases {
		_, err := dpop.ParseClaims([]byte(tc.payload), tc.token)
		if err == nil {
			t.Errorf("%s: accepted", tc.name)
			continue
		}
		if got := code(t, err); got != tc.want {
			t.Errorf("%s: code %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestCheckRequest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	exp := dpop.Expect{Method: "GET", URL: "https://chat.example.com/api/v1/me", Now: now, MaxAge: 60 * time.Second, Leeway: 30 * time.Second}
	base := dpop.Claims{JTI: "j", HTM: "GET", HTU: "https://chat.example.com/api/v1/me", IAT: now, Nonce: "n"}
	htu, err := dpop.CheckRequest(base, exp)
	if err != nil || htu != "https://chat.example.com/api/v1/me" {
		t.Fatalf("CheckRequest = %q, %v", htu, err)
	}
	mod := func(f func(*dpop.Claims)) dpop.Claims { c := base; f(&c); return c }
	accepted := map[string]dpop.Claims{
		"query and fragment": mod(func(c *dpop.Claims) { c.HTU = "https://CHAT.example.com:443/api/v1/me?x=1#y" }),
		"iat at max age":     mod(func(c *dpop.Claims) { c.IAT = now.Add(-60 * time.Second) }),
		"iat at leeway":      mod(func(c *dpop.Claims) { c.IAT = now.Add(30 * time.Second) }),
	}
	for name, c := range accepted {
		if _, err := dpop.CheckRequest(c, exp); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	rejected := map[string]dpop.Claims{
		"htm":             mod(func(c *dpop.Claims) { c.HTM = "POST" }),
		"htm case":        mod(func(c *dpop.Claims) { c.HTM = "get" }),
		"htu host":        mod(func(c *dpop.Claims) { c.HTU = "https://other.example.com/api/v1/me" }),
		"htu path":        mod(func(c *dpop.Claims) { c.HTU = "https://chat.example.com/api/v1/you" }),
		"htu scheme":      mod(func(c *dpop.Claims) { c.HTU = "http://chat.example.com/api/v1/me" }),
		"htu malformed":   mod(func(c *dpop.Claims) { c.HTU = "chat.example.com/api/v1/me" }),
		"iat too old":     mod(func(c *dpop.Claims) { c.IAT = now.Add(-61 * time.Second) }),
		"iat beyond skew": mod(func(c *dpop.Claims) { c.IAT = now.Add(31 * time.Second) }),
	}
	for name, c := range rejected {
		_, err := dpop.CheckRequest(c, exp)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if got := code(t, err); got != dpop.CodeInvalidProof {
			t.Errorf("%s: code %s", name, got)
		}
	}
}

func TestCheckBinding(t *testing.T) {
	t.Parallel()
	tok := "access.token.value"
	sum := sha256.Sum256([]byte(tok))
	ath := base64.RawURLEncoding.EncodeToString(sum[:])
	if dpop.ATH(tok) != ath {
		t.Fatalf("ATH = %s", dpop.ATH(tok))
	}
	c := dpop.Claims{ATH: ath, HasATH: true}
	if err := dpop.CheckBinding(c, tok, "jkt1", "jkt1"); err != nil {
		t.Fatal(err)
	}
	if err := dpop.CheckBinding(dpop.Claims{}, "", "jkt1", ""); err != nil {
		t.Fatalf("no token, no expected key: %v", err)
	}
	for name, err := range map[string]error{
		"wrong ath":   dpop.CheckBinding(dpop.Claims{ATH: "x", HasATH: true}, tok, "jkt1", ""),
		"other key":   dpop.CheckBinding(c, tok, "jkt1", "jkt2"),
		"ath no tok":  dpop.CheckBinding(c, "", "jkt1", ""),
		"tok, no ath": dpop.CheckBinding(dpop.Claims{}, tok, "jkt1", ""),
	} {
		if err == nil || code(t, err) != dpop.CodeInvalidProof {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestNonce(t *testing.T) {
	t.Parallel()
	key, err := dpop.NonceKey([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := dpop.NonceKey([]byte("another pepper of thirty-two byt"))
	ttl := 5 * time.Minute
	now := time.Date(2026, 10, 12, 9, 2, 0, 0, time.UTC)
	w := dpop.NonceWindow(now, ttl)
	if w != uint64(now.Unix())/300 { //nolint:gosec // G115: a time after 1970
		t.Fatalf("window %d", w)
	}
	cur := dpop.EncodeNonce(key, w)
	if raw, err := base64.RawURLEncoding.DecodeString(cur); err != nil || len(raw) != 24 {
		t.Fatalf("nonce %q decodes to %d bytes, %v", cur, len(raw), err)
	}
	if stale, err := dpop.CheckNonce(key, cur, now, ttl); err != nil || stale {
		t.Fatalf("current: stale %v, %v", stale, err)
	}
	if stale, err := dpop.CheckNonce(key, dpop.EncodeNonce(key, w-1), now, ttl); err != nil || !stale {
		t.Fatalf("previous: stale %v, %v", stale, err)
	}
	// The previous window stays accepted until the end of the current one.
	if _, err := dpop.CheckNonce(key, cur, now.Add(ttl), ttl); err != nil {
		t.Fatalf("one window later: %v", err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(cur)
	raw[20] ^= 1 // a MAC byte; the window stays the current one
	forged := base64.RawURLEncoding.EncodeToString(raw)
	for name, n := range map[string]string{
		"two windows ago": dpop.EncodeNonce(key, w-2),
		"next window":     dpop.EncodeNonce(key, w+1),
		"forged MAC":      forged,
		"other pepper":    dpop.EncodeNonce(other, w),
		"empty":           "",
		"padded":          cur + "=",
		"short":           cur[:20],
	} {
		_, err := dpop.CheckNonce(key, n, now, ttl)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if got := code(t, err); got != dpop.CodeUseNonce {
			t.Errorf("%s: code %s", name, got)
		}
	}
	if _, err := dpop.NonceKey(nil); err == nil {
		t.Error("empty pepper accepted")
	}
}

func TestReplayKey(t *testing.T) {
	t.Parallel()
	a := dpop.ReplayKey("https://example.com/a", "j")
	want := sha256.Sum256([]byte("https://example.com/a\x00j"))
	if a != want {
		t.Fatal("ReplayKey is not SHA-256(htu || 0x00 || jti)")
	}
	if a == dpop.ReplayKey("https://example.com/b", "j") || a == dpop.ReplayKey("https://example.com/a", "k") {
		t.Fatal("different htu or jti gave the same key")
	}
}
