// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// FuzzParseHostKeys: any input gives findings or a valid result, never a panic and never a key
// that is not 32 bytes.
func FuzzParseHostKeys(f *testing.F) {
	noise, jwk, _ := testKeys(f)
	f.Add(noise, "OKP", "Ed25519", jwk["x"].(string), "")
	f.Add("", "", "", "", "d")
	f.Add("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "EC", "P-256", "x", "kid")
	f.Fuzz(func(t *testing.T, noise, kty, crv, x, extra string) {
		m := map[string]any{"kty": kty, "crv": crv, "x": x}
		if extra != "" {
			m[extra] = "v"
		}
		k, findings := domain.ParseHostKeys(noise, m)
		if len(findings) > 0 {
			return
		}
		if len(k.Noise) != 32 || len(k.AuthRaw) != 32 || k.JKT == "" || len(k.JWK) == 0 {
			t.Fatalf("valid result with keys of %d and %d bytes", len(k.Noise), len(k.AuthRaw))
		}
	})
}

// FuzzNormalizeFingerprintPrefix: any input gives an error or a prefix of exactly the length from
// the Crockford alphabet, never a panic.
func FuzzNormalizeFingerprintPrefix(f *testing.F) {
	f.Add("f0cc6qs6")
	f.Add("F0CC-6QS")
	f.Add("ıııııııı")
	f.Fuzz(func(t *testing.T, s string) {
		p, err := domain.NormalizeFingerprintPrefix(s, 8)
		if err != nil {
			return
		}
		if len(p) != 8 {
			t.Fatalf("prefix of %d bytes", len(p))
		}
		for _, c := range p {
			if c < '0' || c > 'Z' || c == 'I' || c == 'L' || c == 'O' || c == 'U' || (c > '9' && c < 'A') {
				t.Fatalf("character %q outside the alphabet", c)
			}
		}
	})
}
