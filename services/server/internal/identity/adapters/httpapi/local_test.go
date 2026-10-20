// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/passwordhash"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// The paths of the local account operations (M1-39).
const (
	passwordLoginPath = "/api/v1/auth/password/login" //nolint:gosec // G101: a path, no credential
	localCredPath     = "/api/v1/me/local-credential" //nolint:gosec // G101: a path, no credential
)

// testPassword is the password of the local accounts in these tests.
const testPassword = "EXAMPLE-correct-horse-1"

// notFoundTimingTolerance is the standard of authz.not_found_timing_tolerance
// (docs/operations/configuration.md), the allowed difference of the medians in ID-T21.
const notFoundTimingTolerance = 20 * time.Millisecond

// countingHasher counts the verifications of the hasher it wraps.
type countingHasher struct {
	app.PasswordHasher
	n atomic.Int64
}

// Verify counts one computation.
func (c *countingHasher) Verify(phc, password string) (bool, bool, error) {
	c.n.Add(1)
	return c.PasswordHasher.Verify(phc, password)
}

// VerifyDummy counts one computation.
func (c *countingHasher) VerifyDummy(password string) {
	c.n.Add(1)
	c.PasswordHasher.VerifyDummy(password)
}

// constantHasher is a hasher of constant, tiny cost for the timing test under the race detector:
// the stored value is a SHA-256 of the password, and every verification hashes once.
type constantHasher struct{}

// Hash implements app.PasswordHasher.
func (constantHasher) Hash(password string) (string, error) {
	sum := sha256.Sum256([]byte(password))
	return "constant$" + hex.EncodeToString(sum[:]), nil
}

// Verify implements app.PasswordHasher.
func (h constantHasher) Verify(phc, password string) (bool, bool, error) {
	want, _ := h.Hash(password)
	return subtle.ConstantTimeCompare([]byte(want), []byte(phc)) == 1, false, nil
}

// VerifyDummy implements app.PasswordHasher.
func (h constantHasher) VerifyDummy(password string) { _, _, _ = h.Verify("constant$", password) }

// newCountingHasher returns a counting hasher with the floor parameters.
func newCountingHasher(t *testing.T) *countingHasher {
	t.Helper()
	h, err := passwordhash.New(passwordhash.Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1})
	if err != nil {
		t.Fatal(err)
	}
	return &countingHasher{PasswordHasher: h}
}

// localAccount is a person with an active local identity whose login name is name, a session on
// an active phone with a step_up key (no step-up yet) and, for alex, the unused first-login code.
type localAccount struct {
	user  ids.ID
	name  string
	code  string
	phone *stepClient
}

// newLocalWorld builds a session world with o and bootstraps the owner alex, which creates the
// local provider.
func newLocalWorld(t *testing.T, o worldOptions) (*sworld, localAccount) {
	t.Helper()
	w := newSessionWorldWith(t, pgtest.New(t), clock.NewFake(sessStart), ids.New(), keyDir(t), o)
	res := w.bootstrap("alex")
	return w, localAccount{user: res.OwnerID, name: "alex", code: res.Code.Code, phone: w.phone(res.OwnerID)}
}

// localPerson creates an active person handle with an identity at the local provider and logs it
// in on a phone.
func (w *sworld) localPerson(handle string) localAccount {
	w.t.Helper()
	id := w.person(handle, false)
	w.sql(`INSERT INTO user_identities (id, user_id, provider_id, subject)
		SELECT $1, $2, id, $3 FROM identity_providers WHERE kind = 'local'`, ids.New().String(), id.String(), handle)
	return localAccount{user: id, name: handle, phone: w.phone(id)}
}

// cred scans column col of the local credentials of user into dst.
func (w *sworld) cred(user ids.ID, col string, dst any) {
	w.t.Helper()
	w.scalar(dst, `SELECT c.`+col+` FROM local_credentials c JOIN user_identities i ON i.id = c.identity_id WHERE i.user_id = $1`, user.String())
}

