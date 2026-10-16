// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The provider and credential paths of docs/api/openapi.yaml.
const (
	providersPath = "/api/v1/identity-providers"
	overviewPath  = "/api/v1/me/credentials"
)

// The configurations of the tests; the secret reference points to nothing real.
const (
	oidcConfig   = `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:BOTHY_TEST_OIDC_SECRET"}`
	headerConfig = `{"login_header":"X-Example-Login"}`
)

// providerPath is the path of provider id.
func providerPath(id string) string { return providersPath + "/" + id }

// userCredentialsPath is the credential overview of user.
func userCredentialsPath(user ids.ID) string {
	return "/api/v1/users/" + user.String() + "/credentials"
}

// identityPath is the unlink path of identity id of user.
func identityPath(user, id ids.ID) string {
	return "/api/v1/users/" + user.String() + "/identities/" + id.String()
}

// postWith sends an authenticated POST with extra header pairs.
func (c *client) postWith(path, body string, headers ...string) *httpservertest.Response {
	c.w.t.Helper()
	hs := append([]string{"Authorization", "DPoP " + c.access, "DPoP", c.proof(c.key, http.MethodPost, path, c.access)}, headers...)
	return c.w.h.Do(c.w.h.NewRequest(http.MethodPost, path, body, hs...))
}

// admin returns the owner Alex on a phone with a fresh step-up.
func (w *sworld) admin() *stepClient {
	w.t.Helper()
	c := w.phone(w.person("alex", true))
	c.stepUp()
	return c
}

// createProvider creates a provider through the API and returns its id.
func (c *stepClient) createProvider(kind, name, config string) string {
	c.w.t.Helper()
	body := `{"kind":"` + kind + `","name":"` + name + `","config":` + config + `}`
	m := c.w.want(c.call(http.MethodPost, providersPath, body), http.StatusCreated, "")
	return m["id"].(string)
}

