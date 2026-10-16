// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The paths of the console operations (M6-01).
const (
	consoleSessionPath   = "/api/v1/console/session"
	consoleViewAsPath    = "/api/v1/console/session/view-as"
	stepUpPasswordPath   = "/api/v1/auth/step-up/password" //nolint:gosec // G101: an API path, no secret
	stepUpOIDCPath       = "/api/v1/auth/step-up/oidc"
	stepUpOIDCDonePath   = "/api/v1/auth/step-up/oidc/complete"
	logoutPath           = "/api/v1/auth/logout"
	profilePath          = "/api/v1/me/profile"
	consoleLoginCallback = httpservertest.PublicURL + "/console/login/callback"
	consoleStepCallback  = httpservertest.PublicURL + "/console/step-up/callback"
	consoleCookieName    = "__Host-console_session"
)

// publicURL is the origin the console cookie belongs to.
var publicURL, _ = url.Parse(httpservertest.PublicURL + "/")

// cworld is the OIDC world with the console of M6-01: the owner Alex with a local account
// (password and TOTP), a passkey from virtualauthn and a linked identity at the fake provider; Sam
// is a person without owner rights (07 sections 10 and 11.1).
type cworld struct {
	*oworld
	alex, sam ids.ID
	secret    []byte
	passkey   *virtualauthn.Authenticator
	phone     *stepClient
}

// newConsoleWorld builds the console world.
func newConsoleWorld(t *testing.T) *cworld {
	t.Helper()
	w := &cworld{oworld: newOIDCWorld(t, oidcWorldOpts{})}
	w.alex = w.bootstrap("alex").OwnerID
	w.phone = w.sworld.phone(w.alex)
	w.secret = w.establish(w.phone.client)
	w.passkey = authenticator()
	w.phone.stepUp()
	w.want(w.phone.register(w.passkey, "Laptop"), http.StatusCreated, "")
	w.linkIdentity(w.alex, "alex-sub")
	w.sam = w.person("sam", false)
	return w
}

// consoleClient is the browser tab of the console: a cookie jar for the origin of
// server.public_url, which sends the __Host- cookie back like a browser.
type consoleClient struct {
	w   *cworld
	jar *cookiejar.Jar
	// last is the cookie value of the last answer that set one.
	last string
}

// newConsole returns a tab without a cookie.
func (w *cworld) newConsole() *consoleClient {
	w.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		w.t.Fatal(err)
	}
	return &consoleClient{w: w, jar: jar}
}

// cookie returns the current cookie value of the tab, "" for none.
func (c *consoleClient) cookie() string {
	for _, k := range c.jar.Cookies(publicURL) {
		if k.Name == consoleCookieName {
			return k.Value
		}
	}
	return ""
}

// raw sends a request with exactly the headers given and the cookie of the tab, and stores the
// cookies of the answer.
func (c *consoleClient) raw(method, path, body string, headers ...string) *httpservertest.Response {
	c.w.t.Helper()
	req := c.w.h.NewRequest(method, path, body, headers...)
	for _, k := range c.jar.Cookies(publicURL) {
		req.AddCookie(k)
	}
	r := c.w.h.Do(req)
	if set := r.Cookies(); len(set) > 0 {
		c.jar.SetCookies(publicURL, set)
		for _, k := range set {
			if k.Name == consoleCookieName && k.Value != "" {
				c.last = k.Value
			}
		}
	}
	return r
}

// do sends a request as the console does: the CSRF header and the Origin on every request.
func (c *consoleClient) do(method, path, body string) *httpservertest.Response {
	c.w.t.Helper()
	return c.raw(method, path, body, httpserver.HeaderConsoleRequest, "1", "Origin", httpservertest.PublicURL)
}

// passwordLogin logs the tab in with the console form of loginWithPassword as Alex.
func (c *consoleClient) passwordLogin() *httpservertest.Response {
	c.w.t.Helper()
	c.w.clk.Advance(30 * time.Second) // a TOTP code is used once
	b, _ := json.Marshal(map[string]any{"client": "console", "login_name": "alex", "password": testPassword, "totp_code": c.w.code(c.w.secret)})
	return c.do(http.MethodPost, passwordLoginPath, string(b))
}

