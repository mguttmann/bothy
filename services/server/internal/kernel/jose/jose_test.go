// SPDX-License-Identifier: Apache-2.0

package jose_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// The thumbprint vectors are quoted from the RFCs named in the test names.
func TestJWKThumbprint_RFC9449Example(t *testing.T) {
	t.Parallel()
	k, err := jose.ParseJWK([]byte(`{"kty":"EC","crv":"P-256","x":"l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs","y":"9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := k.Thumbprint(), "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"; got != want {
		t.Fatalf("Thumbprint = %s, want %s (canonical %s)", got, want, k.Canonical())
	}
}

func TestJWKThumbprint_RFC8037Example(t *testing.T) {
	t.Parallel()
	k, err := jose.ParseJWK([]byte(`{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := k.Thumbprint(), "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k"; got != want {
		t.Fatalf("Thumbprint = %s, want %s (canonical %s)", got, want, k.Canonical())
	}
}

func TestJWKThumbprint_IgnoresUnknownMembers(t *testing.T) {
	t.Parallel()
	k, err := jose.ParseJWK([]byte(`{"use":"sig","kid":"a","kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo","alg":"EdDSA"}`))
	if err != nil {
		t.Fatal(err)
	}
	if k.Thumbprint() != "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k" || k.Alg != "EdDSA" {
		t.Fatalf("thumbprint %s alg %q", k.Thumbprint(), k.Alg)
	}
}

// rfc7515A3 is the ES256 example of RFC 7515 appendix A.3: its public key and compact JWS.
const (
	rfc7515A3Key = `{"kty":"EC","crv":"P-256","x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}`
	// The segments stand apart so that secret scanners do not take the public example for a token.
	rfc7515A3Header    = "eyJhbGciOiJFUzI1NiJ9"
	rfc7515A3Payload   = "eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ"
	rfc7515A3Signature = "DtEhU3ljbEg8L38VWAfUAqOyKAM6-Xx-F4GawxaepmXFCgfTjDxw5djxLa8ISlSApmWQxfKTUJqPP3-Kg6NU1Q"
	rfc7515A3JWS       = rfc7515A3Header + "." + rfc7515A3Payload + "." + rfc7515A3Signature
)

func TestJWS_RFC7515ExampleES256(t *testing.T) {
	t.Parallel()
	k, err := jose.ParseJWK([]byte(rfc7515A3Key))
	if err != nil {
		t.Fatal(err)
	}
	j, err := jose.ParseCompactJWS(rfc7515A3JWS, []string{"ES256"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Verify(k); err != nil {
		t.Fatalf("Verify = %v", err)
	}
	if j.Header.Alg != "ES256" || !strings.Contains(string(j.Payload), `"iss":"joe"`) {
		t.Fatalf("header %+v payload %s", j.Header, j.Payload)
	}
}

func TestJWK_Rejects(t *testing.T) {
	t.Parallel()
	const x = "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs"
	const y = "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA"
	short := base64.RawURLEncoding.EncodeToString(make([]byte, 31))
	long := base64.RawURLEncoding.EncodeToString(make([]byte, 33))
	// A point that is not on P-256: x of the example with y = 1.
	offCurve := base64.RawURLEncoding.EncodeToString(append(make([]byte, 31), 1))
	cases := map[string]string{
		"private member d":    `{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `","d":"AA"}`,
		"private member p":    `{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `","p":"AA"}`,
		"symmetric member k":  `{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `","k":"AA"}`,
		"kty RSA":             `{"kty":"RSA","n":"AQAB","e":"AQAB"}`,
		"kty oct":             `{"kty":"oct","k":"AAAA"}`,
		"wrong curve":         `{"kty":"EC","crv":"P-384","x":"` + x + `","y":"` + y + `"}`,
		"OKP wrong curve":     `{"kty":"OKP","crv":"X25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`,
		"x of 31 bytes":       `{"kty":"EC","crv":"P-256","x":"` + short + `","y":"` + y + `"}`,
		"x of 33 bytes":       `{"kty":"EC","crv":"P-256","x":"` + long + `","y":"` + y + `"}`,
		"OKP x of 31 bytes":   `{"kty":"OKP","crv":"Ed25519","x":"` + short + `"}`,
		"OKP x of 33 bytes":   `{"kty":"OKP","crv":"Ed25519","x":"` + long + `"}`,
		"point not on curve":  `{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + offCurve + `"}`,
		"padded base64":       `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo="}`,
		"standard base64":     `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS/7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`,
		"newline in base64":   `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7\nhcvPapiMlrwIaaPcHURo"}`,
		"duplicate keys":      `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`,
		"alg member mismatch": `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo","alg":"ES256"}`,
		"EC alg EdDSA":        `{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `","alg":"EdDSA"}`,
		"missing y":           `{"kty":"EC","crv":"P-256","x":"` + x + `"}`,
		"x not a string":      `{"kty":"OKP","crv":"Ed25519","x":1}`,
		"x null":              `{"kty":"OKP","crv":"Ed25519","x":null}`,
		"alg null":            `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo","alg":null}`,
		"OKP with empty y":    `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo","y":""}`,
		"not an object":       `["kty"]`,
		"trailing data":       `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"} {}`,
		"invalid UTF-8":       "{\"kty\":\"OKP\",\"crv\":\"Ed25519\",\"x\":\"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo\",\"n\":\"\xff\"}",
		"empty":               ``,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := jose.ParseJWK([]byte(raw)); !errors.Is(err, jose.ErrInvalidJWK) {
				t.Fatalf("ParseJWK = %v, want ErrInvalidJWK", err)
			}
		})
	}
}

// keys returns a fresh ES256 and EdDSA signer with their public JWKs.
func keys(t *testing.T) (*ecdsa.PrivateKey, ed25519.PrivateKey) {
	t.Helper()
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, ed, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return ec, ed
}

func TestSignCompact_RoundTrip(t *testing.T) {
	t.Parallel()
	ec, ed := keys(t)
	for _, s := range []jose.Signer{jose.ES256Signer{Key: ec}, jose.EdDSASigner{Key: ed}} {
		pub, err := jose.JWKFromPublicKey(publicOf(s))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(pub)
		if err != nil {
			t.Fatal(err)
		}
		tok, err := jose.SignCompact(s, jose.Header{Typ: "dpop+jwt", JWK: raw}, []byte(`{"a":1}`))
		if err != nil {
			t.Fatal(err)
		}
		j, err := jose.ParseCompactJWS(tok, []string{"ES256", "EdDSA"}, "DPoP+JWT")
		if err != nil {
			t.Fatalf("%s: %v", s.Alg(), err)
		}
		if j.Header.Alg != s.Alg() || string(j.Payload) != `{"a":1}` {
			t.Fatalf("%s: header %+v payload %s", s.Alg(), j.Header, j.Payload)
		}
		hk, err := jose.ParseJWK(j.Header.JWK)
		if err != nil {
			t.Fatal(err)
		}
		if err := j.Verify(hk); err != nil {
			t.Fatalf("%s: Verify = %v", s.Alg(), err)
		}
		if hk.Thumbprint() != pub.Thumbprint() {
			t.Fatal("thumbprint changed through the header")
		}
	}
}

func TestSignCompact_Rejects(t *testing.T) {
	t.Parallel()
	ec, _ := keys(t)
	s := jose.ES256Signer{Key: ec}
	if _, err := jose.SignCompact(s, jose.Header{Alg: "EdDSA"}, []byte(`{}`)); err == nil {
		t.Error("header alg differing from the signer was accepted")
	}
	if _, err := jose.SignCompact(s, jose.Header{}, []byte(`not json`)); err == nil {
		t.Error("payload that is no JSON object was accepted")
	}
}

func publicOf(s jose.Signer) any {
	switch k := s.(type) {
	case jose.ES256Signer:
		return &k.Key.PublicKey
	case jose.EdDSASigner:
		return k.Key.Public()
	}
	return nil
}

// segment encodes s as a base64url segment without padding.
func segment(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func TestJWS_Rejects(t *testing.T) {
	t.Parallel()
	ec, _ := keys(t)
	s := jose.ES256Signer{Key: ec}
	good, err := jose.SignCompact(s, jose.Header{Typ: "JWT"}, []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, ".")
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	der, err := asn1.Marshal(struct{ R, S *big.Int }{new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])})
	if err != nil {
		t.Fatal(err)
	}
	flipped := append([]byte(nil), sig...)
	flipped[10] ^= 0x01
	payload := segment(`{"a":1}`)
	head := func(h string) string { return segment(h) + "." + payload + "." + parts[2] }
	allowed := []string{"ES256", "EdDSA"}
	parseCases := map[string]string{
		"two segments":            parts[0] + "." + parts[1],
		"four segments":           good + "." + parts[2],
		"padding":                 parts[0] + "=." + parts[1] + "." + parts[2],
		"plus sign":               strings.Replace(good, "-", "+", 1) + "+",
		"whitespace":              parts[0] + " ." + parts[1] + "." + parts[2],
		"alg none":                head(`{"alg":"none"}`),
		"alg HS256":               head(`{"alg":"HS256"}`),
		"alg RS256":               head(`{"alg":"RS256"}`),
		"alg not a string":        head(`{"alg":1}`),
		"alg missing":             head(`{"typ":"JWT"}`),
		"typ null":                head(`{"alg":"ES256","typ":null}`),
		"crit":                    head(`{"alg":"ES256","crit":["exp"],"exp":1}`),
		"jku":                     head(`{"alg":"ES256","jku":"https://example.com/k"}`),
		"x5u":                     head(`{"alg":"ES256","x5u":"https://example.com/c"}`),
		"x5c":                     head(`{"alg":"ES256","x5c":["AA"]}`),
		"zip":                     head(`{"alg":"ES256","zip":"DEF"}`),
		"duplicate header key":    head(`{"alg":"ES256","alg":"ES256"}`),
		"header not an object":    head(`["ES256"]`),
		"payload not an object":   parts[0] + "." + segment(`"text"`) + "." + parts[2],
		"payload duplicate key":   parts[0] + "." + segment(`{"a":1,"a":2}`) + "." + parts[2],
		"payload too deep":        parts[0] + "." + segment(`{"a":`+strings.Repeat("[", 40)+strings.Repeat("]", 40)+`}`) + "." + parts[2],
		"payload invalid UTF-8":   parts[0] + "." + segment("{\"a\":\"\xff\"}") + "." + parts[2],
		"empty signature segment": parts[0] + "." + parts[1] + ".",
	}
	for name, tok := range parseCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := jose.ParseCompactJWS(tok, allowed, ""); !errors.Is(err, jose.ErrInvalidJWS) {
				t.Fatalf("ParseCompactJWS = %v, want ErrInvalidJWS", err)
			}
		})
	}
	t.Run("alg outside the allowlist", func(t *testing.T) {
		t.Parallel()
		if _, err := jose.ParseCompactJWS(good, []string{"EdDSA"}, ""); !errors.Is(err, jose.ErrInvalidJWS) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("none in the allowlist stays rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := jose.ParseCompactJWS(head(`{"alg":"none"}`), []string{"none", "HS256"}, ""); !errors.Is(err, jose.ErrInvalidJWS) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("wrong typ", func(t *testing.T) {
		t.Parallel()
		if _, err := jose.ParseCompactJWS(good, allowed, "dpop+jwt"); !errors.Is(err, jose.ErrInvalidJWS) {
			t.Fatalf("got %v", err)
		}
	})
	pub, err := jose.JWKFromPublicKey(&ec.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	verifyCases := map[string][]byte{"DER-encoded ES256 signature": der, "flipped signature bit": flipped, "truncated signature": sig[:63]}
	for name, bad := range verifyCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tok := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(bad)
			j, err := jose.ParseCompactJWS(tok, allowed, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := j.Verify(pub); !errors.Is(err, jose.ErrSignature) {
				t.Fatalf("Verify = %v, want ErrSignature", err)
			}
		})
	}
	t.Run("key of another algorithm", func(t *testing.T) {
		t.Parallel()
		_, ed := keys(t)
		other, err := jose.JWKFromPublicKey(ed.Public())
		if err != nil {
			t.Fatal(err)
		}
		j, err := jose.ParseCompactJWS(good, allowed, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := j.Verify(other); !errors.Is(err, jose.ErrSignature) {
			t.Fatalf("Verify = %v, want ErrSignature", err)
		}
	})
}

// TestSignerFunc: a function signer signs like the key it wraps; without a function it fails.
func TestSignerFunc(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s := jose.SignerFunc{Algorithm: jose.AlgEdDSA, Fn: func(in []byte) ([]byte, error) { return ed25519.Sign(priv, in), nil }}
	tok, err := jose.SignCompact(s, jose.Header{Typ: "at+jwt"}, []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	j, err := jose.ParseCompactJWS(tok, []string{jose.AlgEdDSA}, "at+jwt")
	if err != nil {
		t.Fatal(err)
	}
	k, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Verify(k); err != nil {
		t.Fatal(err)
	}
	if _, err := (jose.SignerFunc{Algorithm: jose.AlgEdDSA}).Sign([]byte("x")); err == nil {
		t.Fatal("a signer without a function signed")
	}
}
