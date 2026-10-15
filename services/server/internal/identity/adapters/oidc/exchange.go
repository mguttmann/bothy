// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/url"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// AssertionType is client_assertion_type of private_key_jwt (RFC 7523 section 2.2).
const AssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"

// tokenResponse is the part of a token response the server reads; access_token and
// refresh_token of the provider are never decoded (08 section 2.2: no provider tokens).
type tokenResponse struct {
	IDToken string `json:"id_token"`
}

// ExchangeCode implements app.OIDCProviderClient (brief M1-41 "handleOidcCallback" items 4 and
// 5): the token request with grant_type authorization_code, code, redirect_uri, code_verifier and
// the client authentication of the configuration, then the JWS of the ID token: compact only, an
// algorithm of IDTokenAlgs that fits the key, the key by kid from the JWKS (an unknown kid reloads
// the set once, single-flight per provider). It returns the parsed claims; the claim rules are the
// caller's.
func (c *Client) ExchangeCode(ctx context.Context, providerID ids.ID, code, codeVerifier, redirectURI string) (app.IDTokenClaims, error) {
	st, err := c.providerOf(ctx, providerID)
	if err != nil {
		return app.IDTokenClaims{}, err
	}
	st.mu.Lock()
	meta, err := c.metadata(ctx, st)
	st.mu.Unlock()
	if err != nil {
		return app.IDTokenClaims{}, err
	}
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}, "code_verifier": {codeVerifier},
	}
	basic, err := c.authenticate(ctx, st.conf, meta.TokenEndpoint, form)
	if err != nil {
		return app.IDTokenClaims{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return app.IDTokenClaims{}, fail("building the token request", nil)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basic != "" {
		req.Header.Set("Authorization", basic)
	}
	var tok tokenResponse
	if err := c.do(st, req, &tok); err != nil {
		return app.IDTokenClaims{}, err
	}
	if tok.IDToken == "" {
		return app.IDTokenClaims{}, fail("the token response has no id_token", nil)
	}
	return c.verifyIDToken(ctx, st, tok.IDToken)
}

// authenticate adds the client authentication of conf: client_secret_basic as RFC 6749 section
// 2.3.1 (form-urlencoded id and secret in the Basic header, returned as the Authorization value),
// or private_key_jwt as RFC 7523 (client_id, client_assertion_type and a signed client_assertion
// in form, "" returned).
func (c *Client) authenticate(ctx context.Context, conf provider.OIDCConfig, tokenEndpoint string, form url.Values) (string, error) {
	switch conf.ClientAuth {
	case provider.ClientAuthSecretBasic:
		secret, err := c.d.Secrets.Resolve(ctx, conf.ClientSecretRef)
		if err != nil {
			return "", fail("resolving the client secret", nil)
		}
		pair := url.QueryEscape(conf.ClientID) + ":" + url.QueryEscape(strings.TrimRight(string(secret), "\r\n"))
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(pair)), nil
	case provider.ClientAuthPrivateKeyJWT:
		assertion, err := c.clientAssertion(ctx, conf, tokenEndpoint)
		if err != nil {
			return "", err
		}
		form.Set("client_id", conf.ClientID)
		form.Set("client_assertion_type", AssertionType)
		form.Set("client_assertion", assertion)
		return "", nil
	}
	return "", fail("unknown client authentication", nil)
}

// assertionClaims are the claims of a client assertion (RFC 7523 section 3, Microsoft Entra ID
// certificate credentials).
type assertionClaims struct {
	Aud string `json:"aud"`
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
	Iss string `json:"iss"`
	Jti string `json:"jti"`
	Nbf int64  `json:"nbf"`
	Sub string `json:"sub"`
}

// clientAssertion signs the client assertion of conf for tokenEndpoint: header alg from
// client_key_alg, typ JWT, kid and x5t#S256 when configured; claims iss and sub the client id,
// aud the token endpoint, jti a new UUIDv7, iat and nbf now, exp now plus
// auth.oidc_client_assertion_ttl.
func (c *Client) clientAssertion(ctx context.Context, conf provider.OIDCConfig, tokenEndpoint string) (string, error) {
	pemBytes, err := c.d.Secrets.Resolve(ctx, conf.ClientKeyRef)
	if err != nil {
		return "", fail("resolving the client key", nil)
	}
	signer, err := keySigner(pemBytes, conf.ClientKeyAlg)
	if err != nil {
		return "", err
	}
	now := c.d.Clock.Now()
	payload, err := json.Marshal(assertionClaims{
		Aud: tokenEndpoint, Exp: now.Add(c.cfg.AssertionTTL).Unix(), Iat: now.Unix(), Iss: conf.ClientID, Jti: ids.New().String(),
		Nbf: now.Unix(), Sub: conf.ClientID,
	})
	if err != nil {
		return "", fail("encoding the client assertion", nil)
	}
	tok, err := jose.SignCompact(signer, jose.Header{Typ: "JWT", Kid: conf.ClientKeyID, X5tS256: conf.ClientCertX5tS256}, payload)
	if err != nil {
		return "", fail("signing the client assertion", nil)
	}
	return tok, nil
}

