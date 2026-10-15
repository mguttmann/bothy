// SPDX-License-Identifier: Apache-2.0

package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// providers is a provider store with one provider; the other methods are never called.
type providers struct {
	app.ProviderStore
	p identity.Provider
}

// Provider implements the one method the client uses.
func (s *providers) Provider(_ context.Context, id ids.ID) (identity.Provider, bool, error) {
	return s.p, id == s.p.ID, nil
}

// secrets is a secret source over a map of references.
type secrets map[string][]byte

// Resolve implements oidc.Secrets.
func (s secrets) Resolve(_ context.Context, ref string) ([]byte, error) {
	if v, ok := s[ref]; ok {
		return v, nil
	}
	return nil, errors.New("unknown reference")
}

// env is a client over a fake provider.
type env struct {
	idp    *oidctest.IDP
	client *oidc.Client
	id     ids.ID
	clk    *clock.Fake
}

const callback = "https://chat.example.com/api/v1/auth/oidc/callback"

// newEnv starts a fake provider with opts and a client for a provider configured with conf
// (issuer added); allow opens the provider's port in egress.allowed_hosts.
func newEnv(t *testing.T, opts oidctest.Options, conf map[string]any, sec secrets, allow bool) *env {
	t.Helper()
	clk := clock.NewFake(time.Now())
	opts.Now = clk.Now
	idp := oidctest.New(t, opts)
	conf["issuer"] = idp.Issuer()
	raw, _ := json.Marshal(conf)
	p := identity.Provider{ID: ids.New(), Kind: identity.ProviderOIDC, Name: "Fake", Config: raw, Enabled: true}
	eg := egress.Config{Timeout: 5 * time.Second, MaxResponseBytes: 1 << 20}
	if allow {
		eg.AllowedHosts = []egress.Rule{{Host: "127.0.0.1", Port: port(t, idp), AllowPrivate: true}}
	}
	c, err := oidc.New(oidc.Config{CacheTTL: time.Hour, AssertionTTL: 2 * time.Minute, Egress: eg},
		oidc.Deps{Providers: &providers{p: p}, Secrets: sec, RootCAs: idp.RootCAs(), Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return &env{idp: idp, client: c, id: p.ID, clk: clk}
}

// basicConf is a client_secret_basic configuration with the test's secret reference.
func basicConf() map[string]any {
	return map[string]any{"client_id": "chat", "client_auth": "client_secret_basic", "client_secret_ref": "env:BOTHY_IDP_S"} //nolint:gosec // G101: a reference, no secret
}

// withTenants adds allowed_tenant_ids to conf.
func withTenants(conf map[string]any, tenants ...string) map[string]any {
	conf["allowed_tenant_ids"] = tenants
	return conf
}

// port is the port of the fake provider.
func port(t *testing.T, idp *oidctest.IDP) uint16 {
	t.Helper()
	p, err := strconv.ParseUint(idp.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return uint16(p)
}

// code runs /authorize with the challenge of the test verifier and returns the code.
func (e *env) code(t *testing.T, nonce string) string {
	t.Helper()
	q := url.Values{"client_id": {"chat"}, "response_type": {"code"}, "redirect_uri": {callback}, "state": {"s"}, "nonce": {nonce},
		"code_challenge": {oidcflow.S256(verifier)}, "code_challenge_method": {"S256"}}
	loc := e.idp.Authorize(t, e.idp.URL()+"/authorize?"+q.Encode())
	return loc.Query().Get("code")
}

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func TestOIDCClientAuth_SecretBasic(t *testing.T) {
	t.Parallel()
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE secret:1"},
		map[string]any{"client_id": "chat", "client_auth": "client_secret_basic", "client_secret_ref": "env:BOTHY_IDP_SECRET"}, //nolint:gosec // G101: a reference, no secret
		secrets{"env:BOTHY_IDP_SECRET": []byte("EXAMPLE secret:1\n")}, true)
	e.idp.NextLogin(oidctest.Login{Subject: "alex-sub"})
	claims, err := e.client.ExchangeCode(context.Background(), e.id, e.code(t, "n1"), verifier, callback)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "alex-sub" || claims.Nonce != "n1" || claims.Issuer != e.idp.URL() {
		t.Fatalf("%+v", claims)
	}
	if e.idp.BasicLogins() != 1 {
		t.Fatal("no Basic authentication")
	}
	// A wrong verifier fails at the provider (PKCE), and so does a code used twice.
	if _, err := e.client.ExchangeCode(context.Background(), e.id, e.code(t, "n2"), verifier[:42]+"A", callback); !errors.Is(err, oidc.ErrProvider) {
		t.Fatalf("wrong verifier: %v", err)
	}
}

func TestOIDCClientAuth_PrivateKeyJWT(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	const x5t = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	e := newEnv(t, oidctest.Options{ClientID: "chat", AssertionKey: &key.PublicKey},
		map[string]any{"client_id": "chat", "client_auth": "private_key_jwt", "client_key_ref": "file:/idp/key.pem", "client_key_alg": "PS256",
			"client_key_id": "cert-1", "client_cert_x5t_s256": x5t},
		secrets{"file:/idp/key.pem": keyPEM}, true)
	e.idp.NextLogin(oidctest.Login{Subject: "sam-sub"})
	for i := range 2 {
		if _, err := e.client.ExchangeCode(context.Background(), e.id, e.code(t, "n"), verifier, callback); err != nil {
			t.Fatalf("exchange %d: %v", i, err)
		}
	}
	as := e.idp.Assertions()
	if len(as) != 2 {
		t.Fatalf("%d assertions", len(as))
	}
	now := e.clk.Now().Unix()
	for _, a := range as {
		h, c := a.Header, a.Claims
		if h["alg"] != "PS256" || h["typ"] != "JWT" || h["kid"] != "cert-1" || h["x5t#S256"] != x5t {
			t.Fatalf("header %v", h)
		}
		if c["iss"] != "chat" || c["sub"] != "chat" || c["aud"] != e.idp.URL()+"/token" || int64(c["iat"].(float64)) != now ||
			int64(c["nbf"].(float64)) != now || int64(c["exp"].(float64)) != now+120 {
			t.Fatalf("claims %v", c)
		}
	}
	if as[0].Claims["jti"] == as[1].Claims["jti"] {
		t.Fatal("jti repeated")
	}
	// A key that does not fit client_key_alg is refused before any request.
	e2 := newEnv(t, oidctest.Options{ClientID: "chat", AssertionKey: &key.PublicKey},
		map[string]any{"client_id": "chat", "client_auth": "private_key_jwt", "client_key_ref": "file:/idp/key.pem", "client_key_alg": "ES256"},
		secrets{"file:/idp/key.pem": keyPEM}, true)
	if _, err := e2.client.ExchangeCode(context.Background(), e2.id, e2.code(t, "n"), verifier, callback); !errors.Is(err, oidc.ErrProvider) {
		t.Fatalf("mismatching key: %v", err)
	}
	if e2.idp.Count("token") != 0 {
		t.Fatal("a token request was sent with an unusable key")
	}
}

func TestJWKS_UnknownKidReloadsOnce(t *testing.T) {
	t.Parallel()
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE"},
		basicConf(),
		secrets{"env:BOTHY_IDP_S": []byte("EXAMPLE")}, true)
	e.idp.NextLogin(oidctest.Login{Subject: "a"})
	ctx := context.Background()
	if _, err := e.client.ExchangeCode(ctx, e.id, e.code(t, "n"), verifier, callback); err != nil {
		t.Fatal(err)
	}
	if n := e.idp.Count("jwks"); n != 1 {
		t.Fatalf("jwks loaded %d times", n)
	}
	// The cached set serves the next token.
	if _, err := e.client.ExchangeCode(ctx, e.id, e.code(t, "n"), verifier, callback); err != nil || e.idp.Count("jwks") != 1 {
		t.Fatalf("second exchange: %v, jwks %d", err, e.idp.Count("jwks"))
	}
	// A rotated key: one reload finds it.
	e.idp.RotateKey("k2", false)
	if _, err := e.client.ExchangeCode(ctx, e.id, e.code(t, "n"), verifier, callback); err != nil || e.idp.Count("jwks") != 2 {
		t.Fatalf("after rotation: %v, jwks %d", err, e.idp.Count("jwks"))
	}
	// A kid the provider never publishes: exactly one more reload, then failure.
	e.idp.SetHeader("kid", "unknown")
	if _, err := e.client.ExchangeCode(ctx, e.id, e.code(t, "n"), verifier, callback); !errors.Is(err, oidc.ErrProvider) || e.idp.Count("jwks") != 3 {
		t.Fatalf("unknown kid: %v, jwks %d", err, e.idp.Count("jwks"))
	}
}

func TestMetadataCacheTTL(t *testing.T) {
	t.Parallel()
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE", SendIss: true},
		basicConf(),
		secrets{"env:BOTHY_IDP_S": []byte("EXAMPLE")}, true)
	ctx := context.Background()
	m, err := e.client.Metadata(ctx, e.id)
	if err != nil {
		t.Fatal(err)
	}
	if !m.IssParameterSupported || m.TokenEndpoint != e.idp.URL()+"/token" || m.Issuer != e.idp.URL() {
		t.Fatalf("%+v", m)
	}
	e.clk.Advance(time.Hour - time.Second)
	if _, err := e.client.Metadata(ctx, e.id); err != nil || e.idp.Count("discovery") != 1 {
		t.Fatalf("inside the TTL: %v, discovery %d", err, e.idp.Count("discovery"))
	}
	e.clk.Advance(time.Second)
	if _, err := e.client.Metadata(ctx, e.id); err != nil || e.idp.Count("discovery") != 2 {
		t.Fatalf("after the TTL: %v, discovery %d", err, e.idp.Count("discovery"))
	}
}