// login logs the tab in with the password and expects 200.
func (c *consoleClient) login() {
	c.w.t.Helper()
	c.w.want(c.passwordLogin(), http.StatusOK, "")
}

// sessionID returns the console session of the tab's current cookie from the database.
func (c *consoleClient) sessionID() ids.ID {
	c.w.t.Helper()
	var id string
	c.w.scalar(&id, `SELECT id::text FROM auth_sessions WHERE client_component = 'console' AND revoked_at IS NULL ORDER BY created_at DESC LIMIT 1`)
	return ids.MustParse(id)
}

// challenge asks for a step-up challenge and returns the answer.
func (c *consoleClient) challenge() map[string]any {
	c.w.t.Helper()
	return c.w.want(c.do(http.MethodPost, challengePath, ""), http.StatusOK, "")
}

// passkeyStepUp confirms a step-up with the passkey of Alex.
func (c *consoleClient) passkeyStepUp() *httpservertest.Response {
	c.w.t.Helper()
	m := c.challenge()
	cred, err := c.w.passkey.Get(options(c.w.t, m, "passkey_options"))
	if err != nil {
		c.w.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "credential": json.RawMessage(cred)})
	return c.do(http.MethodPost, stepUpPasskeyPath, string(body))
}

// passwordStepUp confirms a step-up with password and TOTP.
func (c *consoleClient) passwordStepUp(password string) *httpservertest.Response {
	c.w.t.Helper()
	m := c.challenge()
	c.w.clk.Advance(30 * time.Second)
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "password": password, "totp_code": c.w.code(c.w.secret)})
	return c.do(http.MethodPost, stepUpPasswordPath, string(body))
}

// oidcStepUp runs the OIDC re-login with an auth_time shift relative to the clock and returns the
// answer of the completion and the authorization URL.
func (c *consoleClient) oidcStepUp(authTimeShift time.Duration) (*httpservertest.Response, string) {
	c.w.t.Helper()
	m := c.challenge()
	verifier := randomText(c.w.t)
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "provider_id": c.w.prov.String(),
		"app_redirect_uri": consoleStepCallback, "client_challenge": oidcflow.S256(verifier)})
	begun := c.w.want(c.do(http.MethodPost, stepUpOIDCPath, string(body)), http.StatusOK, "")
	authURL := begun["authorization_url"].(string)
	c.w.idp.SetClaim("auth_time", c.w.clk.Now().Add(authTimeShift).Unix())
	defer c.w.idp.Reset()
	f := &flow{verifier: verifier, attempt: begun["login_attempt_id"].(string), authURL: authURL}
	c.w.want(c.w.callback(f, oidctest.Login{Subject: "alex-sub", Name: "Alex"}), http.StatusFound, "")
	if !strings.HasPrefix(f.redirect.String(), consoleStepCallback+"?") || f.redirect.Query().Get("login_code") == "" {
		c.w.t.Fatalf("redirect %s", f.redirect)
	}
	done, _ := json.Marshal(map[string]any{"login_code": f.redirect.Query().Get("login_code"), "client_verifier": verifier})
	return c.do(http.MethodPost, stepUpOIDCDonePath, string(done)), authURL
}

// stepMethod returns step_up_method of the session id.
func (w *cworld) stepMethod(id ids.ID) string {
	w.t.Helper()
	var m string
	w.scalar(&m, `SELECT coalesce(step_up_method, '') FROM auth_sessions WHERE id = $1`, id.String())
	return m
}

// wantCleared checks a 401 that removes the cookie.
func (w *cworld) wantCleared(r *httpservertest.Response, code string) {
	w.t.Helper()
	w.want(r, http.StatusUnauthorized, code)
	if got := r.Header.Get("Set-Cookie"); got != consoleCookieName+"=; Path=/; Max-Age=0; Secure; HttpOnly; SameSite=Strict" {
		w.t.Fatalf("Set-Cookie of the refusal = %q", got)
	}
}