// credRows counts the local credentials of user.
func (w *sworld) credRows(user ids.ID) int {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM local_credentials c JOIN user_identities i ON i.id = c.identity_id WHERE i.user_id = $1`, user.String())
}

// setLocal sends setMyLocalCredential with body.
func (c *client) setLocal(body string) *httpservertest.Response {
	c.w.t.Helper()
	return c.call(http.MethodPut, localCredPath, body)
}

// totpSecret returns the secret of an otpauth URI.
func totpSecret(t *testing.T, uri string) []byte {
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

// code is the TOTP code of secret at the clock of the world.
func (w *sworld) code(secret []byte) string {
	return domain.TOTPCode(secret, domain.TOTPStep(w.clk.Now()), domain.TOTPDigits)
}

// establish sets the password, enrolls and confirms TOTP through the API without step-up and
// returns the secret; the clock moves one step on, so the confirmation code is not the next code.
func (w *sworld) establish(c *client) []byte {
	w.t.Helper()
	m := w.want(c.setLocal(`{"password":"`+testPassword+`","totp_enroll":true}`), http.StatusOK, "")
	if m["password_set"] != true || m["totp_enabled"] != false {
		w.t.Fatalf("enroll: %v", m)
	}
	secret := totpSecret(w.t, m["totp_secret_uri"].(string))
	m = w.want(c.setLocal(`{"totp_code":"`+w.code(secret)+`"}`), http.StatusOK, "")
	if m["totp_enabled"] != true || m["totp_secret_uri"] != nil {
		w.t.Fatalf("confirm: %v", m)
	}
	w.clk.Advance(domain.TOTPPeriod)
	return secret
}

// loginBody is the PasswordLoginRequest of p.
func (p *newPhone) loginBody(name, password, code string) string {
	b, _ := json.Marshal(map[string]any{
		"login_name": name, "password": password, "totp_code": code,
		"device": map[string]any{"device_id": p.id.String(), "name": "Alex Phone", "platform": "ios", "public_jwk": json.RawMessage(p.pub.JWK), "alg": "ES256"},
	})
	return string(b)
}

// passwordLogin sends loginWithPassword from p.
func (w *sworld) passwordLogin(p *newPhone, name, password, code string) *httpservertest.Response {
	w.t.Helper()
	c := &client{w: w, key: p.key}
	proof := c.proof(p.key, http.MethodPost, passwordLoginPath, "")
	return w.h.Do(w.h.NewRequest(http.MethodPost, passwordLoginPath, p.loginBody(name, password, code), "DPoP", proof))
}

// unauthenticatedBody is the body of a rejected password login: the answer to an unknown name.
func (w *sworld) unauthenticatedBody() string {
	w.t.Helper()
	r := w.passwordLogin(w.phoneKeys(), "nobody", testPassword, "000000")
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	return withoutCorrelation(w.t, r)
}

// wantRejected checks the one 401 of every rejected password login.
func (w *sworld) wantRejected(r *httpservertest.Response, body string) {
	w.t.Helper()
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if got := withoutCorrelation(w.t, r); got != body {
		w.t.Fatalf("body %s, want %s", got, body)
	}
}

// localCase is one case of TestLocalAccounts on the shared world.
type localCase struct {
	name string
	run  func(t *testing.T, w *sworld, alex localAccount, h *countingHasher)
}

// TestLocalAccounts runs the integration cases of M1-39 (brief "Tests") one after the other on
// one world with a counting hasher: a world costs seconds to build and a case tenths, and one
// world per case passed the default test timeout on a loaded machine (as M1-44/13). Each case
// works with its own person; the cases that switch the password path off or need other limits
// have their own tests.
func TestLocalAccounts(t *testing.T) {
	t.Parallel()
	h := newCountingHasher(t)
	w, alex := newLocalWorld(t, worldOptions{hasher: h})
	for _, c := range []localCase{
		{"TestPasswordLoginHappyPath", casePasswordLoginHappyPath},
		{"TestPasswordLoginKnownDeviceStaysActive", casePasswordLoginKnownDeviceStaysActive},
		{"TestLockedAccountStillHashes", caseLockedAccountStillHashes},
		{"TestLockoutOnlyPasswordPath", caseLockoutOnlyPasswordPath},
		{"TestPasswordLoginFailureWindow", casePasswordLoginFailureWindow},
		{"TestTOTPSecretEncryptedAtRest", caseTOTPSecretEncryptedAtRest},
		{"TestTOTPEnrolmentNeedsConfirmation", caseTOTPEnrolmentNeedsConfirmation},
		{"TestLocalCredentialStepUpOnlyWhenEstablished", caseLocalCredentialStepUpOnlyWhenEstablished},
		{"TestSetLocalCredentialValidation", caseSetLocalCredentialValidation},
		{"TestPasswordAndTOTPNeverLogged", casePasswordAndTOTPNeverLogged},
		{"TestCredentialOverviewFlags", caseCredentialOverviewFlags},
		// Last: it disables the local provider of the world.
		{"TestPasswordLoginWithoutLocalProvider", casePasswordLoginWithoutLocalProvider},
	} {
		t.Run(c.name, func(t *testing.T) {
			parent := w.t
			w.t = t
			defer func() { w.t = parent }()
			c.run(t, w, alex, h)
		})
	}
}

func casePasswordLoginHappyPath(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("robin")
	secret := w.establish(a.phone.client)
	p := w.phoneKeys()
	code := w.code(secret)
	// The login name is trimmed and lowercased.
	m := w.want(w.passwordLogin(p, "  Robin ", testPassword, code), http.StatusCreated, "")
	dev := m["device"].(map[string]any)
	sess := m["session"].(map[string]any)
	if dev["state"] != "pending" || dev["id"] != p.id.String() || sess["auth_method"] != "password_totp" || sess["user_id"] != a.user.String() {
		t.Fatalf("device %v session %v", dev, sess)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions s JOIN user_identities i ON i.id = s.identity_id
		WHERE s.device_id = $1 AND i.subject = 'robin' AND s.auth_method = 'password_totp'`, p.id.String()); n != 1 {
		t.Fatal("session without the local identity")
	}
	var step int64
	w.cred(a.user, "totp_last_step", &step)
	if step != domain.TOTPStep(w.clk.Now()) {
		t.Fatalf("totp_last_step %d", step)
	}
	var allow []sauditRow
	for _, e := range w.audits("auth.login.complete") {
		if e.Decision == "allow" && e.Details["session_id"] == sess["id"] {
			allow = append(allow, e)
		}
	}
	if len(allow) != 1 || allow[0].Details["auth_method"] != "password_totp" || allow[0].Details["device_state"] != "pending" {
		t.Fatalf("audit %+v", allow)
	}
	// ID-T20: the same code a second time in the same step is rejected.
	w.wantRejected(w.passwordLogin(w.phoneKeys(), "robin", testPassword, code), w.unauthenticatedBody())
}

