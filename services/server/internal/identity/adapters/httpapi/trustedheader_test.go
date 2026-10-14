// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The paths of loginWithTrustedHeader and getMe.
const (
	headerLoginPath = "/api/v1/auth/trusted-header/login"
	mePath          = "/api/v1/me"
)

// The headers the proxy of the tests sets (the names of tailscale serve, 08 section 2.4).
const (
	loginHeader = "Tailscale-User-Login"
	nameHeader  = "Tailscale-User-Name"
	headerCfg   = `{"login_header":"tailscale-user-login","name_header":"Tailscale-User-Name"}`
)

// The logins of the tests (white label).
const (
	alexLogin = "alex@example.com"
	samLogin  = "sam@example.com"
)

// headerWorld is a session world whose identity trusts the given proxies and binds requests when
// bind is true; loopback is the injected flag of the public listener.
func headerWorld(t *testing.T, trusted string, bind, loopback bool) *sworld {
	t.Helper()
	cfg := sessCfg
	cfg.TrustedProxies = []netip.Prefix{netip.MustParsePrefix(trusted)}
	cfg.TrustedHeaderBindRequests = bind
	w := newSessionWorldWith(t, pgtest.New(t), clock.NewFake(sessStart), ids.New(), keyDir(t), worldOptions{cfg: func(c *app.Config) { *c = cfg }})
	w.id.SetPublicListenerLoopback(loopback)
	return w
}

