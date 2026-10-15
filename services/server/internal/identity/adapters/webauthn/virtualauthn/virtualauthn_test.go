// SPDX-License-Identifier: Apache-2.0

package virtualauthn_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
)

const (
	creation = `{"challenge":"AAAAAAAAAAAAAAAAAAAAAA","rp":{"id":"chat.example.com"},"user":{"id":"AAAAAAAAAAAAAAAAAAAAAA"}}`
	request  = `{"challenge":"AAAAAAAAAAAAAAAAAAAAAA","rpId":"chat.example.com"}`
)

// flags returns the flags byte and the counter of the authenticator data of an assertion.
func flags(t *testing.T, resp []byte) (byte, uint32) {
	t.Helper()
	var m struct {
		Response struct {
			AuthenticatorData string `json:"authenticatorData"`
			UserHandle        string `json:"userHandle"`
		} `json:"response"`
	}
	if err := json.Unmarshal(resp, &m); err != nil {
		t.Fatal(err)
	}
	b, err := base64.RawURLEncoding.DecodeString(m.Response.AuthenticatorData)
	if err != nil || len(b) != 37 {
		t.Fatalf("authenticator data %x, %v", b, err)
	}
	return b[32], uint32(b[33])<<24 | uint32(b[34])<<16 | uint32(b[35])<<8 | uint32(b[36])
}

func TestAuthenticatorKnobs(t *testing.T) {
	t.Parallel()
	for _, alg := range []virtualauthn.Alg{virtualauthn.ES256, virtualauthn.ES384, virtualauthn.EdDSA, virtualauthn.RS256} {
		a := virtualauthn.New("https://chat.example.com", alg)
		if _, err := a.Get([]byte(request)); err == nil {
			t.Fatal("Get before Create")
		}
		if _, err := a.Create([]byte(creation)); err != nil || len(a.CredentialID()) != 32 {
			t.Fatalf("alg %d: %v", alg, err)
		}
		if _, err := a.Get([]byte(request)); err != nil {
			t.Fatalf("alg %d get: %v", alg, err)
		}
	}
	a := virtualauthn.New("https://chat.example.com", virtualauthn.ES256)
	if _, err := a.Create([]byte(`{}`)); err == nil {
		t.Fatal("unreadable options accepted")
	}
	if _, err := a.Create([]byte(`{"challenge":"x","rp":{"id":"a.b"},"user":{"id":"%%"}}`)); err == nil {
		t.Fatal("bad user id accepted")
	}
	if _, err := a.Create([]byte(creation)); err != nil {
		t.Fatal(err)
	}
	id := a.CredentialID()
	a.Reuse = true
	if _, err := a.Create([]byte(creation)); err != nil || string(a.CredentialID()) != string(id) {
		t.Fatal("Reuse made a new credential")
	}
	if _, err := a.Get([]byte(`nope`)); err == nil {
		t.Fatal("unreadable request accepted")
	}
	a.NoUserVerification, a.BackupEligible, a.BackupState, a.Discoverable, a.RPID = true, true, true, true, "evil.example.org"
	a.SignWith = virtualauthn.New("https://chat.example.com", virtualauthn.ES256)
	_, _ = a.SignWith.Create([]byte(creation))
	resp, err := a.Get([]byte(request))
	if err != nil {
		t.Fatal(err)
	}
	f, c := flags(t, resp)
	if f != 0x01|0x08|0x10 || c != 1 {
		t.Fatalf("flags %x counter %d", f, c)
	}
	a.FreezeCounter = true
	resp, _ = a.Get([]byte(request))
	if _, c := flags(t, resp); c != 1 {
		t.Fatalf("frozen counter %d", c)
	}
	if _, err := virtualauthn.New("x", virtualauthn.Alg(-999)).Create([]byte(creation)); err == nil {
		t.Fatal("unknown algorithm accepted")
	}
}
