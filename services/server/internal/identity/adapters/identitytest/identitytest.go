// SPDX-License-Identifier: Apache-2.0

// Package identitytest holds test helpers of the identity module (docs/spec/02-architecture.md "6.
// Code-Layout"): device keys that sign like a client, DPoP proofs with every claim steerable, and
// compact JWS. The tests of M1-33 and later tickets (sessions M1-34, step-up M1-35, gateway
// M1-64, end-to-end M1-73) use them. Only _test.go files import this package;
// TestIdentitytest_NotInBinary fails when the server binary depends on it.
//
// Every helper panics on a failure of the standard library (key generation, signing): that is a
// broken test environment, not a case to test.
//
// Example:
//
//	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
//	proof := identitytest.SignDPoPProof(key, identitytest.ProofClaims{
//		HTM: "GET", HTU: "https://chat.example.com/api/v1/me", IAT: now, Nonce: verifier.CurrentNonce()})
package identitytest

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// Key is a device key held by a test client: a jose.Signer with its public JWK.
type Key struct {
	jose.Signer
	// JWK is the public JWK the key registers and puts into the header of its proofs.
	JWK json.RawMessage
	// Thumbprint is the RFC 7638 thumbprint of JWK.
	Thumbprint string
}

// NewDeviceKey returns a fresh key of alg (jose.AlgES256 or jose.AlgEdDSA) and its public key in
// the form Devices.Register takes. It panics for another algorithm.
func NewDeviceKey(alg string) (*Key, identity.PublicKey) {
	var signer jose.Signer
	var pub any
	switch alg {
	case jose.AlgES256:
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}
		signer, pub = jose.ES256Signer{Key: k}, &k.PublicKey
	case jose.AlgEdDSA:
		p, k, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			panic(err)
		}
		signer, pub = jose.EdDSASigner{Key: k}, p
	default:
		panic("identitytest: unsupported algorithm " + alg)
	}
	jwk, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		panic(err)
	}
	raw, err := json.Marshal(jwk)
	if err != nil {
		panic(err)
	}
	return &Key{Signer: signer, JWK: raw, Thumbprint: jwk.Thumbprint()}, identity.PublicKey{JWK: raw, Alg: alg}
}

// ProofClaims steer one DPoP proof. Zero values have defaults: JTI a fresh UUIDv7, Typ
// "dpop+jwt", the header jwk the key's own; IAT zero leaves iat out, Nonce "" leaves nonce out,
// ATH "" leaves ath out (use dpop.ATH for the right value).
type ProofClaims struct {
	JTI, HTM, HTU string
	IAT           time.Time
	Nonce         string
	ATH           string
	// Typ replaces the header typ; JWK replaces the header jwk.
	Typ string
	JWK json.RawMessage
	// Extra are further payload members; a value replaces a claim of the same name.
	Extra map[string]any
}

// SignDPoPProof returns the compact proof of c signed with key.
func SignDPoPProof(key *Key, c ProofClaims) string {
	payload := map[string]any{"jti": c.JTI, "htm": c.HTM, "htu": c.HTU}
	if c.JTI == "" {
		payload["jti"] = ids.New().String()
	}
	if !c.IAT.IsZero() {
		payload["iat"] = json.Number(strconv.FormatInt(c.IAT.Unix(), 10))
	}
	if c.Nonce != "" {
		payload["nonce"] = c.Nonce
	}
	if c.ATH != "" {
		payload["ath"] = c.ATH
	}
	for k, v := range c.Extra {
		payload[k] = v
	}
	h := jose.Header{Typ: c.Typ, JWK: c.JWK}
	if h.Typ == "" {
		h.Typ = "dpop+jwt"
	}
	if h.JWK == nil {
		h.JWK = key.JWK
	}
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return SignJWS(key, h, body)
}

// SignJWS returns the compact JWS of payload with header, signed by signer (jose.SignCompact).
func SignJWS(signer jose.Signer, header jose.Header, payload []byte) string {
	s, err := jose.SignCompact(signer, header, payload)
	if err != nil {
		panic(err)
	}
	return s
}
