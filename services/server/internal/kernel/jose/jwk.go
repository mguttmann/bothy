// SPDX-License-Identifier: Apache-2.0

package jose

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
)

// The algorithms of the profile (ADR 0067).
const (
	// AlgES256 is ECDSA with P-256 and SHA-256, signature R || S with 64 bytes.
	AlgES256 = "ES256"
	// AlgEdDSA is Ed25519 (RFC 8037).
	AlgEdDSA = "EdDSA"
	// AlgRS256 is RSASSA-PKCS1-v1_5 with SHA-256 (RFC 7518 section 3.3): ID tokens of identity
	// providers and client assertions only (M1-41).
	AlgRS256 = "RS256"
	// AlgPS256 is RSASSA-PSS with SHA-256, MGF1 with SHA-256 and a salt of 32 bytes (RFC 7518
	// section 3.5): ID tokens and client assertions only (M1-41).
	AlgPS256 = "PS256"
)

// The key types and curves of the profile.
const (
	ktyEC     = "EC"
	ktyOKP    = "OKP"
	ktyRSA    = "RSA"
	crvP256   = "P-256"
	crvEd     = "Ed25519"
	coordSize = 32
)

// ErrInvalidJWK is wrapped by every error of ParseJWK and JWKFromPublicKey. The wrapping error
// names the rule, never the key material.
var ErrInvalidJWK = errors.New("jose: invalid JWK")

// privateMembers are the JWK members that carry private or symmetric key material (RFC 7518
// sections 6.2.2, 6.3.2 and 6.4.1); a public key never has them.
var privateMembers = []string{"d", "p", "q", "dp", "dq", "qi", "k", "oth"}

// JWK is a validated public key of the profile: EC P-256 or OKP Ed25519, and RSA for the keys of
// identity providers (ParseVerificationJWK, M1-41). Build it with ParseJWK, ParseVerificationJWK
// or JWKFromPublicKey; the zero value is no key.
type JWK struct {
	// Kty is EC, OKP or RSA.
	Kty string
	// Crv is P-256 or Ed25519; empty for RSA.
	Crv string
	// X and Y are the base64url coordinates; Y is empty for OKP, both are empty for RSA.
	X, Y string
	// N and E are the base64url modulus and exponent of an RSA key; empty otherwise.
	N, E string
	// Alg is the alg member of the JWK, "" when absent.
	Alg string
	// pub is *ecdsa.PublicKey, ed25519.PublicKey or *rsa.PublicKey.
	pub any
}

