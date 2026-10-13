// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// accessTokenTyp is the header typ of an access token (RFC 9068 section 2.1): it keeps access
// tokens apart from every other JWS of the server (08 "7.2").
const accessTokenTyp = "at+jwt"

// accessTokenAlgs is the allowlist of access token algorithms: the algorithm of the server
// signing keys (instance.AlgEdDSA, ADR 0066).
var accessTokenAlgs = []string{instance.AlgEdDSA}

// invalidTokenChallenge is the WWW-Authenticate of a rejected access token (RFC 9449 section 7.1).
const invalidTokenChallenge = `DPoP error="invalid_token", algs="` + identity.DPoPAlgs + `"`

// AccessTokenConfig are the values of the access token issuer.
type AccessTokenConfig struct {
	// InstanceID is instance.instance_id, the issuer (iss).
	InstanceID ids.ID
	// PublicURL is server.public_url; aud is its origin plus /api/v1.
	PublicURL string
	// TTL is auth.access_token_ttl.
	TTL time.Duration
	// Leeway is auth.clock_leeway for exp and nbf.
	Leeway time.Duration
}

// AccessTokens issues and verifies the access tokens of people (08 "7.2") with the server signing
// keys of purpose access_token (instance.Keys). It is safe for concurrent use.
type AccessTokens struct {
	cfg  AccessTokenConfig
	aud  string
	keys instance.Keys
	clk  clock.Clock
}

// NewAccessTokens returns the issuer. Errors: a zero instance id, a public URL that is no origin,
// a TTL not above zero, a negative leeway or no key port.
func NewAccessTokens(cfg AccessTokenConfig, keys instance.Keys, clk clock.Clock) (*AccessTokens, error) {
	aud, err := session.Audience(cfg.PublicURL)
	if err != nil {
		return nil, err
	}
	if cfg.InstanceID.IsZero() || cfg.TTL <= 0 || cfg.Leeway < 0 || keys == nil {
		return nil, errors.New("identity: access tokens need the instance id, auth.access_token_ttl above 0, auth.clock_leeway not negative and the signing keys")
	}
	if clk == nil {
		clk = clock.System()
	}
	return &AccessTokens{cfg: cfg, aud: aud, keys: keys, clk: clk}, nil
}

// Issue returns a new access token of session s bound to jkt, with its claims. Errors: those of
// the signing key port (unavailable without an active key).
func (t *AccessTokens) Issue(ctx context.Context, s SessionRecord, jkt string) (string, session.Claims, error) {
	sg, err := t.keys.Signer(ctx, instance.PurposeAccessToken)
	if err != nil {
		return "", session.Claims{}, err
	}
	c := session.BuildClaims(session.ClaimsInput{
		InstanceID: t.cfg.InstanceID, Audience: t.aud, UserID: s.UserID, SessionID: s.ID, DeviceID: s.DeviceID,
		JTI: ids.New(), Now: t.clk.Now(), TTL: t.cfg.TTL, SessionEnd: s.ExpiresAt, JKT: jkt, AuthMethod: string(s.AuthMethod),
	})
	payload, err := json.Marshal(c)
	if err != nil {
		return "", session.Claims{}, errs.Wrap(errs.CodeInternalError, err, "identity: access token claims")
	}
	sign := jose.SignerFunc{Algorithm: sg.Alg(), Fn: func(in []byte) ([]byte, error) { return sg.Sign(ctx, in) }}
	raw, err := jose.SignCompact(sign, jose.Header{Typ: accessTokenTyp, Kid: sg.KeyID()}, payload)
	if err != nil {
		if _, ok := errs.As(err); ok {
			return "", session.Claims{}, err
		}
		return "", session.Claims{}, errs.Wrap(errs.CodeInternalError, err, "identity: signing an access token")
	}
	return raw, c, nil
}

// rejected is the 401 of an access token that is not valid: code with the invalid_token challenge.
func rejected(code errs.Code) error {
	return errs.New(code, "").WithHeader("WWW-Authenticate", invalidTokenChallenge)
}

// key returns the verification key of the header of j when its state is active or retiring;
// found is false for an unknown, next or retired key. Other errors (database) come back.
func (t *AccessTokens) key(ctx context.Context, kid string) (jose.JWK, bool, error) {
	if kid == "" {
		return jose.JWK{}, false, nil
	}
	pk, err := t.keys.VerificationKey(ctx, instance.PurposeAccessToken, kid)
	if errs.KindOf(err) == errs.KindNotFound {
		return jose.JWK{}, false, nil
	}
	if err != nil {
		return jose.JWK{}, false, err
	}
	if pk.State != instance.KeyActive && pk.State != instance.KeyRetiring {
		return jose.JWK{}, false, nil
	}
	k, err := jose.JWKFromPublicKey(pk.Key)
	if err != nil {
		return jose.JWK{}, false, nil
	}
	return k, true, nil
}

// Verify checks raw in the order of 08 "7.2": header typ at+jwt and the algorithm allowlist, kid
// of an access_token key in state active or retiring, signature, iss, aud, then nbf <= now +
// leeway and exp > now - leeway. Errors: 401 unauthenticated for a malformed token, an unknown or
// unusable key, a bad signature or a wrong iss or aud, 401 token_expired for an expired one, both
// with the invalid_token challenge; errors of the key port (database) come back unchanged.
func (t *AccessTokens) Verify(ctx context.Context, raw string) (session.Claims, error) {
	j, err := jose.ParseCompactJWS(raw, accessTokenAlgs, accessTokenTyp)
	if err != nil {
		return session.Claims{}, rejected(errs.CodeUnauthenticated)
	}
	k, found, err := t.key(ctx, j.Header.Kid)
	if err != nil {
		return session.Claims{}, err
	}
	if !found || j.Verify(k) != nil {
		return session.Claims{}, rejected(errs.CodeUnauthenticated)
	}
	var c session.Claims
	dec := json.NewDecoder(bytes.NewReader(j.Payload))
	dec.DisallowUnknownFields()
	if dec.Decode(&c) != nil || c.Issuer != t.cfg.InstanceID.String() || c.Audience != t.aud || !wellFormed(c) {
		return session.Claims{}, rejected(errs.CodeUnauthenticated)
	}
	now := t.clk.Now()
	if time.Unix(c.NotBefore, 0).After(now.Add(t.cfg.Leeway)) {
		return session.Claims{}, rejected(errs.CodeUnauthenticated)
	}
	if !time.Unix(c.Expiry, 0).After(now.Add(-t.cfg.Leeway)) {
		return session.Claims{}, rejected(errs.CodeTokenExpired)
	}
	return c, nil
}

// wellFormed reports claims whose ids parse and whose key binding is set.
func wellFormed(c session.Claims) bool {
	for _, s := range []string{c.Subject, c.SessionID, c.DeviceID} {
		if _, err := ids.Parse(s); err != nil {
			return false
		}
	}
	return c.Cnf.JKT != "" && len(c.AMR) == 1
}

// IsOwnAccessToken reports whether raw has the form of this server's access token: header typ
// at+jwt and the kid of an access_token key in state active or retiring. The Bearer branch of the
// Authenticator uses it to answer invalid_dpop_proof for a DPoP-bound token sent as Bearer (RFC
// 9449 section 7.2, ID-T14). It checks no signature: the answer is 401 either way.
func (t *AccessTokens) IsOwnAccessToken(ctx context.Context, raw string) bool {
	j, err := jose.ParseCompactJWS(raw, accessTokenAlgs, accessTokenTyp)
	if err != nil {
		return false
	}
	_, found, err := t.key(ctx, j.Header.Kid)
	return err == nil && found
}
