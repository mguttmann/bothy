// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The fake identity provider of the scenario (oidctest, M1-41).
const (
	idpClientID   = "chat"
	idpSecretFile = "idp/canary_client_secret"
	appRedirect   = "com.example.chat:/oidc"
	groupsClaim   = "groups"
)

// IDP is the fake OIDC provider of a run and the client secret the server reads from its
// environment.
type IDP struct {
	*oidctest.IDP
	Secret string
}

// NewIDP starts the fake provider with a generated client secret.
func NewIDP(t *testing.T, secret string) *IDP {
	t.Helper()
	return &IDP{IDP: oidctest.New(t, oidctest.Options{ClientID: idpClientID, Secret: secret}), Secret: secret}
}

// IDPRunOptions are the RunOptions that let the server reach p: its client secret as the file
// idp/canary_client_secret of the secrets directory, its TLS root and an egress rule for its
// loopback port.
func IDPRunOptions(p *IDP) RunOptions {
	return RunOptions{
		Config: map[string]string{"egress.allowed_hosts": "127.0.0.1:" + p.Port() + ";allow_private", "webhook.allowed_hosts": "hooks.example.com"},
		Harness: func(o *harness.Options) {
			o.PublicHost = "localhost"
			o.OIDCRootCAs = p.RootCAs()
			o.SecretFiles = map[string][]byte{idpSecretFile: []byte(p.Secret)}
		},
	}
}

// auth drives the login and session ways of M1 with canary inputs: enrollment (also a wrong
// code), local password with TOTP (also an unknown name and a wrong password), OIDC with
// canary claims, identity link and OIDC re-authentication, trusted header login, passkeys
// (registration, login, step-up, enrollment), device pairing and approval, refresh rotation with
// a reuse, extension sessions, the session and device lists, and invalid Authorization headers.
func (sc *Scenario) auth(p *IDP) {
	sc.T.Helper()
	sc.enrollment()
	sc.password()
	prov := sc.oidc(p)
	sc.consoleSteps(p, prov)
	sc.trustedHeader()
	sc.passkeys()
	sc.pairing()
	sc.sessions()
	sc.badCredentials()
}

// device returns a new device client of the server with the run's observer.
func (sc *Scenario) device(who string) *harness.Client {
	return sc.Watch(sc.Server.NewDevice(), who)
}

// adopt takes over the token pair of a login answer.
func (sc *Scenario) adopt(c *harness.Client, r harness.Response) bool {
	sc.T.Helper()
	if r.Status != http.StatusOK && r.Status != http.StatusCreated {
		return false
	}
	if err := c.Adopt(r.Body); err != nil {
		sc.T.Errorf("adopt: %v", err)
		return false
	}
	return true
}

// anon sends r as c without access token and reports a status outside want.
func (sc *Scenario) anon(c *harness.Client, r req, want ...int) harness.Response {
	sc.T.Helper()
	resp := c.DoAnonymous(sc.Ctx, harness.Request{Method: r.method, Path: r.path, Body: r.body, Header: r.header})
	if len(want) > 0 && !contains(want, resp.Status) {
		sc.T.Errorf("%s %s (anonymous) = %d %s, want %v", r.method, routeOf(r.method, r.path), resp.Status, clip(resp.Body), want)
	}
	return resp
}

// enrollment: an enrollment code issued for the new person, a wrong code with a canary, the
// passkey enrollment (beginEnrollment, completeEnrollment) and a second code redeemed.
func (sc *Scenario) enrollment() {
	sc.T.Helper()
	o := sc.W.Owner
	user := sc.newUser
	dev := sc.device("enrollee")
	sc.anon(dev, req{method: http.MethodPost, path: "/auth/enroll/begin", body: map[string]any{"enrollment_code": sc.Reg.New(ClassSecret, "", "wrong enrollment code")}},
		http.StatusBadRequest, http.StatusUnauthorized)
	sc.anon(dev, req{method: http.MethodPost, path: "/auth/enroll/redeem", body: map[string]any{
		"enrollment_code": sc.Reg.New(ClassSecret, "", "wrong enrollment code 2"), "device": dev.Registration(sc.Reg.New(ClassContent, "", "device name")),
	}}, http.StatusBadRequest, http.StatusUnauthorized)
	code := str(obj(sc.post(o, "/users/"+user+"/enrollment-codes", nil, created...)), "code")
	begin := sc.anon(dev, req{method: http.MethodPost, path: "/auth/enroll/begin", body: map[string]any{"enrollment_code": code}}, ok...)
	ch := obj(begin)
	cred := sc.webauthnCreate(str(ch, "challenge_id"), begin.Body)
	if cred == nil {
		return
	}
	done := sc.anon(dev, req{method: http.MethodPost, path: "/auth/enroll/complete", body: map[string]any{
		"challenge_id": str(ch, "challenge_id"), "credential": cred, "passkey_name": sc.Reg.New(ClassContent, "", "passkey name (enrollment)"),
		"device": dev.Registration(sc.Reg.New(ClassContent, "", "device name (enrollment)")),
	}}, created...)
	if sc.adopt(dev, done) {
		sc.enrollee = dev
		sc.get(dev, "/me", ok...)
	}
}

