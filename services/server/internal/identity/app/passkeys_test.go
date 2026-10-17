// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// passkeyURL is the htu of the login proofs of the passkey use cases.
const passkeyURL = loginURL + "/api/v1/auth/passkey/complete"

// publicKey returns the options of a ceremony challenge.
func publicKey(t *testing.T, c identity.WebAuthnChallenge) []byte {
	t.Helper()
	if len(c.PublicKey) == 0 || c.ID.IsZero() {
		t.Fatalf("challenge %+v", c)
	}
	return c.PublicKey
}

// device returns a new device registration with a login proof of its key for passkeyURL.
func (w *peopleWorld) device(id *wire.Identity) (identity.RegisterDevice, identity.ProofRequest) {
	w.t.Helper()
	k := freshKey(w.t)
	proof := identitytest.SignDPoPProof(k.key, identitytest.ProofClaims{HTM: http.MethodPost, HTU: passkeyURL, IAT: w.clk.Now(), Nonce: id.DPoP.CurrentNonce()})
	return identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: k.pub},
		identity.ProofRequest{Proofs: []string{proof}, Method: http.MethodPost, URL: passkeyURL}
}

// count returns the integer result of query as the database owner.
func (w *peopleWorld) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// TestPasskeys_UseCases runs the passkey use cases of app.Passkeys and the passkey step-up of
// app.StepUps directly with the virtual authenticator (the HTTP tests cover the same through the
// operations): enrollment with a failed attempt, login with name, decoy and replay, the step-up,
// the registration with its conditional step-up, the credential overview and the revocation.
func TestPasskeys_UseCases(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	ctx := context.Background()
	pk := id.Passkeys
	res, err := id.Logins.BootstrapOwner(ctx, app.BootstrapOwnerCmd{Handle: "alex", DisplayName: "Alex"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pk.BeginEnrollment(ctx, "en_0000-0000-0000-0000-0000-0000-0000-0000"); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("unknown code: %v", err)
	}
	// A response from another origin counts one failed attempt.
	c, err := pk.BeginEnrollment(ctx, res.Code.Code)
	if err != nil {
		t.Fatal(err)
	}
	evil := virtualauthn.New("https://evil.example.org", virtualauthn.ES256)
	cred, _ := evil.Create(publicKey(t, c))
	dev, proof := w.device(id)
	_, err = pk.CompleteEnrollment(ctx, identity.PasskeyEnrollment{ChallengeID: c.ID, Credential: cred, PasskeyName: "Laptop", Device: dev, Proof: proof})
	wantKind(t, err, errs.KindUnauthenticated)
	if attempts := w.count(`SELECT attempts FROM enrollment_codes WHERE id = $1`, res.Code.ID.String()); attempts != 1 {
		t.Fatalf("attempts %d", attempts)
	}
	a := virtualauthn.New(loginURL, virtualauthn.ES256)
	c, _ = pk.BeginEnrollment(ctx, res.Code.Code)
	cred, _ = a.Create(publicKey(t, c))
	dev, proof = w.device(id)
	enrolled, err := pk.CompleteEnrollment(ctx, identity.PasskeyEnrollment{ChallengeID: c.ID, Credential: cred, PasskeyName: "Laptop", Device: dev, Proof: proof})
	if err != nil || enrolled.Tokens.Device.State != identity.DeviceStateActive {
		t.Fatalf("enrollment %+v, %v", enrolled.Tokens.Device, err)
	}

	// Login with the name, then the same assertion again.
	c, err = pk.BeginPasskeyLogin(ctx, "Alex")
	if err != nil {
		t.Fatal(err)
	}
	cred, _ = a.Get(publicKey(t, c))
	dev, proof = w.device(id)
	login, err := pk.CompletePasskeyLogin(ctx, identity.PasskeyLogin{ChallengeID: c.ID, Credential: cred, Device: dev, Proof: proof})
	if err != nil || login.Tokens.Session.AuthMethod != identity.AuthPasskey || login.Tokens.Device.State != identity.DeviceStatePending {
		t.Fatalf("login %+v, %v", login.Tokens.Session, err)
	}
	dev, proof = w.device(id)
	_, err = pk.CompletePasskeyLogin(ctx, identity.PasskeyLogin{ChallengeID: c.ID, Credential: cred, Device: dev, Proof: proof})
	wantKind(t, err, errs.KindUnauthenticated)
	dev, proof = w.device(id)
	_, err = pk.CompletePasskeyLogin(ctx, identity.PasskeyLogin{ChallengeID: ids.New(), Credential: json.RawMessage(`{}`), Device: dev, Proof: proof})
	wantKind(t, err, errs.KindUnauthenticated)
	// Unknown name: one decoy; no name: discoverable.
	for _, name := range []string{"robin", ""} {
		c, err := pk.BeginPasskeyLogin(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		var o map[string]any
		_ = json.Unmarshal(c.PublicKey, &o)
		allow, _ := o["allowCredentials"].([]any)
		if (name == "") != (len(allow) == 0) {
			t.Fatalf("%q: allowCredentials %v", name, allow)
		}
	}

	// Alex in the enrolled session: the step-up with the passkey, then a second passkey.
	s := enrolled.Tokens.Session
	w.creds.PutDevice(s.DeviceID, fake.Device{PrincipalID: res.OwnerID, State: authz.DeviceActive})
	w.creds.PutSession(s.ID, fake.Session{PrincipalID: res.OwnerID, DeviceID: s.DeviceID, ExpiresAt: w.clk.Now().Add(24 * time.Hour)})
	alex := actor.Actor{ID: res.OwnerID, Kind: actor.KindUser, SessionID: s.ID, DeviceID: s.DeviceID}
	if _, err := pk.BeginPasskeyRegistration(ctx, alex); pkCode(err) != string(errs.CodeStepUpRequired) {
		t.Fatalf("second passkey without step-up: %v", err)
	}
	ch, err := id.StepUps.CreateChallenge(ctx, alex)
	if err != nil || len(ch.PasskeyOptions) == 0 {
		t.Fatalf("step-up challenge %+v, %v", ch, err)
	}
	cred, _ = a.Get(ch.PasskeyOptions)
	r, err := id.StepUps.CompleteWithPasskey(ctx, alex, ch.ID, cred)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := id.StepUps.CompleteWithPasskey(ctx, alex, ch.ID, cred); pkCode(err) != string(errs.CodeStepUpRequired) {
		t.Fatalf("reused step-up challenge: %v", err)
	}
	alex.StepUpAt = r.StepUpAt
	c, err = pk.BeginPasskeyRegistration(ctx, alex)
	if err != nil {
		t.Fatal(err)
	}
	second := virtualauthn.New(loginURL, virtualauthn.EdDSA)
	cred, _ = second.Create(publicKey(t, c))
	p, err := pk.CompletePasskeyRegistration(ctx, alex, c.ID, cred, "Phone")
	if err != nil || p.Name != "Phone" {
		t.Fatalf("registration %+v, %v", p, err)
	}
	if _, err := pk.CompletePasskeyRegistration(ctx, alex, c.ID, cred, "Phone"); errs.KindOf(err) != errs.KindBadRequest {
		t.Fatalf("used registration challenge: %v", err)
	}
	c, _ = pk.BeginPasskeyRegistration(ctx, alex)
	second.Reuse = true
	cred, _ = second.Create(publicKey(t, c))
	if _, err := pk.CompletePasskeyRegistration(ctx, alex, c.ID, cred, "Again"); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("duplicate: %v", err)
	}

	// The overview lists both passkeys; revoke one twice, an unknown one is not found.
	o, err := id.Identities.Credentials(ctx, alex, res.OwnerID)
	if err != nil || len(o.Passkeys) != 2 {
		t.Fatalf("overview %+v, %v", o.Passkeys, err)
	}
	for range 2 {
		if err := pk.RevokePasskey(ctx, alex, res.OwnerID, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	wantKind(t, pk.RevokePasskey(ctx, alex, res.OwnerID, ids.New()), errs.KindNotFound)
}

// TestPasskeys_DisabledRelyingParty: without a relying party every passkey operation answers
// feature_disabled and the step-up challenge has no passkey options.
func TestPasskeys_DisabledRelyingParty(t *testing.T) {
	t.Parallel()
	cfg := loginCfg
	cfg.PublicURL = "https://192.0.2.1"
	rp, err := domain.ResolveRelyingParty(cfg.PublicURL, "", nil)
	if err != nil || rp.Enabled {
		t.Fatalf("relying party %+v, %v", rp, err)
	}
	if _, err := app.NewPasskeys(cfg, nil, app.PasskeysDeps{}); err == nil {
		t.Fatal("NewPasskeys without collaborators")
	}
	w := newPeopleWorld(t)
	id := w.identityOf()
	ctx := context.Background()
	repo := postgres.NewRepository(w.pool)
	disabled, err := app.NewPasskeys(loginCfg, []byte("0123456789abcdef0123456789abcdef"), app.PasskeysDeps{
		Passkeys: repo, Challenges: repo, Logins: id.Logins, UoW: w.uow, Authorizer: w.az, Audit: w.rec,
	})
	if err != nil {
		t.Fatal(err)
	}
	a := actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New()}
	checks := []error{
		func() error { _, err := disabled.BeginPasskeyLogin(ctx, ""); return err }(),
		func() error { _, err := disabled.CompletePasskeyLogin(ctx, identity.PasskeyLogin{}); return err }(),
		func() error { _, err := disabled.BeginEnrollment(ctx, ""); return err }(),
		func() error { _, err := disabled.CompleteEnrollment(ctx, identity.PasskeyEnrollment{}); return err }(),
		func() error { _, err := disabled.BeginPasskeyRegistration(ctx, a); return err }(),
		func() error { _, err := disabled.CompletePasskeyRegistration(ctx, a, ids.New(), nil, ""); return err }(),
	}
	for i, err := range checks {
		if pkCode(err) != string(errs.CodeFeatureDisabled) {
			t.Errorf("operation %d: %v, want feature_disabled", i, err)
		}
	}
}

// pkCode returns the public code of a kernel/errs error; "" for any other error.
func pkCode(err error) string {
	if e, ok := errs.As(err); ok {
		return string(e.PublicCode())
	}
	return ""
}