func casePasswordLoginKnownDeviceStaysActive(t *testing.T, w *sworld, alex localAccount, _ *countingHasher) {
	secret := w.establish(alex.phone.client)
	// The phone of the enrollment code is active.
	p := w.phoneKeys()
	w.want(w.redeem(p, alex.code, nil), http.StatusCreated, "")
	m := w.want(w.passwordLogin(p, "alex", testPassword, w.code(secret)), http.StatusCreated, "")
	if dev := m["device"].(map[string]any); dev["state"] != "active" || dev["id"] != p.id.String() {
		t.Fatalf("device %v", dev)
	}
	sid := m["session"].(map[string]any)["id"]
	var seen int
	for _, e := range w.audits("auth.login.complete") {
		if e.Details["session_id"] != sid {
			continue
		}
		seen++
		if e.Details["auth_method"] != "password_totp" || e.Details["new_device"] != false {
			t.Fatalf("audit %+v", e)
		}
	}
	if seen != 1 {
		t.Fatalf("%d allow entries of the session, want 1", seen)
	}
}

func caseLockedAccountStillHashes(t *testing.T, w *sworld, _ localAccount, h *countingHasher) {
	a := w.localPerson("sam")
	secret := w.establish(a.phone.client)
	want := w.unauthenticatedBody()
	for range sessCfg.LoginFailLimit {
		w.wantRejected(w.passwordLogin(w.phoneKeys(), "sam", "EXAMPLE-wrong-password", w.code(secret)), want)
	}
	var lockedUntil time.Time
	var failed int
	w.cred(a.user, "locked_until", &lockedUntil)
	w.cred(a.user, "failed_count", &failed)
	if !lockedUntil.Equal(w.clk.Now().Add(sessCfg.LoginLockDuration)) || failed != 0 {
		t.Fatalf("locked_until %v failed %d", lockedUntil, failed)
	}
	h.n.Store(0)
	w.wantRejected(w.passwordLogin(w.phoneKeys(), "sam", testPassword, w.code(secret)), want)
	if n := h.n.Load(); n != 1 {
		t.Fatalf("%d Argon2id computations while locked, want 1", n)
	}
	// The lock is one deny entry with {lock: true} on the person.
	var locks int
	for _, e := range w.audits("auth.login.complete") {
		if e.Decision == "deny" && e.Details["lock"] == true && e.ResourceID == a.user.String() {
			locks++
		}
	}
	if locks != 1 {
		t.Fatalf("%d lock entries, want 1", locks)
	}
	// Exactly one computation for an unknown name too.
	h.n.Store(0)
	w.unauthenticatedBody()
	if n := h.n.Load(); n != 1 {
		t.Fatalf("%d computations for an unknown name, want 1", n)
	}
}