// headerProvider inserts the enabled trusted_header provider and returns its id.
func (w *sworld) headerProvider(provisioning bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO identity_providers (id, kind, name, config, enabled, allow_provisioning) VALUES ($1, 'trusted_header', 'Proxy', $2, true, $3)`,
		id.String(), `{"login_header":"Tailscale-User-Login","name_header":"Tailscale-User-Name"}`, provisioning)
	return id
}

// knownPerson inserts an active person with an identity at provider for login.
func (w *sworld) knownPerson(handle string, provider ids.ID, login string) ids.ID {
	w.t.Helper()
	user := w.person(handle, false)
	w.link(user, provider.String(), "", login)
	return user
}

// headerLogin sends loginWithTrustedHeader for p with the header pairs and an optional code.
func (w *sworld) headerLogin(p *newPhone, code string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	body := map[string]any{
		"device": map[string]any{"device_id": p.id.String(), "name": "Alex Phone", "platform": "ios", "public_jwk": json.RawMessage(p.pub.JWK), "alg": "ES256"},
	}
	if code != "" {
		body["enrollment_code"] = code
	}
	b, _ := json.Marshal(body)
	proof := identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{
		HTM: http.MethodPost, HTU: httpservertest.PublicURL + headerLoginPath, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(),
	})
	req := w.h.NewRequest(http.MethodPost, headerLoginPath, string(b), "DPoP", proof)
	// A name with a leading "+" adds a further field line of the header instead of replacing it.
	for i := 0; i+1 < len(headers); i += 2 {
		if name, ok := strings.CutPrefix(headers[i], "+"); ok {
			req.Header.Add(name, headers[i+1])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	return w.h.Do(req)
}

// alexClient logs in through the header as Alex and returns the app with its tokens.
func (w *sworld) alexClient() *client {
	w.t.Helper()
	p := w.phoneKeys()
	m := w.want(w.headerLogin(p, "", loginHeader, alexLogin), http.StatusCreated, "")
	sess := m["session"].(map[string]any)
	return &client{w: w, key: p.key, user: ids.MustParse(sess["user_id"].(string)), device: p.id,
		session: ids.MustParse(sess["id"].(string)), access: m["access_token"].(string), refresh: m["refresh_token"].(string)}
}

// getMe sends getMe of c with the header pairs.
func (c *client) getMe(headers ...string) *httpservertest.Response {
	c.w.t.Helper()
	hs := append([]string{"Authorization", "DPoP " + c.access, "DPoP", c.proof(c.key, http.MethodGet, mePath, c.access)}, headers...)
	return c.w.h.Do(c.w.h.NewRequest(http.MethodGet, mePath, "", hs...))
}

// refreshAs refreshes c with the header pairs.
func (c *client) refreshAs(headers ...string) *httpservertest.Response {
	c.w.t.Helper()
	body, _ := json.Marshal(map[string]string{"refresh_token": c.refresh})
	hs := append([]string{"DPoP", c.proof(c.key, http.MethodPost, refreshPath, "")}, headers...)
	return c.w.h.Do(c.w.h.NewRequest(http.MethodPost, refreshPath, string(body), hs...))
}

// readyz serves /readyz of a registry with the trusted header check of w.
func (w *sworld) readyz() *httptest.ResponseRecorder {
	w.t.Helper()
	reg := health.NewRegistry()
	wire.RegisterTrustedHeaderReadiness(reg, w.id)
	reg.SetStarted()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, health.PathReadiness, nil))
	return rec
}

// loginDenials returns the deny entries of auth.login.complete.
func (w *sworld) loginDenials() []sauditRow {
	w.t.Helper()
	var out []sauditRow
	for _, r := range w.audits("auth.login.complete") {
		if r.Decision == "deny" {
			out = append(out, r)
		}
	}
	return out
}

// bidiLogin is a login with a right-to-left override, which the driver treats as absent.
const bidiLogin = "=?utf-8?q?sam=E2=80=AE@example.com?="

func TestIDT08_DriverOffWithoutLoopback(t *testing.T) {
	t.Parallel()
	w := headerWorld(t, "127.0.0.1/32", true, false)
	admin := w.admin()
	// Enabling is refused: create (enabled by default) and update.
	r := admin.call(http.MethodPost, providersPath, `{"kind":"trusted_header","name":"Proxy","config":`+headerCfg+`}`)
	m := w.want(r, http.StatusConflict, "conflict")
	if m["reason"] != "trusted_header_requires_loopback" || !strings.Contains(m["hint"].(string), "server.listen_addr") {
		t.Fatalf("create problem %v", m)
	}
	if n := w.count(`SELECT count(*) FROM identity_providers WHERE kind = 'trusted_header'`); n != 0 {
		t.Fatalf("%d providers after the refused create", n)
	}
	prov := ids.New()
	w.sql(`INSERT INTO identity_providers (id, kind, name, config, enabled) VALUES ($1, 'trusted_header', 'Proxy', $2, false)`,
		prov.String(), `{"login_header":"Tailscale-User-Login"}`)
	m = w.want(admin.call(http.MethodPatch, providerPath(prov.String()), `{"enabled":true}`), http.StatusConflict, "conflict")
	if m["reason"] != "trusted_header_requires_loopback" {
		t.Fatalf("update problem %v", m)
	}
	var refused int
	for _, e := range w.audits("identity_provider.manage") {
		if e.Decision == "deny" && e.Reason == "trusted_header_requires_loopback" {
			refused++
		}
	}
	// Equal denials within the aggregation window share one entry (05 section 2.4).
	if refused == 0 {
		t.Fatal("no deny entry for the refused enabling")
	}
	// Enabled anyway (an older start or a direct write): the driver stays off and readiness fails.
	w.sql(`UPDATE identity_providers SET enabled = true WHERE id = $1`, prov.String())
	w.knownPerson("robin", prov, alexLogin)
	rec := w.readyz()
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"failed":["trusted_header_requires_loopback"]`) {
		t.Fatalf("readyz %d %s", rec.Code, rec.Body)
	}
	w.want(w.headerLogin(w.phoneKeys(), "", loginHeader, alexLogin), http.StatusNotFound, "not_found")
	// With the loopback listener the same provider is ready and logs in.
	w.id.SetPublicListenerLoopback(true)
	if rec := w.readyz(); rec.Code != http.StatusOK {
		t.Fatalf("readyz with loopback %d %s", rec.Code, rec.Body)
	}
	w.want(w.headerLogin(w.phoneKeys(), "", loginHeader, alexLogin), http.StatusCreated, "")
}

