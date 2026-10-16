// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	authzhttp "github.com/OWNER/bothy/services/server/internal/authz/adapters/httpapi"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The OIDC paths (operations of M1-41).
const (
	oidcBeginPath    = "/api/v1/auth/oidc/begin"
	oidcCallbackPath = "/api/v1/auth/oidc/callback"
	oidcCompletePath = "/api/v1/auth/oidc/complete"
	linkBeginPath    = "/api/v1/me/identities/begin"
	linkCompletePath = "/api/v1/me/identities/complete"
	appRedirect      = "http://127.0.0.1:4711/callback"
	clientID         = "chat"
	clientSecretRef  = "env:BOTHY_IDP_TEST" //nolint:gosec // G101: a secret reference, no secret
)

// fakeSecrets is the secret source of the OIDC world: one generated client secret.
type fakeSecrets map[string][]byte

// Resolve implements oidc.Secrets.
func (s fakeSecrets) Resolve(_ context.Context, ref string) ([]byte, error) {
	if v, ok := s[ref]; ok {
		return v, nil
	}
	return nil, errors.New("unknown reference")
}

// oworld is the session world with the OIDC login bound to a fake provider.
type oworld struct {
	*sworld
	idp  *oidctest.IDP
	prov ids.ID
	// admin are the authz Admin use cases behind the claim group sync (M1-42).
	admin *authzapp.AdminService
}

// oidcWorldOpts change the provider of an OIDC world.
type oidcWorldOpts struct {
	idp          oidctest.Options
	provisioning bool
	tenants      []string
	noAllow      bool
	issuer       string
	// groupsClaim is groups_claim of the provider; "" configures none (M1-42).
	groupsClaim string
}

// newOIDCWorld starts a fake provider and a world whose OIDC provider points to it; the provider
// is client_secret_basic with a secret generated for the test.
func newOIDCWorld(t *testing.T, o oidcWorldOpts) *oworld {
	t.Helper()
	secret := randomText(t)
	clk := clock.NewFake(sessStart)
	o.idp.ClientID, o.idp.Secret, o.idp.Now = clientID, secret, clk.Now
	idp := oidctest.New(t, o.idp)
	port, err := strconv.ParseUint(idp.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	eg := egress.Config{Timeout: 10 * time.Second, MaxResponseBytes: 1 << 20}
	if !o.noAllow {
		eg.AllowedHosts = []egress.Rule{{Host: "127.0.0.1", Port: uint16(port), AllowPrivate: true}}
	}
	var admin *authzapp.AdminService
	bind := func(w *sworld) {
		var err error
		admin, err = authzapp.NewAdminService(authzapp.AdminDeps{
			UoW: w.uow, Authorizer: w.authz, Store: authzpg.NewRepository(w.pool), Principals: w.principals, Structure: w.structure,
			Rights: w.rights, Audit: w.rec, Clock: w.clk, Events: w.appender, Scopes: w.authz, Cursors: authzhttp.Cursors{},
			Config: authzapp.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 2, PageSizeMax: 3},
			Views:  app.NewConsoleViews(postgres.NewRepository(w.pool)),
		})
		if err != nil {
			t.Fatal(err)
		}
		// viewAsUser of the console tests (M6-01).
		w.access = authzhttp.New(admin)
		if err := w.id.BindOIDC(wire.OIDCDeps{Egress: eg, Secrets: fakeSecrets{clientSecretRef: []byte(secret)}, RootCAs: idp.RootCAs(),
			ClaimGroups: admin}); err != nil {
			t.Fatal(err)
		}
	}
	w := &oworld{sworld: newSessionWorldOn(t, pgtest.New(t), clk, ids.New(), keyDir(t), bind), idp: idp, prov: ids.New()}
	w.admin = admin
	// The callback answers 302 to the app; the harness client must hand it back, not follow it.
	w.h.Server.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	issuer := idp.Issuer()
	if o.issuer != "" {
		issuer = o.issuer
	}
	conf := map[string]any{"issuer": issuer, "client_id": clientID, "client_auth": "client_secret_basic", "client_secret_ref": clientSecretRef,
		"app_redirect_uris": []string{"com.example.chat:/oidc"}}
	if len(o.tenants) > 0 {
		conf["allowed_tenant_ids"] = o.tenants
	}
	if o.groupsClaim != "" {
		conf["groups_claim"] = o.groupsClaim
	}
	raw, _ := json.Marshal(conf)
	w.sql(`INSERT INTO identity_providers (id, kind, name, config, allow_provisioning) VALUES ($1, 'oidc', 'Company', $2, $3)`,
		w.prov.String(), string(raw), o.provisioning)
	return w
}