// jwsPattern matches a compact JWS (three base64url parts, the first starting with eyJ).
var jwsPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)

// noToken fails when the raw answer (body and every header but Set-Cookie) carries a token.
func noToken(t *testing.T, r *httpservertest.Response) {
	t.Helper()
	raw := string(r.Body)
	for k, vs := range r.Header {
		if k == "Set-Cookie" {
			continue
		}
		raw += "\n" + k + ": " + strings.Join(vs, ", ")
	}
	if jwsPattern.MatchString(raw) || strings.Contains(raw, "bothy_") || strings.Contains(raw, "access_token") || strings.Contains(raw, "refresh_token") {
		t.Fatalf("a token reached the browser: %s", raw)
	}
}

// TestConsoleLogin_Password_SetsCookieOnly (ID-T19 part 1, D-M6-01-1, D-M6-01-2): the console
// form of loginWithPassword answers 200 ConsoleSession with the cookie and exactly its
// attributes; no token in body or headers; a console session with a cookie hash on an active web
// device and no refresh family.
func TestConsoleLogin_Password_SetsCookieOnly(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	c := w.newConsole()
	r := c.passwordLogin()
	m := w.want(r, http.StatusOK, "")
	if m["user_id"] != w.alex.String() || m["display_name"] != "Alex" || m["idle_timeout_seconds"] != float64(1800) || m["step_up_at"] != nil {
		t.Fatalf("body %v", m)
	}
	set := r.Header.Values("Set-Cookie")
	if len(set) != 1 || !regexp.MustCompile(`^__Host-console_session=[A-Za-z0-9_-]{43}; Path=/; Max-Age=43200; Secure; HttpOnly; SameSite=Strict$`).MatchString(set[0]) {
		t.Fatalf("Set-Cookie %q", set)
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control %q", r.Header.Get("Cache-Control"))
	}
	noToken(t, r)
	value := c.cookie()
	if value == "" || strings.Contains(string(r.Body), value) {
		t.Fatal("the cookie is missing or in the body")
	}
	sid := c.sessionID()
	if n := w.count(`SELECT count(*) FROM auth_sessions s JOIN devices d ON d.id = s.device_id
		WHERE s.id = $1 AND s.cookie_hash IS NOT NULL AND s.cookie_hash <> convert_to($2, 'UTF8') AND d.platform = 'web' AND d.state = 'active'
		AND s.auth_method = 'password_totp' AND s.expires_at = s.created_at + interval '12 hours'`, sid.String(), value); n != 1 {
		t.Fatal("no console session with a cookie hash on an active web device")
	}
	if n := w.count(`SELECT count(*) FROM refresh_token_families WHERE session_id = $1`, sid.String()); n != 0 {
		t.Fatalf("%d refresh families of a console session", n)
	}
	if n := w.count(`SELECT count(*) FROM device_keys k JOIN auth_sessions s ON s.device_id = k.device_id WHERE s.id = $1`, sid.String()); n != 0 {
		t.Fatalf("%d keys on the web device", n)
	}
	logins := w.audits("auth.login.complete")
	if last := logins[len(logins)-1]; last.Details["client_component"] != "console" || last.ResourceID != sid.String() {
		t.Fatalf("login entry %+v", last)
	}
	// The cookie authenticates getMe; the DPoP form of the app is untouched.
	w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
}