// rsaMembers is the wire form of an RSA key in the member order of RFC 7638 (e, kty, n).
type rsaMembers struct {
	E   string `json:"e"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	Alg string `json:"alg,omitempty"`
}

// The bounds of an RSA key of an identity provider: at least 2048 bits (NIST SP 800-131A), at
// most 8192 so that one key cannot make a check expensive, and a public exponent that is odd and
// fits crypto/rsa.
const (
	rsaMinBits = 2048
	rsaMaxBits = 8192
	rsaMaxE    = 1<<31 - 1
)

// jwkMembers is the wire form of the members the profile reads; Y and Alg stay out when empty.
type jwkMembers struct {
	Crv string `json:"crv"`
	Kty string `json:"kty"`
	X   string `json:"x"`
	Y   string `json:"y,omitempty"`
	Alg string `json:"alg,omitempty"`
}

// ParseJWK validates raw as a public JWK of the profile (08 "7.3", brief M1-33 "Device
// registration" item 6): one JSON object without duplicate members; for kty EC the curve P-256
// with x and y of exactly 32 bytes each and a point on the curve; for kty OKP the curve Ed25519
// with x of exactly 32 bytes; no private or symmetric member (d, p, q, dp, dq, qi, k, oth); an alg
// member, if present, ES256 for EC and EdDSA for OKP. Unknown members are ignored and never part
// of the thumbprint.
//
// Errors wrap ErrInvalidJWK.
//
// Example:
//
//	k, err := jose.ParseJWK([]byte(`{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`))
//	jkt := k.Thumbprint() // "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k"
func ParseJWK(raw []byte) (JWK, error) {
	if err := checkObject(raw); err != nil {
		return JWK{}, fmt.Errorf("%w: %w", ErrInvalidJWK, err)
	}
	k, err := readMembers(raw)
	if err != nil {
		return JWK{}, err
	}
	switch k.Kty {
	case ktyEC:
		err = k.parseEC()
	case ktyOKP:
		err = k.parseOKP()
	default:
		err = fmt.Errorf("%w: kty must be EC or OKP", ErrInvalidJWK)
	}
	if err != nil {
		return JWK{}, err
	}
	if k.Alg != "" && k.Alg != k.keyAlg() {
		return JWK{}, fmt.Errorf("%w: alg member does not match the key type", ErrInvalidJWK)
	}
	return k, nil
}

// ParseVerificationJWK validates raw as a public key that verifies signatures of an identity
// provider (ID tokens, ADR 0067, M1-41): every key ParseJWK accepts, and kty RSA with n of 2048 to
// 8192 bits (odd, no leading zero byte) and e odd between 3 and 2^31-1 (no leading zero byte); an
// alg member of an RSA key is RS256 or PS256. Private members are refused for every kty. Device
// keys and DPoP proofs keep ParseJWK, which refuses RSA.
//
// Errors wrap ErrInvalidJWK.
//
// Example:
//
//	k, err := jose.ParseVerificationJWK(jwksEntry)
//	if err == nil {
//		err = idToken.Verify(k) // wraps jose.ErrSignature
//	}
func ParseVerificationJWK(raw []byte) (JWK, error) {
	if err := checkObject(raw); err != nil {
		return JWK{}, fmt.Errorf("%w: %w", ErrInvalidJWK, err)
	}
	k, err := readMembers(raw)
	if err != nil {
		return JWK{}, err
	}
	if k.Kty != ktyRSA {
		return ParseJWK(raw)
	}
	if err := k.parseRSA(); err != nil {
		return JWK{}, err
	}
	if k.Alg != "" && k.Alg != AlgRS256 && k.Alg != AlgPS256 {
		return JWK{}, fmt.Errorf("%w: alg member does not match the key type", ErrInvalidJWK)
	}
	return k, nil
}

// parseRSA checks an RSA key: n and e as unsigned big-endian integers without leading zero bytes,
// the size and exponent bounds above, and an odd modulus.
func (k *JWK) parseRSA() error {
	n, okN := decodeSegment(k.N)
	e, okE := decodeSegment(k.E)
	if !okN || !okE || len(n) == 0 || len(e) == 0 || n[0] == 0 || e[0] == 0 {
		return fmt.Errorf("%w: n and e must be unsigned integers in base64url without padding", ErrInvalidJWK)
	}
	mod := new(big.Int).SetBytes(n)
	if bits := mod.BitLen(); bits < rsaMinBits || bits > rsaMaxBits || mod.Bit(0) == 0 {
		return fmt.Errorf("%w: the modulus must be odd and have 2048 to 8192 bits", ErrInvalidJWK)
	}
	exp := new(big.Int).SetBytes(e)
	if !exp.IsInt64() || exp.Int64() < 3 || exp.Int64() > rsaMaxE || exp.Bit(0) == 0 {
		return fmt.Errorf("%w: the exponent must be odd and between 3 and 2^31-1", ErrInvalidJWK)
	}
	k.Crv, k.X, k.Y = "", "", ""
	k.pub = &rsa.PublicKey{N: mod, E: int(exp.Int64())}
	return nil
}

// readMembers reads kty, crv, x, y and alg of raw (a checked JSON object) and refuses private
// and symmetric members.
func readMembers(raw []byte) (JWK, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return JWK{}, fmt.Errorf("%w: not an object", ErrInvalidJWK)
	}
	for _, m := range privateMembers {
		if _, ok := all[m]; ok {
			return JWK{}, fmt.Errorf("%w: private or symmetric member %q", ErrInvalidJWK, m)
		}
	}
	var k JWK
	for _, f := range []struct {
		name string
		dst  *string
	}{{"kty", &k.Kty}, {"crv", &k.Crv}, {"x", &k.X}, {"y", &k.Y}, {"n", &k.N}, {"e", &k.E}, {"alg", &k.Alg}} {
		v, ok := all[f.name]
		if !ok {
			continue
		}
		if !stringOf(v, f.dst) {
			return JWK{}, fmt.Errorf("%w: member %q is not a string", ErrInvalidJWK, f.name)
		}
	}
	if _, hasY := all["y"]; hasY && k.Kty == ktyOKP {
		return JWK{}, fmt.Errorf("%w: OKP keys have no y", ErrInvalidJWK)
	}
	return k, nil
}

// parseEC checks an EC key: curve P-256, coordinates of 32 bytes, a point on the curve. The
// check goes through crypto/ecdh, which refuses points outside the curve.
func (k *JWK) parseEC() error {
	if k.Crv != crvP256 {
		return fmt.Errorf("%w: EC keys must use P-256", ErrInvalidJWK)
	}
	x, okX := decodeSegment(k.X)
	y, okY := decodeSegment(k.Y)
	if !okX || !okY || len(x) != coordSize || len(y) != coordSize {
		return fmt.Errorf("%w: x and y must be 32 bytes base64url without padding", ErrInvalidJWK)
	}
	point := append(append([]byte{4}, x...), y...)
	if _, err := ecdh.P256().NewPublicKey(point); err != nil {
		return fmt.Errorf("%w: the point is not on P-256", ErrInvalidJWK)
	}
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
	if err != nil {
		return fmt.Errorf("%w: the point is not on P-256", ErrInvalidJWK)
	}
	k.pub = pub
	return nil
}

// parseOKP checks an OKP key: curve Ed25519 and x of 32 bytes; y must be absent.
func (k *JWK) parseOKP() error {
	if k.Crv != crvEd {
		return fmt.Errorf("%w: OKP keys must use Ed25519", ErrInvalidJWK)
	}
	x, ok := decodeSegment(k.X)
	if !ok || len(x) != ed25519.PublicKeySize || k.Y != "" {
		return fmt.Errorf("%w: x must be 32 bytes base64url without padding", ErrInvalidJWK)
	}
	k.pub = ed25519.PublicKey(x)
	return nil
}

// keyAlg is the one algorithm of the key type.
func (k JWK) keyAlg() string {
	if k.Kty == ktyEC {
		return AlgES256
	}
	return AlgEdDSA
}

// Canonical returns the input of the RFC 7638 thumbprint: the required members in lexicographic
// order without whitespace, for example {"crv":"P-256","kty":"EC","x":"...","y":"..."} or
// {"e":"AQAB","kty":"RSA","n":"..."}. The server stores this form as the public JWK
// (device_keys.public_jwk).
func (k JWK) Canonical() []byte {
	if k.Kty == ktyRSA {
		b, _ := json.Marshal(rsaMembers{E: k.E, Kty: k.Kty, N: k.N}) // strings only: never fails
		return b
	}
	b, _ := json.Marshal(jwkMembers{Crv: k.Crv, Kty: k.Kty, X: k.X, Y: k.Y}) // strings only: never fails
	return b
}

// Thumbprint returns the RFC 7638 thumbprint: base64url without padding of SHA-256 over
// Canonical. It is the jkt of DPoP (RFC 9449 section 6.1).
func (k JWK) Thumbprint() string {
	sum := sha256.Sum256(k.Canonical())
	return encodeSegment(sum[:])
}

// MarshalJSON writes the required members and alg when set, never private material.
func (k JWK) MarshalJSON() ([]byte, error) {
	if k.Kty == ktyRSA {
		return json.Marshal(rsaMembers{E: k.E, Kty: k.Kty, N: k.N, Alg: k.Alg})
	}
	return json.Marshal(jwkMembers{Crv: k.Crv, Kty: k.Kty, X: k.X, Y: k.Y, Alg: k.Alg})
}

// PublicKey returns *ecdsa.PublicKey for EC, ed25519.PublicKey for OKP and *rsa.PublicKey for RSA;
// nil for the zero JWK.
func (k JWK) PublicKey() any { return k.pub }

// JWKFromPublicKey builds the JWK of a public key: *ecdsa.PublicKey on P-256,
// ed25519.PublicKey, or *rsa.PublicKey within the bounds of ParseVerificationJWK. The result has
// no alg member. Errors wrap ErrInvalidJWK.
func JWKFromPublicKey(pub any) (JWK, error) {
	switch p := pub.(type) {
	case *rsa.PublicKey:
		if p == nil || p.N == nil {
			return JWK{}, fmt.Errorf("%w: invalid RSA public key", ErrInvalidJWK)
		}
		e := big.NewInt(int64(p.E)).Bytes()
		return ParseVerificationJWK(mustMarshal(rsaMembers{E: encodeSegment(e), Kty: ktyRSA, N: encodeSegment(p.N.Bytes())}))
	case *ecdsa.PublicKey:
		if p == nil || p.Curve != elliptic.P256() {
			return JWK{}, fmt.Errorf("%w: EC keys must use P-256", ErrInvalidJWK)
		}
		b, err := p.Bytes()
		if err != nil || len(b) != 1+2*coordSize {
			return JWK{}, fmt.Errorf("%w: invalid EC public key", ErrInvalidJWK)
		}
		return ParseJWK(mustMarshal(jwkMembers{Crv: crvP256, Kty: ktyEC, X: encodeSegment(b[1 : 1+coordSize]), Y: encodeSegment(b[1+coordSize:])}))
	case ed25519.PublicKey:
		return ParseJWK(mustMarshal(jwkMembers{Crv: crvEd, Kty: ktyOKP, X: encodeSegment(p)}))
	}
	return JWK{}, fmt.Errorf("%w: unsupported public key type", ErrInvalidJWK)
}

// mustMarshal marshals a value of string members, which cannot fail.
func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