// keySigner reads a PKCS #8 PEM private key and returns the signer of alg: an RSA key of at least
// 2048 bits for PS256 and RS256, a P-256 key for ES256. The key never leaves this function.
func keySigner(pemBytes []byte, alg string) (jose.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fail("the client key is not a PKCS #8 PEM block", nil)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fail("the client key does not parse", nil)
	}
	switch k := key.(type) {
	case *rsa.PrivateKey:
		if k.N.BitLen() < 2048 {
			break
		}
		switch alg {
		case jose.AlgPS256:
			return jose.PS256Signer{Key: k}, nil
		case jose.AlgRS256:
			return jose.RS256Signer{Key: k}, nil
		}
	case *ecdsa.PrivateKey:
		if alg == jose.AlgES256 && k.Curve == elliptic.P256() {
			return jose.ES256Signer{Key: k}, nil
		}
	}
	return nil, fail("the client key does not fit client_key_alg", nil)
}

// verifyIDToken checks the JWS of an ID token and parses its claims.
func (c *Client) verifyIDToken(ctx context.Context, st *providerState, token string) (app.IDTokenClaims, error) {
	j, err := jose.ParseCompactJWS(token, IDTokenAlgs, "")
	if err != nil {
		return app.IDTokenClaims{}, fail("the ID token is not an allowed compact JWS", err)
	}
	key, err := c.keyFor(ctx, st, j.Header.Kid)
	if err != nil {
		return app.IDTokenClaims{}, err
	}
	if err := j.Verify(key); err != nil {
		return app.IDTokenClaims{}, fail("the ID token signature does not verify", err)
	}
	claims, err := oidcflow.ParseClaims(j.Payload)
	if err != nil {
		return app.IDTokenClaims{}, fail("the ID token claims do not parse", err)
	}
	return claims, nil
}

// keyFor returns the key kid of the provider's key set (without kid: the only key of the set). A
// stale set or an unknown kid loads the set once; a caller that waited while another caller
// loaded the set uses that load instead of loading again (single-flight per provider).
func (c *Client) keyFor(ctx context.Context, st *providerState, kid string) (jose.JWK, error) {
	before := st.keysGen.Load()
	st.mu.Lock()
	defer st.mu.Unlock()
	fresh := st.hasKeys && c.d.Clock.Now().Before(st.keysAt.Add(c.cfg.CacheTTL))
	if fresh {
		if k, ok := pick(st.keys, kid); ok {
			return k, nil
		}
		if st.keysGen.Load() != before {
			return jose.JWK{}, fail("no key of the provider has the kid of the ID token", nil)
		}
	}
	if err := c.loadKeys(ctx, st); err != nil {
		return jose.JWK{}, err
	}
	if k, ok := pick(st.keys, kid); ok {
		return k, nil
	}
	return jose.JWK{}, fail("no key of the provider has the kid of the ID token", nil)
}

// pick finds kid in keys; without kid the set must hold exactly one key.
func pick(keys []providerKey, kid string) (jose.JWK, bool) {
	if kid == "" {
		if len(keys) == 1 {
			return keys[0].key, true
		}
		return jose.JWK{}, false
	}
	for _, k := range keys {
		if k.kid == kid {
			return k.key, true
		}
	}
	return jose.JWK{}, false
}

// jwks is a JSON Web Key Set (RFC 7517 section 5).
type jwks struct {
	Keys []json.RawMessage `json:"keys"`
}

// loadKeys reads the key set of the document's jwks_uri; keys for encryption or of a type the
// profile does not verify are skipped. The caller holds st.mu.
func (c *Client) loadKeys(ctx context.Context, st *providerState) error {
	meta, err := c.metadata(ctx, st)
	if err != nil {
		return err
	}
	var set jwks
	if err := c.getJSON(ctx, st, meta.JWKSURI, &set); err != nil {
		return err
	}
	keys := make([]providerKey, 0, len(set.Keys))
	for _, raw := range set.Keys {
		var head struct {
			Kid string `json:"kid"`
			Use string `json:"use"`
		}
		if json.Unmarshal(raw, &head) != nil || head.Use != "" && head.Use != "sig" {
			continue
		}
		k, err := jose.ParseVerificationJWK(raw)
		if err != nil {
			continue
		}
		keys = append(keys, providerKey{kid: head.Kid, key: k})
	}
	st.keys, st.keysAt, st.hasKeys = keys, c.d.Clock.Now(), true
	st.keysGen.Add(1)
	return nil
}