// TestConsoleLogin_OIDC_And_Passkey (D-M6-01-1): the console form through the fake provider (the
// redirect must be the console callback; another is 400) and through virtualauthn; the trusted
// header form runs on a loopback listener.
func TestConsoleLogin_OIDC_And_Passkey(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	t.Run("OIDC", func(t *testing.T) {
		_, r := w.begin(oidcBeginPath, nil, map[string]any{"client": "console"})
		w.want(r, http.StatusBadRequest, "validation_failed")
		f, r := w.begin(oidcBeginPath, nil, map[string]any{"client": "console", "app_redirect_uri": consoleLoginCallback})
		w.want(r, http.StatusOK, "")
		w.want(w.callback(f, oidctest.Login{Subject: "alex-sub", Name: "Alex"}), http.StatusFound, "")
		if !strings.HasPrefix(f.redirect.String(), consoleLoginCallback+"?") {
			t.Fatalf("redirect %s", f.redirect)
		}
		code := f.redirect.Query().Get("login_code")
		// The app form cannot redeem a console flow.
		w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusBadRequest, "validation_failed")
		c := w.newConsole()
		b, _ := json.Marshal(map[string]any{"client": "console", "login_code": code, "client_verifier": f.verifier})
		r = c.do(http.MethodPost, oidcCompletePath, string(b))
		m := w.want(r, http.StatusOK, "")
		noToken(t, r)
		if m["user_id"] != w.alex.String() || c.cookie() == "" {
			t.Fatalf("body %v", m)
		}
		if w.stepMethod(c.sessionID()) != "" {
			t.Fatal("a login is no step-up")
		}
		// An app flow cannot be redeemed in the console form.
		g, code2 := w.loginCode(oidctest.Login{Subject: "alex-sub"}, nil)
		b, _ = json.Marshal(map[string]any{"client": "console", "login_code": code2, "client_verifier": g.verifier})
		w.want(w.newConsole().do(http.MethodPost, oidcCompletePath, string(b)), http.StatusBadRequest, "validation_failed")
	})
	t.Run("Passkey", func(t *testing.T) {
		id, opts := w.loginBegin("alex")
		cred, err := w.passkey.Get(opts)
		if err != nil {
			t.Fatal(err)
		}
		c := w.newConsole()
		b, _ := json.Marshal(map[string]any{"client": "console", "challenge_id": id, "credential": json.RawMessage(cred)})
		r := c.do(http.MethodPost, passkeyCompletePath, string(b))
		w.want(r, http.StatusOK, "")
		noToken(t, r)
		w.want(c.do(http.MethodGet, consoleSessionPath, ""), http.StatusOK, "")
		// The console form takes no device block.
		b, _ = json.Marshal(map[string]any{"client": "console", "login_name": "alex", "password": testPassword, "totp_code": "000000",
			"device": w.phoneKeys().device()})
		w.want(w.newConsole().do(http.MethodPost, passwordLoginPath, string(b)), http.StatusBadRequest, "validation_failed")
	})
}

// TestConsoleLogin_TrustedHeader (D-M6-01-1): the console form of the header login on a loopback
// listener; every later request of the console session must carry the same header login.
func TestConsoleLogin_TrustedHeader(t *testing.T) {
	t.Parallel()
	w := headerWorld(t, "127.0.0.1/32", true, true)
	prov := w.headerProvider(false)
	w.knownPerson("alex", prov, alexLogin)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &consoleClient{w: &cworld{oworld: &oworld{sworld: w}}, jar: jar}
	r := c.raw(http.MethodPost, headerLoginPath, `{"client":"console"}`, httpserver.HeaderConsoleRequest, "1", "Origin", httpservertest.PublicURL,
		loginHeader, alexLogin)
	w.want(r, http.StatusOK, "")
	noToken(t, r)
	w.want(c.raw(http.MethodGet, mePath, "", loginHeader, alexLogin), http.StatusOK, "")
	c.w.wantCleared(c.raw(http.MethodGet, mePath, "", loginHeader, samLogin), "session_revoked")
	w.want(c.raw(http.MethodPost, headerLoginPath, `{"client":"console","enrollment_code":"en_EXAMPLE"}`, httpserver.HeaderConsoleRequest, "1",
		"Origin", httpservertest.PublicURL, loginHeader, alexLogin), http.StatusBadRequest, "validation_failed")
}

