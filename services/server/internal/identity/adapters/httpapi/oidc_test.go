// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// oidcCase is one case of an OIDC group; the cases of a group share one world and run in order.
type oidcCase struct {
	name string
	fn   func(t *testing.T, w *oworld)
}

// runOIDCCases runs the cases on one new OIDC world, each as a subtest named after the test of the
// brief; w.t is the subtest while it runs. One world per group keeps the package inside the
// default test timeout on a loaded machine (as runTokenCases, docs/DECISIONS.md M1-41/13). Cases
// that move the clock come last in their group.
func runOIDCCases(t *testing.T, o oidcWorldOpts, cases []oidcCase) {
	t.Parallel()
	w := newOIDCWorld(t, o)
	for _, c := range cases {
		// Denials of one client aggregate per window (R-AZ-04); each case starts with the earlier
		// ones written, so its own deny entries carry its own details.
		if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Run(c.name, func(t *testing.T) {
			prev := w.t
			w.t = t
			defer func() { w.t = prev }()
			c.fn(t, w)
		})
	}
}

// TestOIDC_Login: begin, callback and complete against the fake provider (ID-T01, ID-T03, ID-T04,
// mapping, enrollment code, leaked URL, provisioning, single use, expiry, re-login, cleanup).
func TestOIDC_Login(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{}, []oidcCase{
		{"TestIDT01_PKCEDowngradeRejected", caseIDT01PKCEDowngradeRejected},
		{"TestIDT04_NoMappingByEmail", caseIDT04NoMappingByEmail},
		{"TestOIDCLogin_KnownIdentity", caseKnownIdentity},
		{"TestOIDCLogin_EnrollmentCodeLinksAndActivatesDevice", caseEnrollmentCodeLinks},
		{"TestOIDCLogin_LeakedAuthorizationURLCannotLink", caseLeakedAuthorizationURL},
		{"TestOIDCLogin_ProvisioningPending", caseProvisioningPending},
		{"TestOIDCLogin_DisabledPersonRejected", caseDisabledPersonRejected},
		{"TestOIDCLogin_LoginCodeSingleUse", caseLoginCodeSingleUse},
		{"TestOIDCCallback_Replay", caseCallbackReplay},
		{"TestOIDCCallback_ProviderError", caseProviderError},
		{"TestIDT03_IDTokenRejected", caseIDT03IDTokenRejected},
		{"TestOIDCSecretsNeverLogged", caseSecretsNeverLogged},
		{"TestOIDCBegin_Checks", caseBeginChecks},
		{"TestOIDCLogin_LoginCodeExpires", caseLoginCodeExpires},
		{"TestOIDCLogin_FlowExpires", caseFlowExpires},
		{"TestLoginFlowCleanup", caseLoginFlowCleanup},
		{"TestIDPRecheckForcesRelogin", caseIDPRecheckForcesRelogin},
	})
}

// TestOIDC_IssParameter: a provider that sends the iss parameter (ID-T02).
func TestOIDC_IssParameter(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{idp: oidctest.Options{SendIss: true}}, []oidcCase{
		{"TestIDT02_StateIssAndVerifier", caseIDT02StateIssAndVerifier},
	})
}

// TestOIDC_Entra: the Entra issuer template with a tenant allow-list (ID-T05).
func TestOIDC_Entra(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{idp: oidctest.Options{Entra: true, SendIss: true}, tenants: []string{"t-allowed"}}, []oidcCase{
		{"TestIDT05_EntraTenantAllowList", caseIDT05EntraTenantAllowList},
	})
}

// TestIdentityLink: linking a further identity (identity.link with step-up).
func TestIdentityLink(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{}, []oidcCase{
		{"TestIdentityLink_OtherActor404", caseLinkOtherActor404},
		{"TestIdentityLink_SubjectAlreadyLinked409", caseLinkSubjectAlreadyLinked409},
		{"TestIdentityLink_Success", caseLinkSuccess},
		{"TestIdentityLink_StepUpRequired", caseLinkStepUpRequired},
	})
}

// login returns a provider account with a subject unique in the world.
func (w *oworld) login() oidctest.Login {
	h := w.handle("p")
	return oidctest.Login{Subject: h + "-sub", Name: "Alex Example", PreferredUsername: h + "@example.com", Email: h + "@example.com"}
}

