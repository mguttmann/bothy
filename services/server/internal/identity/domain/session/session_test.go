// SPDX-License-Identifier: Apache-2.0

package session_test

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var now = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// TestSessionDecide holds the rotation decision of 03 refresh_tokens items 1 to 3: every outcome,
// the grace boundary exactly at auth.refresh_reuse_grace and generation current - 2 as reuse.
func TestSessionDecide(t *testing.T) {
	t.Parallel()
	const grace = 30 * time.Second
	valid := now.Add(time.Hour)
	cases := []struct {
		name string
		f    session.Family
		tok  session.Token
		want session.Outcome
	}{
		{"current unused", session.Family{CurrentGeneration: 3, RotatedAt: now.Add(-time.Minute)},
			session.Token{Generation: 3, ExpiresAt: valid}, session.OutcomeRotate},
		{"first generation", session.Family{}, session.Token{Generation: 0, ExpiresAt: valid}, session.OutcomeRotate},
		{"previous within grace", session.Family{CurrentGeneration: 3, RotatedAt: now.Add(-grace + time.Nanosecond)},
			session.Token{Generation: 2, UsedAt: now.Add(-grace), ExpiresAt: valid}, session.OutcomeGraceRotate},
		{"previous exactly at grace", session.Family{CurrentGeneration: 3, RotatedAt: now.Add(-grace)},
			session.Token{Generation: 2, UsedAt: now.Add(-grace), ExpiresAt: valid}, session.OutcomeReuse},
		{"previous after grace", session.Family{CurrentGeneration: 3, RotatedAt: now.Add(-time.Hour)},
			session.Token{Generation: 2, UsedAt: now.Add(-time.Hour), ExpiresAt: valid}, session.OutcomeReuse},
		{"current minus two within grace", session.Family{CurrentGeneration: 3, RotatedAt: now.Add(-time.Second)},
			session.Token{Generation: 1, UsedAt: now.Add(-time.Minute), ExpiresAt: valid}, session.OutcomeReuse},
		{"current already used", session.Family{CurrentGeneration: 3, RotatedAt: now.Add(-time.Second)},
			session.Token{Generation: 3, UsedAt: now.Add(-time.Second), ExpiresAt: valid}, session.OutcomeReuse},
		{"newer than current", session.Family{CurrentGeneration: 3},
			session.Token{Generation: 4, ExpiresAt: valid}, session.OutcomeReuse},
		{"never rotated, previous generation", session.Family{CurrentGeneration: 1},
			session.Token{Generation: 0, UsedAt: now, ExpiresAt: valid}, session.OutcomeReuse},
		{"expired", session.Family{CurrentGeneration: 3}, session.Token{Generation: 3, ExpiresAt: now}, session.OutcomeExpired},
		{"family revoked", session.Family{CurrentGeneration: 3, Revoked: true},
			session.Token{Generation: 3, ExpiresAt: valid}, session.OutcomeFamilyRevoked},
		{"family revoked wins over expiry", session.Family{Revoked: true},
			session.Token{ExpiresAt: now.Add(-time.Hour)}, session.OutcomeFamilyRevoked},
	}
	for _, c := range cases {
		if got := session.Decide(c.f, c.tok, now, grace); got != c.want {
			t.Errorf("%s: Decide = %s, want %s", c.name, got, c.want)
		}
	}
}

// TestRefreshExpiry: now + auth.refresh_token_ttl, never beyond the session end.
func TestRefreshExpiry(t *testing.T) {
	t.Parallel()
	end := now.Add(10 * 24 * time.Hour)
	if got := session.RefreshExpiry(now, 14*24*time.Hour, end); !got.Equal(end) {
		t.Errorf("capped expiry %v, want %v", got, end)
	}
	if got := session.RefreshExpiry(now, time.Hour, end); !got.Equal(now.Add(time.Hour)) {
		t.Errorf("expiry %v, want %v", got, now.Add(time.Hour))
	}
}

