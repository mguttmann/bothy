// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/base32"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/passwordhash"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/secretbox"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// passwordURL is the htu of the login proofs of the password login.
const passwordURL = loginURL + "/api/v1/auth/password/login"

// localKeyRef is the TOTP key reference of the use case test; localKeys resolves it.
const localKeyRef = "file:<secrets.dir>/totp_key"

// localKeys resolves localKeyRef to a fixed test key (32 bytes as hex, never a secret).
type localKeys struct{}

// Resolve implements secretbox.Resolver.
func (localKeys) Resolve(_ context.Context, ref string) ([]byte, error) {
	if ref != localKeyRef {
		return nil, errors.New("unknown reference")
	}
	return []byte(strings.Repeat("6e", 32)), nil
}

// bindLocal binds the local accounts of id with the TOTP box of localKeys and an Argon2id hasher
// with the floor parameters.
func (w *peopleWorld) bindLocal(id *wire.Identity) *app.LocalCredentials {
	w.t.Helper()
	box, err := secretbox.New(context.Background(), localKeys{}, localKeyRef)
	if err != nil {
		w.t.Fatal(err)
	}
	h, err := passwordhash.New(passwordhash.Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1})
	if err != nil {
		w.t.Fatal(err)
	}
	if err := id.BindLocal(box, h); err != nil {
		w.t.Fatal(err)
	}
	return id.Local
}

// passwordDevice returns a new device registration with a login proof of its key for passwordURL.
func (w *peopleWorld) passwordDevice(id *wire.Identity) (identity.RegisterDevice, identity.ProofRequest) {
	w.t.Helper()
	k := freshKey(w.t)
	proof := identitytest.SignDPoPProof(k.key, identitytest.ProofClaims{HTM: http.MethodPost, HTU: passwordURL, IAT: w.clk.Now(), Nonce: id.DPoP.CurrentNonce()})
	return identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: k.pub},
		identity.ProofRequest{Proofs: []string{proof}, Method: http.MethodPost, URL: passwordURL}
}

// secretOf returns the TOTP secret of an otpauth URI.
func secretOf(t *testing.T, uri string) []byte {
	t.Helper()
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(u.Query().Get("secret"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestLocalCredentials_UseCases runs the use cases of app.LocalCredentials directly (the HTTP tests
// of M1-39 cover the same through the operations; INT-W18/7): the validation, setting the password
// with a TOTP enrollment, a wrong and a right confirmation, the step-up once the path is
// established, the login with a pending device, a replayed code, a wrong password and an unknown
// name, and the credential overview.
func TestLocalCredentials_UseCases(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	lc := w.bindLocal(id)
	ctx := context.Background()
	res, err := id.Logins.BootstrapOwner(ctx, app.BootstrapOwnerCmd{Handle: "alex", DisplayName: "Alex"})
	if err != nil {
		t.Fatal(err)
	}
	dev, sess := ids.New(), ids.New()
	w.creds.PutDevice(dev, fake.Device{PrincipalID: res.OwnerID, State: authz.DeviceActive})
	w.creds.PutSession(sess, fake.Session{PrincipalID: res.OwnerID, DeviceID: dev, ExpiresAt: w.clk.Now().Add(24 * time.Hour)})
	alex := actor.Actor{ID: res.OwnerID, Kind: actor.KindUser, SessionID: sess, DeviceID: dev}

	// Validation: nothing set, too short, on the common list.
	_, err = lc.SetLocalCredential(ctx, alex, identity.SetLocalCredential{})
	wantKind(t, err, errs.KindBadRequest)
	for _, pw := range []string{"EXAMPLE-sh", "unbelievable"} {
		_, err = lc.SetLocalCredential(ctx, alex, identity.SetLocalCredential{Password: &pw})
		wantKind(t, err, errs.KindBadRequest)
	}

	// Password and enrollment without step-up; a wrong code does not confirm, the right one does.
	pw, yes := "EXAMPLE-correct-horse-1", true
	r, err := lc.SetLocalCredential(ctx, alex, identity.SetLocalCredential{Password: &pw, TOTPEnroll: &yes})
	if err != nil || !r.PasswordSet || r.TOTPEnabled || r.TOTPSecretURI.IsZero() {
		t.Fatalf("enroll %+v, %v", r, err)
	}
	secret := secretOf(t, r.TOTPSecretURI.Reveal())
	code := domain.TOTPCode(secret, domain.TOTPStep(w.clk.Now()), domain.TOTPDigits)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	_, err = lc.SetLocalCredential(ctx, alex, identity.SetLocalCredential{TOTPCode: &wrong})
	wantKind(t, err, errs.KindBadRequest)
	r, err = lc.SetLocalCredential(ctx, alex, identity.SetLocalCredential{TOTPCode: &code})
	if err != nil || !r.TOTPEnabled || !r.TOTPSecretURI.IsZero() {
		t.Fatalf("confirm %+v, %v", r, err)
	}
	// Established: a change needs step-up now.
	if _, err := lc.SetLocalCredential(ctx, alex, identity.SetLocalCredential{Password: &pw}); pkCode(err) != string(errs.CodeStepUpRequired) {
		t.Fatalf("change without step-up: %v", err)
	}

	// Login one step later: a new device is pending; the same code again, a wrong password and an
	// unknown name are the one unauthenticated.
	w.clk.Advance(domain.TOTPPeriod)
	code = domain.TOTPCode(secret, domain.TOTPStep(w.clk.Now()), domain.TOTPDigits)
	d, proof := w.passwordDevice(id)
	login, err := lc.LoginWithPassword(ctx, identity.PasswordLogin{LoginName: " Alex ", Password: pw, TOTPCode: code, Device: d, Proof: proof})
	if err != nil || login.Tokens.Session.AuthMethod != identity.AuthPasswordTOTP || login.Tokens.Device.State != identity.DeviceStatePending {
		t.Fatalf("login %+v, %v", login.Tokens.Session, err)
	}
	for _, in := range []struct{ name, password string }{{"alex", pw}, {"alex", "EXAMPLE-wrong-horse-2"}, {"nobody", pw}} {
		d, proof := w.passwordDevice(id)
		_, err := lc.LoginWithPassword(ctx, identity.PasswordLogin{LoginName: in.name, Password: in.password, TOTPCode: code, Device: d, Proof: proof})
		wantKind(t, err, errs.KindUnauthenticated)
	}

	// The overview reports the password and the TOTP enrollment.
	o, err := id.Identities.Credentials(ctx, alex, res.OwnerID)
	if err != nil || !o.LocalPasswordSet || !o.TOTPEnrolled {
		t.Fatalf("overview %+v, %v", o, err)
	}
}
