// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// coordLen is the length of a P-256 coordinate and of each half of an ES256 signature.
const coordLen = 32

// errNoKey is the error of an operation on the zero Key.
var errNoKey = errors.New("smoke: no key")

// Key is an ES256 private key (ECDSA P-256) that lives in memory only: the DPoP key or the step-up
// key of the smoke device (docs/spec/08-identity.md 6.1). The zero Key is unusable.
type Key struct {
	priv *ecdsa.PrivateKey
}

// NewKey returns a fresh P-256 key from crypto/rand. The error reports a failure of the random
// source.
func NewKey() (Key, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Key{}, fmt.Errorf("smoke: generate key: %w", err)
	}
	return Key{priv: k}, nil
}

// coords returns the base64url coordinates x and y of the public key.
func (k Key) coords() (x, y string) {
	pub, err := k.priv.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil || len(pub) != 1+2*coordLen {
		return "", ""
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc(pub[1 : 1+coordLen]), enc(pub[1+coordLen:])
}

// PublicJWK returns the public key as a JWK (RFC 7517) with exactly the members kty, crv, x and
// y, never the private member d. It returns nil for the zero Key.
func (k Key) PublicJWK() rest.JsonObject {
	if k.priv == nil {
		return nil
	}
	x, y := k.coords()
	return rest.JsonObject{"kty": "EC", "crv": "P-256", "x": x, "y": y}
}

// Thumbprint returns the JWK thumbprint of the public key (RFC 7638): base64url of SHA-256 over
// the members crv, kty, x and y in this order without white space. It returns "" for the zero Key.
func (k Key) Thumbprint() string {
	if k.priv == nil {
		return ""
	}
	x, y := k.coords()
	sum := sha256.Sum256([]byte(`{"crv":"P-256","kty":"EC","x":"` + x + `","y":"` + y + `"}`))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// sign returns the compact JWS of header and claims: base64url segments without padding and the
// ES256 signature as R || S of 32 bytes each (RFC 7518 3.4).
func (k Key) sign(header, claims map[string]any) (string, error) {
	if k.priv == nil {
		return "", errNoKey
	}
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString
	input := enc(h) + "." + enc(c)
	sum := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, k.priv, sum[:])
	if err != nil {
		return "", fmt.Errorf("smoke: sign: %w", err)
	}
	sig := make([]byte, 2*coordLen)
	r.FillBytes(sig[:coordLen])
	s.FillBytes(sig[coordLen:])
	return input + "." + enc(sig), nil
}

// NewProof returns a DPoP proof (RFC 9449) signed with k: header typ dpop+jwt, alg ES256 and the
// public jwk; claims jti (random), htm = method, htu without query and fragment, iat = now, nonce
// when nonce is not empty and ath (base64url SHA-256 of accessToken) when accessToken is not
// empty. The error reports the zero Key, an htu that is no URL or a failure of the random source.
//
// Example:
//
//	proof, err := NewProof(k, "GET", "https://chat.example.com/api/v1/me", nonce, token, time.Now())
func NewProof(k Key, method, htu, nonce, accessToken string, now time.Time) (string, error) {
	u, err := url.Parse(htu)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("smoke: htu is no absolute URL")
	}
	u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = "", "", "", false
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	claims := map[string]any{
		"jti": base64.RawURLEncoding.EncodeToString(jti), "htm": method, "htu": u.String(), "iat": now.Unix(),
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if accessToken != "" {
		sum := sha256.Sum256([]byte(accessToken))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	return k.sign(map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": k.PublicJWK()}, claims)
}

// StepUpSignature returns the step-up JWS of completeStepUpWithDeviceKey in the format M1-35
// documents (docs/spec/08-identity.md 6.2): header {"alg":"ES256"} only, payload {challenge, sid,
// iat} signed with the step-up key k. sessionID is the session of the access token. The error
// reports the zero Key or a failure of the random source.
func StepUpSignature(k Key, challenge, sessionID string, now time.Time) (string, error) {
	return k.sign(map[string]any{"alg": "ES256"}, map[string]any{"challenge": challenge, "sid": sessionID, "iat": now.Unix()})
}