// knownPerson inserts a person with an identity for a new provider account and returns both.
func (w *oworld) knownPerson() (ids.ID, oidctest.Login) {
	w.t.Helper()
	l := w.login()
	p := w.person(w.handle("alex"), false)
	w.linkIdentity(p, l.Subject)
	return p, l
}

// linkCode issues an identity_link enrollment code for a new person without a local identity and
// returns the person and the code.
func (w *oworld) linkCode() (ids.ID, app.IssuedEnrollmentCode) {
	w.t.Helper()
	p := w.person(w.handle("sam"), false)
	var out app.IssuedEnrollmentCode
	err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		out, err = w.id.Enrollment.Issue(ctx, p, app.PurposeIdentityLink, nil)
		return err
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return p, out
}

// auditOf returns the entries of action whose details name the flow attempt.
func (w *oworld) auditOf(action, attempt string) []sauditRow {
	w.t.Helper()
	var out []sauditRow
	for _, a := range w.audits(action) {
		if a.Details["login_attempt_id"] == attempt {
			out = append(out, a)
		}
	}
	return out
}

// assertFailed checks a callback answer that redirects with code and no login_code, the flow
// failed with code and one deny entry naming the flow.
func assertFailed(t *testing.T, w *oworld, r *httpservertest.Response, attempt, code string) {
	t.Helper()
	if r.StatusCode != http.StatusFound {
		t.Fatalf("status %d: %s", r.StatusCode, r.Body)
	}
	loc := r.Header.Get("Location")
	if !strings.HasPrefix(loc, appRedirect+"?") || !strings.Contains(loc, "error="+code) || strings.Contains(loc, "login_code") ||
		!strings.Contains(loc, "login_attempt_id="+attempt) {
		t.Fatalf("Location %q", loc)
	}
	if st, e := w.flowStatus(attempt); st != "failed" || e != code {
		t.Fatalf("flow %s %s, want failed %s", st, e, code)
	}
	d := w.auditOf("auth.login.begin", attempt)
	if len(d) != 1 || d[0].Decision != "deny" || d[0].Details["error"] != code {
		t.Fatalf("deny entries of flow %s: %+v", attempt, d)
	}
}

// mustQuery returns the query of raw.
func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

// linkBody is the IdentityLinkComplete of code and verifier.
func linkBody(code, verifier string) string {
	b, _ := json.Marshal(map[string]string{"login_code": code, "client_verifier": verifier})
	return string(b)
}

func caseIDT01PKCEDowngradeRejected(t *testing.T, w *oworld) {
	flows := w.count(`SELECT count(*) FROM auth_login_flows`)
	for name, challenge := range map[string]any{"missing": nil, "plain verifier": strings.Repeat("a", 50), "short": "abc", "padded": strings.Repeat("A", 42) + "="} {
		_, r := w.begin(oidcBeginPath, nil, map[string]any{"client_challenge": challenge})
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d: %s", name, r.StatusCode, r.Body)
		}
	}
	if n := w.count(`SELECT count(*) FROM auth_login_flows`); n != flows {
		t.Fatalf("%d flows after refused begins, want %d", n, flows)
	}
	// Every exchange the provider sees uses S256 and the verifier of its challenge (the fake
	// provider refuses anything else).
	_, l := w.knownPerson()
	before, _ := w.idp.Seen()
	for range 2 {
		f, code := w.loginCode(l, nil)
		w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	}
	methods, verifiers := w.idp.Seen()
	methods, verifiers = methods[len(before):], verifiers[len(verifiers)-2:]
	if len(methods) != 2 || methods[0] != "S256" || methods[1] != "S256" || verifiers[0] == verifiers[1] || !oidcflow.ValidVerifier(verifiers[0]) {
		t.Fatalf("methods %v, verifiers %d", methods, len(verifiers))
	}
}