func TestSECT01_HeaderFromUntrustedPeerIgnored(t *testing.T) {
	t.Parallel()
	// The peer of the test client is 127.0.0.1, outside the trusted documentation range.
	w := headerWorld(t, "192.0.2.0/24", true, true)
	prov := w.headerProvider(true)
	w.knownPerson("alex", prov, alexLogin)
	w.want(w.headerLogin(w.phoneKeys(), "", loginHeader, alexLogin, "X-Forwarded-For", "192.0.2.10"), http.StatusUnauthorized, "unauthenticated")
	if n := w.count(`SELECT count(*) FROM auth_sessions`) + w.count(`SELECT count(*) FROM principals WHERE status = 'pending'`); n != 0 {
		t.Fatalf("an untrusted peer created %d rows", n)
	}
}

func TestBinding_OffDoesNotReadHeader(t *testing.T) {
	t.Parallel()
	w := headerWorld(t, "127.0.0.1/32", false, true)
	prov := w.headerProvider(false)
	w.knownPerson("alex", prov, alexLogin)
	c := w.alexClient()
	w.want(c.getMe(loginHeader, samLogin), http.StatusOK, "")
	w.want(c.getMe(), http.StatusOK, "")
	w.want(c.refreshAs(), http.StatusOK, "")
	if got := w.sessionState(c.session); got != "" {
		t.Fatalf("revoke_reason %q with binding off", got)
	}
}

// TestTrustedHeaderLogin runs the login and binding cases on one world with binding on and a
// provider without provisioning, in order: a world costs seconds, a case tenths (as M1-44/13).
func TestTrustedHeaderLogin(t *testing.T) {
	t.Parallel()
	w := headerWorld(t, "127.0.0.1/32", true, true)
	prov := w.headerProvider(false)
	w.knownPerson("alex", prov, alexLogin)
	t.Run("TestIDT09_MissingLoginHeader401", func(t *testing.T) {
		// A tagged device of the tailnet reaches the server without the login header.
		w.want(w.headerLogin(w.phoneKeys(), ""), http.StatusUnauthorized, "unauthenticated")
		if d := w.loginDenials(); len(d) != 1 || d[0].Reason != "unauthenticated" || d[0].Details["cause"] != "header_absent" {
			t.Fatalf("denials %+v", d)
		}
		if n := w.count(`SELECT count(*) FROM auth_sessions`); n != 0 {
			t.Fatalf("%d sessions after a login without header", n)
		}
	})
	t.Run("KnownIdentity", func(t *testing.T) { knownIdentity(t, w, prov) })
	t.Run("TestIDT10_BindingMismatchRevokesSession", func(t *testing.T) { bindingMismatch(t, w) })
	t.Run("TestBinding_MissingHeaderRevokes", func(t *testing.T) {
		c := w.alexClient()
		w.want(c.getMe(), http.StatusUnauthorized, "session_revoked")
		if got := w.sessionState(c.session); got != "trusted_header_mismatch" {
			t.Fatalf("revoke_reason %q", got)
		}
		// The refresh path binds too: a second session, refreshed with another login.
		d := w.alexClient()
		w.want(d.refreshAs(loginHeader, samLogin), http.StatusUnauthorized, "session_revoked")
		if got := w.sessionState(d.session); got != "trusted_header_mismatch" {
			t.Fatalf("refresh: revoke_reason %q", got)
		}
	})
	t.Run("DisabledPerson401", func(t *testing.T) {
		dana := w.knownPerson("dana", prov, "dana@example.com")
		w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, dana.String())
		w.want(w.headerLogin(w.phoneKeys(), "", loginHeader, "dana@example.com"), http.StatusUnauthorized, "unauthenticated")
	})
	t.Run("MultipleValuesAbsent", func(t *testing.T) {
		w.want(w.headerLogin(w.phoneKeys(), "", loginHeader, alexLogin, "+"+loginHeader, alexLogin), http.StatusUnauthorized, "unauthenticated")
		if !w.logs.Has("trusted header login with several values") {
			t.Fatal("no warning for several values")
		}
	})
	t.Run("NoMappingNeutral401", func(t *testing.T) {
		before := w.count(`SELECT count(*) FROM principals`)
		r := w.headerLogin(w.phoneKeys(), "", loginHeader, samLogin)
		w.want(r, http.StatusUnauthorized, "unauthenticated")
		wrong := w.headerLogin(w.phoneKeys(), "en_0000-0000-0000-0000-0000-0000-0000-0000", loginHeader, samLogin)
		if a, b := withoutCorrelation(t, r), withoutCorrelation(t, wrong); a != b {
			t.Fatalf("bodies differ:\n%s\n%s", a, b)
		}
		if n := w.count(`SELECT count(*) FROM principals`); n != before {
			t.Fatalf("%d principals after an unmapped login, want %d", n, before)
		}
	})
	t.Run("EnrollmentCodeLinks", func(t *testing.T) { enrollmentLinks(t, w, prov) })
}

