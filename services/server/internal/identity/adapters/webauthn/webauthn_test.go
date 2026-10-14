// SPDX-License-Identifier: Apache-2.0

package webauthn_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

const origin = "https://chat.example.com"

// verifier returns the adapter for chat.example.com with the defaults of server.public_url.
func verifier(t *testing.T) *webauthn.Verifier {
	t.Helper()
	rp, err := domain.ResolveRelyingParty(origin, "", []string{origin, "android:apk-key-hash:abc"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := webauthn.New(rp, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// challenge returns 32 random bytes.
func challenge(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

// register creates a passkey of alg for u through the adapter and returns the authenticator and
// the stored form.
func register(t *testing.T, v *webauthn.Verifier, u app.CeremonyUser, alg virtualauthn.Alg) (*virtualauthn.Authenticator, app.StoredPasskey, error) {
	t.Helper()
	c := challenge(t)
	opts, err := v.CreationOptions(u, c)
	if err != nil {
		t.Fatal(err)
	}
	a := virtualauthn.New(origin, alg)
	resp, err := a.Create(opts)
	if err != nil {
		t.Fatal(err)
	}
	np, err := v.VerifyCreation(u, c, resp)
	if err != nil {
		return a, app.StoredPasskey{}, err
	}
	return a, app.StoredPasskey{ID: ids.New(), UserID: u.ID, CredentialID: np.CredentialID, PublicKeyCOSE: np.PublicKeyCOSE,
		SignCount: np.SignCount, BackupEligible: np.BackupEligible, BackupState: np.BackupState}, nil
}

// assert runs a request ceremony of a with u as the person the lookup finds.
func assert(t *testing.T, v *webauthn.Verifier, a *virtualauthn.Authenticator, u app.CeremonyUser) (app.AssertionResult, error) {
	t.Helper()
	c := challenge(t)
	opts, err := v.RequestOptions(u.Credentials, c)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Get(opts)
	if err != nil {
		t.Fatal(err)
	}
	return v.VerifyAssertion(c, resp, func(id, _ []byte) (app.CeremonyUser, error) { return u, nil })
}

func TestVirtualAuthenticatorRoundTrip(t *testing.T) {
	t.Parallel()
	v := verifier(t)
	u := app.CeremonyUser{ID: ids.New(), Name: "alex", DisplayName: "Alex"}
	// The creation options carry the fixed policy (08 "2.5").
	opts, err := v.CreationOptions(u, challenge(t))
	if err != nil {
		t.Fatal(err)
	}
	var o map[string]any
	if err := json.Unmarshal(opts, &o); err != nil {
		t.Fatal(err)
	}
	sel := o["authenticatorSelection"].(map[string]any)
	if o["attestation"] != "none" || sel["userVerification"] != "required" || sel["residentKey"] != "preferred" ||
		o["rp"].(map[string]any)["id"] != "chat.example.com" || o["rp"].(map[string]any)["name"] != "chat.example.com" {
		t.Fatalf("creation options %s", opts)
	}
	var algs []float64
	for _, p := range o["pubKeyCredParams"].([]any) {
		algs = append(algs, p.(map[string]any)["alg"].(float64))
	}
	if len(algs) != 3 || algs[0] != -7 || algs[1] != -8 || algs[2] != -257 {
		t.Fatalf("algorithms %v", algs)
	}
	a, pk, err := register(t, v, u, virtualauthn.ES256)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pk.CredentialID, a.CredentialID()) {
		t.Fatal("credential ID differs")
	}
	u.Credentials = []app.StoredPasskey{pk}
	res, err := assert(t, v, a, u)
	if err != nil || res.SignCount != 1 || res.Passkey.ID != pk.ID || res.User.ID != u.ID {
		t.Fatalf("assertion %+v, %v", res, err)
	}
	// Discoverable: the user handle is the person's.
	a.Discoverable = true
	c := challenge(t)
	ropts, _ := v.RequestOptions(nil, c)
	resp, _ := a.Get(ropts)
	var handle []byte
	if _, err := v.VerifyAssertion(c, resp, func(_, h []byte) (app.CeremonyUser, error) { handle = h; return u, nil }); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(handle, domain.UserHandle(u.ID)) {
		t.Fatalf("user handle %x", handle)
	}
	// A lookup error passes unchanged.
	boom := errors.New("store down")
	resp, _ = a.Get(ropts)
	if _, err := v.VerifyAssertion(c, resp, func(_, _ []byte) (app.CeremonyUser, error) { return app.CeremonyUser{}, boom }); !errors.Is(err, boom) {
		t.Fatalf("lookup error %v", err)
	}
}

func TestVirtualAuthenticatorRejections(t *testing.T) {
	t.Parallel()
	v := verifier(t)
	u := app.CeremonyUser{ID: ids.New(), Name: "alex", DisplayName: "Alex"}
	a, pk, err := register(t, v, u, virtualauthn.ES256)
	if err != nil {
		t.Fatal(err)
	}
	u.Credentials = []app.StoredPasskey{pk}
	other, _, _ := register(t, v, app.CeremonyUser{ID: ids.New(), Name: "sam", DisplayName: "Sam"}, virtualauthn.ES256)
	cases := map[string]func(){
		"no user verification": func() { a.NoUserVerification = true },
		"wrong origin":         func() { a.Origin = "https://evil.example.org" },
		"wrong RP ID":          func() { a.RPID = "evil.example.org" },
		"another key":          func() { a.SignWith = other },
		"backup eligibility changed": func() {
			a.BackupEligible = true
		},
	}
	for name, turn := range cases {
		a.NoUserVerification, a.Origin, a.RPID, a.SignWith, a.BackupEligible = false, origin, "", nil, false
		turn()
		if _, err := assert(t, v, a, u); !errors.Is(err, app.ErrPasskeyRejected) {
			t.Errorf("%s: %v, want rejected", name, err)
		}
	}
	a.NoUserVerification, a.Origin, a.RPID, a.SignWith, a.BackupEligible = false, origin, "", nil, false
	// Another challenge than the one the authenticator signed.
	opts, _ := v.RequestOptions(u.Credentials, challenge(t))
	resp, _ := a.Get(opts)
	if _, err := v.VerifyAssertion(challenge(t), resp, func(_, _ []byte) (app.CeremonyUser, error) { return u, nil }); !errors.Is(err, app.ErrPasskeyRejected) {
		t.Errorf("wrong challenge: %v", err)
	}
	// A credential that is not one of the person.
	if _, err := assert(t, v, other, u); !errors.Is(err, app.ErrPasskeyRejected) {
		t.Errorf("foreign credential: %v", err)
	}
	if _, err := v.VerifyAssertion(challenge(t), []byte(`{}`), nil); !errors.Is(err, app.ErrPasskeyRejected) {
		t.Errorf("garbage: %v", err)
	}
	if _, err := v.VerifyCreation(u, challenge(t), []byte(`{}`)); !errors.Is(err, app.ErrPasskeyRejected) {
		t.Errorf("garbage creation: %v", err)
	}
	// A creation without user verification is refused too.
	c := challenge(t)
	copts, _ := v.CreationOptions(u, c)
	nv := virtualauthn.New(origin, virtualauthn.ES256)
	nv.NoUserVerification = true
	cresp, _ := nv.Create(copts)
	if _, err := v.VerifyCreation(u, c, cresp); !errors.Is(err, app.ErrPasskeyRejected) {
		t.Errorf("creation without UV: %v", err)
	}
}

func TestVirtualAuthenticatorAlgorithms(t *testing.T) {
	t.Parallel()
	v := verifier(t)
	for _, alg := range []virtualauthn.Alg{virtualauthn.ES256, virtualauthn.EdDSA, virtualauthn.RS256} {
		u := app.CeremonyUser{ID: ids.New(), Name: "alex", DisplayName: "Alex"}
		a, pk, err := register(t, v, u, alg)
		if err != nil {
			t.Fatalf("alg %d: %v", alg, err)
		}
		u.Credentials = []app.StoredPasskey{pk}
		if _, err := assert(t, v, a, u); err != nil {
			t.Fatalf("alg %d assertion: %v", alg, err)
		}
	}
	if _, _, err := register(t, v, app.CeremonyUser{ID: ids.New(), Name: "sam", DisplayName: "Sam"}, virtualauthn.ES384); !errors.Is(err, app.ErrPasskeyRejected) {
		t.Fatalf("ES384: %v, want rejected", err)
	}
}

func TestNewRefusesDisabledAndUnknownAppOrigins(t *testing.T) {
	t.Parallel()
	if _, err := webauthn.New(domain.RelyingParty{}, time.Minute); err == nil {
		t.Fatal("disabled relying party accepted")
	}
	rp := domain.RelyingParty{ID: "chat.example.com", Origins: []string{origin, "made-up:thing"}, Enabled: true}
	if _, err := webauthn.New(rp, time.Minute); err == nil {
		t.Fatal("unknown app origin accepted")
	}
}