func caseIDT02StateIssAndVerifier(t *testing.T, w *oworld) {
	_, l := w.knownPerson()
	// Unknown state: 400 without redirect, audit deny.
	r := w.h.Do(w.h.NewRequest(http.MethodGet, oidcCallbackPath+"?state=unknown&code=x", ""))
	w.want(r, http.StatusBadRequest, "validation_failed")
	if r.Header.Get("Location") != "" {
		t.Fatal("redirect for an unknown state")
	}
	if d := w.audits("auth.login.begin"); len(d) != 1 || d[0].Decision != "deny" || d[0].Details["cause"] != "unknown_flow" {
		t.Fatalf("audit %+v", d)
	}
	// A foreign iss parameter: redirect with login_failed, no login_code.
	f, br := w.begin(oidcBeginPath, nil, nil)
	w.want(br, http.StatusOK, "")
	w.idp.NextLogin(l)
	loc := w.idp.Authorize(t, f.authURL)
	q := loc.Query()
	q.Set("iss", "https://idp.example.org")
	r = w.h.Do(w.h.NewRequest(http.MethodGet, oidcCallbackPath+"?"+q.Encode(), ""))
	assertFailed(t, w, r, f.attempt, "login_failed")
	// A missing iss while discovery announces it: login_failed.
	f, br = w.begin(oidcBeginPath, nil, nil)
	w.want(br, http.StatusOK, "")
	w.idp.OmitIss()
	assertFailed(t, w, w.callback(f, l), f.attempt, "login_failed")
	w.idp.Reset()
	// A wrong client_verifier at complete: 401, flow failed, audit deny.
	f, code := w.loginCode(l, nil)
	w.want(w.complete(w.phoneKeys(), code, randomText(t)), http.StatusUnauthorized, "unauthenticated")
	if st, e := w.flowStatus(f.attempt); st != "failed" || e != "login_failed" {
		t.Fatalf("flow %s %s", st, e)
	}
	d := w.auditOf("auth.login.complete", f.attempt)
	if len(d) != 1 || d[0].Decision != "deny" || d[0].Details["cause"] != "verifier_mismatch" {
		t.Fatalf("audit %+v", d)
	}
	// The right verifier cannot redeem the failed flow afterwards.
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusUnauthorized, "unauthenticated")
	if n := w.count(`SELECT count(*) FROM auth_sessions`); n != 0 {
		t.Fatalf("%d sessions", n)
	}
}

func caseIDT03IDTokenRejected(t *testing.T, w *oworld) {
	_, l := w.knownPerson()
	cases := map[string]func(){
		"alg none":                  func() { w.idp.SetHeader("alg", "none") },
		"HS256":                     func() { w.idp.SetHeader("alg", "HS256") },
		"wrong aud":                 func() { w.idp.SetClaim("aud", "other") },
		"wrong nonce":               func() { w.idp.SetClaim("nonce", "other") },
		"expired":                   func() { w.idp.SetClaim("exp", w.clk.Now().Add(-time.Hour).Unix()) },
		"two audiences without azp": func() { w.idp.SetClaim("aud", []string{clientID, "other"}) },
		"foreign iss":               func() { w.idp.SetClaim("iss", "https://idp.example.org") },
	}
	defer w.idp.Reset()
	for name, knob := range cases {
		w.idp.Reset()
		knob()
		f, r := w.begin(oidcBeginPath, nil, nil)
		w.want(r, http.StatusOK, "")
		assertFailed(t, w, w.callback(f, l), f.attempt, "login_failed")
		if n := w.count(`SELECT count(*) FROM auth_login_flows WHERE id = $1 AND login_code_hash IS NOT NULL`, f.attempt); n != 0 {
			t.Fatalf("%s: login code issued", name)
		}
	}
}

func caseIDT04NoMappingByEmail(t *testing.T, w *oworld) {
	h := w.handle("alex")
	p := w.person(h, false)
	w.sql(`UPDATE users SET email = $2 WHERE principal_id = $1`, p.String(), h+"@example.com")
	f, code := w.loginCode(oidctest.Login{Subject: h + "-unknown", Email: h + "@example.com", Name: h}, nil)
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusUnauthorized, "unauthenticated")
	if n := w.count(`SELECT count(*) FROM user_identities WHERE subject = $1 OR user_id = $2`, h+"-unknown", p.String()); n != 0 {
		t.Fatalf("%d identities", n)
	}
	if st, _ := w.flowStatus(f.attempt); st != "failed" {
		t.Fatalf("flow %s", st)
	}
	if d := w.auditOf("auth.login.complete", f.attempt); len(d) != 1 || d[0].Decision != "deny" || d[0].Details["cause"] != "no_mapping" {
		t.Fatalf("audit %+v", d)
	}
}