func TestMetadata_EntraTemplate(t *testing.T) {
	t.Parallel()
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE", Entra: true},
		withTenants(basicConf(), "t1"),
		secrets{"env:BOTHY_IDP_S": []byte("EXAMPLE")}, true)
	m, err := e.client.Metadata(context.Background(), e.id)
	if err != nil || m.Issuer != e.idp.Issuer() {
		t.Fatalf("%+v %v", m, err)
	}
}

func TestOIDCEgress_PrivateIssuerBlocked(t *testing.T) {
	t.Parallel()
	// Without an entry in egress.allowed_hosts the loopback issuer is refused at connect time.
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE"},
		basicConf(),
		secrets{"env:BOTHY_IDP_S": []byte("EXAMPLE")}, false)
	if _, err := e.client.Metadata(context.Background(), e.id); !errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("Metadata = %v, want a refusal of egress", err)
	}
	if e.idp.Count("discovery") != 0 {
		t.Fatal("the provider was reached")
	}
}

func TestOIDCEgress_PrivateJWKSBlocked(t *testing.T) {
	t.Parallel()
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE", JWKSURI: "https://169.254.169.254/jwks"},
		basicConf(),
		secrets{"env:BOTHY_IDP_S": []byte("EXAMPLE")}, true)
	e.idp.NextLogin(oidctest.Login{Subject: "a"})
	_, err := e.client.ExchangeCode(context.Background(), e.id, e.code(t, "n"), verifier, callback)
	if !errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("ExchangeCode = %v, want a refusal of egress", err)
	}
}

