// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The passkey paths of docs/api/openapi.yaml (M1-40).
const (
	passkeyBeginPath     = "/api/v1/auth/passkey/begin"    //nolint:gosec // G101: an API path, no secret
	passkeyCompletePath  = "/api/v1/auth/passkey/complete" //nolint:gosec // G101: an API path, no secret
	enrollBeginPath      = "/api/v1/auth/enroll/begin"
	enrollCompletePath   = "/api/v1/auth/enroll/complete"
	registerBeginPath    = "/api/v1/me/passkeys/begin"
	registerCompletePath = "/api/v1/me/passkeys/complete"
	stepUpPasskeyPath    = "/api/v1/auth/step-up/passkey" //nolint:gosec // G101: an API path, no secret
	myCredentialsPath    = "/api/v1/me/credentials"       //nolint:gosec // G101: an API path, no secret
)

// authenticator is a virtual ES256 platform authenticator at the origin of the test server.
func authenticator() *virtualauthn.Authenticator {
	return virtualauthn.New(httpservertest.PublicURL, virtualauthn.ES256)
}

// options returns the public_key member of a WebAuthnChallenge as JSON.
func options(t *testing.T, m map[string]any, member string) []byte {
	t.Helper()
	b, err := json.Marshal(m[member])
	if err != nil || len(b) < 3 {
		t.Fatalf("%s: %s, %v", member, b, err)
	}
	return b
}

// device is the DeviceRegistration of p.
func (p *newPhone) device() map[string]any {
	return map[string]any{"device_id": p.id.String(), "name": "Alex Phone", "platform": "ios", "public_jwk": json.RawMessage(p.pub.JWK), "alg": "ES256"}
}

// anonPost sends an anonymous POST with a login DPoP proof of signer (none when nil).
func (w *sworld) anonPost(path string, body any, signer *identitytest.Key) *httpservertest.Response {
	w.t.Helper()
	b, _ := json.Marshal(body)
	if signer == nil {
		return w.h.Do(w.h.NewRequest(http.MethodPost, path, string(b)))
	}
	proof := identitytest.SignDPoPProof(signer, identitytest.ProofClaims{
		HTM: http.MethodPost, HTU: httpservertest.PublicURL + path, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(),
	})
	return w.h.Do(w.h.NewRequest(http.MethodPost, path, string(b), "DPoP", proof))
}

// enrollBegin starts an enrollment with code and returns the challenge id and the options.
func (w *sworld) enrollBegin(code string) (string, []byte) {
	w.t.Helper()
	m := w.want(w.anonPost(enrollBeginPath, map[string]any{"enrollment_code": code}, nil), http.StatusOK, "")
	return m["challenge_id"].(string), options(w.t, m, "public_key")
}

// enrollComplete completes the enrollment id from p with the creation response cred.
func (w *sworld) enrollComplete(p *newPhone, id string, cred []byte) *httpservertest.Response {
	w.t.Helper()
	return w.anonPost(enrollCompletePath, map[string]any{
		"challenge_id": id, "credential": json.RawMessage(cred), "passkey_name": "Laptop", "device": p.device(),
	}, p.key)
}

// clientOf returns the client of a TokenResponse m on the device p.
func (w *sworld) clientOf(p *newPhone, m map[string]any) *client {
	sess := m["session"].(map[string]any)
	return &client{w: w, key: p.key, user: ids.MustParse(sess["user_id"].(string)), device: p.id,
		session: ids.MustParse(sess["id"].(string)), access: m["access_token"].(string), refresh: m["refresh_token"].(string)}
}

// loginBegin starts a passkey login with name (none for "") and returns the challenge id and
// options.
func (w *sworld) loginBegin(name string) (string, []byte) {
	w.t.Helper()
	body := map[string]any{}
	if name != "" {
		body["login_name"] = name
	}
	m := w.want(w.anonPost(passkeyBeginPath, body, nil), http.StatusOK, "")
	return m["challenge_id"].(string), options(w.t, m, "public_key")
}

// loginComplete sends the assertion cred for the challenge id from p.
func (w *sworld) loginComplete(p *newPhone, id string, cred []byte) *httpservertest.Response {
	w.t.Helper()
	return w.anonPost(passkeyCompletePath, map[string]any{"challenge_id": id, "credential": json.RawMessage(cred), "device": p.device()}, p.key)
}