func caseIDT05EntraTenantAllowList(t *testing.T, w *oworld) {
	p := w.person("alex", false)
	// The identity is stored under the tenant's issuer.
	w.sql(`INSERT INTO user_identities (id, user_id, provider_id, subject, issuer) VALUES ($1, $2, $3, $4, $5)`,
		ids.New().String(), p.String(), w.prov.String(), "alex-sub", w.idp.TenantIssuer("t-allowed"))
	f, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	assertFailed(t, w, w.callback(f, oidctest.Login{Subject: "alex-sub", TenantID: "t-other"}), f.attempt, "login_failed")
	f, code := w.loginCode(oidctest.Login{Subject: "alex-sub", TenantID: "t-allowed"}, nil)
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
}

func caseKnownIdentity(t *testing.T, w *oworld) {
	h := w.handle("alex")
	p := w.person(h, false)
	identity := w.linkIdentity(p, h+"-sub")
	f, code := w.loginCode(oidctest.Login{Subject: h + "-sub"}, nil)
	// While the code waits, the flow holds the verified identity, never the provider tokens.
	var ext string
	w.scalar(&ext, `SELECT external_identity::text FROM auth_login_flows WHERE id = $1`, f.attempt)
	if !strings.Contains(ext, `"subject": "`+h+`-sub"`) || strings.Contains(ext, "provider-") {
		t.Fatalf("external identity %s", ext)
	}
	m := w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	sess := m["session"].(map[string]any)
	dev := m["device"].(map[string]any)
	if sess["auth_method"] != "oidc" || sess["user_id"] != p.String() || dev["state"] != "pending" {
		t.Fatalf("session %v device %v", sess, dev)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE identity_id = $1 AND auth_method = 'oidc'`, identity.String()); n != 1 {
		t.Fatal("session without the identity")
	}
	if n := w.count(`SELECT count(*) FROM user_identities WHERE id = $1 AND last_login_at IS NOT NULL`, identity.String()); n != 1 {
		t.Fatal("last_login_at not set")
	}
	if n := w.count(`SELECT count(*) FROM auth_login_flows WHERE id = $1 AND status = 'completed' AND external_identity IS NULL`, f.attempt); n != 1 {
		t.Fatal("flow not completed or identity kept")
	}
	found := false
	for _, a := range w.audits("auth.login.complete") {
		if a.Details["session_id"] == sess["id"] && a.Decision == "allow" && a.Details["auth_method"] == "oidc" {
			found = true
		}
	}
	if !found {
		t.Fatal("no allow entry for the session")
	}
}

func caseEnrollmentCodeLinks(t *testing.T, w *oworld) {
	p, c := w.linkCode()
	l := w.login()
	f, code := w.loginCode(l, map[string]any{"enrollment_code": c.Code})
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_at IS NULL`, c.ID.String()); n != 1 {
		t.Fatal("the code was consumed before complete")
	}
	ph := w.phoneKeys()
	m := w.want(w.complete(ph, code, f.verifier), http.StatusCreated, "")
	if m["device"].(map[string]any)["state"] != "active" || m["session"].(map[string]any)["user_id"] != p.String() {
		t.Fatalf("%v", m)
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_at IS NOT NULL AND used_by_device_id = $2`,
		c.ID.String(), ph.id.String()); n != 1 {
		t.Fatal("code not consumed by the device")
	}
	if n := w.count(`SELECT count(*) FROM user_identities WHERE user_id = $1 AND subject = $2 AND issuer = $3 AND email_at_link = $4`,
		p.String(), l.Subject, w.idp.Issuer(), l.Email); n != 1 {
		t.Fatal("identity not linked")
	}
	var links []sauditRow
	for _, a := range w.audits("identity.link") {
		if a.Details["enrollment_code_id"] == c.ID.String() {
			links = append(links, a)
		}
	}
	if len(links) != 1 || links[0].Decision != "allow" {
		t.Fatalf("audit %+v", links)
	}
	var actorID string
	w.scalar(&actorID, `SELECT coalesce(actor_principal_id::text, '') FROM audit_events WHERE action = 'identity.link' AND details->>'enrollment_code_id' = $1`,
		c.ID.String())
	if actorID != p.String() {
		t.Fatalf("actor %q", actorID)
	}
	// A second use of the same code fails already at begin.
	_, r := w.begin(oidcBeginPath, nil, map[string]any{"enrollment_code": c.Code})
	w.want(r, http.StatusUnauthorized, "unauthenticated")
}

func caseLeakedAuthorizationURL(t *testing.T, w *oworld) {
	_, c := w.linkCode()
	// The app of the code's person begins; the URL leaks, and another person finishes the provider
	// login with an own account and obtains the login_code from the redirect.
	f, br := w.begin(oidcBeginPath, nil, map[string]any{"enrollment_code": c.Code})
	w.want(br, http.StatusOK, "")
	other := w.login()
	w.want(w.callback(f, other), http.StatusFound, "")
	code := f.redirect.Query().Get("login_code")
	if code == "" {
		t.Fatal("no login_code")
	}
	// After the callback no account has changed.
	if n := w.count(`SELECT count(*) FROM user_identities WHERE subject = $1`, other.Subject); n != 0 {
		t.Fatal("the callback linked an identity")
	}
	// Without the verifier of the app that began the flow the code is worthless.
	w.want(w.complete(w.phoneKeys(), code, randomText(t)), http.StatusUnauthorized, "unauthenticated")
	if n := w.count(`SELECT count(*) FROM user_identities WHERE subject = $1`, other.Subject); n != 0 {
		t.Fatal("identity linked without the verifier")
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_at IS NULL`, c.ID.String()); n != 1 {
		t.Fatal("the code was consumed")
	}
}