// TestConsoleBeforeStepUp_OnlyOwnProfile (ID-T19 last case, D-M6-01-8): after the login getMe and
// getConsoleSession answer, listUsers and viewAsUser are 401 step_up_required.
func TestConsoleBeforeStepUp_OnlyOwnProfile(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	c := w.newConsole()
	c.login()
	w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
	m := w.want(c.do(http.MethodGet, consoleSessionPath, ""), http.StatusOK, "")
	if got := m["step_up_methods_available"]; len(got.([]any)) != 3 {
		t.Fatalf("methods %v", got)
	}
	for _, path := range []string{usersPath, "/api/v1/users/" + w.sam.String() + "/view-as", "/api/v1/identity-providers"} {
		p := w.want(c.do(http.MethodGet, path, ""), http.StatusUnauthorized, "step_up_required")
		if p["reason"] != "console_step_up_required" {
			t.Fatalf("%s: %v", path, p)
		}
	}
	if c.cookie() == "" {
		t.Fatal("the gate must not clear the cookie")
	}
}

// TestConsoleStepUp_RotatesCookie (D-M6-01-2, D-M6-01-6): each step-up method gives a new cookie;
// the old one answers 401 and clears itself, the new one works; step_up_method records the method.
func TestConsoleStepUp_RotatesCookie(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	rotated := func(t *testing.T, c *consoleClient, old string, r *httpservertest.Response, method string) {
		t.Helper()
		w.want(r, http.StatusOK, "")
		if c.last == old || c.cookie() == old || c.cookie() == "" {
			t.Fatal("the cookie was not rotated")
		}
		if m := w.stepMethod(c.sessionID()); m != method {
			t.Fatalf("step_up_method %q, want %q", m, method)
		}
		stale := w.newConsole()
		stale.jar.SetCookies(publicURL, []*http.Cookie{{Name: consoleCookieName, Value: old, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}})
		w.wantCleared(stale.do(http.MethodGet, mePath, ""), "unauthenticated")
		w.want(c.do(http.MethodGet, usersPath, ""), http.StatusOK, "")
	}
	t.Run("Passkey", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		old := c.cookie()
		rotated(t, c, old, c.passkeyStepUp(), "passkey")
	})
	t.Run("PasswordTOTP", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		w.want(c.passwordStepUp("wrong-password-123"), http.StatusUnauthorized, "step_up_required")
		old := c.cookie()
		rotated(t, c, old, c.passwordStepUp(testPassword), "password_totp")
	})
	t.Run("OIDCReauth", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		r, authURL := c.oidcStepUp(-time.Hour)
		w.want(r, http.StatusUnauthorized, "step_up_required")
		u, _ := url.Parse(authURL)
		if q := u.Query(); q.Get("prompt") != "login" || q.Get("max_age") != "0" {
			t.Fatalf("authorization request %s", authURL)
		}
		old := c.cookie()
		r, _ = c.oidcStepUp(time.Second)
		rotated(t, c, old, r, "oidc_reauth")
		// Another redirect is 400.
		m := c.challenge()
		body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "provider_id": w.prov.String(),
			"app_redirect_uri": appRedirect, "client_challenge": oidcflow.S256(randomText(t))})
		w.want(c.do(http.MethodPost, stepUpOIDCPath, string(body)), http.StatusBadRequest, "validation_failed")
	})
	t.Run("NotFromAppSessions", func(t *testing.T) {
		m := w.want(w.phone.call(http.MethodPost, challengePath, ""), http.StatusOK, "")
		body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "password": testPassword, "totp_code": "000000"})
		w.want(w.phone.call(http.MethodPost, stepUpPasswordPath, string(body)), http.StatusForbidden, "forbidden")
	})
}

// TestConsoleCSRF (ID-T19, SEC-CORS-4, D-M6-01-5): a state-changing request with the cookie and
// without X-Console-Request, or with a foreign Origin, is 403 csrf_check_failed; with both it
// succeeds.
func TestConsoleCSRF(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	c := w.newConsole()
	c.login()
	for _, headers := range [][]string{
		{"Origin", httpservertest.PublicURL},
		{httpserver.HeaderConsoleRequest, "1", "Origin", "https://evil.example.com"},
		{httpserver.HeaderConsoleRequest, "1"},
	} {
		p := w.want(c.raw(http.MethodPost, logoutPath, "", headers...), http.StatusForbidden, "forbidden")
		if p["reason"] != "csrf_check_failed" {
			t.Fatalf("%v: %v", headers, p)
		}
	}
	if !w.h.Logs.Has(httpserver.LogConsoleRequestRefused) {
		t.Fatal("the refusal is not logged")
	}
	w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
	r := c.do(http.MethodPost, logoutPath, "")
	w.want(r, http.StatusNoContent, "")
	if got := r.Header.Get("Set-Cookie"); !strings.HasPrefix(got, consoleCookieName+"=; Path=/; Max-Age=0") || c.cookie() != "" {
		t.Fatalf("logout Set-Cookie %q", got)
	}
}