// passkeyLogin runs a passkey login with name and authenticator a from p.
func (w *sworld) passkeyLogin(name string, a *virtualauthn.Authenticator, p *newPhone) *httpservertest.Response {
	w.t.Helper()
	id, opts := w.loginBegin(name)
	cred, err := a.Get(opts)
	if err != nil {
		w.t.Fatal(err)
	}
	return w.loginComplete(p, id, cred)
}

// register adds a passkey of authenticator a through the API for c and returns the response.
func (c *client) register(a *virtualauthn.Authenticator, name string) *httpservertest.Response {
	c.w.t.Helper()
	m := c.w.want(c.call(http.MethodPost, registerBeginPath, ""), http.StatusOK, "")
	cred, err := a.Create(options(c.w.t, m, "public_key"))
	if err != nil {
		c.w.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "credential": json.RawMessage(cred), "name": name})
	return c.call(http.MethodPost, registerCompletePath, string(body))
}

// passkeyStepUp asks for a step-up challenge and confirms it with authenticator a.
func (c *client) passkeyStepUp(a *virtualauthn.Authenticator) (*httpservertest.Response, string) {
	c.w.t.Helper()
	m := c.w.want(c.call(http.MethodPost, challengePath, ""), http.StatusOK, "")
	cred, err := a.Get(options(c.w.t, m, "passkey_options"))
	if err != nil {
		c.w.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "credential": json.RawMessage(cred)})
	return c.call(http.MethodPost, stepUpPasskeyPath, string(body)), string(body)
}

// passkeys returns the number of passkeys of user (active only when active is true).
func (w *sworld) passkeys(user ids.ID, active bool) int {
	w.t.Helper()
	q := `SELECT count(*) FROM passkey_credentials WHERE user_id = $1`
	if active {
		q += ` AND revoked_at IS NULL`
	}
	return w.count(q, user.String())
}

