// SPDX-License-Identifier: Apache-2.0

// Package sessiontest holds the test doubles of the session manager (brief M2-45 Design note 17):
// a signer of terminal session tokens with its JWKS, the fake server with the control and the data
// endpoint on 127.0.0.1 port 0, and the Go test device (Noise initiator and frames). Never imported
// by production code.
package sessiontest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"
)

var b64 = base64.RawURLEncoding

// Signer signs session tokens with an Ed25519 key of purpose terminal.
type Signer struct {
	Kid string
	Key ed25519.PrivateKey
}

// NewSigner returns a signer with a fresh key and kid.
func NewSigner(kid string) *Signer {
	_, k, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return &Signer{Kid: kid, Key: k}
}

// JWK returns the public key as the jwks of hello carries it.
func (s *Signer) JWK() map[string]any {
	pub, _ := s.Key.Public().(ed25519.PublicKey)
	return map[string]any{"kty": "OKP", "crv": "Ed25519", "x": b64.EncodeToString(pub), "kid": s.Kid, "alg": "EdDSA", "use": "sig"}
}

// SignRaw signs the header and the claims as given (a test may put anything into them).
func (s *Signer) SignRaw(header, claims any) string {
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	input := b64.EncodeToString(hb) + "." + b64.EncodeToString(cb)
	return input + "." + b64.EncodeToString(ed25519.Sign(s.Key, []byte(input)))
}

// Sign signs claims with the header of 24 5.3.
func (s *Signer) Sign(claims map[string]any) string {
	return s.SignRaw(map[string]any{"alg": "EdDSA", "kid": s.Kid, "typ": "terminal-session+jwt"}, claims)
}

// Claims returns valid claims of a session token: issued now, valid for ttl.
func Claims(host, session, sub, osu, prf, cek, att string, now time.Time, ttl time.Duration) map[string]any {
	return map[string]any{
		"iss": "5f0c6a52-0000-7000-8000-000000000001", "aud": host, "sub": sub, "sid": session, "jti": NewID(),
		"iat": now.Unix(), "exp": now.Add(ttl).Unix(), "dev": "5f0c6a52-0000-7000-8000-0000000000d1", "prf": prf,
		"cek": cek, "own": false, "osu": osu, "att": att,
	}
}

// NewID returns a random UUID of version 7 layout (the bytes are random; only the form matters).
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x70
	b[8] = b[8]&0x3f | 0x80
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, c := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[c>>4], hex[c&0x0f])
	}
	return string(out)
}