func caseProvisioningPending(t *testing.T, w *oworld) {
	w.sql(`UPDATE identity_providers SET allow_provisioning = true WHERE id = $1`, w.prov.String())
	defer w.sql(`UPDATE identity_providers SET allow_provisioning = false WHERE id = $1`, w.prov.String())
	w.person("robin", false) // the handle is taken: the new person gets a suffix
	sub := w.handle("robin") + "-sub"
	f, code := w.loginCode(oidctest.Login{Subject: sub, Name: "Robin\u202e Example", PreferredUsername: "Robin@example.com", Email: "robin@example.com"}, nil)
	ph := w.phoneKeys()
	m := w.want(w.complete(ph, code, f.verifier), http.StatusCreated, "")
	user := m["session"].(map[string]any)["user_id"].(string)
	var status, handle, name string
	w.scalar(&status, `SELECT status FROM principals WHERE id = $1`, user)
	w.scalar(&handle, `SELECT handle FROM principals WHERE id = $1`, user)
	w.scalar(&name, `SELECT display_name FROM principals WHERE id = $1`, user)
	if status != "pending" || !strings.HasPrefix(handle, "robin-") || len(handle) != len("robin-")+4 || name != "Robin Example" {
		t.Fatalf("status %s handle %s name %q", status, handle, name)
	}
	if n := w.count(`SELECT count(*) FROM users WHERE principal_id = $1 AND email IS NULL`, user); n != 1 {
		t.Fatal("users.email is not NULL")
	}
	if n := w.count(`SELECT count(*) FROM grants WHERE subject_principal_id = $1`, user); n != 0 {
		t.Fatal("the provisioned person has grants")
	}
	if m["device"].(map[string]any)["state"] != "pending" {
		t.Fatalf("device %v", m["device"])
	}
	// The tokens give the self information only.
	c := &client{w: w.sworld, key: ph.key, access: m["access_token"].(string)}
	w.want(c.call(http.MethodGet, "/api/v1/me", ""), http.StatusOK, "")
	found := false
	for _, a := range w.audits("auth.login.complete") {
		if a.Details["session_id"] == m["session"].(map[string]any)["id"] && a.Details["provisioned"] == true {
			found = true
		}
	}
	if !found {
		t.Fatal("the allow entry does not say provisioned")
	}
	// A second login of the pending person maps to the same person.
	f, code = w.loginCode(oidctest.Login{Subject: sub}, nil)
	m = w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	if m["session"].(map[string]any)["user_id"] != user {
		t.Fatal("second login made another person")
	}
}

func caseDisabledPersonRejected(t *testing.T, w *oworld) {
	p, l := w.knownPerson()
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, p.String())
	f, code := w.loginCode(l, nil)
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusUnauthorized, "unauthenticated")
	if n := w.count(`SELECT (SELECT count(*) FROM devices WHERE user_id = $1) + (SELECT count(*) FROM auth_sessions WHERE user_id = $1)`,
		p.String()); n != 0 {
		t.Fatalf("%d rows written", n)
	}
}