// knownIdentity logs Alex in through the header: a pending new device, the identity and
// last_login_at on the session, and the allow entry with provider and device.
func knownIdentity(t *testing.T, w *sworld, prov ids.ID) {
	t.Helper()
	p := w.phoneKeys()
	r := w.headerLogin(p, "", strings.ToLower(loginHeader), alexLogin)
	m := w.want(r, http.StatusCreated, "")
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control %q", r.Header.Get("Cache-Control"))
	}
	sess, dev := m["session"].(map[string]any), m["device"].(map[string]any)
	if sess["auth_method"] != "trusted_header" || dev["state"] != "pending" {
		t.Fatalf("session %v device %v", sess, dev)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions s JOIN user_identities i ON i.id = s.identity_id
		WHERE s.id = $1 AND i.subject = $2 AND i.user_id = s.user_id AND i.last_login_at IS NOT NULL`, sess["id"], alexLogin); n != 1 {
		t.Fatal("session without the identity or last_login_at")
	}
	var allow *sauditRow
	for _, e := range w.audits("auth.login.complete") {
		if e.Decision == "allow" {
			allow = &e
		}
	}
	if allow == nil || allow.Details["provider_id"] != prov.String() || allow.Details["device_id"] != p.id.String() {
		t.Fatalf("allow entry %+v", allow)
	}
}

// bindingMismatch is ID-T10: a later request with another login revokes the session with the
// reason, its family, session.revoked and an audit entry; the refresh fails afterwards.
func bindingMismatch(t *testing.T, w *sworld) {
	t.Helper()
	c := w.alexClient()
	w.want(c.getMe(loginHeader, alexLogin), http.StatusOK, "")
	w.want(c.getMe(loginHeader, samLogin), http.StatusUnauthorized, "session_revoked")
	if got := w.sessionState(c.session); got != "trusted_header_mismatch" {
		t.Fatalf("revoke_reason %q", got)
	}
	if reason, _ := w.familyState(c.session); reason != "session_revoked" {
		t.Fatalf("family %q", reason)
	}
	var event bool
	for _, ev := range w.revokedEvents() {
		event = event || ev["resource"] == c.session.String() && strings.Contains(ev["payload"], "trusted_header_mismatch")
	}
	if !event {
		t.Fatal("no session.revoked with trusted_header_mismatch")
	}
	var found bool
	for _, r := range w.audits("self.read") {
		found = found || r.Decision == "deny" && r.Reason == "session_revoked" && r.ResourceID == c.session.String()
	}
	if !found {
		t.Fatal("no deny entry session_revoked for self.read")
	}
	w.want(c.refreshAs(loginHeader, alexLogin), http.StatusUnauthorized, "session_revoked")
}

// enrollmentLinks links Sam's login through an identity_link code: device active, code used,
// identity linked, identity.link audited; a wrong code links nothing.
func enrollmentLinks(t *testing.T, w *sworld, prov ids.ID) {
	t.Helper()
	sam := w.person("sam", false)
	var code app.IssuedEnrollmentCode
	if err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		code, err = w.id.Enrollment.Issue(ctx, sam, app.PurposeIdentityLink, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	w.want(w.headerLogin(w.phoneKeys(), "en_0000-0000-0000-0000-0000-0000-0000-0000", loginHeader, samLogin), http.StatusUnauthorized, "unauthenticated")
	p := w.phoneKeys()
	m := w.want(w.headerLogin(p, code.Code, loginHeader, samLogin), http.StatusCreated, "")
	if dev := m["device"].(map[string]any); dev["state"] != "active" {
		t.Fatalf("device %v", dev)
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_at IS NOT NULL AND used_by_device_id = $2`, code.ID.String(), p.id.String()); n != 1 {
		t.Fatal("code not used by the device")
	}
	if n := w.count(`SELECT count(*) FROM user_identities WHERE user_id = $1 AND provider_id = $2 AND subject = $3`, sam.String(), prov.String(), samLogin); n != 1 {
		t.Fatal("identity not linked")
	}
	if links := w.audits("identity.link"); len(links) != 1 || links[0].Decision != "allow" {
		t.Fatalf("identity.link entries %+v", links)
	}
}