// randomText returns 32 random bytes as base64url (verifiers and secrets of a test run).
func randomText(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// flow is one login of an app instance: its verifier and the answers of begin and callback.
type flow struct {
	verifier, attempt, authURL string
	redirect                   *url.URL
}

// begin starts a flow at path with extra members in the body.
func (w *oworld) begin(path string, c *client, extra map[string]any) (*flow, *httpservertest.Response) {
	w.t.Helper()
	f := &flow{verifier: randomText(w.t)}
	body := map[string]any{"provider_id": w.prov.String(), "app_redirect_uri": appRedirect, "client_challenge": oidcflow.S256(f.verifier)}
	for k, v := range extra {
		if v == nil {
			delete(body, k)
		} else {
			body[k] = v
		}
	}
	b, _ := json.Marshal(body)
	var r *httpservertest.Response
	if c == nil {
		r = w.h.Do(w.h.NewRequest(http.MethodPost, path, string(b)))
	} else {
		r = c.call(http.MethodPost, path, string(b))
	}
	if r.StatusCode == http.StatusOK {
		var m map[string]any
		_ = json.Unmarshal(r.Body, &m)
		f.attempt, f.authURL = m["login_attempt_id"].(string), m["authorization_url"].(string)
	}
	return f, r
}

// callback plays the browser at the provider for login and sends the provider's redirect to the
// server; it returns the server's answer.
func (w *oworld) callback(f *flow, login oidctest.Login) *httpservertest.Response {
	w.t.Helper()
	w.idp.NextLogin(login)
	loc := w.idp.Authorize(w.t, f.authURL)
	if loc.Path != oidcCallbackPath {
		w.t.Fatalf("the provider redirects to %s", loc.Path)
	}
	r := w.h.Do(w.h.NewRequest(http.MethodGet, loc.Path+"?"+loc.RawQuery, ""))
	if r.StatusCode == http.StatusFound {
		u, err := url.Parse(r.Header.Get("Location"))
		if err != nil {
			w.t.Fatal(err)
		}
		f.redirect = u
	}
	return r
}

// loginCode runs begin and callback and returns the flow with its login_code.
func (w *oworld) loginCode(login oidctest.Login, extra map[string]any) (*flow, string) {
	w.t.Helper()
	f, r := w.begin(oidcBeginPath, nil, extra)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, login), http.StatusFound, "")
	q := f.redirect.Query()
	if q.Get("login_attempt_id") != f.attempt || q.Get("login_code") == "" || q.Get("error") != "" {
		w.t.Fatalf("redirect %s", f.redirect)
	}
	return f, q.Get("login_code")
}

// complete redeems code with verifier from the new device p.
func (w *oworld) complete(p *newPhone, code, verifier string) *httpservertest.Response {
	w.t.Helper()
	b, _ := json.Marshal(map[string]any{
		"login_code": code, "client_verifier": verifier,
		"device": map[string]any{"device_id": p.id.String(), "name": "Alex Phone", "platform": "ios", "public_jwk": json.RawMessage(p.pub.JWK), "alg": "ES256"},
	})
	proof := identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{
		HTM: http.MethodPost, HTU: httpservertest.PublicURL + oidcCompletePath, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(),
	})
	return w.h.Do(w.h.NewRequest(http.MethodPost, oidcCompletePath, string(b), "DPoP", proof))
}

// linkIdentity inserts an active OIDC identity of user with subject at the world's provider.
func (w *oworld) linkIdentity(user ids.ID, subject string) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO user_identities (id, user_id, provider_id, subject, issuer) VALUES ($1, $2, $3, $4, $5)`,
		id.String(), user.String(), w.prov.String(), subject, w.idp.Issuer())
	return id
}

// flowStatus returns status and error of the flow id.
func (w *oworld) flowStatus(id string) (string, string) {
	w.t.Helper()
	var st, e string
	w.scalar(&st, `SELECT status FROM auth_login_flows WHERE id = $1`, id)
	w.scalar(&e, `SELECT coalesce(error, '') FROM auth_login_flows WHERE id = $1`, id)
	return st, e
}
