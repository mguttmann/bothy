// SPDX-License-Identifier: Apache-2.0

// Package oidc is the OIDC provider client of the identity module (brief M1-41, 08 section 2.2):
// discovery, the JWKS cache, the token request with client authentication (client_secret_basic or
// private_key_jwt) and the JWS check of the ID token. It implements app.OIDCProviderClient. Every
// request goes through platform/egress, one client per provider whose host policy allows the
// issuer host and, once the discovery document passed its checks, the hosts of token_endpoint and
// jwks_uri (19 SEC-NET-1, SEC-NET-2, SEC-NET-4); private addresses stay closed unless the operator
// lists them in egress.allowed_hosts. JOSE runs through internal/kernel/jose; no OIDC, OAuth or
// JOSE library is used, so every check is visible here.
//
// Errors never carry a code, a token, a secret or a response body: they name the step and wrap
// ErrProvider.
//
// Example:
//
//	c, err := oidc.New(oidc.Config{CacheTTL: time.Hour, AssertionTTL: 2 * time.Minute, Egress: eg},
//		oidc.Deps{Providers: repo, Secrets: secretSource})
//	meta, err := c.Metadata(ctx, providerID)
//	claims, err := c.ExchangeCode(ctx, providerID, code, verifier, redirectURI)
package oidc

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// ErrProvider is wrapped by every error of the client.
var ErrProvider = errors.New("oidc: provider request failed")

// fail returns an error of step that wraps ErrProvider; cause, when given, is wrapped too but
// carries no value (egress errors name scheme and host only, jose errors the rule only).
func fail(step string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrProvider, step)
	}
	return fmt.Errorf("%w: %s: %w", ErrProvider, step, cause)
}

// IDTokenAlgs are the algorithms of an ID token (08 section 2.2 step 5): asymmetric only.
var IDTokenAlgs = []string{jose.AlgRS256, jose.AlgPS256, jose.AlgES256, jose.AlgEdDSA}

// Config holds the configuration of the client.
type Config struct {
	// CacheTTL is auth.oidc_jwks_cache_ttl: how long a discovery document and a key set are used.
	CacheTTL time.Duration
	// AssertionTTL is auth.oidc_client_assertion_ttl: the lifetime of a client assertion.
	AssertionTTL time.Duration
	// Egress is egress.* (allowed hosts, timeout, response size).
	Egress egress.Config
}

// Secrets resolves a secret reference of a provider configuration (client_secret_ref,
// client_key_ref). internal/wire implements it over platform/secrets with the confinement of
// docs/DECISIONS.md M1-36/14. Errors never carry the value.
type Secrets interface {
	Resolve(ctx context.Context, ref string) ([]byte, error)
}

// Deps are the collaborators of the client.
type Deps struct {
	// Providers reads the provider configuration.
	Providers app.ProviderStore
	// Secrets resolves the client secret and the client key.
	Secrets Secrets
	// RootCAs are the trusted roots of TLS; nil means the system roots (tests pass the fake
	// provider's).
	RootCAs *x509.CertPool
	// Resolver resolves host names; nil means net.DefaultResolver.
	Resolver egress.Resolver
	// DenyAll refuses every provider request (egress.Deps.DenyAll); only test servers set it.
	DenyAll bool
	// Logger receives the refusals of the egress client; nil discards them.
	Logger *slog.Logger
	// Metrics counts refused egress attempts; nil counts nothing.
	Metrics egress.Metrics
	// Clock is the time of the cache and the client assertion; nil means clock.System().
	Clock clock.Clock
}

// Client implements app.OIDCProviderClient. It is safe for concurrent use.
type Client struct {
	cfg Config
	d   Deps

	mu    sync.Mutex
	state map[ids.ID]*providerState
}

// Compile-time check: the outgoing port of the OIDC use cases.
var _ app.OIDCProviderClient = (*Client)(nil)