// TestTrustedHeaderProvisioning runs the cases of a provider with provisioning on one world.
func TestTrustedHeaderProvisioning(t *testing.T) {
	t.Parallel()
	w := headerWorld(t, "127.0.0.1/32", true, true)
	prov := w.headerProvider(true)
	w.knownPerson("alex", prov, alexLogin)
	t.Run("ProvisioningPending", func(t *testing.T) {
		w.person("robin.doe", false)
		m := w.want(w.headerLogin(w.phoneKeys(), "", loginHeader, "Robin.Doe@example.com", nameHeader, "=?utf-8?q?R=C3=B6bin?="), http.StatusCreated, "")
		if dev := m["device"].(map[string]any); dev["state"] != "pending" {
			t.Fatalf("device %v", dev)
		}
		user := m["session"].(map[string]any)["user_id"].(string)
		var status, handle, name string
		w.scalar(&status, `SELECT status FROM principals WHERE id = $1`, user)
		w.scalar(&handle, `SELECT handle FROM principals WHERE id = $1`, user)
		w.scalar(&name, `SELECT display_name FROM principals WHERE id = $1`, user)
		if status != "pending" || !strings.HasPrefix(handle, "robin.doe-") || name != "Röbin" {
			t.Fatalf("person %s %s %s", status, handle, name)
		}
		if n := w.count(`SELECT count(*) FROM users WHERE principal_id = $1 AND email IS NULL`, user); n != 1 {
			t.Fatal("provisioned person has an e-mail")
		}
	})
	t.Run("TestTrustedHeaderNeverLogged", func(t *testing.T) { neverLogged(t, w) })
}

// neverLogged checks that no header value reaches a problem body, an audit detail or a log record
// (SEC-LOG-2).
func neverLogged(t *testing.T, w *sworld) {
	t.Helper()
	c := w.alexClient()
	bodies := []string{
		string(w.headerLogin(w.phoneKeys(), "", loginHeader, samLogin, "+"+loginHeader, samLogin).Body),
		string(w.headerLogin(w.phoneKeys(), "", loginHeader, bidiLogin).Body),
		string(c.getMe(loginHeader, samLogin).Body),
	}
	var audit []string
	for _, act := range []string{"auth.login.complete", "self.read", "identity.link"} {
		for _, r := range w.audits(act) {
			audit = append(audit, r.Raw)
		}
	}
	var logs []string
	for _, r := range append(w.logs.Records(), w.h.Logs.Records()...) {
		logs = append(logs, r.Message)
		for k, v := range r.Attrs {
			logs = append(logs, k+"="+v)
		}
	}
	all := strings.Join(append(append(bodies, audit...), logs...), "\n")
	for _, secret := range []string{alexLogin, samLogin, "sam\u202e"} {
		if strings.Contains(all, secret) {
			t.Fatalf("%q appears in a body, audit detail or log", secret)
		}
	}
}