func caseLoginCodeSingleUse(t *testing.T, w *oworld) {
	_, l := w.knownPerson()
	f, code := w.loginCode(l, nil)
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusUnauthorized, "unauthenticated")
}

func caseLoginCodeExpires(t *testing.T, w *oworld) {
	_, l := w.knownPerson()
	f, code := w.loginCode(l, nil)
	w.clk.Advance(sessCfg.ServerCodeTTL)
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusUnauthorized, "unauthenticated")
}

func caseFlowExpires(t *testing.T, w *oworld) {
	f, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	w.clk.Advance(sessCfg.LoginFlowTTL)
	cb := w.callback(f, w.login())
	w.want(cb, http.StatusBadRequest, "validation_failed")
	if cb.Header.Get("Location") != "" {
		t.Fatal("redirect for an expired flow")
	}
}

func caseCallbackReplay(t *testing.T, w *oworld) {
	f, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	w.idp.NextLogin(w.login())
	loc := w.idp.Authorize(t, f.authURL)
	tokens := w.idp.Count("token")
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, loc.Path+"?"+loc.RawQuery, "")), http.StatusFound, "")
	// The same redirect again: the state is used, 400 without redirect.
	again := w.h.Do(w.h.NewRequest(http.MethodGet, loc.Path+"?"+loc.RawQuery, ""))
	w.want(again, http.StatusBadRequest, "validation_failed")
	if n := w.idp.Count("token") - tokens; n != 1 {
		t.Fatalf("%d token requests", n)
	}
}

func caseProviderError(t *testing.T, w *oworld) {
	f, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	w.idp.FailAuthorize("access_denied")
	defer w.idp.Reset()
	assertFailed(t, w, w.callback(f, w.login()), f.attempt, "access_denied")
}

func caseBeginChecks(t *testing.T, w *oworld) {
	_, r := w.begin(oidcBeginPath, nil, map[string]any{"provider_id": ids.New().String()})
	w.want(r, http.StatusNotFound, "not_found")
	for _, uri := range []string{"http://localhost:4711/callback", "http://127.0.0.1:4711/other", "com.example.other:/oidc"} {
		_, r = w.begin(oidcBeginPath, nil, map[string]any{"app_redirect_uri": uri})
		w.want(r, http.StatusBadRequest, "validation_failed")
		if !strings.Contains(string(r.Body), "/app_redirect_uri") {
			t.Fatalf("%s: %s", uri, r.Body)
		}
	}
	// The configured private-scheme redirect works, and the authorization URL carries the
	// server's own request.
	f, r := w.begin(oidcBeginPath, nil, map[string]any{"app_redirect_uri": "com.example.chat:/oidc"})
	w.want(r, http.StatusOK, "")
	q := mustQuery(t, f.authURL)
	if q.Get("response_type") != "code" || q.Get("client_id") != clientID || q.Get("scope") != "openid profile email" ||
		q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || q.Get("state") == "" || q.Get("nonce") == "" ||
		q.Get("redirect_uri") != httpservertest.PublicURL+oidcCallbackPath {
		t.Fatalf("authorization URL %s", f.authURL)
	}
	// State and server challenge are not stored in clear.
	if n := w.count(`SELECT count(*) FROM auth_login_flows WHERE encode(oidc_state_hash, 'escape') = $1 OR client_challenge = $2`,
		q.Get("state"), q.Get("code_challenge")); n != 0 {
		t.Fatal("state or server challenge stored in clear")
	}
	// A wrong enrollment code: 401 without detail.
	_, r = w.begin(oidcBeginPath, nil, map[string]any{"enrollment_code": "en_0000-0000-0000-0000-0000-0000-0000-0000"})
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	// A disabled provider is the same not_found.
	w.sql(`UPDATE identity_providers SET enabled = false WHERE id = $1`, w.prov.String())
	defer w.sql(`UPDATE identity_providers SET enabled = true WHERE id = $1`, w.prov.String())
	_, r = w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusNotFound, "not_found")
}

