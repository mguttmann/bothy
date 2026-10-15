// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/stepup"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCeremonySingleUse checks the decision before a confirmation (brief M1-40 "Tests"): an
// issued row before its expiry is open, a used or failed row is finished and an expired row is
// expired; the stored session never contains the challenge value.
func TestCeremonySingleUse(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		status  string
		expires time.Time
		want    domain.CeremonyOutcome
	}{
		{"issued", now.Add(time.Second), domain.CeremonyOpen},
		{"issued", now, domain.CeremonyExpired},
		{"issued", now.Add(-time.Second), domain.CeremonyExpired},
		{"used", now.Add(time.Minute), domain.CeremonyFinished},
		{"failed", now.Add(time.Minute), domain.CeremonyFinished},
	}
	for _, c := range cases {
		if got := domain.CheckCeremony(c.status, c.expires, now); got != c.want {
			t.Errorf("%s expiring %v: %v, want %v", c.status, c.expires.Sub(now), got, c.want)
		}
	}
	value, err := stepup.NewChallengeValue()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := domain.DecodeChallenge(value)
	s := domain.NewWebAuthnSession("chat.example.com", [][]byte{[]byte("cred-1")}, domain.UserHandle(ids.New()))
	b, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), value) || bytes.Contains(b, raw) || strings.Contains(string(b), "challenge") {
		t.Fatalf("stored session %s contains the challenge", b)
	}
	back, err := domain.ParseWebAuthnSession(b)
	if err != nil || back.RPID != "chat.example.com" || !back.Allows([]byte("cred-1")) || back.Allows([]byte("cred-2")) {
		t.Fatalf("round trip %+v, %v", back, err)
	}
	for _, bad := range []string{`{}`, `{"schema":"webauthn_session.v2","rp_id":"a.b","user_verification":"required"}`,
		`{"schema":"webauthn_session.v1","rp_id":"a.b","user_verification":"preferred"}`, `nope`} {
		if _, err := domain.ParseWebAuthnSession([]byte(bad)); err == nil {
			t.Errorf("%s parsed", bad)
		}
	}
	if !domain.NewWebAuthnSession("a.b", nil, nil).Allows([]byte("any")) {
		t.Fatal("a session without allowed IDs must allow every credential (discoverable login)")
	}
}

// credential builds a PublicKeyCredential JSON whose client data names challenge.
func credential(challenge string) []byte {
	cd, _ := json.Marshal(map[string]string{"type": "webauthn.get", "challenge": challenge, "origin": "https://chat.example.com"})
	b, _ := json.Marshal(map[string]any{"response": map[string]string{"clientDataJSON": base64.RawURLEncoding.EncodeToString(cd)}})
	return b
}

func TestChallengeOfClientData(t *testing.T) {
	t.Parallel()
	pepper := []byte("0123456789abcdef0123456789abcdef")
	value, err := stepup.NewChallengeValue()
	if err != nil {
		t.Fatal(err)
	}
	stored := stepup.ChallengeHash(pepper, value)
	got, ok := domain.ClientDataChallenge(credential(value))
	if !ok || got != value || !domain.ChallengeMatches(pepper, stored, got) {
		t.Fatalf("challenge %q ok %v", got, ok)
	}
	// Padding a client adds does not change the value.
	raw, _ := domain.DecodeChallenge(value)
	if !domain.ChallengeMatches(pepper, stored, base64.URLEncoding.EncodeToString(raw)) {
		t.Fatal("padded challenge rejected")
	}
	other, _ := stepup.NewChallengeValue()
	if domain.ChallengeMatches(pepper, stored, other) || domain.ChallengeMatches(pepper, stored, "%%%") {
		t.Fatal("another challenge matched")
	}
	for _, bad := range [][]byte{[]byte(`{}`), []byte(`{"response":{"clientDataJSON":"%%%"}}`), []byte(`[]`),
		credential("")} {
		if _, ok := domain.ClientDataChallenge(bad); ok {
			t.Errorf("%s gave a challenge", bad)
		}
	}
}

// TestSignCountRule is the table of brief M1-40 "Sign counter".
func TestSignCountRule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stored, received, keep uint32
		regression             bool
	}{
		{0, 0, 0, false}, // synced passkeys keep zero
		{0, 5, 5, false}, // first counter
		{5, 0, 5, false}, // a counter that drops to zero is no regression, the higher value stays
		{5, 6, 6, false}, // increase
		{5, 5, 5, true},  // equal
		{7, 3, 7, true},  // decrease
		{1, 1 << 31, 1 << 31, false},
	}
	for _, c := range cases {
		keep, reg := domain.SignCount(c.stored, c.received)
		if keep != c.keep || reg != c.regression {
			t.Errorf("SignCount(%d, %d) = %d, %v; want %d, %v", c.stored, c.received, keep, reg, c.keep, c.regression)
		}
	}
}

