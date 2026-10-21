// SPDX-License-Identifier: Apache-2.0

package control

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strconv"
	"time"
)

// ProtocolVersions are the protocol versions of hostControl this host service speaks.
var ProtocolVersions = []int{1}

// ChooseVersion returns the highest version in both own and server; false when they share none
// (the host service stops and logs "update the host service or the server").
func ChooseVersion(own, server []int) (int, bool) {
	best, ok := 0, false
	for _, v := range server {
		if slices.Contains(own, v) && (!ok || v > best) {
			best, ok = v, true
		}
	}
	return best, ok
}

// b64 is base64url without padding (RFC 7515).
var b64 = base64.RawURLEncoding

// Thumbprint returns the RFC 7638 thumbprint of the Ed25519 public key pub: SHA-256 of
// {"crv":"Ed25519","kty":"OKP","x":"<x>"} in base64url without padding, the kid of the proof and
// hosts.auth_jkt on the server.
func Thumbprint(pub ed25519.PublicKey) string {
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` + b64.EncodeToString(pub) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	return b64.EncodeToString(sum[:])
}

// Proof returns the proof of identify (24 section 3.5): a compact JWS with the header
// {"alg":"EdDSA","typ":"host-auth+jwt","kid":<thumbprint>} over exactly the claims
// {"challenge","host_id","iat"} (iat in seconds), signed with key.
func Proof(key ed25519.PrivateKey, hostID, challenge string, iat time.Time) string {
	pub, _ := key.Public().(ed25519.PublicKey)
	header, _ := json.Marshal(struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}{"EdDSA", "host-auth+jwt", Thumbprint(pub)}) // strings only: never fails
	claims := `{"challenge":` + strconv.Quote(challenge) + `,"host_id":` + strconv.Quote(hostID) + `,"iat":` + strconv.FormatInt(iat.Unix(), 10) + `}`
	input := b64.EncodeToString(header) + "." + b64.EncodeToString([]byte(claims))
	return input + "." + b64.EncodeToString(ed25519.Sign(key, []byte(input)))
}

// PublicJWK returns the public JWK of key as the server stores it (hosts.auth_public_jwk).
func PublicJWK(key ed25519.PrivateKey) map[string]string {
	pub, _ := key.Public().(ed25519.PublicKey)
	return map[string]string{"kty": "OKP", "crv": "Ed25519", "x": b64.EncodeToString(pub)}
}
