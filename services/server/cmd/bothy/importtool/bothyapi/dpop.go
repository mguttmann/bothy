// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// dpopTyp is the explicit type of a proof (RFC 9449 section 4.2).
const dpopTyp = "dpop+jwt"

// jtiBytes is the randomness of a jti: 128 bits (RFC 9449 section 11.1).
const jtiBytes = 16

// errInvalidKey is a session key that is not an Ed25519 private JWK. It never quotes the key.
var errInvalidKey = errors.New("bothyapi: the DPoP key of the session is not an Ed25519 private JWK")

// proofKey is the Ed25519 DPoP key of the device (EdDSA, docs/DECISIONS.md D-M8-10-7). It has no
// String method that could print it; the session file holds it as a private JWK.
type proofKey struct {
	priv ed25519.PrivateKey
	pub  json.RawMessage
}

// newProofKey generates a fresh key.
func newProofKey() (*proofKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return keyOf(priv)
}

// keyOf returns the proof key of priv with its public JWK.
func keyOf(priv ed25519.PrivateKey) (*proofKey, error) {
	jwk, err := jose.JWKFromPublicKey(priv.Public())
	if err != nil {
		return nil, errInvalidKey
	}
	pub, err := json.Marshal(jwk)
	if err != nil {
		return nil, errInvalidKey
	}
	return &proofKey{priv: priv, pub: pub}, nil
}

// privateJWK is the RFC 8037 form of an Ed25519 private key.
type privateJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	D   string `json:"d"`
}

// parseProofKey reads the private JWK of the session file. Errors: errInvalidKey.
func parseProofKey(raw json.RawMessage) (*proofKey, error) {
	var k privateJWK
	if json.Unmarshal(raw, &k) != nil || k.Kty != "OKP" || k.Crv != "Ed25519" {
		return nil, errInvalidKey
	}
	seed, err := base64.RawURLEncoding.DecodeString(k.D)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errInvalidKey
	}
	pk, err := keyOf(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		return nil, err
	}
	if pubX, err := base64.RawURLEncoding.DecodeString(k.X); err != nil || !pk.priv.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(pubX)) {
		return nil, errInvalidKey
	}
	return pk, nil
}

// privateJWK returns the key as a private JWK for the session file.
func (k *proofKey) privateJWK() json.RawMessage {
	pub := k.priv.Public().(ed25519.PublicKey)
	raw, _ := json.Marshal(privateJWK{Kty: "OKP", Crv: "Ed25519",
		X: base64.RawURLEncoding.EncodeToString(pub), D: base64.RawURLEncoding.EncodeToString(k.priv.Seed())})
	return raw
}

// publicJWK returns the public JWK of the key, without private members.
func (k *proofKey) publicJWK() json.RawMessage { return k.pub }

// proofHTU returns the htu of a request: the origin of the server URL plus the request path,
// without query and fragment (08 7.3). origin is a checked origin without a trailing slash.
func proofHTU(origin, path string) string { return origin + path }

// proof returns a DPoP proof (RFC 9449 section 4.2) for htm and htu at now: typ dpop+jwt, alg
// EdDSA, the public jwk, a random jti of 128 bits, iat, ath of token when token is not empty and
// nonce when it is not empty.
func (k *proofKey) proof(htm, htu, token, nonce string, now time.Time) (string, error) {
	jti := make([]byte, jtiBytes)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	claims := map[string]any{
		"jti": base64.RawURLEncoding.EncodeToString(jti),
		"htm": htm,
		"htu": htu,
		"iat": json.Number(strconv.FormatInt(now.Unix(), 10)),
	}
	if token != "" {
		sum := sha256.Sum256([]byte(token))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return jose.SignCompact(jose.EdDSASigner{Key: k.priv}, jose.Header{Typ: dpopTyp, JWK: k.pub}, payload)
}