func TestDecoyCredentialID(t *testing.T) {
	t.Parallel()
	pepper := []byte("0123456789abcdef0123456789abcdef")
	a := domain.DecoyCredentialID(pepper, "Sam")
	if len(a) != 32 || !bytes.Equal(a, domain.DecoyCredentialID(pepper, "sam")) {
		t.Fatalf("decoy %x not stable or not 32 bytes", a)
	}
	if bytes.Equal(a, domain.DecoyCredentialID(pepper, "robin")) || bytes.Equal(a, domain.DecoyCredentialID([]byte("another pepper"), "sam")) {
		t.Fatal("decoys of different names or peppers are equal")
	}
}

func TestUserHandle(t *testing.T) {
	t.Parallel()
	id := ids.New()
	h := domain.UserHandle(id)
	back, ok := domain.UserOfHandle(h)
	if len(h) != 16 || !ok || back != id {
		t.Fatalf("handle %x back %v %v", h, back, ok)
	}
	if _, ok := domain.UserOfHandle([]byte("short")); ok {
		t.Fatal("short handle accepted")
	}
}

// TestWebAuthnConfigValidation checks the start validation of auth.webauthn_rp_id and
// auth.webauthn_origins (brief M1-40 "Design notes").
func TestWebAuthnConfigValidation(t *testing.T) {
	t.Parallel()
	rp, err := domain.ResolveRelyingParty("https://chat.example.com:8443", "", nil)
	if err != nil || !rp.Enabled || rp.ID != "chat.example.com" || len(rp.Origins) != 1 || rp.Origins[0] != "https://chat.example.com:8443" {
		t.Fatalf("defaults %+v, %v", rp, err)
	}
	ok := []struct {
		rpID    string
		origins []string
	}{
		{"example.com", []string{"https://chat.example.com", "https://example.com"}},
		{"chat.example.com", []string{"https://chat.example.com", "android:apk-key-hash:abc"}},
		{"localhost", []string{"http://localhost:8080"}},
	}
	for _, c := range ok {
		if _, err := domain.ResolveRelyingParty("https://chat.example.com", c.rpID, c.origins); err != nil {
			t.Errorf("%s %v: %v", c.rpID, c.origins, err)
		}
	}
	bad := []struct {
		rpID    string
		origins []string
	}{
		{"chat.example.com", []string{"https://example.com"}},           // RP ID is no suffix of the host
		{"example.com", []string{"https://badexample.com"}},             // suffix only on a label boundary
		{"chat.example.com", []string{"http://chat.example.com"}},       // http only for localhost
		{"Chat.Example.com", nil},                                       // RP ID in lower case
		{"com", []string{"https://example.com"}},                        // one label
		{"192.0.2.1", []string{"https://192.0.2.1"}},                    // an IP address
		{"chat.example.com", []string{"https://chat.example.com/path"}}, // not an origin
		{"chat.example.com", []string{"no origin"}},                     // neither http nor app origin
		{"chat.example.com:443", []string{"https://chat.example.com"}},  // a port in the RP ID
	}
	for _, c := range bad {
		_, err := domain.ResolveRelyingParty("https://chat.example.com", c.rpID, c.origins)
		if err == nil {
			t.Errorf("%s %v accepted", c.rpID, c.origins)
			continue
		}
		for _, o := range c.origins {
			if strings.Contains(err.Error(), o) {
				t.Errorf("error %q repeats the value", err)
			}
		}
	}
	// An IP address as host of server.public_url without auth.webauthn_rp_id: no passkeys, no
	// start failure.
	rp, err = domain.ResolveRelyingParty("https://192.0.2.1:8443", "", nil)
	if err != nil || rp.Enabled {
		t.Fatalf("IP public URL %+v, %v", rp, err)
	}
	// http without localhost and nothing configured: no passkeys either; configured, it fails.
	if rp, err = domain.ResolveRelyingParty("http://chat.example.com", "", nil); err != nil || rp.Enabled {
		t.Fatalf("http public URL %+v, %v", rp, err)
	}
	// A single-label host without configuration: no passkeys, and the start keeps working.
	if rp, err = domain.ResolveRelyingParty("https://bothy:8443", "", nil); err != nil || rp.Enabled {
		t.Fatalf("single-label public URL %+v, %v", rp, err)
	}
	if _, err := domain.ResolveRelyingParty("http://chat.example.com", "chat.example.com", nil); err == nil {
		t.Fatal("explicit RP ID with an http origin accepted")
	}
	if _, err := domain.ResolveRelyingParty("::not a url", "", nil); err == nil {
		t.Fatal("broken public URL accepted")
	}
}