func caseLockoutOnlyPasswordPath(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("kim")
	secret := w.establish(a.phone.client)
	var code app.IssuedEnrollmentCode
	err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		code, err = w.id.Enrollment.Issue(ctx, a.user, app.PurposeRecovery, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := w.unauthenticatedBody()
	for range sessCfg.LoginFailLimit {
		w.wantRejected(w.passwordLogin(w.phoneKeys(), "kim", "EXAMPLE-wrong-password", "000000"), want)
	}
	w.clk.Advance(domain.TOTPPeriod)
	w.wantRejected(w.passwordLogin(w.phoneKeys(), "kim", testPassword, w.code(secret)), want)
	// An enrollment code still logs in while the password path is locked (R-ID-10).
	w.want(w.redeem(w.phoneKeys(), code.Code, nil), http.StatusCreated, "")
	// After auth.login_lock_duration the password path is open again.
	w.clk.Advance(sessCfg.LoginLockDuration)
	w.want(w.passwordLogin(w.phoneKeys(), "kim", testPassword, w.code(secret)), http.StatusCreated, "")
	var failed int
	var locked *time.Time
	w.cred(a.user, "failed_count", &failed)
	w.cred(a.user, "locked_until", &locked)
	if failed != 0 || locked != nil {
		t.Fatalf("after success: failed %d locked %v", failed, locked)
	}
}

func casePasswordLoginFailureWindow(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("lee")
	secret := w.establish(a.phone.client)
	want := w.unauthenticatedBody()
	// Failures below the limit, then the window ends: the next failure starts a new window and
	// does not lock.
	for range sessCfg.LoginFailLimit - 1 {
		w.wantRejected(w.passwordLogin(w.phoneKeys(), "lee", "EXAMPLE-wrong-password", "000000"), want)
	}
	w.clk.Advance(sessCfg.LoginFailWindow)
	w.wantRejected(w.passwordLogin(w.phoneKeys(), "lee", "EXAMPLE-wrong-password", "000000"), want)
	var failed int
	w.cred(a.user, "failed_count", &failed)
	if failed != 1 {
		t.Fatalf("failed_count %d, want 1 in the new window", failed)
	}
	// A wrong TOTP code with the right password counts as a failure too.
	w.wantRejected(w.passwordLogin(w.phoneKeys(), "lee", testPassword, "000000"), want)
	w.cred(a.user, "failed_count", &failed)
	if failed != 2 {
		t.Fatalf("failed_count %d after a wrong code", failed)
	}
	w.want(w.passwordLogin(w.phoneKeys(), "lee", testPassword, w.code(secret)), http.StatusCreated, "")
}

func caseTOTPSecretEncryptedAtRest(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("ari")
	secret := w.establish(a.phone.client)
	var ct []byte
	var ref, phc string
	w.cred(a.user, "totp_secret_ciphertext", &ct)
	w.cred(a.user, "totp_key_ref", &ref)
	w.cred(a.user, "password_hash", &phc)
	if bytes.Contains(ct, secret) || len(ct) != 1+12+len(secret)+16 || ct[0] != 0x01 {
		t.Fatalf("ciphertext of %d bytes holds the secret or has the wrong layout", len(ct))
	}
	if ref != sessCfg.TOTPKeyRef {
		t.Fatalf("totp_key_ref %q", ref)
	}
	if !strings.HasPrefix(phc, "$argon2id$v=19$m=19456,t=2,p=1$") || strings.Contains(phc, testPassword) {
		t.Fatal("password hash not in PHC form")
	}
}

func caseTOTPEnrolmentNeedsConfirmation(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("noa")
	m := w.want(a.phone.setLocal(`{"password":"`+testPassword+`","totp_enroll":true}`), http.StatusOK, "")
	uri := m["totp_secret_uri"].(string)
	if !strings.HasPrefix(uri, "otpauth://totp/chat.example.com:noa?") {
		t.Fatalf("uri prefix of %q", strings.SplitN(uri, "?", 2)[0])
	}
	secret := totpSecret(t, uri)
	want := w.unauthenticatedBody()
	// Before the confirmation the password path is not open, also with a valid code.
	w.wantRejected(w.passwordLogin(w.phoneKeys(), "noa", testPassword, w.code(secret)), want)
	// A wrong code does not confirm.
	w.want(a.phone.setLocal(`{"totp_code":"000000"}`), http.StatusBadRequest, "validation_failed")
	w.clk.Advance(domain.TOTPPeriod)
	w.want(a.phone.setLocal(`{"totp_code":"`+w.code(secret)+`"}`), http.StatusOK, "")
	w.clk.Advance(domain.TOTPPeriod)
	w.want(w.passwordLogin(w.phoneKeys(), "noa", testPassword, w.code(secret)), http.StatusCreated, "")
}

func caseLocalCredentialStepUpOnlyWhenEstablished(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("eli")
	// First set-up and confirmation work without step-up.
	w.establish(a.phone.client)
	// Later changes need a fresh step-up.
	w.want(a.phone.setLocal(`{"password":"EXAMPLE-new-password-2"}`), http.StatusUnauthorized, "step_up_required")
	a.phone.stepUp()
	w.want(a.phone.setLocal(`{"password":"EXAMPLE-new-password-2"}`), http.StatusOK, "")
	// A personal token can never set the local credential.
	pat := a.phone.personalToken()
	w.want(w.withToken(pat, http.MethodPut, localCredPath, `{"password":"EXAMPLE-new-password-3"}`), http.StatusForbidden, "forbidden")
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'local_credential.set' AND decision = 'allow' AND resource_id = $1`,
		a.user.String()); n != 3 {
		t.Fatalf("%d allow entries of local_credential.set, want 3", n)
	}
}

func caseSetLocalCredentialValidation(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("val")
	field := func(r *httpservertest.Response, pointer, code string) {
		t.Helper()
		m := w.want(r, http.StatusBadRequest, "validation_failed")
		errs, _ := m["errors"].([]any)
		if len(errs) != 1 || errs[0].(map[string]any)["pointer"] != pointer || errs[0].(map[string]any)["code"] != code {
			t.Fatalf("errors %v, want %s %s", m["errors"], pointer, code)
		}
	}
	field(a.phone.setLocal(`{"password":"EXAMPLE-sh"}`), "/password", "too_short")
	field(a.phone.setLocal(`{"password":"unbelievable"}`), "/password", "common_password")
	field(a.phone.setLocal(`{}`), "", "required")
	field(a.phone.setLocal(`{"totp_enroll":true}`), "/password", "required")
	field(a.phone.setLocal(`{"password":"`+testPassword+`","totp_code":"123456"}`), "/totp_code", "totp_invalid")
	if n := w.credRows(a.user); n != 0 {
		t.Fatal("a rejected request wrote a row")
	}
	// A person without a local identity.
	other := w.phone(w.person("tay", false))
	w.wantReason(other.setLocal(`{"password":"`+testPassword+`"}`), http.StatusConflict, "conflict", "no_local_identity")
}

func casePasswordAndTOTPNeverLogged(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("ray")
	m := w.want(a.phone.setLocal(`{"password":"`+testPassword+`","totp_enroll":true}`), http.StatusOK, "")
	uri := m["totp_secret_uri"].(string)
	secret := totpSecret(t, uri)
	confirm := w.code(secret)
	w.want(a.phone.setLocal(`{"totp_code":"`+confirm+`"}`), http.StatusOK, "")
	w.clk.Advance(domain.TOTPPeriod)
	login := w.code(secret)
	w.want(w.passwordLogin(w.phoneKeys(), "ray", testPassword, login), http.StatusCreated, "")
	w.unauthenticatedBody()
	b32 := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	var details string
	w.scalar(&details, `SELECT coalesce(string_agg(details::text, ' '), '') FROM audit_events`)
	var logs []string
	for _, r := range append(w.logs.Records(), w.h.Logs.Records()...) {
		logs = append(logs, fmt.Sprint(r))
	}
	if len(logs) == 0 || details == "" {
		t.Fatal("nothing captured")
	}
	text := strings.Join(logs, "\n")
	for _, s := range []string{testPassword, b32, uri, string(secret)} {
		if strings.Contains(text, s) || strings.Contains(details, s) {
			t.Errorf("a secret reached the logs or the audit details")
		}
	}
	// The six-digit codes are searched as JSON values only: as plain substrings they could occur
	// by chance in ids and times.
	for _, s := range []string{confirm, login} {
		if strings.Contains(text, `"`+s+`"`) || strings.Contains(text, "="+s+" ") || strings.Contains(details, `"`+s+`"`) {
			t.Errorf("a TOTP code reached the logs or the audit details")
		}
	}
}

func caseCredentialOverviewFlags(t *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("ivy")
	flags := func() (any, any) {
		m := w.want(a.phone.call(http.MethodGet, "/api/v1/me/credentials", ""), http.StatusOK, "")
		return m["local_password_set"], m["totp_enrolled"]
	}
	if p, e := flags(); p != false || e != false {
		t.Fatalf("before: %v %v", p, e)
	}
	m := w.want(a.phone.setLocal(`{"password":"`+testPassword+`","totp_enroll":true}`), http.StatusOK, "")
	if p, e := flags(); p != true || e != false {
		t.Fatalf("enrolled, not confirmed: %v %v", p, e)
	}
	secret := totpSecret(t, m["totp_secret_uri"].(string))
	w.want(a.phone.setLocal(`{"totp_code":"`+w.code(secret)+`"}`), http.StatusOK, "")
	if p, e := flags(); p != true || e != true {
		t.Fatalf("confirmed: %v %v", p, e)
	}
}

func casePasswordLoginWithoutLocalProvider(_ *testing.T, w *sworld, _ localAccount, _ *countingHasher) {
	a := w.localPerson("ola")
	w.establish(a.phone.client)
	// The established path asks for step-up before the identity is looked up.
	a.phone.stepUp()
	w.sql(`UPDATE identity_providers SET enabled = false WHERE kind = 'local'`)
	w.want(w.passwordLogin(w.phoneKeys(), "ola", testPassword, "123456"), http.StatusNotFound, "not_found")
	w.wantReason(a.phone.setLocal(`{"password":"`+testPassword+`"}`), http.StatusConflict, "conflict", "no_local_identity")
}

func TestPasswordDisabledGives404(t *testing.T) {
	t.Parallel()
	w, alex := newLocalWorld(t, worldOptions{cfg: func(c *app.Config) { c.LocalPasswordEnabled = false }})
	w.want(w.passwordLogin(w.phoneKeys(), "alex", testPassword, "123456"), http.StatusNotFound, "not_found")
	w.want(alex.phone.setLocal(`{"password":"`+testPassword+`"}`), http.StatusForbidden, "feature_disabled")
	if n := w.count(`SELECT count(*) FROM local_credentials`); n != 0 {
		t.Fatal("a row was written while the password path is off")
	}
}

// median returns the median of d.
func median(d []time.Duration) time.Duration {
	s := slices.Clone(d)
	slices.Sort(s)
	return s[len(s)/2]
}

// TestIDT21_UnknownNameTimingEqualsWrongPassword (ID-T21): samples of an unknown login name and
// of a known name with a wrong password, taken in pairs back to back (the order alternates),
// give the same status and body, each request computes exactly one hash, and the median of the
// pair differences stays below authz.not_found_timing_tolerance; pairing cancels the drift of a
// loaded machine, which moves both samples of a pair alike. The race detector instruments
// Argon2id and makes one hash take hundreds of milliseconds with a spread far above the
// tolerance, although both groups compute the same hash (TestDummyUsesConfiguredParameters,
// TestStaleParametersCostLikeUnknownNames); under it the test swaps in a hasher of constant cost
// and so measures where an unknown name could still differ, the database work. Run without the
// race detector it measures the real Argon2id. It runs in parallel with the other tests of the
// package: the pairs carry the load of both samples alike.
func TestIDT21_UnknownNameTimingEqualsWrongPassword(t *testing.T) {
	t.Parallel()
	h := newCountingHasher(t)
	if raceEnabled {
		h = &countingHasher{PasswordHasher: constantHasher{}}
	}
	// A limit no sample reaches: every wrong password counts and writes the counter.
	w, alex := newLocalWorld(t, worldOptions{hasher: h, cfg: func(c *app.Config) { c.LoginFailLimit = 1000 }})
	w.establish(alex.phone.client)
	const samples = 31
	sample := func(name, pw string) (time.Duration, string) {
		before := h.n.Load()
		start := time.Now()
		r := w.passwordLogin(w.phoneKeys(), name, pw, "123456")
		took := time.Since(start)
		if n := h.n.Load() - before; n != 1 {
			t.Fatalf("%s computed %d hashes, want 1", name, n)
		}
		w.want(r, http.StatusUnauthorized, "unauthenticated")
		return took, withoutCorrelation(t, r)
	}
	var diffs, unknown, wrong []time.Duration
	for i := range samples {
		var u, k time.Duration
		var ub, kb string
		if i%2 == 0 {
			u, ub = sample("nobody", testPassword)
			k, kb = sample("alex", "EXAMPLE-wrong-password")
		} else {
			k, kb = sample("alex", "EXAMPLE-wrong-password")
			u, ub = sample("nobody", testPassword)
		}
		if ub != kb {
			t.Fatalf("bodies differ:\n%s\n%s", ub, kb)
		}
		diffs, unknown, wrong = append(diffs, k-u), append(unknown, u), append(wrong, k)
	}
	d := median(diffs)
	t.Logf("race detector %v: medians unknown name %v, wrong password %v, median pair difference %v",
		raceEnabled, median(unknown), median(wrong), d)
	// Under the race detector the bound grows (testkit.TimingBound, PL/21).
	tolerance := testkit.TimingBound(notFoundTimingTolerance)
	if d.Abs() >= tolerance {
		t.Fatalf("median pair difference %v reaches the tolerance %v", d, tolerance)
	}
	var failed int
	w.cred(alex.user, "failed_count", &failed)
	if failed != samples {
		t.Fatalf("failed_count %d, want %d", failed, samples)
	}
}