func TestMetadata_DiscoveryChecks(t *testing.T) {
	t.Parallel()
	e := newEnv(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE"},
		basicConf(),
		secrets{"env:BOTHY_IDP_S": []byte("EXAMPLE")}, true)
	// A configuration whose issuer differs from the document's (trailing slash) fails.
	conf := basicConf()
	conf["issuer"] = e.idp.Issuer() + "/x"
	raw, _ := json.Marshal(conf)
	p := identity.Provider{ID: ids.New(), Kind: identity.ProviderOIDC, Config: raw, Enabled: true}
	c, err := oidc.New(oidc.Config{CacheTTL: time.Hour, AssertionTTL: time.Minute, Egress: egress.Config{Timeout: 5 * time.Second,
		MaxResponseBytes: 1 << 20, AllowedHosts: []egress.Rule{{Host: "127.0.0.1", Port: port(t, e.idp), AllowPrivate: true}}}},
		oidc.Deps{Providers: &providers{p: p}, Secrets: secrets{}, RootCAs: e.idp.RootCAs()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Metadata(context.Background(), p.ID); !errors.Is(err, oidc.ErrProvider) {
		t.Fatalf("issuer mismatch: %v", err)
	}
	// A disabled or unknown provider is refused without a request.
	if _, err := c.Metadata(context.Background(), ids.New()); !errors.Is(err, oidc.ErrProvider) {
		t.Fatalf("unknown provider: %v", err)
	}
}