// TestConsoleIdleAndAbsoluteExpiry (D-M6-01-3): idle exactly at the idle timeout plus its
// hundredth after the last stored activity, one tick earlier it passes; the absolute end at
// auth.console_session_max_age; both revoke with expired and clear the cookie; the cleanup job
// revokes an idle session without a request, with one audit entry.
func TestConsoleIdleAndAbsoluteExpiry(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	lastSeen := func(id ids.ID) time.Time {
		var at time.Time
		w.scalar(&at, `SELECT last_seen_at FROM auth_sessions WHERE id = $1`, id.String())
		return at
	}
	const lag = 30*time.Minute + 18*time.Second
	c := w.newConsole()
	c.login()
	sid := c.sessionID()
	w.clk.Set(lastSeen(sid).Add(lag - time.Microsecond))
	w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
	if !lastSeen(sid).Equal(w.clk.Now().Truncate(time.Microsecond)) {
		t.Fatalf("last_seen_at %v not written at %v", lastSeen(sid), w.clk.Now())
	}
	// A request inside the write bound does not write last_seen_at.
	before := lastSeen(sid)
	w.clk.Advance(10 * time.Second)
	w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
	if !lastSeen(sid).Equal(before) {
		t.Fatal("last_seen_at written inside the write bound")
	}
	w.clk.Set(before.Add(lag))
	w.wantCleared(c.do(http.MethodGet, mePath, ""), "session_revoked")
	if w.sessionState(sid) != "expired" {
		t.Fatalf("revoke_reason %q", w.sessionState(sid))
	}
	entries := w.audits("session.revoke")
	if last := entries[len(entries)-1]; last.ResourceID != sid.String() || last.Details["cause"] != "idle" {
		t.Fatalf("entry %+v", last)
	}
	t.Run("Absolute", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		sid := c.sessionID()
		var end time.Time
		w.scalar(&end, `SELECT expires_at FROM auth_sessions WHERE id = $1`, sid.String())
		w.clk.Set(end.Add(-time.Microsecond))
		w.sql(`UPDATE auth_sessions SET last_seen_at = $2 WHERE id = $1`, sid.String(), w.clk.Now().Add(-time.Minute))
		w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
		w.clk.Set(end)
		w.wantCleared(c.do(http.MethodGet, mePath, ""), "session_revoked")
		if w.sessionState(sid) != "expired" {
			t.Fatal("not expired at the absolute end")
		}
	})
	t.Run("CleanupJob", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		sid := c.sessionID()
		w.clk.Advance(lag)
		if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if w.sessionState(sid) != "expired" {
			t.Fatal("the job did not end the idle session")
		}
		if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range w.audits("session.revoke") {
			if e.ResourceID == sid.String() {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("%d entries for one expiry", n)
		}
		w.wantCleared(c.do(http.MethodGet, mePath, ""), "session_revoked")
	})
}

