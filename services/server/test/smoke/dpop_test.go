// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

// jwsParts splits a compact JWS into its decoded header and payload and the raw signature.
func jwsParts(t *testing.T, jws string) (header, payload map[string]any, input string, sig []byte) {
	t.Helper()
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		t.Fatalf("JWS has %d parts, want 3", len(parts))
	}
	decode := func(s string, v any) {
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatalf("segment is not base64url without padding: %v", err)
		}
		if err := json.Unmarshal(raw, v); err != nil {
			t.Fatalf("segment is not JSON: %v", err)
		}
	}
	decode(parts[0], &header)
	decode(parts[1], &payload)
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("signature: %v", err)
	}
	return header, payload, parts[0] + "." + parts[1], raw
}

// publicKeyOf rebuilds the P-256 public key of a JWK member map.
func publicKeyOf(t *testing.T, jwk map[string]any) *ecdsa.PublicKey {
	t.Helper()
	coord := func(name string) *big.Int {
		s, _ := jwk[name].(string)
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil || len(raw) != 32 {
			t.Fatalf("jwk %s: %d bytes, %v", name, len(raw), err)
		}
		return new(big.Int).SetBytes(raw)
	}
	if jwk["kty"] != "EC" || jwk["crv"] != "P-256" {
		t.Fatalf("jwk kty %v crv %v", jwk["kty"], jwk["crv"])
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: coord("x"), Y: coord("y")}
}

func newTestKey(t *testing.T) Key {
	t.Helper()
	k, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestNewProofClaims: header typ, alg and the public jwk only; claims jti, htm, htu without query
// and fragment, iat, nonce when set and ath only with a token; jti differs between two proofs.
func TestNewProofClaims(t *testing.T) {
	t.Parallel()
	k := newTestKey(t)
	now := time.Unix(1_790_000_000, 0)
	proof, err := NewProof(k, "POST", "https://chat.example.com/api/v1/spaces?x=1#frag", "n-1", "tok", now)
	if err != nil {
		t.Fatal(err)
	}
	h, c, _, _ := jwsParts(t, proof)
	if h["typ"] != "dpop+jwt" || h["alg"] != "ES256" {
		t.Errorf("header typ %v alg %v", h["typ"], h["alg"])
	}
	jwk, _ := h["jwk"].(map[string]any)
	if _, private := jwk["d"]; private || len(jwk) != 4 {
		t.Errorf("header jwk has members %v, want exactly kty, crv, x, y", jwk)
	}
	sum := sha256.Sum256([]byte("tok"))
	want := map[string]any{
		"htm": "POST", "htu": "https://chat.example.com/api/v1/spaces", "iat": float64(now.Unix()),
		"nonce": "n-1", "ath": base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	for name, v := range want {
		if c[name] != v {
			t.Errorf("claim %s = %v, want %v", name, c[name], v)
		}
	}
	if s, _ := c["jti"].(string); len(s) < 16 {
		t.Errorf("jti %q is too short", s)
	}

	other, err := NewProof(k, "GET", "https://chat.example.com/api/v1/me", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	_, c2, _, _ := jwsParts(t, other)
	if c2["jti"] == c["jti"] {
		t.Error("two proofs share a jti")
	}
	for _, name := range []string{"ath", "nonce"} {
		if _, ok := c2[name]; ok {
			t.Errorf("claim %s present without a value", name)
		}
	}
}

// TestNewProofVerifiesWithPublicKey: the signature (R || S) verifies with the public JWK of the
// header and with the key's PublicJWK.
func TestNewProofVerifiesWithPublicKey(t *testing.T) {
	t.Parallel()
	k := newTestKey(t)
	proof, err := NewProof(k, "GET", "https://chat.example.com/api/v1/gateway", "n", "tok", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h, _, input, sig := jwsParts(t, proof)
	if len(sig) != 64 {
		t.Fatalf("signature has %d bytes, want 64 (R || S)", len(sig))
	}
	hash := sha256.Sum256([]byte(input))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	jwk, _ := h["jwk"].(map[string]any)
	if !ecdsa.Verify(publicKeyOf(t, jwk), hash[:], r, s) {
		t.Error("proof does not verify with the header jwk")
	}
	if !ecdsa.Verify(publicKeyOf(t, k.PublicJWK()), hash[:], r, s) {
		t.Error("proof does not verify with PublicJWK")
	}
}

// TestThumbprintRFC7638: SHA-256 over the canonical JSON {"crv","kty","x","y"}, computed here
// without the package.
func TestThumbprintRFC7638(t *testing.T) {
	t.Parallel()
	k := newTestKey(t)
	jwk := k.PublicJWK()
	canonical := `{"crv":"P-256","kty":"EC","x":"` + jwk["x"].(string) + `","y":"` + jwk["y"].(string) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	if got, want := k.Thumbprint(), base64.RawURLEncoding.EncodeToString(sum[:]); got != want {
		t.Errorf("Thumbprint = %s, want %s", got, want)
	}
}

// TestStepUpSignature: header alg ES256 without typ or jwk, payload challenge, sid and iat, and a
// signature of the step-up key.
func TestStepUpSignature(t *testing.T) {
	t.Parallel()
	k := newTestKey(t)
	now := time.Unix(1_790_000_000, 0)
	jws, err := StepUpSignature(k, "ch-1", "0192f000-0000-7000-8000-000000000001", now)
	if err != nil {
		t.Fatal(err)
	}
	h, p, input, sig := jwsParts(t, jws)
	if len(h) != 1 || h["alg"] != "ES256" {
		t.Errorf("header %v, want only alg ES256", h)
	}
	if p["challenge"] != "ch-1" || p["sid"] != "0192f000-0000-7000-8000-000000000001" || p["iat"] != float64(now.Unix()) || len(p) != 3 {
		t.Errorf("payload %v", p)
	}
	hash := sha256.Sum256([]byte(input))
	if !ecdsa.Verify(publicKeyOf(t, k.PublicJWK()), hash[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Error("step-up signature does not verify")
	}
}