// New returns the client. Errors: a missing collaborator or a duration that is not positive.
func New(cfg Config, d Deps) (*Client, error) {
	if d.Providers == nil || d.Secrets == nil {
		return nil, errors.New("oidc: New needs the provider store and the secret source")
	}
	if cfg.CacheTTL <= 0 || cfg.AssertionTTL <= 0 {
		return nil, errors.New("oidc: auth.oidc_jwks_cache_ttl and auth.oidc_client_assertion_ttl must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Client{cfg: cfg, d: d, state: map[ids.ID]*providerState{}}, nil
}

// providerState is the cache of one provider for one stored configuration: its egress client,
// the allowed hosts, the discovery document and the key set. mu makes every load single-flight.
type providerState struct {
	raw  string
	conf provider.OIDCConfig
	http *egress.Client

	hostsMu sync.RWMutex
	hosts   map[string]bool

	mu      sync.Mutex
	meta    app.ProviderMetadata
	metaAt  time.Time
	hasMeta bool
	keys    []providerKey
	keysAt  time.Time
	hasKeys bool
	// keysGen counts the loads of the key set; keyFor reads it before it waits for mu.
	keysGen atomic.Int64
}

// providerKey is a key of the provider's JWKS.
type providerKey struct {
	kid string
	key jose.JWK
}

// providerOf returns the cache of the enabled OIDC provider id, rebuilt when its stored
// configuration changed.
func (c *Client) providerOf(ctx context.Context, id ids.ID) (*providerState, error) {
	p, found, err := c.d.Providers.Provider(ctx, id)
	if err != nil {
		return nil, fail("reading the provider", err)
	}
	if !found || !p.Enabled || p.Kind != identity.ProviderOIDC {
		return nil, fail("the provider is not an enabled OIDC provider", nil)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.state[id]; ok && st.raw == string(p.Config) {
		return st, nil
	}
	st := &providerState{raw: string(p.Config), hosts: map[string]bool{}}
	if err := json.Unmarshal(p.Config, &st.conf); err != nil {
		return nil, fail("reading the configuration", nil)
	}
	issuer, err := url.Parse(discoveryURL(st.conf.Issuer))
	if err != nil {
		return nil, fail("the issuer does not parse", nil)
	}
	st.allow(issuer)
	st.http, err = egress.New(c.cfg.Egress, egress.Options{Purpose: "oidc", Hosts: st.policy, Redirects: egress.RedirectChecked},
		egress.Deps{Resolver: c.d.Resolver, RootCAs: c.d.RootCAs, Logger: c.d.Logger, Metrics: c.d.Metrics, DenyAll: c.d.DenyAll})
	if err != nil {
		return nil, fail("building the egress client", err)
	}
	c.state[id] = st
	return st, nil
}

// discoveryURL is the discovery document of issuer (OpenID Connect Discovery 1.0 section 4). A
// multi-tenant issuer names {tenantid}; its document is the one of the organizations alias, whose
// issuer member is the template itself (08 section 2.3, docs/DECISIONS.md M1-41).
func discoveryURL(issuer string) string {
	base := strings.Replace(issuer, provider.TenantPlaceholder, "organizations", 1)
	return strings.TrimSuffix(base, "/") + "/.well-known/openid-configuration"
}

// hostKey is the canonical host and port of u the way the egress host policy sees them.
func hostKey(u *url.URL) string {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if a, err := netip.ParseAddr(host); err == nil {
		host = a.Unmap().WithZone("").String()
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return host + "|" + port
}

// allow adds the host and port of u to the policy.
func (s *providerState) allow(u *url.URL) {
	s.hostsMu.Lock()
	defer s.hostsMu.Unlock()
	s.hosts[hostKey(u)] = true
}

// policy is the egress host policy of the provider: https to the allowed hosts only.
func (s *providerState) policy(_ context.Context, scheme, host string, port uint16) bool {
	if scheme != "https" {
		return false
	}
	s.hostsMu.RLock()
	defer s.hostsMu.RUnlock()
	return s.hosts[host+"|"+strconv.Itoa(int(port))]
}

// discovery is the part of a discovery document the server reads.
type discovery struct {
	Issuer                        string    `json:"issuer"`
	AuthorizationEndpoint         string    `json:"authorization_endpoint"`
	TokenEndpoint                 string    `json:"token_endpoint"`
	JWKSURI                       string    `json:"jwks_uri"`
	ResponseTypesSupported        []string  `json:"response_types_supported"`
	CodeChallengeMethodsSupported *[]string `json:"code_challenge_methods_supported"`
	IDTokenSigningAlgs            []string  `json:"id_token_signing_alg_values_supported"`
	IssParameterSupported         bool      `json:"authorization_response_iss_parameter_supported"`
}

// Metadata implements app.OIDCProviderClient: the cached document, or a single-flight load of
// the discovery document with the checks of brief M1-41 "beginOidcLogin" item 5.
func (c *Client) Metadata(ctx context.Context, providerID ids.ID) (app.ProviderMetadata, error) {
	st, err := c.providerOf(ctx, providerID)
	if err != nil {
		return app.ProviderMetadata{}, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return c.metadata(ctx, st)
}

// metadata returns the document of st; the caller holds st.mu.
func (c *Client) metadata(ctx context.Context, st *providerState) (app.ProviderMetadata, error) {
	now := c.d.Clock.Now()
	if st.hasMeta && now.Before(st.metaAt.Add(c.cfg.CacheTTL)) {
		return st.meta, nil
	}
	var doc discovery
	if err := c.getJSON(ctx, st, discoveryURL(st.conf.Issuer), &doc); err != nil {
		return app.ProviderMetadata{}, err
	}
	meta, err := checkDiscovery(doc, st.conf.Issuer)
	if err != nil {
		return app.ProviderMetadata{}, err
	}
	for _, raw := range []string{meta.TokenEndpoint, meta.JWKSURI} {
		u, _ := url.Parse(raw) // checked by checkDiscovery
		st.allow(u)
	}
	if st.hasMeta && st.meta.JWKSURI != meta.JWKSURI {
		st.hasKeys = false
	}
	st.meta, st.metaAt, st.hasMeta = meta, now, true
	return meta, nil
}

// checkDiscovery applies the checks of a discovery document: issuer equal to the configured
// string, response type code, S256 when PKCE methods are listed, a signing algorithm of the
// profile, and https endpoints with a host.
func checkDiscovery(doc discovery, issuer string) (app.ProviderMetadata, error) {
	switch {
	case doc.Issuer != issuer:
		return app.ProviderMetadata{}, fail("discovery: the issuer differs from the configuration", nil)
	case !slices.Contains(doc.ResponseTypesSupported, "code"):
		return app.ProviderMetadata{}, fail("discovery: response type code is not supported", nil)
	case doc.CodeChallengeMethodsSupported != nil && !slices.Contains(*doc.CodeChallengeMethodsSupported, "S256"):
		return app.ProviderMetadata{}, fail("discovery: PKCE S256 is not supported", nil)
	case !slices.ContainsFunc(doc.IDTokenSigningAlgs, func(a string) bool { return slices.Contains(IDTokenAlgs, a) }):
		return app.ProviderMetadata{}, fail("discovery: no allowed ID token algorithm", nil)
	}
	for _, raw := range []string{doc.AuthorizationEndpoint, doc.TokenEndpoint, doc.JWKSURI} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return app.ProviderMetadata{}, fail("discovery: every endpoint must be an https URL", nil)
		}
	}
	return app.ProviderMetadata{
		Issuer: doc.Issuer, AuthorizationEndpoint: doc.AuthorizationEndpoint, TokenEndpoint: doc.TokenEndpoint, JWKSURI: doc.JWKSURI,
		IssParameterSupported: doc.IssParameterSupported,
	}, nil
}

// getJSON reads a JSON document with GET through the provider's egress client.
func (c *Client) getJSON(ctx context.Context, st *providerState, target string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fail("building a request", nil)
	}
	req.Header.Set("Accept", "application/json")
	return c.do(st, req, dst)
}

// do sends req and decodes a 200 JSON answer into dst.
func (c *Client) do(st *providerState, req *http.Request, dst any) error {
	resp, err := st.http.Do(req)
	if err != nil {
		return fail("request to "+req.URL.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fail("the provider answered "+strconv.Itoa(resp.StatusCode), nil)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fail("the answer is not the expected JSON", nil)
	}
	return nil
}