// approve activates the pending device of c: the owner approves it with the verification code
// the device reads about itself (ID-T12).
func (sc *Scenario) approve(c *harness.Client) {
	sc.T.Helper()
	dev := obj(sc.get(c, "/devices/"+c.DeviceID, ok...))
	if str(dev, "state") != "pending" {
		return
	}
	vc := str(dev, "verification_code")
	sc.Reg.Register(vc, ClassSecret, "", "verification code")
	sc.post(sc.W.Owner, "/devices/"+c.DeviceID+"/approve", map[string]any{"verification_code": vc}, ok...)
}

// webauthnCreate answers the creation options of a WebAuthnChallenge body with a new virtual
// authenticator and keeps it for logins.
func (sc *Scenario) webauthnCreate(challengeID string, body []byte) map[string]any {
	sc.T.Helper()
	var ch struct {
		PublicKey json.RawMessage `json:"public_key"`
	}
	if json.Unmarshal(body, &ch) != nil || len(ch.PublicKey) == 0 {
		sc.T.Errorf("no public_key in %s", clip(body))
		return nil
	}
	a := virtualauthn.New(sc.Server.PublicURL, virtualauthn.ES256)
	raw, err := a.Create(ch.PublicKey)
	if err != nil {
		sc.T.Errorf("virtual authenticator create: %v", err)
		return nil
	}
	sc.authn = a
	var cred map[string]any
	_ = json.Unmarshal(raw, &cred)
	sc.Reg.Register(string(raw), ClassSecret, "", "passkey credential "+challengeID)
	return cred
}

// webauthnGet answers request options with the kept authenticator.
func (sc *Scenario) webauthnGet(options json.RawMessage) map[string]any {
	sc.T.Helper()
	if sc.authn == nil {
		return nil
	}
	raw, err := sc.authn.Get(options)
	if err != nil {
		sc.T.Errorf("virtual authenticator get: %v", err)
		return nil
	}
	var cred map[string]any
	_ = json.Unmarshal(raw, &cred)
	return cred
}