// TestConsoleRevocation (D-M6-01-9): logout, revokeSession by the owner from an app session,
// revokeDevice of the web device and disableUser each make the next console request 401.
func TestConsoleRevocation(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	t.Run("Logout", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		old := c.cookie()
		w.want(c.do(http.MethodPost, logoutPath, ""), http.StatusNoContent, "")
		stale := w.newConsole()
		stale.jar.SetCookies(publicURL, []*http.Cookie{{Name: consoleCookieName, Value: old, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}})
		w.wantCleared(stale.do(http.MethodGet, mePath, ""), "session_revoked")
	})
	t.Run("RevokeSession", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		w.phone.stepUp()
		w.want(w.phone.call(http.MethodPost, sessionRevokePath(c.sessionID()), ""), http.StatusNoContent, "")
		w.wantCleared(c.do(http.MethodGet, mePath, ""), "session_revoked")
	})
	t.Run("RevokeDevice", func(t *testing.T) {
		c := w.newConsole()
		c.login()
		var dev string
		w.scalar(&dev, `SELECT device_id::text FROM auth_sessions WHERE id = $1`, c.sessionID().String())
		w.phone.stepUp()
		w.want(w.phone.call(http.MethodPost, deviceRevokePath(ids.MustParse(dev)), reasonJSON("Lost")), http.StatusOK, "")
		w.wantCleared(c.do(http.MethodGet, mePath, ""), "session_revoked")
	})
	t.Run("DisableUser", func(t *testing.T) {
		robin := w.localPerson("robin")
		secret := w.establish(robin.phone.client)
		c := w.newConsole()
		w.clk.Advance(30 * time.Second)
		b, _ := json.Marshal(map[string]any{"client": "console", "login_name": "robin", "password": testPassword, "totp_code": w.code(secret)})
		w.want(c.do(http.MethodPost, passwordLoginPath, string(b)), http.StatusOK, "")
		w.phone.stepUp()
		w.want(w.phone.call(http.MethodPost, disablePath(robin.user), reasonJSON("Left")), http.StatusOK, "")
		w.wantCleared(c.do(http.MethodGet, mePath, ""), "session_revoked")
	})
}

// TestConsoleViewAs (07 section 9.2, D-M6-01-7): the stored view with start, end and expiry,
// read-only, audited at start and end; logout and a step-up work during a view.
func TestConsoleViewAs(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	viewAs := "/api/v1/users/" + w.sam.String() + "/view-as"
	start := func(t *testing.T) *consoleClient {
		t.Helper()
		c := w.newConsole()
		c.login()
		w.want(c.passkeyStepUp(), http.StatusOK, "")
		w.want(c.do(http.MethodGet, viewAs, ""), http.StatusOK, "")
		if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND view_as_user_id = $2`, c.sessionID().String(), w.sam.String()); n != 1 {
			t.Fatal("the view is not stored")
		}
		return c
	}
	t.Run("ReadOnlyAndEnd", func(t *testing.T) {
		c := start(t)
		m := w.want(c.do(http.MethodGet, consoleSessionPath, ""), http.StatusOK, "")
		if v, ok := m["view_as"].(map[string]any); !ok || v["user_id"] != w.sam.String() || v["display_name"] != "sam" {
			t.Fatalf("view_as %v", m["view_as"])
		}
		p := w.want(c.do(http.MethodPatch, profilePath, `{"display_name":"Alex B"}`), http.StatusForbidden, "forbidden")
		if p["reason"] != "view_as_read_only" {
			t.Fatalf("write during the view: %v", p)
		}
		// A second view while one is stored is a write too.
		w.want(c.do(http.MethodGet, viewAs, ""), http.StatusForbidden, "forbidden")
		w.want(c.passkeyStepUp(), http.StatusOK, "")
		w.want(c.do(http.MethodDelete, consoleViewAsPath, ""), http.StatusNoContent, "")
		w.want(c.do(http.MethodDelete, consoleViewAsPath, ""), http.StatusNoContent, "")
		ends := w.audits("access.view_as.end")
		if len(ends) != 1 || ends[0].Details["ended_reason"] != "ended" || ends[0].Details["target_user_id"] != w.sam.String() {
			t.Fatalf("end entries %+v", ends)
		}
		w.want(c.do(http.MethodPatch, profilePath, `{"display_name":"Alex"}`), http.StatusOK, "")
		var start *sauditRow
		for i, e := range w.audits("access.view_as") {
			if e.Decision == "allow" {
				start = &w.audits("access.view_as")[i]
			}
		}
		if start == nil || start.Details["started_at"] == nil || start.Details["target_user_id"] != w.sam.String() {
			t.Fatalf("start entry %+v", start)
		}
	})
	t.Run("LogoutDuringView", func(t *testing.T) {
		c := start(t)
		w.want(c.do(http.MethodPost, logoutPath, ""), http.StatusNoContent, "")
	})
	t.Run("Expiry", func(t *testing.T) {
		c := start(t)
		w.clk.Advance(15 * time.Minute)
		w.want(c.do(http.MethodGet, mePath, ""), http.StatusUnauthorized, "step_up_required")
		ends := w.audits("access.view_as.end")
		if last := ends[len(ends)-1]; last.Details["ended_reason"] != "expired" {
			t.Fatalf("end entry %+v", last)
		}
		if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND view_as_user_id IS NULL`, c.sessionID().String()); n != 1 {
			t.Fatal("the view is still stored")
		}
		w.want(c.do(http.MethodGet, mePath, ""), http.StatusOK, "")
	})
}