func caseSecretsNeverLogged(t *testing.T, w *oworld) {
	_, c := w.linkCode()
	f, r := w.begin(oidcBeginPath, nil, map[string]any{"enrollment_code": c.Code})
	w.want(r, http.StatusOK, "")
	q := mustQuery(t, f.authURL)
	w.idp.NextLogin(w.login())
	loc := w.idp.Authorize(t, f.authURL)
	providerCode := loc.Query().Get("code")
	cb := w.h.Do(w.h.NewRequest(http.MethodGet, loc.Path+"?"+loc.RawQuery, ""))
	w.want(cb, http.StatusFound, "")
	loginCode := mustQuery(t, cb.Header.Get("Location")).Get("login_code")
	m := w.want(w.complete(w.phoneKeys(), loginCode, f.verifier), http.StatusCreated, "")
	// A failing second flow logs and audits its cause too.
	f2, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	w.idp.SetClaim("nonce", "other")
	w.callback(f2, w.login())
	w.idp.Reset()
	var stored, events string
	w.scalar(&stored, `SELECT coalesce(string_agg(details::text, ' '), '') FROM audit_events`)
	w.scalar(&events, `SELECT coalesce(string_agg(payload::text, ' '), '') FROM event_log`)
	text := w.logText() + stored + events
	for name, s := range map[string]string{
		"state": q.Get("state"), "nonce": q.Get("nonce"), "provider code": providerCode, "enrollment code": c.Code,
		"client_verifier": f.verifier, "login_code": loginCode, "access token": m["access_token"].(string),
		"refresh token": m["refresh_token"].(string), "provider access token": "provider-access-", "provider refresh token": "provider-refresh-",
	} {
		if s == "" || strings.Contains(text, s) {
			t.Fatalf("%s is empty or in the logs, the audit or the events", name)
		}
	}
}

func caseLoginFlowCleanup(t *testing.T, w *oworld) {
	old, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	w.clk.Advance(sessCfg.LoginFlowTTL + time.Second)
	fresh, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := w.count(`SELECT count(*) FROM auth_login_flows WHERE id = $1`, old.attempt); n != 0 {
		t.Fatal("expired flow kept")
	}
	if n := w.count(`SELECT count(*) FROM auth_login_flows WHERE id = $1`, fresh.attempt); n != 1 {
		t.Fatal("live flow deleted")
	}
}

func caseIDPRecheckForcesRelogin(t *testing.T, w *oworld) {
	_, l := w.knownPerson()
	f, code := w.loginCode(l, nil)
	ph := w.phoneKeys()
	m := w.want(w.complete(ph, code, f.verifier), http.StatusCreated, "")
	c := &client{w: w.sworld, key: ph.key, refresh: m["refresh_token"].(string)}
	sid := m["session"].(map[string]any)["id"].(string)
	// Inside the interval the refresh rotates.
	w.clk.Advance(sessCfg.IDPRecheckInterval - time.Hour)
	rm := w.want(c.refreshWith(c.refresh, c.key), http.StatusOK, "")
	c.refresh = rm["refresh_token"].(string)
	w.clk.Advance(time.Hour)
	w.want(c.refreshWith(c.refresh, c.key), http.StatusUnauthorized, "unauthenticated")
	var reason string
	w.scalar(&reason, `SELECT coalesce(revoke_reason, '') FROM auth_sessions WHERE id = $1`, sid)
	if reason != "expired" {
		t.Fatalf("revoke_reason %q", reason)
	}
	if n := w.count(`SELECT count(*) FROM refresh_token_families WHERE session_id = $1 AND revoked_at IS NULL`, sid); n != 0 {
		t.Fatal("family not revoked")
	}
	found := false
	for _, a := range w.audits("auth.token.refresh") {
		if a.Decision == "deny" && a.Details["cause"] == "idp_recheck" && a.ResourceID == sid {
			found = true
		}
	}
	if !found {
		t.Fatal("no deny entry")
	}
}

func caseLinkStepUpRequired(t *testing.T, w *oworld) {
	c := w.phone(w.person(w.handle("alex"), false))
	_, r := w.begin(linkBeginPath, c.client, nil)
	w.want(r, http.StatusUnauthorized, "step_up_required")
	c.stepUp()
	f, r := w.begin(linkBeginPath, c.client, nil)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, w.login()), http.StatusFound, "")
	code := f.redirect.Query().Get("login_code")
	// The step-up runs out before complete: 401 step_up_required, and the code stays usable.
	w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
	w.want(c.call(http.MethodPost, linkCompletePath, linkBody(code, f.verifier)), http.StatusUnauthorized, "step_up_required")
	if st, _ := w.flowStatus(f.attempt); st != "code_issued" {
		t.Fatalf("flow %s", st)
	}
}