// TestAccessToken_Claims holds the claim set of 08 "7.2": exactly iss, aud, sub, sid, did, jti,
// iat, nbf, exp, cnf.jkt and amr, no rights, exp capped at the session end.
func TestAccessToken_Claims(t *testing.T) {
	t.Parallel()
	in := session.ClaimsInput{
		InstanceID: ids.New(), Audience: "https://chat.example.com/api/v1", UserID: ids.New(), SessionID: ids.New(),
		DeviceID: ids.New(), JTI: ids.New(), Now: now.Add(400 * time.Millisecond), TTL: 10 * time.Minute,
		SessionEnd: now.Add(time.Hour), JKT: "thumb", AuthMethod: "oidc",
	}
	c := session.BuildClaims(in)
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"amr", "aud", "cnf", "did", "exp", "iat", "iss", "jti", "nbf", "sid", "sub"}
	if !slices.Equal(keys, want) {
		t.Fatalf("claims %v, want %v", keys, want)
	}
	if m["iss"] != in.InstanceID.String() || m["aud"] != in.Audience || m["sub"] != in.UserID.String() ||
		m["sid"] != in.SessionID.String() || m["did"] != in.DeviceID.String() || m["jti"] != in.JTI.String() {
		t.Errorf("identity claims %v", m)
	}
	if !reflect.DeepEqual(m["cnf"], map[string]any{"jkt": "thumb"}) || !reflect.DeepEqual(m["amr"], []any{"oidc"}) {
		t.Errorf("cnf %v amr %v", m["cnf"], m["amr"])
	}
	if c.IssuedAt != now.Unix() || c.NotBefore != now.Unix() || c.Expiry != now.Add(10*time.Minute).Unix() {
		t.Errorf("iat %d nbf %d exp %d", c.IssuedAt, c.NotBefore, c.Expiry)
	}
	if got := c.ExpiresIn(in.Now); got != 600 {
		t.Errorf("ExpiresIn = %d, want 600", got)
	}

	// The session ends earlier: exp is the session end, rounded down to a second, never after it.
	in.SessionEnd = now.Add(5*time.Minute + 700*time.Millisecond)
	c = session.BuildClaims(in)
	if c.Expiry != now.Add(5*time.Minute).Unix() {
		t.Errorf("capped exp %d, want %d", c.Expiry, now.Add(5*time.Minute).Unix())
	}
	if time.Unix(c.Expiry, 0).After(in.SessionEnd) {
		t.Error("exp after the session end")
	}
}

// TestNewRefreshToken_IDT27: 32 bytes from the cryptographic generator, base64url without padding
// (43 characters), distinct across many draws.
func TestNewRefreshToken_IDT27(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for range 2000 {
		tok, err := session.NewRefreshToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 43 {
			t.Fatalf("length %d, want 43", len(tok))
		}
		b, err := base64.RawURLEncoding.DecodeString(tok)
		if err != nil || len(b) != 32 {
			t.Fatalf("decoded %d bytes, %v", len(b), err)
		}
		if seen[tok] {
			t.Fatal("token drawn twice")
		}
		seen[tok] = true
	}
}

// TestTokenHash: HMAC-SHA-256 with the pepper, 32 bytes, depends on pepper and token.
func TestTokenHash(t *testing.T) {
	t.Parallel()
	a := session.TokenHash([]byte("pepper-a"), "token")
	if len(a) != 32 {
		t.Fatalf("hash length %d", len(a))
	}
	if slices.Equal(a, session.TokenHash([]byte("pepper-b"), "token")) || slices.Equal(a, session.TokenHash([]byte("pepper-a"), "other")) {
		t.Fatal("hash ignores pepper or token")
	}
	if !slices.Equal(a, session.TokenHash([]byte("pepper-a"), "token")) {
		t.Fatal("hash not deterministic")
	}
}

// TestAudience: origin of server.public_url plus /api/v1.
func TestAudience(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"https://chat.example.com":      "https://chat.example.com/api/v1",
		"https://chat.example.com/":     "https://chat.example.com/api/v1",
		"https://chat.example.com:8443": "https://chat.example.com:8443/api/v1",
	} {
		got, err := session.Audience(in)
		if err != nil || got != want {
			t.Errorf("Audience(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := session.Audience("not a url"); err == nil {
		t.Error("Audience accepts a value without scheme and host")
	}
}
