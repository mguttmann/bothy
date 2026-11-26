// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

const (
	tHost    = "0192f3a4-5b6c-7d8e-9f01-0000000000a1"
	tSession = "0192f3a4-5b6c-7d8e-9f01-23456789abcd"
	tRun     = "0192f3a4-5b6c-7d8e-9f01-0000000000b1"
	tAgent   = "0192f3a4-5b6c-7d8e-9f01-0000000000c1"
	tOwner   = "0192f3a4-5b6c-7d8e-9f01-0000000000d1"
	tCek     = "Y2VrLWhhc2gtb2YtdGhlLWVwaGVtZXJhbC1rZXk"
)

// claimsAt returns valid claims of an attach token issued at now (27 5.2).
func claimsAt(now time.Time) map[string]any {
	return map[string]any{
		"iss": "0192f3a4-5b6c-7d8e-9f01-0000000000e1", "aud": tHost, "sub": tOwner, "sid": tSession,
		"jti": sessiontest.NewID(), "iat": now.Unix(), "exp": now.Add(30 * time.Second).Unix(),
		"run": tRun, "agt": tAgent, "cek": tCek, "own": true, "gen": 3,
	}
}

func signComputer(s *sessiontest.Signer, claims map[string]any) string {
	return s.SignRaw(map[string]any{"alg": "EdDSA", "kid": s.Kid, "typ": TokenType}, claims)
}

func checkOf(s *sessiontest.Signer, tok string, now time.Time) Check {
	return Check{Token: tok, Keys: []map[string]any{s.JWK()}, HostID: tHost, SessionID: tSession, Now: now,
		Leeway: 30 * time.Second, Cek: tCek, Open: OpenBinding{SessionID: tSession, RunID: tRun, AgentID: tAgent, UserID: tOwner}}
}

// TestVerifySteps runs the token steps 1 to 8 of docs/spec/27-computer-use.md 5.3 one by one
// (CU-T07) and the terminal token on a computer connection (CU-T08).
func TestVerifySteps(t *testing.T) {
	s := sessiontest.NewSigner("k1")
	other := sessiontest.NewSigner("k2")
	now := time.Unix(1_800_000_000, 0)
	set := func(k string, v any) func(map[string]any) { return func(c map[string]any) { c[k] = v } }
	cases := []struct {
		name   string
		edit   func(map[string]any)
		sign   func(map[string]any) string
		check  func(*Check)
		detail string
	}{
		{name: "valid"},
		{name: "unknown key", sign: func(c map[string]any) string { return signComputer(other, c) }, detail: DetailTokenInvalid},
		{name: "terminal typ", sign: func(c map[string]any) string { return s.Sign(c) }, detail: DetailTokenInvalid},
		{name: "terminal token", sign: func(map[string]any) string {
			return s.Sign(sessiontest.Claims(tHost, tSession, tOwner, "alex", "shell", tCek, "open", now, time.Minute))
		}, detail: DetailTokenInvalid},
		{name: "aud", edit: set("aud", "0192f3a4-5b6c-7d8e-9f01-0000000000a2"), detail: DetailTokenInvalid},
		{name: "sid", edit: set("sid", "0192f3a4-5b6c-7d8e-9f01-0000000000a3"), detail: DetailTokenInvalid},
		{name: "expired", edit: set("exp", now.Add(-31*time.Second).Unix()), detail: DetailTokenExpired},
		{name: "cek", edit: set("cek", "other"), detail: DetailTokenInvalid},
		{name: "other run", edit: set("run", "0192f3a4-5b6c-7d8e-9f01-0000000000b2"), detail: DetailTokenInvalid},
		{name: "other agent", edit: set("agt", "0192f3a4-5b6c-7d8e-9f01-0000000000c2"), detail: DetailTokenInvalid},
		{name: "other person", edit: set("sub", "0192f3a4-5b6c-7d8e-9f01-0000000000d2"), detail: DetailTokenInvalid},
		{name: "live already", check: func(c *Check) { c.Live = true }, detail: DetailTokenInvalid},
		{name: "own false", edit: set("own", false), detail: DetailTokenInvalid},
		{name: "own missing", edit: func(c map[string]any) { delete(c, "own") }, detail: DetailTokenInvalid},
		{name: "gen missing", edit: func(c map[string]any) { delete(c, "gen") }, detail: DetailTokenInvalid},
		{name: "gen float", edit: set("gen", 1.5), detail: DetailTokenInvalid},
		{name: "run null", edit: set("run", nil), detail: DetailTokenInvalid},
	}
	for _, c := range cases {
		claims := claimsAt(now)
		if c.edit != nil {
			c.edit(claims)
		}
		sign := func(m map[string]any) string { return signComputer(s, m) }
		if c.sign != nil {
			sign = c.sign
		}
		ch := checkOf(s, sign(claims), now)
		if c.check != nil {
			c.check(&ch)
		}
		v := NewVerifier()
		cl, detail := v.Verify(ch)
		if detail != c.detail {
			t.Errorf("%s: detail %q, want %q", c.name, detail, c.detail)
		}
		if c.detail == "" && (cl.Gen != 3 || cl.Run != tRun || !cl.Own) {
			t.Errorf("%s: claims %+v", c.name, cl)
		}
	}
}

// TestVerifyReplay: a jti is remembered until exp plus the leeway; a second use is token_replayed,
// also after a failure of a later step.
func TestVerifyReplay(t *testing.T) {
	s := sessiontest.NewSigner("k1")
	now := time.Unix(1_800_000_000, 0)
	claims := claimsAt(now)
	claims["run"] = "0192f3a4-5b6c-7d8e-9f01-0000000000b2" // fails at step 7
	tok := signComputer(s, claims)
	v := NewVerifier()
	if _, d := v.Verify(checkOf(s, tok, now)); d != DetailTokenInvalid {
		t.Fatalf("first use: %q", d)
	}
	if _, d := v.Verify(checkOf(s, tok, now)); d != DetailTokenReplayed {
		t.Errorf("replay: %q", d)
	}
}

// TestCloseCodes: 4101 for the details without tampering, 4104 otherwise (27 5.3).
func TestCloseCodes(t *testing.T) {
	for _, d := range []string{DetailTokenExpired, DetailTokenReplayed, DetailHelperAbsent, DetailNotEnabled, DetailBusy} {
		if CloseCodeOf(d) != 4101 {
			t.Errorf("%s: %d", d, CloseCodeOf(d))
		}
	}
	for _, d := range []string{DetailTokenInvalid, DetailHandshakeFailed, DetailHelperUntrusted} {
		if CloseCodeOf(d) != 4104 {
			t.Errorf("%s: %d", d, CloseCodeOf(d))
		}
	}
}