// link links subject at provider to user through the port, the way the login tickets will.
func (w *sworld) link(user ids.ID, provider, issuer, subject string) identity.Identity {
	w.t.Helper()
	i, err := w.id.Identities.Link(context.Background(), user, identity.ExternalIdentity{
		ProviderID: ids.MustParse(provider), Issuer: issuer, Subject: subject, EmailHint: subject + "@example.com",
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return i
}

// signIn registers an active phone of user and starts a session with the identity.
func (w *sworld) signIn(user ids.ID, identityID ids.ID) *client {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	dev, err := w.id.Devices.Register(context.Background(), identity.RegisterDevice{
		DeviceID: ids.New(), UserID: user, Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: pub,
		State: identity.DeviceStateActive, ApprovedBy: &user,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	set, err := w.id.Sessions.StartSession(context.Background(), identity.StartSession{
		UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthOIDC, IdentityID: &identityID,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	c := &client{w: w, key: key, user: user, device: dev.ID}
	c.take(set)
	return c
}

// noDevices revokes every device of user, so only identities remain as sign-in methods.
func (w *sworld) noDevices(user ids.ID) {
	w.t.Helper()
	w.sql(`UPDATE devices SET state = 'revoked', revoked_at = now(), revoke_reason = 'revoked_by_owner' WHERE user_id = $1`, user.String())
}

func TestProviders_OwnerWithStepUp(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	id := c.createProvider("oidc", "  Company SSO ", oidcConfig)
	got := w.want(c.call(http.MethodGet, providerPath(id), ""), http.StatusOK, "")
	if got["name"] != "Company SSO" || got["kind"] != "oidc" || got["enabled"] != true || got["allow_provisioning"] != false {
		t.Fatalf("provider %v", got)
	}
	cfg := got["config"].(map[string]any)
	if cfg["client_secret_ref"] != "env:BOTHY_TEST_OIDC_SECRET" || cfg["issuer"] != "https://login.example.com" {
		t.Fatalf("config %v", cfg)
	}
	list := w.want(c.call(http.MethodGet, providersPath, ""), http.StatusOK, "")
	if items := list["items"].([]any); len(items) != 1 {
		t.Fatalf("list %v", list)
	}
	w.clk.Advance(time.Second)
	upd := w.want(c.call(http.MethodPatch, providerPath(id), `{"name":"Partner SSO","allow_provisioning":true}`), http.StatusOK, "")
	if upd["name"] != "Partner SSO" || upd["allow_provisioning"] != true || upd["config"].(map[string]any)["client_id"] != "bothy" {
		t.Fatalf("updated %v", upd)
	}
	if !parseMS(t, upd["updated_at"]).After(parseMS(t, got["updated_at"])) {
		t.Errorf("updated_at %v not after %v", upd["updated_at"], got["updated_at"])
	}
	w.want(c.call(http.MethodDelete, providerPath(id), ""), http.StatusNoContent, "")
	w.want(c.call(http.MethodGet, providerPath(id), ""), http.StatusNotFound, "not_found")
	if n := len(w.audits("identity_provider.manage")); n != 3 {
		t.Errorf("%d manage audit entries, want 3", n)
	}
}

func TestProviders_WithoutStepUp(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	id := c.createProvider("oidc", "Company SSO", oidcConfig)
	plain := w.phone(c.user)
	w.want(plain.call(http.MethodPost, providersPath, `{"kind":"local","name":"Local","config":{}}`), http.StatusUnauthorized, "step_up_required")
	w.want(plain.call(http.MethodPatch, providerPath(id), `{"enabled":false}`), http.StatusUnauthorized, "step_up_required")
	w.want(plain.call(http.MethodDelete, providerPath(id), ""), http.StatusUnauthorized, "step_up_required")
	w.want(plain.call(http.MethodGet, providersPath, ""), http.StatusOK, "")
	w.want(plain.call(http.MethodGet, providerPath(id), ""), http.StatusOK, "")
}

func TestProviders_NonOwner(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	id := w.admin().createProvider("oidc", "Company SSO", oidcConfig)
	sam := w.phone(w.person("sam", false))
	sam.stepUp()
	for _, r := range []*httpservertest.Response{
		sam.call(http.MethodGet, providersPath, ""),
		sam.call(http.MethodGet, providerPath(id), ""),
		sam.call(http.MethodGet, providerPath(ids.New().String()), ""),
		sam.call(http.MethodPost, providersPath, `{"kind":"local","name":"Local","config":{}}`),
		sam.call(http.MethodPatch, providerPath(id), `{"enabled":false}`),
		sam.call(http.MethodDelete, providerPath(id), ""),
	} {
		p := w.want(r, http.StatusForbidden, "forbidden")
		if p["reason"] != "owner_only" {
			t.Errorf("reason %v, want owner_only", p["reason"])
		}
	}
}

func TestProviders_SecondTrustedHeaderOrLocal(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	c.createProvider("trusted_header", "Proxy", headerConfig)
	c.createProvider("local", "Local", `{}`)
	c.createProvider("oidc", "Company SSO", oidcConfig)
	c.createProvider("oidc", "Partner SSO", oidcConfig)
	for _, body := range []string{
		`{"kind":"trusted_header","name":"Second proxy","config":` + headerConfig + `}`,
		`{"kind":"local","name":"Second local","config":{}}`,
		`{"kind":"oidc","name":"COMPANY sso","config":` + oidcConfig + `}`,
	} {
		p := w.want(c.call(http.MethodPost, providersPath, body), http.StatusConflict, "conflict")
		if strings.Contains(string(mustJSON(t, p)), "identity_providers_") {
			t.Errorf("constraint name leaked: %v", p)
		}
	}
}

// mustJSON encodes v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProviders_KindImmutable(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	id := c.createProvider("trusted_header", "Proxy", headerConfig)
	w.want(c.call(http.MethodPatch, providerPath(id), `{"kind":"oidc"}`), http.StatusBadRequest, "")
	got := w.want(c.call(http.MethodGet, providerPath(id), ""), http.StatusOK, "")
	if got["kind"] != "trusted_header" {
		t.Fatalf("kind changed: %v", got)
	}
}

func TestProviders_ConfigValidatedOnUpdate(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	id := c.createProvider("trusted_header", "Proxy", headerConfig)
	p := w.want(c.call(http.MethodPatch, providerPath(id), `{"config":`+oidcConfig+`}`), http.StatusBadRequest, "validation_failed")
	if !strings.Contains(string(mustJSON(t, p["errors"])), "/config/") {
		t.Fatalf("errors %v, want pointers into /config", p["errors"])
	}
	w.want(c.call(http.MethodPost, providersPath, `{"kind":"oidc","name":"Inline","config":{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret":"x"}}`),
		http.StatusBadRequest, "validation_failed")
	got := w.want(c.call(http.MethodPatch, providerPath(id), `{"config":{"login_header":"x-other-login"}}`), http.StatusOK, "")
	if got["config"].(map[string]any)["login_header"] != "X-Other-Login" {
		t.Fatalf("config %v", got["config"])
	}
}

func TestProviders_DeleteWithIdentities(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	used := c.createProvider("oidc", "Company SSO", oidcConfig)
	free := c.createProvider("oidc", "Partner SSO", oidcConfig)
	i := w.link(c.user, used, "https://login.example.com", "sub-alex")
	w.sql(`UPDATE user_identities SET unlinked_at = now() WHERE id = $1`, i.ID.String())
	p := w.want(c.call(http.MethodDelete, providerPath(used), ""), http.StatusConflict, "conflict")
	if p["hint"] != "disable the provider instead" {
		t.Errorf("hint %v", p["hint"])
	}
	w.want(c.call(http.MethodDelete, providerPath(free), ""), http.StatusNoContent, "")
	w.want(c.call(http.MethodDelete, providerPath(free), ""), http.StatusNotFound, "not_found")
}

func TestProviders_Idempotency(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	key := ids.New().String()
	body := `{"kind":"oidc","name":"Company SSO","config":` + oidcConfig + `}`
	first := w.want(c.postWith(providersPath, body, "Idempotency-Key", key), http.StatusCreated, "")
	again := w.want(c.postWith(providersPath, body, "Idempotency-Key", key), http.StatusCreated, "")
	if first["id"] != again["id"] {
		t.Fatalf("replay created another provider: %v and %v", first["id"], again["id"])
	}
	if n := w.count(`SELECT count(*) FROM identity_providers`); n != 1 {
		t.Fatalf("%d providers, want 1", n)
	}
}

func TestProviders_AuditNoConfigValues(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	id := c.createProvider("oidc", "Company SSO", oidcConfig)
	w.want(c.call(http.MethodPatch, providerPath(id), `{"config":{"issuer":"https://login.example.com","client_id":"bothy-two","client_auth":"client_secret_basic","client_secret_ref":"env:BOTHY_TEST_OTHER"}}`), http.StatusOK, "")
	rows := w.audits("identity_provider.manage")
	if len(rows) != 2 {
		t.Fatalf("%d audit rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.ResourceID != id || r.Details["provider_kind"] != "oidc" {
			t.Errorf("audit row %+v, want resource %s and provider_kind oidc", r, id)
		}
		for _, secret := range []string{"BOTHY_TEST", "login.example.com", "bothy-two", "Company SSO"} {
			if strings.Contains(r.Raw, secret) {
				t.Errorf("audit details %s contain a config value", r.Raw)
			}
		}
	}
	if changed := mustJSON(t, rows[1].Details["changed_fields"]); string(changed) != `["config"]` {
		t.Errorf("changed_fields %s", changed)
	}
}

func TestResolve_MatchedAndIssuer(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	prov := c.createProvider("oidc", "Company SSO", oidcConfig)
	i := w.link(c.user, prov, "https://login.example.com/tenant-a/v2.0", "sub-alex")
	ctx := context.Background()
	resolve := func(issuer, subject string) identity.Resolution {
		t.Helper()
		r, err := w.id.Identities.Resolve(ctx, identity.ExternalIdentity{ProviderID: ids.MustParse(prov), Issuer: issuer, Subject: subject})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := resolve("https://login.example.com/tenant-a/v2.0", "sub-alex"); r.Outcome != identity.ResolutionMatched || r.UserID != c.user || r.IdentityID != i.ID {
		t.Fatalf("matched: %+v", r)
	}
	if r := resolve("https://login.example.com/tenant-b/v2.0", "sub-alex"); r.Outcome != identity.ResolutionUnmatched {
		t.Errorf("other issuer: %+v", r)
	}
	w.sql(`UPDATE identity_providers SET enabled = false WHERE id = $1`, prov)
	if r := resolve("https://login.example.com/tenant-a/v2.0", "sub-alex"); r.Outcome != identity.ResolutionUnmatched {
		t.Errorf("disabled provider: %+v", r)
	}
	w.sql(`UPDATE identity_providers SET enabled = true WHERE id = $1`, prov)
	w.sql(`UPDATE user_identities SET unlinked_at = now() WHERE id = $1`, i.ID.String())
	if r := resolve("https://login.example.com/tenant-a/v2.0", "sub-alex"); r.Outcome != identity.ResolutionUnmatched {
		t.Errorf("unlinked identity: %+v", r)
	}
	if err := w.id.Identities.MarkLogin(ctx, ids.New(), w.clk.Now()); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("MarkLogin of an unknown identity: %v", err)
	}
}

func TestLink_Conflicts(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.admin()
	sam := w.person("sam", false)
	prov := c.createProvider("trusted_header", "Proxy", headerConfig)
	ext := identity.ExternalIdentity{ProviderID: ids.MustParse(prov), Issuer: "ignored", Subject: "alex@example.com", EmailHint: "not an address"}
	first, err := w.id.Identities.Link(context.Background(), c.user, ext)
	if err != nil {
		t.Fatal(err)
	}
	if first.Issuer != "" || first.EmailAtLink != "" {
		t.Errorf("trusted_header identity %+v, want no issuer and no e-mail", first)
	}
	if _, err := w.id.Identities.Link(context.Background(), sam, ext); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("active subject twice: %v, want conflict", err)
	}
	w.sql(`UPDATE user_identities SET unlinked_at = now() WHERE id = $1`, first.ID.String())
	if _, err := w.id.Identities.Link(context.Background(), sam, ext); err != nil {
		t.Fatalf("relink after unlink: %v", err)
	}
	if _, err := w.id.Identities.Link(context.Background(), sam, identity.ExternalIdentity{ProviderID: ids.New(), Subject: "x"}); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("unknown provider: %v", err)
	}
}

func TestCredentials_SelfOwnerOther(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	owner := w.admin()
	prov := owner.createProvider("oidc", "Company SSO", oidcConfig)
	samID := w.person("sam", false)
	w.link(samID, prov, "https://login.example.com", "sub-sam")
	sam := w.phone(samID)
	for _, r := range []*httpservertest.Response{
		sam.call(http.MethodGet, overviewPath, ""),
		sam.call(http.MethodGet, userCredentialsPath(samID), ""),
		owner.call(http.MethodGet, userCredentialsPath(samID), ""),
	} {
		m := w.want(r, http.StatusOK, "")
		list := m["identities"].([]any)
		if len(list) != 1 {
			t.Fatalf("identities %v", m)
		}
		item := list[0].(map[string]any)
		if item["provider_kind"] != "oidc" || item["subject"] != "sub-sam" || item["email_at_link"] != "sub-sam@example.com" || item["issuer"] != "https://login.example.com" {
			t.Errorf("identity %v", item)
		}
		if _, ok := item["last_login_at"]; ok {
			t.Errorf("last_login_at present before a login: %v", item)
		}
		if p, ok := m["passkeys"].([]any); !ok || len(p) != 0 || m["local_password_set"] != false || m["totp_enrolled"] != false {
			t.Errorf("overview %v", m)
		}
	}
	robin := w.phone(w.person("robin", false))
	w.want(robin.call(http.MethodGet, userCredentialsPath(samID), ""), http.StatusNotFound, "not_found")
	w.want(owner.call(http.MethodGet, userCredentialsPath(ids.New()), ""), http.StatusNotFound, "not_found")
	pending := w.loginDevice(samID, identity.DeviceStatePending, identity.PlatformIOS, false)
	w.want(pending.call(http.MethodGet, overviewPath, ""), http.StatusForbidden, "device_pending")
}

func TestUnlink_LastMethod(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	owner := w.admin()
	prov := owner.createProvider("oidc", "Company SSO", oidcConfig)
	other := owner.createProvider("oidc", "Partner SSO", oidcConfig)
	samID := w.person("sam", false)
	sam := w.phone(samID)
	sam.stepUp()
	only := w.link(samID, prov, "https://login.example.com", "sub-sam")

	// Only the active phone remains: allowed.
	w.want(sam.call(http.MethodDelete, identityPath(samID, only.ID), ""), http.StatusNoContent, "")
	// Already unlinked: 204 without change.
	w.want(sam.call(http.MethodDelete, identityPath(samID, only.ID), ""), http.StatusNoContent, "")

	// The only identity and no device: refused.
	last := w.link(samID, prov, "https://login.example.com", "sub-sam-2")
	w.noDevices(samID)
	p := w.want(owner.call(http.MethodDelete, identityPath(samID, last.ID), ""), http.StatusConflict, "conflict")
	if p["reason"] != "no_login_method_left" || p["hint"] == nil {
		t.Fatalf("problem %v", p)
	}
	// A second identity at a disabled provider does not count.
	w.link(samID, other, "https://login.example.com", "sub-sam-other")
	w.want(owner.call(http.MethodPatch, providerPath(other), `{"enabled":false}`), http.StatusOK, "")
	w.want(owner.call(http.MethodDelete, identityPath(samID, last.ID), ""), http.StatusConflict, "conflict")
	// Enabled again, the owner unlinks for the other person.
	w.want(owner.call(http.MethodPatch, providerPath(other), `{"enabled":true}`), http.StatusOK, "")
	w.want(owner.call(http.MethodDelete, identityPath(samID, last.ID), ""), http.StatusNoContent, "")

	// Without step-up and for someone else as a non-owner.
	robinID := w.person("robin", false)
	robin := w.phone(robinID)
	ri := w.link(robinID, prov, "https://login.example.com", "sub-robin")
	w.want(robin.call(http.MethodDelete, identityPath(robinID, ri.ID), ""), http.StatusUnauthorized, "step_up_required")
	robin.stepUp()
	w.want(robin.call(http.MethodDelete, identityPath(samID, last.ID), ""), http.StatusNotFound, "not_found")
	w.want(robin.call(http.MethodDelete, identityPath(robinID, ids.New()), ""), http.StatusNotFound, "not_found")
	if n := w.count(`SELECT count(*) FROM user_identities WHERE unlinked_at IS NULL AND user_id = $1`, samID.String()); n != 1 {
		t.Errorf("%d active identities of sam, want 1", n)
	}
}

func TestUnlink_ConcurrentLastTwo(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	owner := w.admin()
	prov := owner.createProvider("oidc", "Company SSO", oidcConfig)
	samID := w.person("sam", false)
	a := w.link(samID, prov, "https://login.example.com", "sub-a")
	b := w.link(samID, prov, "https://login.example.com", "sub-b")
	w.noDevices(samID)
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for n, target := range []ids.ID{a.ID, b.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.id.Identities.Unlink(context.Background(), owner.actor(), samID, target)
			switch {
			case err == nil:
				codes[n] = http.StatusNoContent
			case errs.KindOf(err) == errs.KindConflict:
				codes[n] = http.StatusConflict
			default:
				t.Errorf("unlink: %v", err)
			}
		}()
	}
	wg.Wait()
	if codes[0]+codes[1] != http.StatusNoContent+http.StatusConflict || codes[0] == codes[1] {
		t.Fatalf("results %v, want exactly one 204 and one 409", codes)
	}
	if n := w.count(`SELECT count(*) FROM user_identities WHERE unlinked_at IS NULL AND user_id = $1`, samID.String()); n != 1 {
		t.Fatalf("%d active identities, want 1", n)
	}
}

func TestUnlink_EndsItsSessions(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	owner := w.admin()
	prov := owner.createProvider("oidc", "Company SSO", oidcConfig)
	samID := w.person("sam", false)
	first := w.link(samID, prov, "https://login.example.com", "sub-1")
	second := w.link(samID, prov, "https://login.example.com", "sub-2")
	third := w.link(samID, prov, "https://login.example.com", "sub-3")
	viaFirst := w.signIn(samID, first.ID)
	viaSecond := w.signIn(samID, second.ID)
	viaThird := w.signIn(samID, third.ID)
	// The person unlinks the second identity from the session of the first one, with a fresh
	// step-up on that session.
	w.sql(`UPDATE auth_sessions SET step_up_at = $2, step_up_method = 'device_key' WHERE id = $1`, viaFirst.session.String(), w.clk.Now())
	w.want(viaFirst.call(http.MethodDelete, identityPath(samID, second.ID), ""), http.StatusNoContent, "")
	if got := w.sessionState(viaSecond.session); got != "logout" {
		t.Errorf("session of the unlinked identity: %q, want logout", got)
	}
	if got := w.sessionState(viaFirst.session); got != "" {
		t.Errorf("session of another identity: %q, want active", got)
	}
	if reason, _ := w.familyState(viaSecond.session); reason == "" {
		t.Error("family of the revoked session stays active")
	}
	w.want(owner.call(http.MethodDelete, identityPath(samID, third.ID), ""), http.StatusNoContent, "")
	if got := w.sessionState(viaThird.session); got != "admin" {
		t.Errorf("session after an owner unlink: %q, want admin", got)
	}
	var payloads []string
	for _, e := range w.revokedEvents() {
		payloads = append(payloads, e["payload"])
	}
	if len(payloads) != 2 || !strings.Contains(payloads[0], viaSecond.session.String()) || !strings.Contains(payloads[1], viaThird.session.String()) {
		t.Errorf("session.revoked payloads %v", payloads)
	}
}

func TestDisableProvider_EndsSessions(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	owner := w.admin()
	prov := owner.createProvider("oidc", "Company SSO", oidcConfig)
	other := owner.createProvider("oidc", "Partner SSO", oidcConfig)
	samID := w.person("sam", false)
	robinID := w.person("robin", false)
	viaSam := w.signIn(samID, w.link(samID, prov, "https://login.example.com", "sub-sam").ID)
	viaRobin := w.signIn(robinID, w.link(robinID, prov, "https://login.example.com", "sub-robin").ID)
	viaOther := w.signIn(samID, w.link(samID, other, "https://login.example.com", "sub-sam").ID)
	w.want(owner.call(http.MethodPatch, providerPath(prov), `{"enabled":false}`), http.StatusOK, "")
	for _, c := range []*client{viaSam, viaRobin} {
		if got := w.sessionState(c.session); got != "admin" {
			t.Errorf("session %s: %q, want admin", c.session, got)
		}
	}
	if got := w.sessionState(viaOther.session); got != "" {
		t.Errorf("session of another provider: %q", got)
	}
	if n := len(w.revokedEvents()); n != 2 {
		t.Errorf("%d session.revoked, want 2", n)
	}
	if _, err := w.id.Providers.Enabled(context.Background(), ids.MustParse(prov)); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("Enabled of a disabled provider: %v", err)
	}
	r, err := w.id.Identities.Resolve(context.Background(), identity.ExternalIdentity{ProviderID: ids.MustParse(prov), Issuer: "https://login.example.com", Subject: "sub-sam"})
	if err != nil || r.Outcome != identity.ResolutionUnmatched {
		t.Errorf("sign-in through a disabled provider: %+v, %v", r, err)
	}
	w.want(owner.call(http.MethodPatch, providerPath(prov), `{"enabled":true}`), http.StatusOK, "")
	if got := w.sessionState(viaSam.session); got != "admin" {
		t.Errorf("re-enabling restored a session: %q", got)
	}
}

func TestListForDiscovery(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	owner := w.admin()
	owner.createProvider("oidc", "b Partner SSO", oidcConfig)
	off := owner.createProvider("local", "Local", `{}`)
	first := owner.createProvider("oidc", "A Company SSO", oidcConfig)
	w.want(owner.call(http.MethodPatch, providerPath(off), `{"enabled":false}`), http.StatusOK, "")
	list, err := w.id.Providers.ListForDiscovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID.String() != first || list[0].Name != "A Company SSO" || list[0].Kind != identity.ProviderOIDC || list[1].Name != "b Partner SSO" {
		t.Fatalf("discovery %+v", list)
	}
}
