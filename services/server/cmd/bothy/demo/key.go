// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// coordLen is the length of a P-256 coordinate and of each half of an ES256 signature.
const coordLen = 32

// errNoKey is the error of an operation on the zero key.
var errNoKey = errors.New("demo: no key")

// key is an ES256 private key (ECDSA P-256) that lives in memory only: the DPoP key or the
// step-up key of a demo device (08 6.1), or the DPoP key an agent token is bound to (08 12). The
// zero key is unusable.
type key struct {
	priv *ecdsa.PrivateKey
}

// newKey returns a fresh P-256 key from crypto/rand. The error reports a failure of the random
// source.
func newKey() (key, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return key{}, fmt.Errorf("demo: generate key: %w", err)
	}
	return key{priv: k}, nil
}

// coords returns the base64url coordinates x and y of the public key.
func (k key) coords() (x, y string) {
	pub, err := k.priv.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil || len(pub) != 1+2*coordLen {
		return "", ""
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc(pub[1 : 1+coordLen]), enc(pub[1+coordLen:])
}

// publicJWK returns the public key as a JWK (RFC 7517) with exactly kty, crv, x and y.
func (k key) publicJWK() rest.JsonObject {
	if k.priv == nil {
		return nil
	}
	x, y := k.coords()
	return rest.JsonObject{"kty": "EC", "crv": "P-256", "x": x, "y": y}
}

// privateJWK returns the private key as a JWK with kty, crv, x, y and d: the form the hello of the
// Fake-Agent takes (D-M2-31-5). Only the exchange file of the demo agent receives it.
func (k key) privateJWK() map[string]any {
	if k.priv == nil {
		return nil
	}
	x, y := k.coords()
	d, err := k.priv.Bytes()
	if err != nil {
		return nil
	}
	return map[string]any{"kty": "EC", "crv": "P-256", "x": x, "y": y, "d": base64.RawURLEncoding.EncodeToString(d)}
}

// thumbprint returns the JWK thumbprint of the public key (RFC 7638): base64url of SHA-256 over
// crv, kty, x and y in this order without white space.
func (k key) thumbprint() string {
	if k.priv == nil {
		return ""
	}
	x, y := k.coords()
	sum := sha256.Sum256([]byte(`{"crv":"P-256","kty":"EC","x":"` + x + `","y":"` + y + `"}`))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// sign returns the compact JWS of header and claims with the ES256 signature R || S (RFC 7518 3.4).
func (k key) sign(header, claims map[string]any) (string, error) {
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
		return "", fmt.Errorf("demo: sign: %w", err)
	}
	sig := make([]byte, 2*coordLen)
	r.FillBytes(sig[:coordLen])
	s.FillBytes(sig[coordLen:])
	return input + "." + enc(sig), nil
}

// proof returns a DPoP proof (RFC 9449) signed with k: header typ dpop+jwt, alg ES256 and the
// public jwk; claims jti, htm, htu, iat and, when set, nonce and ath (base64url SHA-256 of the
// access token).
func (k key) proof(method, htu, nonce, accessToken string, now time.Time) (string, error) {
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	claims := map[string]any{
		"jti": base64.RawURLEncoding.EncodeToString(jti), "htm": method, "htu": htu, "iat": now.Unix(),
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if accessToken != "" {
		sum := sha256.Sum256([]byte(accessToken))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	return k.sign(map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": k.publicJWK()}, claims)
}

// stepUpSignature returns the step-up JWS of completeStepUpWithDeviceKey (08 6.2, M1-35): header
// {"alg":"ES256"} only, payload {challenge, sid, iat}, signed with the step-up key k.
func (k key) stepUpSignature(challenge, sessionID string, now time.Time) (string, error) {
	return k.sign(map[string]any{"alg": "ES256"}, map[string]any{"challenge": challenge, "sid": sessionID, "iat": now.Unix()})
}

// errPublicURL is the error of a public URL the server would not accept as server.public_url.
var errPublicURL = errors.New("demo: the public URL must be an absolute http or https URL without query, fragment or user")

// publicOrigin parses publicURL like the server parses server.public_url.
func publicOrigin(publicURL string) (*url.URL, error) {
	u, err := url.Parse(publicURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(publicURL, "#") {
		return nil, errPublicURL
	}
	return u, nil
}

// restHTU returns the htu a REST proof for path names (08 7.3): scheme and host of the public URL
// plus the escaped path, never the loopback address the command dials. A query is dropped.
func restHTU(public *url.URL, path string) string {
	p, _, _ := strings.Cut(path, "?")
	return public.Scheme + "://" + public.Host + (&url.URL{Path: p}).EscapedPath()
}

// newUUIDv7 returns a fresh UUIDv7 (RFC 9562 5.7): device_id, client_msg_id and Idempotency-Key
// are UUIDv7 in the contract. It panics when the system has no randomness, like crypto/rand.
func newUUIDv7() rest.Uuid {
	var u [16]byte
	if _, err := rand.Read(u[6:]); err != nil {
		panic(err)
	}
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(time.Now().UnixMilli())) // #nosec G115 -- the clock is after 1970
	copy(u[:6], ms[2:])
	u[6] = u[6]&0x0f | 0x70
	u[8] = u[8]&0x3f | 0x80
	return rest.Uuid(u)
}