// TestConsoleCookieNotAcceptedWithAuthorization (D-M6-01-4): both credentials in one request is
// 400.
func TestConsoleCookieNotAcceptedWithAuthorization(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	c := w.newConsole()
	c.login()
	r := c.raw(http.MethodGet, mePath, "", "Authorization", "DPoP "+w.phone.access, "DPoP", w.phone.proof(w.phone.key, http.MethodGet, mePath, w.phone.access))
	w.want(r, http.StatusBadRequest, "validation_failed")
}

// TestConsoleFormRejectedForPairingAndEnrollment (D-M6-01-1): the enrollment code operations and
// device pairing never take the console form.
func TestConsoleFormRejectedForPairingAndEnrollment(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	for _, path := range []string{"/api/v1/auth/enroll/redeem", "/api/v1/auth/pairing/complete"} {
		// The proof header is required there, so the console form is refused by the schema.
		r := w.newConsole().raw(http.MethodPost, path, `{"client":"console"}`, httpserver.HeaderConsoleRequest, "1",
			"Origin", httpservertest.PublicURL, "DPoP", "x")
		w.want(r, http.StatusBadRequest, "validation_failed")
	}
	_, r := w.begin(oidcBeginPath, nil, map[string]any{"client": "console", "app_redirect_uri": consoleLoginCallback, "enrollment_code": "en_EXAMPLE"})
	w.want(r, http.StatusBadRequest, "validation_failed")
}

// TestConsoleCanaries (SEC-T10 scope of M6-01): the password, a TOTP code, the cookie value and a
// client_verifier appear in no log line, audit detail or error body.
func TestConsoleCanaries(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	c := w.newConsole()
	c.login()
	cookie := c.cookie()
	code := w.code(w.secret)
	bodies := []string{
		string(c.passwordStepUp("canary-password-1234").Body),
		string(c.raw(http.MethodPost, logoutPath, "").Body),
	}
	verifier := randomText(t)
	b, _ := json.Marshal(map[string]any{"login_code": "lc_canary", "client_verifier": verifier})
	bodies = append(bodies, string(c.do(http.MethodPost, stepUpOIDCDonePath, string(b)).Body))
	r, _ := c.oidcStepUp(time.Second)
	bodies = append(bodies, string(r.Body))
	var details string
	w.scalar(&details, `SELECT coalesce(string_agg(details::text, ' '), '') FROM audit_events`)
	for _, canary := range []string{"canary-password-1234", testPassword, cookie, c.cookie(), verifier} {
		if canary == "" {
			t.Fatal("an empty canary")
		}
		if w.logs.Contains(canary) || w.h.Logs.Contains(canary) || strings.Contains(details, canary) {
			t.Fatalf("a canary reached a log or an audit entry")
		}
		for _, body := range bodies {
			if strings.Contains(body, canary) {
				t.Fatalf("a canary reached an answer: %s", body)
			}
		}
	}
	if strings.Contains(details, `"`+code+`"`) {
		t.Fatal("a TOTP code reached an audit entry")
	}
}