func caseLinkOtherActor404(t *testing.T, w *oworld) {
	ca, cs := w.phone(w.person(w.handle("alex"), false)), w.phone(w.person(w.handle("sam"), false))
	ca.stepUp()
	cs.stepUp()
	f, r := w.begin(linkBeginPath, ca.client, nil)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, w.login()), http.StatusFound, "")
	code := f.redirect.Query().Get("login_code")
	w.want(cs.call(http.MethodPost, linkCompletePath, linkBody(code, f.verifier)), http.StatusNotFound, "not_found")
	w.want(ca.call(http.MethodPost, linkCompletePath, linkBody(code, f.verifier)), http.StatusCreated, "")
}

func caseLinkSubjectAlreadyLinked409(t *testing.T, w *oworld) {
	_, taken := w.knownPerson()
	c := w.phone(w.person(w.handle("alex"), false))
	c.stepUp()
	f, r := w.begin(linkBeginPath, c.client, nil)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, taken), http.StatusFound, "")
	w.want(c.call(http.MethodPost, linkCompletePath, linkBody(f.redirect.Query().Get("login_code"), f.verifier)), http.StatusConflict, "conflict")
	if st, _ := w.flowStatus(f.attempt); st != "failed" {
		t.Fatalf("flow %s", st)
	}
}

func caseLinkSuccess(t *testing.T, w *oworld) {
	p := w.person(w.handle("alex"), false)
	c := w.phone(p)
	c.stepUp()
	l := w.login()
	f, r := w.begin(linkBeginPath, c.client, nil)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, l), http.StatusFound, "")
	// A wrong verifier first: 401 and the flow is failed.
	w.want(c.call(http.MethodPost, linkCompletePath, linkBody(f.redirect.Query().Get("login_code"), randomText(t))), http.StatusUnauthorized, "unauthenticated")
	if st, _ := w.flowStatus(f.attempt); st != "failed" {
		t.Fatalf("flow %s", st)
	}
	f, r = w.begin(linkBeginPath, c.client, nil)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, l), http.StatusFound, "")
	m := w.want(c.call(http.MethodPost, linkCompletePath, linkBody(f.redirect.Query().Get("login_code"), f.verifier)), http.StatusCreated, "")
	if m["subject"] != l.Subject || m["provider_kind"] != "oidc" || m["issuer"] != w.idp.Issuer() || m["email_at_link"] != l.Email {
		t.Fatalf("%v", m)
	}
	var steps []string
	for _, a := range w.audits("identity.link") {
		if a.Decision == "allow" && a.ResourceID == p.String() {
			steps = append(steps, a.Details["step"].(string))
		}
	}
	if strings.Join(steps, ",") != "begin,begin,complete" {
		t.Fatalf("allow entries %v", steps)
	}
	// The linked identity logs in.
	f, code := w.loginCode(oidctest.Login{Subject: l.Subject}, nil)
	m = w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	if m["session"].(map[string]any)["user_id"] != p.String() {
		t.Fatal("the linked identity does not log in as the person")
	}
}

func TestOIDCEgress_PrivateTargetsRejected(t *testing.T) {
	t.Parallel()
	// The issuer on loopback without an entry in egress.allowed_hosts: begin cannot load the
	// metadata and answers 503; the provider was never reached.
	w := newOIDCWorld(t, oidcWorldOpts{noAllow: true})
	_, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusServiceUnavailable, "unavailable")
	if w.idp.Count("discovery") != 0 {
		t.Fatal("the private issuer was reached")
	}
	// A discovered jwks_uri on a private address: the callback fails with login_failed.
	w2 := newOIDCWorld(t, oidcWorldOpts{idp: oidctest.Options{JWKSURI: "https://169.254.169.254/jwks"}})
	f, r := w2.begin(oidcBeginPath, nil, nil)
	w2.want(r, http.StatusOK, "")
	assertFailed(t, w2, w2.callback(f, w2.login()), f.attempt, "login_failed")
}