// totpWait waits until the TOTP step after last, so a code is never used twice (08 "Local
// accounts": one use per step).
func totpWait(last int64) int64 {
	for {
		if s := domain.TOTPStep(time.Now()); s > last {
			return s
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// localAccount is a local credential the scenario set: the password and the TOTP secret.
type localAccount struct {
	password string
	secret   []byte
	step     int64
}

// code returns the TOTP code of the next unused step of a (waiting for it) and registers it.
func (sc *Scenario) code(a *localAccount) string {
	a.step = totpWait(a.step)
	c := domain.TOTPCode(a.secret, a.step, domain.TOTPDigits)
	sc.Reg.Register(c, ClassSecret, "", "TOTP code")
	return c
}

// setLocal sets a password and enrolls TOTP for c (setMyLocalCredential) without confirming it.
func (sc *Scenario) setLocal(c *harness.Client, who string) *localAccount {
	sc.T.Helper()
	pw := sc.Reg.New(ClassSecret, "", "password of "+who) + "-Aa1!"
	set := obj(sc.put(c, "/me/local-credential", map[string]any{"password": pw, "totp_enroll": true}, ok...))
	uri, _ := url.Parse(str(set, "totp_secret_uri"))
	if uri == nil || uri.Query().Get("secret") == "" {
		sc.T.Errorf("no totp_secret_uri for %s", who)
		return nil
	}
	sc.Reg.Register(uri.Query().Get("secret"), ClassSecret, "", "TOTP secret of "+who)
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(uri.Query().Get("secret"))
	if err != nil {
		sc.T.Errorf("totp secret: %v", err)
		return nil
	}
	return &localAccount{password: pw, secret: secret}
}

// password: local credentials with TOTP for Alex and the owner (the TOTP steps of both run side
// by side), a login of a new device of Alex (pending, approved by Alex), an unknown name and a
// wrong password with canaries, and the console login of the owner with the password.
func (sc *Scenario) password() {
	sc.T.Helper()
	alex, o := sc.W.People["Alex"], sc.W.Owner
	a, own := sc.setLocal(alex, "alex"), sc.setLocal(o, "owner")
	if a == nil || own == nil {
		return
	}
	sc.put(alex, "/me/local-credential", map[string]any{"totp_code": sc.code(a)}, ok...)
	own.step = a.step - 1
	sc.put(o, "/me/local-credential", map[string]any{"totp_code": sc.code(own)}, ok...)
	sc.owner = own

	phone := sc.device("alex phone")
	login := func(name, password, totp string, want ...int) harness.Response {
		return sc.anon(phone, req{method: http.MethodPost, path: "/auth/password/login", body: map[string]any{
			"login_name": name, "password": password, "totp_code": totp, "device": phone.Registration(sc.Reg.New(ClassContent, "", "device name (password login)")),
		}}, want...)
	}
	login(sc.Reg.New(ClassContent, "", "unknown login name"), sc.Reg.New(ClassSecret, "", "password (unknown name)"), "123456", http.StatusUnauthorized, http.StatusBadRequest)
	login("alex", sc.Reg.New(ClassSecret, "", "wrong password"), "123456", http.StatusUnauthorized, http.StatusBadRequest)
	r := login("alex", a.password, sc.code(a), http.StatusOK, http.StatusCreated)
	if sc.adopt(phone, r) {
		dev := obj(sc.get(phone, "/devices/"+phone.DeviceID, ok...))
		vc := str(dev, "verification_code")
		sc.Reg.Register(vc, ClassSecret, "", "verification code")
		sc.post(alex, "/devices/"+phone.DeviceID+"/approve", map[string]any{"verification_code": vc}, ok...)
		sc.alexPhone = phone
	}
	// The console of the owner: a cookie session from the console form of loginWithPassword.
	body, _ := json.Marshal(map[string]any{"client": "console", "login_name": "owner", "password": own.password, "totp_code": sc.code(own)})
	resp := sc.console(http.MethodPost, "/auth/password/login", body)
	if resp == nil || resp.StatusCode != http.StatusOK {
		sc.T.Errorf("console password login: %d", statusOf(resp))
	}
}

// console sends a request of the owner's console tab: the CSRF header, the Origin and the
// session cookie; it keeps a new cookie of the answer.
func (sc *Scenario) console(method, path string, body []byte) *http.Response {
	sc.T.Helper()
	h := header("X-Console-Request", "1", "Origin", sc.Server.PublicURL)
	if sc.consoleCookie != nil {
		h.Set("Cookie", sc.consoleCookie.Name+"="+sc.consoleCookie.Value)
	}
	resp := sc.raw(method, path, h, body, "owner console")
	if resp != nil {
		for _, k := range resp.Cookies() {
			if k.Value != "" {
				sc.Reg.Register(k.Value, ClassSecret, "", "console cookie")
				sc.consoleCookie = k
			}
		}
	}
	return resp
}

// consoleSteps drives the console operations of the owner: the session, view-as and its end,
// a step-up with the password and one through the fake provider (the owner's identity was linked
// in oidc), and the logout.
func (sc *Scenario) consoleSteps(p *IDP, prov string) {
	sc.T.Helper()
	if sc.consoleCookie == nil || sc.owner == nil {
		sc.T.Errorf("no console session")
		return
	}
	want := func(r *http.Response, code int, what string) {
		if statusOf(r) != code {
			sc.T.Errorf("console %s: %d, want %d", what, statusOf(r), code)
		}
	}
	want(sc.console(http.MethodGet, "/console/session", nil), http.StatusOK, "getConsoleSession")
	challenge := func() string {
		r := sc.console(http.MethodPost, "/auth/step-up/challenge", nil)
		want(r, http.StatusOK, "createStepUpChallenge")
		return sc.lastConsoleBody("challenge_id")
	}
	b, _ := json.Marshal(map[string]any{"challenge_id": challenge(), "password": sc.owner.password, "totp_code": sc.code(sc.owner)})
	want(sc.console(http.MethodPost, "/auth/step-up/password", b), http.StatusOK, "stepUpWithPassword")
	want(sc.console(http.MethodGet, "/users/"+sc.W.People["Alex"].PrincipalID+"/view-as", nil), http.StatusOK, "viewAsUser")
	want(sc.console(http.MethodDelete, "/console/session/view-as", nil), http.StatusNoContent, "endViewAs")
	t, _ := sc.T.(*testing.T)
	if t != nil && prov != "" {
		verifier := sc.Reg.New(ClassSecret, "", "PKCE verifier (console)") + strings.Repeat("v", 24)
		b, _ = json.Marshal(map[string]any{"provider_id": prov, "challenge_id": challenge(), "app_redirect_uri": sc.Server.PublicURL + "/console/step-up/callback",
			"client_challenge": oidcflow.S256(verifier)})
		r := sc.console(http.MethodPost, "/auth/step-up/oidc", b)
		want(r, http.StatusOK, "beginStepUpOidcReauth")
		if authURL := sc.lastConsoleBody("authorization_url"); authURL != "" {
			p.NextLogin(sc.ownerLogin)
			if code := sc.callback(p.Authorize(t, authURL)); code != "" {
				b, _ = json.Marshal(map[string]any{"login_code": code, "client_verifier": verifier})
				want(sc.console(http.MethodPost, "/auth/step-up/oidc/complete", b), http.StatusOK, "completeStepUpOidcReauth")
			}
		}
	}
	want(sc.console(http.MethodPost, "/auth/logout", nil), http.StatusNoContent, "logout")
}

// lastConsoleBody returns a member of the body of the last console answer.
func (sc *Scenario) lastConsoleBody(member string) string { return str(sc.lastRaw, member) }

// oidc: a provider with canary name, an OIDC login with canary claims that provisions a person,
// the identity link of Alex, an OIDC re-authentication step-up and the unlink.
func (sc *Scenario) oidc(p *IDP) string {
	sc.T.Helper()
	o := sc.W.Owner
	t, _ := sc.T.(*testing.T)
	sc.Reg.Register(p.Secret, ClassSecret, "", "OIDC client secret")
	prov := id(sc.post(o, "/identity-providers", map[string]any{
		"kind": "oidc", "name": sc.Reg.New(ClassContent, "", "identity provider name"), "enabled": true, "allow_provisioning": true,
		"config": map[string]any{
			"issuer": p.Issuer(), "client_id": idpClientID, "client_auth": "client_secret_basic", "client_secret_ref": "file:" + filepath.Join(sc.Server.SecretsDir, idpSecretFile),
			"app_redirect_uris": []string{appRedirect}, "groups_claim": groupsClaim,
		},
	}, created...))
	sc.get(o, "/identity-providers/"+prov, ok...)
	sc.get(o, "/identity-providers", ok...)
	sc.patch(o, "/identity-providers/"+prov, map[string]any{"name": sc.Reg.New(ClassContent, "", "identity provider name (update)")}, ok...)
	extra := id(sc.post(o, "/identity-providers", map[string]any{
		"kind": "trusted_header", "name": sc.Reg.New(ClassContent, "", "identity provider name 2"), "enabled": false,
		"config": map[string]any{"login_header": "X-Example-Login"},
	}, created...))
	sc.del(o, "/identity-providers/"+extra, none...)
	if t == nil {
		return prov
	}
	login := func() oidctest.Login {
		l := oidctest.Login{
			Subject: sc.Reg.New(ClassSecret, "", "OIDC subject"), Name: sc.Reg.New(ClassContent, "", "OIDC name claim"),
			PreferredUsername: sc.Reg.New(ClassContent, "", "OIDC preferred_username"), Email: sc.Reg.New(ClassContent, "", "OIDC e-mail claim") + "@example.com",
		}
		p.SetGroups(groupsClaim, sc.Reg.New(ClassContent, "", "OIDC group claim"))
		return l
	}
	// flow runs begin (as c, or anonymous), the provider and the callback, and returns the verifier
	// and the login code.
	flow := func(c *harness.Client, path string, body map[string]any, l oidctest.Login) (string, string) {
		verifier := sc.Reg.New(ClassSecret, "", "PKCE verifier") + strings.Repeat("v", 24)
		body["provider_id"], body["app_redirect_uri"], body["client_challenge"] = prov, appRedirect, oidcflow.S256(verifier)
		var r harness.Response
		if c.SessionID == "" && c.PrincipalID == "" {
			r = sc.anon(c, req{method: http.MethodPost, path: path, body: body}, ok...)
		} else {
			r = sc.post(c, path, body, ok...)
		}
		authURL := str(obj(r), "authorization_url")
		if authURL == "" {
			return "", ""
		}
		p.NextLogin(l)
		loc := p.Authorize(t, authURL)
		code := sc.callback(loc)
		return verifier, code
	}

	// Login with provisioning.
	dev := sc.device("oidc person")
	l := login()
	verifier, code := flow(dev, "/auth/oidc/begin", map[string]any{}, l)
	if code != "" {
		r := sc.anon(dev, req{method: http.MethodPost, path: "/auth/oidc/complete", body: map[string]any{
			"login_code": code, "client_verifier": verifier, "device": dev.Registration(sc.Reg.New(ClassContent, "", "device name (OIDC)")),
		}}, http.StatusOK, http.StatusCreated)
		if sc.adopt(dev, r) {
			sc.approve(dev)
			sc.get(dev, "/me", ok...)
		}
	}

	// Identity link of Alex and its removal.
	alex := sc.W.People["Alex"]
	v3, c3 := flow(alex, "/me/identities/begin", map[string]any{}, login())
	if c3 != "" {
		link := sc.post(alex, "/me/identities/complete", map[string]any{"login_code": c3, "client_verifier": v3}, created...)
		if identity := firstID(link, "id", "identity.id"); identity != "" {
			sc.get(alex, "/me/credentials", ok...)
			sc.del(o, "/users/"+alex.PrincipalID+"/identities/"+identity, none...)
		}
	}
	// The identity of the owner stays linked for the console re-authentication.
	sc.ownerLogin = login()
	v4, c4 := flow(o, "/me/identities/begin", map[string]any{}, sc.ownerLogin)
	if c4 != "" {
		sc.post(o, "/me/identities/complete", map[string]any{"login_code": c4, "client_verifier": v4}, created...)
	}
	// A callback with a state the server never issued.
	sc.callbackRaw("/auth/oidc/callback?state=" + sc.Reg.New(ClassSecret, "", "OIDC state") + "&code=" + sc.Reg.New(ClassSecret, "", "OIDC code"))
	return prov
}

// callback sends the redirect of the provider to the server's callback and returns the
// login_code of the server's redirect to the app.
func (sc *Scenario) callback(loc *url.URL) string {
	sc.T.Helper()
	if loc == nil {
		return ""
	}
	if c := loc.Query().Get("code"); c != "" {
		sc.Reg.Register(c, ClassSecret, "", "OIDC authorization code")
	}
	if s := loc.Query().Get("state"); s != "" {
		sc.Reg.Register(s, ClassSecret, "", "OIDC state")
	}
	r := sc.callbackRaw(strings.TrimPrefix(loc.Path, "/api/v1") + "?" + loc.RawQuery)
	if r == nil || r.StatusCode != http.StatusFound {
		sc.T.Errorf("handleOidcCallback: %v", statusOf(r))
		return ""
	}
	u, err := url.Parse(r.Header.Get("Location"))
	if err != nil {
		return ""
	}
	code := u.Query().Get("login_code")
	sc.Reg.Register(code, ClassSecret, "", "login_code")
	return code
}

// statusOf returns the status of r or 0.
func statusOf(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}

// callbackRaw sends GET path (relative to /api/v1) like a browser, without following the
// redirect, and records it like a client request.
func (sc *Scenario) callbackRaw(path string) *http.Response {
	sc.T.Helper()
	resp := sc.raw(http.MethodGet, path, nil, nil, "browser")
	return resp
}

// raw sends a request without the harness client (a browser, a host service, a forged
// Authorization header), records coverage and an error response, and returns the response with
// its body consumed into the error sink.
func (sc *Scenario) raw(method, path string, hdr http.Header, body []byte, who string) *http.Response {
	sc.T.Helper()
	var rd *bytes.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	} else {
		rd = bytes.NewReader(nil)
	}
	rq, err := http.NewRequestWithContext(sc.Ctx, method, sc.Server.APIBase+path, rd)
	if err != nil {
		sc.T.Errorf("raw request: %v", err)
		return nil
	}
	for k, vs := range hdr {
		rq.Header[k] = vs
	}
	if body != nil && rq.Header.Get("Content-Type") == "" {
		rq.Header.Set("Content-Type", "application/json")
	}
	cl := &http.Client{Timeout: time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := cl.Do(rq)
	if err != nil {
		sc.T.Errorf("raw %s %s: %v", method, routeOf(method, path), err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	hr := harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: buf.Bytes()}
	sc.lastRaw = obj(hr)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		sc.harvest(buf.Bytes())
	}
	ErrorObserver(sc.Sinks, who, func(r harness.Request, x harness.Response) { sc.Cov.Observe(r.Method, r.Path, x.Status) })(
		harness.Request{Method: method, Path: path}, hr)
	return resp
}

// trustedHeader: a trusted-header driver with provisioning and a login whose header values are
// canaries (M1-43), sent from the loopback proxy of the harness.
func (sc *Scenario) trustedHeader() {
	sc.T.Helper()
	o := sc.W.Owner
	sc.post(o, "/identity-providers", map[string]any{
		"kind": "trusted_header", "name": sc.Reg.New(ClassContent, "", "trusted header driver name"), "enabled": true, "allow_provisioning": true,
		"config": map[string]any{"login_header": "X-Example-Login", "name_header": "X-Example-Name"},
	}, created...)
	loginValue := sc.Reg.New(ClassContent, "", "trusted header login") + "@example.com"
	h := header("X-Example-Login", loginValue, "X-Example-Name", sc.Reg.New(ClassContent, "", "trusted header name"))
	dev := sc.device("header person")
	r := sc.anon(dev, req{method: http.MethodPost, path: "/auth/trusted-header/login", header: h, body: map[string]any{
		"device": dev.Registration(sc.Reg.New(ClassContent, "", "device name (trusted header)")),
	}}, http.StatusOK, http.StatusCreated)
	if r.Status == http.StatusCreated || r.Status == http.StatusOK {
		if sc.adopt(dev, r) {
			sc.call(dev, req{method: http.MethodGet, path: "/me", header: header("X-Example-Login", loginValue)}, ok...)
		}
	} else {
		sc.T.Errorf("loginWithTrustedHeader %d", r.Status)
	}
}

// passkeys: registration for Alex, a passkey login, a step-up with the passkey and the
// revocation.
func (sc *Scenario) passkeys() {
	sc.T.Helper()
	alex := sc.W.People["Alex"]
	b := sc.post(alex, "/me/passkeys/begin", nil, ok...)
	ch := obj(b)
	cred := sc.webauthnCreate(str(ch, "challenge_id"), b.Body)
	if cred == nil {
		return
	}
	pk := sc.post(alex, "/me/passkeys/complete", map[string]any{"challenge_id": str(ch, "challenge_id"), "credential": cred,
		"name": sc.Reg.New(ClassContent, "", "passkey name")}, created...)
	dev := sc.device("alex passkey device")
	lb := sc.anon(dev, req{method: http.MethodPost, path: "/auth/passkey/begin", body: map[string]any{"login_name": "alex"}}, ok...)
	var lch struct {
		ID        string          `json:"challenge_id"`
		PublicKey json.RawMessage `json:"public_key"`
	}
	_ = json.Unmarshal(lb.Body, &lch)
	if got := sc.webauthnGet(lch.PublicKey); got != nil {
		r := sc.anon(dev, req{method: http.MethodPost, path: "/auth/passkey/complete", body: map[string]any{
			"challenge_id": lch.ID, "credential": got, "device": dev.Registration(sc.Reg.New(ClassContent, "", "device name (passkey)")),
		}}, http.StatusOK, http.StatusCreated)
		sc.adopt(dev, r)
	}
	sch := sc.post(alex, "/auth/step-up/challenge", nil, ok...)
	var sv struct {
		ID      string          `json:"challenge_id"`
		Options json.RawMessage `json:"passkey_options"`
	}
	_ = json.Unmarshal(sch.Body, &sv)
	if got := sc.webauthnGet(sv.Options); got != nil {
		sc.post(alex, "/auth/step-up/passkey", map[string]any{"challenge_id": sv.ID, "credential": got}, ok...)
	} else {
		sc.post(alex, "/auth/step-up/passkey", map[string]any{"challenge_id": sv.ID, "credential": map[string]any{"id": sc.Reg.New(ClassSecret, "", "forged credential")}},
			http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden)
	}
	if pid := firstID(pk, "id", "passkey.id"); pid != "" {
		sc.del(sc.W.Owner, "/users/"+alex.PrincipalID+"/passkeys/"+pid, none...)
	}
}

// pairing: a new device of Alex paired by her phone of the password step (beginPairing,
// approvePairing, completePairing).
func (sc *Scenario) pairing() {
	sc.T.Helper()
	alex := sc.W.People["Alex"]
	dev := sc.device("alex paired device")
	b := obj(sc.anon(dev, req{method: http.MethodPost, path: "/auth/pairing/begin", body: map[string]any{
		"device": dev.Registration(sc.Reg.New(ClassContent, "", "device name (pairing)")),
	}}, created...))
	sc.post(alex, "/me/devices/pairings", map[string]any{"pairing_code": str(b, "pairing_code")}, created...)
	sc.adopt(dev, sc.anon(dev, req{method: http.MethodPost, path: "/auth/pairing/complete", body: map[string]any{"pairing_id": str(b, "pairing_id")}}, created...))
}

// sessions: refresh rotation with a replayed refresh token, an extension session, the session
// and device lists, a revoked session, a revoked device and a logout.
func (sc *Scenario) sessions() {
	sc.T.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	sam := sc.Observers["Sam"]
	// Rotation and reuse on a separate session of Robin (the reuse ends that session).
	robin, err := sc.Server.UserSession(sc.Ctx, sc.W.People["Robin"].PrincipalID)
	if err != nil {
		sc.T.Errorf("user session: %v", err)
		return
	}
	sc.Watch(robin, "robin second session")
	_, oldRefresh := robin.Tokens()
	sc.adopt(robin, sc.anon(robin, req{method: http.MethodPost, path: "/auth/token/refresh", body: map[string]any{"refresh_token": oldRefresh}}, ok...))
	_, second := robin.Tokens()
	sc.adopt(robin, sc.anon(robin, req{method: http.MethodPost, path: "/auth/token/refresh", body: map[string]any{"refresh_token": second}}, ok...))
	// The generation before the previous one is a reuse beyond auth.refresh_reuse_grace.
	sc.anon(robin, req{method: http.MethodPost, path: "/auth/token/refresh", body: map[string]any{"refresh_token": oldRefresh}}, http.StatusUnauthorized, http.StatusBadRequest)

	sc.post(alex, "/me/sessions/extensions", map[string]any{"client_component": "share_ext"}, created...)
	sc.get(alex, "/me/sessions", ok...)
	sc.get(o, "/users/"+alex.PrincipalID+"/sessions", ok...)
	sc.get(alex, "/me/devices", ok...)
	sc.get(o, "/users/"+alex.PrincipalID+"/devices", ok...)
	sc.get(o, "/users/"+alex.PrincipalID+"/credentials", ok...)
	sc.get(alex, "/me/credentials", ok...)
	_ = sam
	if sc.alexPhone != nil {
		if s := sc.alexPhone.SessionID; s != "" {
			sc.post(alex, "/sessions/"+s+"/revoke", nil, none...)
		}
		sc.post(alex, "/devices/"+sc.alexPhone.DeviceID+"/revoke", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "device revoke reason")}, ok...)
	}
	if sc.enrollee != nil {
		sc.post(sc.enrollee, "/auth/logout", nil, none...)
	}
}

// badCredentials sends invalid Authorization headers that carry canaries.
func (sc *Scenario) badCredentials() {
	sc.T.Helper()
	sc.raw(http.MethodGet, "/me", header("Authorization", "Bearer bothy_"+sc.Reg.New(ClassSecret, "", "forged API token")), nil, "forged bearer")
	sc.raw(http.MethodGet, "/me", header("Authorization", "DPoP "+sc.Reg.New(ClassSecret, "", "forged DPoP token"), "DPoP", sc.Reg.New(ClassSecret, "", "forged DPoP proof")), nil, "forged dpop")
}

// contains reports whether v is in list.
func contains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