func caseEnrollmentWithPasskey(t *testing.T, w *sworld) {
	v, _ := bootstraps.Load(w)
	res := v.(app.BootstrapResult)
	a := authenticator()
	id, opts := w.enrollBegin(res.Code.Code)
	cred, err := a.Create(opts)
	if err != nil {
		t.Fatal(err)
	}
	p := w.phoneKeys()
	m := w.want(w.enrollComplete(p, id, cred), http.StatusCreated, "")
	dev, sess := m["device"].(map[string]any), m["session"].(map[string]any)
	if dev["state"] != "active" || sess["auth_method"] != "enrollment_code" || sess["user_id"] != res.OwnerID.String() {
		t.Fatalf("device %v session %v", dev, sess)
	}
	// The code works after the failed attempt; begin counted nothing.
	if n := w.attempts(res.Code.ID); n != 1 {
		t.Fatalf("attempts %d after success", n)
	}
	alexes.Store(w, &alexState{c: w.clientOf(p, m), p: p, first: a})
	if n := w.passkeys(res.OwnerID, true); n != 1 {
		t.Fatalf("%d passkeys", n)
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_by_device_id = $2`, res.Code.ID.String(), p.id.String()); n != 1 {
		t.Fatal("code not used by the device")
	}
	var name string
	w.scalar(&name, `SELECT name FROM passkey_credentials WHERE user_id = $1`, res.OwnerID.String())
	if name != "Laptop" {
		t.Fatalf("name %q", name)
	}
	// The same completion again, and a new begin with the used code: 401.
	w.want(w.enrollComplete(w.phoneKeys(), id, cred), http.StatusUnauthorized, "unauthenticated")
	w.want(w.anonPost(enrollBeginPath, map[string]any{"enrollment_code": res.Code.Code}, nil), http.StatusUnauthorized, "unauthenticated")
	// The stored ceremony row never holds the challenge value.
	var o map[string]any
	_ = json.Unmarshal(opts, &o)
	challenge, _ := o["challenge"].(string)
	if n := w.count(`SELECT count(*) FROM auth_challenges WHERE webauthn_session::text LIKE '%' || $1 || '%' OR encode(challenge_hash, 'base64') LIKE '%' || $1 || '%'`,
		challenge); n != 0 || challenge == "" {
		t.Fatalf("challenge %q stored in %d rows", challenge, n)
	}
}

// bootstraps holds per world the bootstrap of Alex, which the enrollment cases share.
var bootstraps sync.Map

func caseEnrollmentFailedVerificationCountsAttempt(t *testing.T, w *sworld) {
	res := w.bootstrap("alex")
	bootstraps.Store(w, res)
	a := authenticator()
	a.Origin = "https://evil.example.org"
	id, opts := w.enrollBegin(res.Code.Code)
	cred, _ := a.Create(opts)
	w.want(w.enrollComplete(w.phoneKeys(), id, cred), http.StatusUnauthorized, "unauthenticated")
	if n := w.attempts(res.Code.ID); n != 1 {
		t.Fatalf("attempts %d, want 1", n)
	}
	if n := w.passkeys(res.OwnerID, false); n != 0 {
		t.Fatalf("%d passkeys after a failed enrollment", n)
	}
}

// alexes holds per world the enrolled phone of Alex and its first authenticator, so the login
// cases share one bootstrap.
var alexes sync.Map

// alexState is the enrolled Alex of a world.
type alexState struct {
	c     *client
	p     *newPhone
	first *virtualauthn.Authenticator
}

// enrolled returns the client and phone of Alex and a passkey of its own after
// caseEnrollmentWithPasskey enrolled the first one: each call adds one more passkey through
// registration with a passkey step-up, so every case turns the knobs of its own authenticator.
func (w *sworld) enrolled() (*client, *newPhone, *virtualauthn.Authenticator) {
	w.t.Helper()
	if v, ok := alexes.Load(w); ok {
		st := v.(*alexState)
		st.c.rotate()
		r, _ := st.c.passkeyStepUp(st.first)
		w.want(r, http.StatusOK, "")
		a := authenticator()
		w.want(st.c.register(a, "Laptop"), http.StatusCreated, "")
		return st.c, st.p, a
	}
	w.t.Fatal("enrolled before caseEnrollmentWithPasskey")
	return nil, nil, nil
}

// signCount returns sign_count of the passkey of authenticator a.
func (w *sworld) signCount(a *virtualauthn.Authenticator) int64 {
	w.t.Helper()
	var n int64
	w.scalar(&n, `SELECT sign_count FROM passkey_credentials WHERE credential_id = $1`, a.CredentialID())
	return n
}

// run runs the cases in order on w, each as a subtest that reports through its own t.
func (w *sworld) run(t *testing.T, cases []namedCase) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w.t = t
			c.fn(t, w)
		})
	}
}

// namedCase is one case of a shared world, named like the test of the brief.
type namedCase struct {
	name string
	fn   func(t *testing.T, w *sworld)
}

// TestPasskeyLogins runs the enrollment and login cases of brief M1-40 on one world with one
// bootstrap (TestPasskeyLoginExpiredChallenge last, because it moves the clock).
func TestPasskeyLogins(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.run(t, []namedCase{
		{"TestEnrollmentFailedVerificationCountsAttempt", caseEnrollmentFailedVerificationCountsAttempt},
		{"TestEnrollmentWithPasskey", caseEnrollmentWithPasskey},
		{"TestPasskeyLoginWithName", casePasskeyLoginWithName},
		{"TestPasskeyLoginDiscoverable", casePasskeyLoginDiscoverable},
		{"TestPasskeyLoginRequiresUserVerification", casePasskeyLoginRequiresUserVerification},
		{"TestPasskeyLoginWrongOriginOrRPID", casePasskeyLoginWrongOriginOrRPID},
		{"TestPasskeyAssertionReplayRejected", casePasskeyAssertionReplayRejected},
		{"TestPasskeyLoginDecoy", casePasskeyLoginDecoy},
		{"TestSignCountRegressionAudited", caseSignCountRegressionAudited},
		{"TestCredentialOverviewListsPasskeys", caseCredentialOverviewListsPasskeys},
		{"TestPasskeyLoginRevokedCredential", casePasskeyLoginRevokedCredential},
		{"TestPasskeyLoginExpiredChallenge", casePasskeyLoginExpiredChallenge},
	})
}

// TestPasskeyRegistrations runs the registration, revocation and step-up cases of brief M1-40 on
// one world (TestRevokePasskey near the end, because it moves the clock; then
// TestNoCeremonyStateInMemory bootstraps and adds a second server instance).
func TestPasskeyRegistrations(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.run(t, []namedCase{
		{"TestPasskeyLoginOnlyWithLocalIdentity", casePasskeyLoginOnlyWithLocalIdentity},
		{"TestPasskeyRegisterStepUpOnlyWhenOneExists", casePasskeyRegisterStepUpOnlyWhenOneExists},
		{"TestPasskeyRegisterBoundToSession", casePasskeyRegisterBoundToSession},
		{"TestPasskeyRegisterDuplicate", casePasskeyRegisterDuplicate},
		{"TestPasskeyAlgorithms", casePasskeyAlgorithms},
		{"TestStepUpWithPasskey", caseStepUpWithPasskey},
		{"TestUnlinkCountsPasskeysOnlyWithLocalIdentity", caseUnlinkCountsPasskeysOnlyWithLocalIdentity},
		{"TestRevokePasskey", caseRevokePasskey},
		{"TestNoCeremonyStateInMemory", caseNoCeremonyStateInMemory},
	})
}

func casePasskeyLoginWithName(t *testing.T, w *sworld) {
	_, first, a := w.enrolled()
	p := w.phoneKeys()
	m := w.want(w.passkeyLogin("Alex", a, p), http.StatusCreated, "")
	if m["session"].(map[string]any)["auth_method"] != "passkey" || m["device"].(map[string]any)["state"] != "pending" {
		t.Fatalf("login %v", m)
	}
	// The known active device stays active.
	m = w.want(w.passkeyLogin("alex", a, first), http.StatusCreated, "")
	if m["device"].(map[string]any)["state"] != "active" {
		t.Fatalf("known device %v", m["device"])
	}
	if n := w.signCount(a); n != 2 {
		t.Fatalf("sign_count %d, want 2", n)
	}
}

func casePasskeyLoginDiscoverable(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	a.Discoverable = true
	id, opts := w.loginBegin("")
	var o map[string]any
	_ = json.Unmarshal(opts, &o)
	if allow, ok := o["allowCredentials"].([]any); ok && len(allow) > 0 {
		t.Fatalf("discoverable options name credentials: %v", allow)
	}
	cred, _ := a.Get(opts)
	m := w.want(w.loginComplete(w.phoneKeys(), id, cred), http.StatusCreated, "")
	if m["session"].(map[string]any)["auth_method"] != "passkey" || m["device"].(map[string]any)["state"] != "pending" {
		t.Fatalf("login %v", m)
	}
}

func casePasskeyLoginRequiresUserVerification(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	a.NoUserVerification = true
	w.want(w.passkeyLogin("alex", a, w.phoneKeys()), http.StatusUnauthorized, "unauthenticated")
}

func casePasskeyLoginWrongOriginOrRPID(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	a.Origin = "https://chat.example.org"
	w.want(w.passkeyLogin("alex", a, w.phoneKeys()), http.StatusUnauthorized, "unauthenticated")
	a.Origin, a.RPID = httpservertest.PublicURL, "example.org"
	w.want(w.passkeyLogin("alex", a, w.phoneKeys()), http.StatusUnauthorized, "unauthenticated")
	// Every refusal has the same body.
	a.RPID = ""
	a.NoUserVerification = true
	r := w.passkeyLogin("alex", a, w.phoneKeys())
	a.NoUserVerification = false
	unknown := w.anonPost(passkeyCompletePath, map[string]any{"challenge_id": ids.New().String(), "credential": map[string]any{}, "device": w.phoneKeys().device()},
		w.phoneKeys().key)
	if withoutCorrelation(t, r) != withoutCorrelation(t, unknown) {
		t.Fatalf("bodies differ: %s / %s", r.Body, unknown.Body)
	}
}

func casePasskeyAssertionReplayRejected(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	id, opts := w.loginBegin("alex")
	cred, _ := a.Get(opts)
	p := w.phoneKeys()
	w.want(w.loginComplete(p, id, cred), http.StatusCreated, "")
	w.want(w.loginComplete(w.phoneKeys(), id, cred), http.StatusUnauthorized, "unauthenticated")
	// A concurrent pair of the same completion: exactly one 201.
	id, opts = w.loginBegin("alex")
	cred, _ = a.Get(opts)
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = w.loginComplete(w.phoneKeys(), id, cred).StatusCode
		}(i)
	}
	wg.Wait()
	one := codes[0] == http.StatusCreated && codes[1] == http.StatusUnauthorized
	other := codes[1] == http.StatusCreated && codes[0] == http.StatusUnauthorized
	if !one && !other {
		t.Fatalf("concurrent completions %v", codes)
	}
}

func casePasskeyLoginExpiredChallenge(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	id, opts := w.loginBegin("alex")
	cred, _ := a.Get(opts)
	w.clk.Advance(sessCfg.LoginFlowTTL)
	w.want(w.loginComplete(w.phoneKeys(), id, cred), http.StatusUnauthorized, "unauthenticated")
}

func casePasskeyLoginRevokedCredential(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	w.sql(`UPDATE passkey_credentials SET revoked_at = now() WHERE credential_id = $1`, a.CredentialID())
	// The discoverable path, because the name now gets a decoy.
	a.Discoverable = true
	w.want(w.passkeyLogin("", a, w.phoneKeys()), http.StatusUnauthorized, "unauthenticated")
}

func casePasskeyLoginOnlyWithLocalIdentity(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	a := authenticator()
	w.want(sam.register(a, "Laptop"), http.StatusCreated, "")
	a.Discoverable = true
	w.want(w.passkeyLogin("", a, w.phoneKeys()), http.StatusUnauthorized, "unauthenticated")
	// The same passkey serves the step-up of the person.
	r, _ := sam.passkeyStepUp(a)
	w.want(r, http.StatusOK, "")
}

func casePasskeyLoginDecoy(t *testing.T, w *sworld) {
	w.enrolled()
	allow := func(name string) []any {
		_, opts := w.loginBegin(name)
		var o map[string]any
		_ = json.Unmarshal(opts, &o)
		list, _ := o["allowCredentials"].([]any)
		return list
	}
	robin := allow("robin")
	if len(robin) != 1 {
		t.Fatalf("decoy list %v", robin)
	}
	id := robin[0].(map[string]any)["id"].(string)
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) != 32 {
		t.Fatalf("decoy id %q (%d bytes)", id, len(raw))
	}
	if again := allow("Robin"); again[0].(map[string]any)["id"] != id {
		t.Fatal("decoy not stable for the same name")
	}
	if other := allow("kim"); other[0].(map[string]any)["id"] == id {
		t.Fatal("decoy equal for another name")
	}
	real := allow("alex")
	if len(real) == 0 {
		t.Fatal("no real credentials")
	}
	for _, c := range real {
		if c.(map[string]any)["id"] == id {
			t.Fatalf("real list %v holds the decoy", real)
		}
	}
}

func caseSignCountRegressionAudited(t *testing.T, w *sworld) {
	_, _, a := w.enrolled()
	w.want(w.passkeyLogin("alex", a, w.phoneKeys()), http.StatusCreated, "")
	a.FreezeCounter = true // the counter stays at 1, equal to the stored one
	w.want(w.passkeyLogin("alex", a, w.phoneKeys()), http.StatusCreated, "")
	var pid string
	w.scalar(&pid, `SELECT id::text FROM passkey_credentials WHERE credential_id = $1`, a.CredentialID())
	var found []sauditRow
	for _, e := range w.audits("auth.login.complete") {
		if e.Details["sign_count_regression"] == true && e.Details["passkey_id"] == pid {
			found = append(found, e)
		}
	}
	if len(found) != 1 || found[0].Decision != "allow" || found[0].Details["stored"] != float64(1) || found[0].Details["received"] != float64(1) ||
		found[0].Details["passkey_id"] == nil {
		t.Fatalf("regression entries %+v", found)
	}
	if n := w.signCount(a); n != 1 {
		t.Fatalf("sign_count %d, want 1", n)
	}
}

func casePasskeyRegisterStepUpOnlyWhenOneExists(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	m := w.want(sam.register(authenticator(), "Laptop"), http.StatusCreated, "")
	if m["name"] != "Laptop" || m["id"] == nil {
		t.Fatalf("passkey %v", m)
	}
	w.want(sam.call(http.MethodPost, registerBeginPath, ""), http.StatusUnauthorized, "step_up_required")
	sam.stepUp()
	w.want(sam.register(authenticator(), "Phone"), http.StatusCreated, "")
	if n := w.passkeys(sam.user, true); n != 2 {
		t.Fatalf("%d passkeys", n)
	}
	allows := 0
	for _, e := range w.audits("passkey.register") {
		if e.Decision == "allow" && e.Details["passkey_id"] != nil && e.ResourceID == sam.user.String() {
			allows++
		}
	}
	if allows != 2 {
		t.Fatalf("%d allow entries of passkey.register, want 2", allows)
	}
	pat := sam.personalToken()
	w.wantReason(w.withToken(pat, http.MethodPost, registerBeginPath, ""), http.StatusForbidden, "forbidden", "step_up_not_possible")
}

func casePasskeyRegisterBoundToSession(t *testing.T, w *sworld) {
	user := w.person(w.handle("sam"), false)
	one, two := w.phone(user), w.phone(user)
	m := w.want(one.call(http.MethodPost, registerBeginPath, ""), http.StatusOK, "")
	cred, _ := authenticator().Create(options(t, m, "public_key"))
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "credential": json.RawMessage(cred), "name": "Laptop"})
	w.want(two.call(http.MethodPost, registerCompletePath, string(body)), http.StatusBadRequest, "validation_failed")
	// The challenge stays usable in its own session.
	w.want(one.call(http.MethodPost, registerCompletePath, string(body)), http.StatusCreated, "")
	// Used once: with the step-up a second passkey needs, the same completion is refused.
	one.stepUp()
	w.want(one.call(http.MethodPost, registerCompletePath, string(body)), http.StatusBadRequest, "validation_failed")
}

func casePasskeyRegisterDuplicate(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	a := authenticator()
	w.want(sam.register(a, "Laptop"), http.StatusCreated, "")
	sam.stepUp()
	a.Reuse = true
	w.want(sam.register(a, "Again"), http.StatusConflict, "conflict")
}

func casePasskeyAlgorithms(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	for i, alg := range []virtualauthn.Alg{virtualauthn.ES256, virtualauthn.EdDSA, virtualauthn.RS256} {
		if i > 0 {
			sam.stepUp()
		}
		a := virtualauthn.New(httpservertest.PublicURL, alg)
		w.want(sam.register(a, "Key"), http.StatusCreated, "")
		r, _ := sam.passkeyStepUp(a)
		w.want(r, http.StatusOK, "")
	}
	sam.stepUp()
	w.want(sam.register(virtualauthn.New(httpservertest.PublicURL, virtualauthn.ES384), "Key"), http.StatusBadRequest, "validation_failed")
}

func caseRevokePasskey(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	w.want(sam.register(authenticator(), "One"), http.StatusCreated, "")
	sam.stepUp()
	w.want(sam.register(authenticator(), "Two"), http.StatusCreated, "")
	var one, two string
	w.scalar(&one, `SELECT id::text FROM passkey_credentials WHERE name = 'One'`)
	w.scalar(&two, `SELECT id::text FROM passkey_credentials WHERE name = 'Two'`)
	path := func(user ids.ID, id string) string { return "/api/v1/users/" + user.String() + "/passkeys/" + id }
	// Self without a fresh step-up: 401; with one: 204.
	w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
	w.want(sam.call(http.MethodDelete, path(sam.user, one), ""), http.StatusUnauthorized, "step_up_required")
	sam.stepUp()
	w.want(sam.call(http.MethodDelete, path(sam.user, one), ""), http.StatusNoContent, "")
	w.want(sam.call(http.MethodDelete, path(sam.user, one), ""), http.StatusNoContent, "")
	// Another person: 404; the owner: 204.
	kim := w.phone(w.person(w.handle("kim"), false))
	kim.stepUp()
	w.want(kim.call(http.MethodDelete, path(sam.user, two), ""), http.StatusNotFound, "not_found")
	w.want(kim.call(http.MethodDelete, path(kim.user, two), ""), http.StatusNotFound, "not_found")
	owner := w.ownerWithStepUp()
	w.want(owner.call(http.MethodDelete, path(sam.user, two), ""), http.StatusNoContent, "")
	w.want(owner.call(http.MethodDelete, path(sam.user, ids.New().String()), ""), http.StatusNotFound, "not_found")
	if n := w.passkeys(sam.user, true); n != 0 {
		t.Fatalf("%d active passkeys", n)
	}
	if e := w.audits("passkey.revoke"); len(e) < 3 { // this case is the only one that revokes
		t.Fatalf("audit %+v", e)
	}
}

func caseStepUpWithPasskey(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	a := authenticator()
	w.want(sam.register(a, "Laptop"), http.StatusCreated, "")
	m := w.want(sam.call(http.MethodPost, challengePath, ""), http.StatusOK, "")
	opts, _ := m["passkey_options"].(map[string]any)
	if opts == nil || opts["challenge"] != m["challenge"] || opts["userVerification"] != "required" {
		t.Fatalf("passkey_options %v of challenge %v", opts, m["challenge"])
	}
	r, body := sam.passkeyStepUp(a)
	res := w.want(r, http.StatusOK, "")
	if res["step_up_at"] == nil {
		t.Fatalf("result %v", res)
	}
	var method string
	w.scalar(&method, `SELECT step_up_method FROM auth_sessions WHERE id = $1`, sam.session.String())
	if method != identity.StepUpMethodPasskey {
		t.Fatalf("step_up_method %q", method)
	}
	// The same challenge again: refused, not counted.
	w.want(sam.call(http.MethodPost, stepUpPasskeyPath, body), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(sam.session); n != 0 {
		t.Fatalf("%d failed challenges after a reused one", n)
	}
	// Another person's passkey fails and counts.
	kim := w.phone(w.person(w.handle("kim"), false))
	other := authenticator()
	w.want(kim.register(other, "Kim"), http.StatusCreated, "")
	m = w.want(sam.call(http.MethodPost, challengePath, ""), http.StatusOK, "")
	cred, _ := other.Get(options(t, m, "passkey_options"))
	b, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "credential": json.RawMessage(cred)})
	w.want(sam.call(http.MethodPost, stepUpPasskeyPath, string(b)), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(sam.session); n != 1 {
		t.Fatalf("%d failed challenges, want 1", n)
	}
	// A cli session: 403 before the assertion is looked at (ID-T29).
	cli := w.loginDevice(sam.user, identity.DeviceStateActive, identity.PlatformCLI, false)
	w.wantReason(cli.call(http.MethodPost, stepUpPasskeyPath, string(b)), http.StatusForbidden, "forbidden", "step_up_not_possible")
}

func caseCredentialOverviewListsPasskeys(t *testing.T, w *sworld) {
	c, _, _ := w.enrolled()
	m := w.want(c.call(http.MethodGet, myCredentialsPath, ""), http.StatusOK, "")
	list, _ := m["passkeys"].([]any)
	if len(list) == 0 || len(list) != w.passkeys(c.user, true) || list[0].(map[string]any)["name"] != "Laptop" {
		t.Fatalf("passkeys %v", m["passkeys"])
	}
}

// caseNoCeremonyStateInMemory begins on one server instance and completes on a second one with
// the same database and secrets.
func caseNoCeremonyStateInMemory(t *testing.T, one *sworld) {
	res := one.bootstrap("robin")
	two := newSessionWorldOn(t, one.db, one.clk, one.instID, one.dir)
	id, opts := one.enrollBegin(res.Code.Code)
	a := authenticator()
	cred, _ := a.Create(opts)
	two.want(two.enrollComplete(two.phoneKeys(), id, cred), http.StatusCreated, "")
	id, opts = two.loginBegin("robin")
	cred, _ = a.Get(opts)
	one.want(one.loginComplete(one.phoneKeys(), id, cred), http.StatusCreated, "")
}

// caseUnlinkCountsPasskeysOnlyWithLocalIdentity: a passkey logs in only with a local identity, so
// it keeps no way in for a person whose last identity is OIDC (docs/DECISIONS.md M1-40/10).
func caseUnlinkCountsPasskeysOnlyWithLocalIdentity(t *testing.T, w *sworld) {
	owner := w.admin()
	prov := owner.createProvider("oidc", "Company SSO", oidcConfig)
	samID := w.person(w.handle("sam"), false)
	sam := w.phone(samID)
	w.want(sam.register(authenticator(), "Laptop"), http.StatusCreated, "")
	only := w.link(samID, prov, "https://login.example.com", "sub-"+samID.String())
	w.noDevices(samID)
	p := w.want(owner.call(http.MethodDelete, identityPath(samID, only.ID), ""), http.StatusConflict, "conflict")
	if p["reason"] != "no_login_method_left" {
		t.Fatalf("problem %v", p)
	}
}
