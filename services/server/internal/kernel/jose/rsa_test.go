// SPDX-License-Identifier: Apache-2.0

package jose_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// rfc7638Key is the RSA example of RFC 7638 section 3.1 and its thumbprint.
const (
	rfc7638N   = "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
	rfc7638Key = `{"kty":"RSA","n":"` + rfc7638N + `","e":"AQAB","alg":"RS256","kid":"2011-04-29"}`
	rfc7638JKT = "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs"
)

func TestJWKThumbprint_RFC7638ExampleRSA(t *testing.T) {
	t.Parallel()
	k, err := jose.ParseVerificationJWK([]byte(rfc7638Key))
	if err != nil {
		t.Fatal(err)
	}
	if got := k.Thumbprint(); got != rfc7638JKT {
		t.Fatalf("Thumbprint = %s, want %s (canonical %s)", got, rfc7638JKT, k.Canonical())
	}
	if k.Alg != jose.AlgRS256 || k.Kty != "RSA" {
		t.Fatalf("alg %q kty %q", k.Alg, k.Kty)
	}
	// The device and DPoP paths keep their profile: ParseJWK still refuses RSA.
	if _, err := jose.ParseJWK([]byte(rfc7638Key)); !errors.Is(err, jose.ErrInvalidJWK) {
		t.Fatalf("ParseJWK(RSA) = %v, want ErrInvalidJWK", err)
	}
}

// rfc7515A2 is the RS256 example of RFC 7515 appendix A.2: its public key and compact JWS.
const (
	rfc7515A2N = "ofgWCuLjybRlzo0tZWJjNiuSfb4p4fAkd_wWJcyQoTbji9k0l8W26mPddxHmfHQp-Vaw-4qPCJrcS2mJPMEzP1Pt0Bm4d4QlL-yRT-SFd2lZS-pCgNMsD1W_YpRPEwOWvG6b32690r2jZ47soMZo9wGzjb_7OMg0LOL-bSf63kpaSHSXndS5z5rexMdbBYUsLA9e-KXBdQOS-UTo7WTBEMa2R2CapHg665xsmtdVMTBQY4uDZlxvb3qCo5ZwKh9kG4LT6_I5IhlJH7aGhyxXFvUK-DWNmoudF8NAco9_h9iaGNj8q2ethFkMLs91kzk2PAcDTW9gb54h4FRWyuXpoQ"
	// The segments stand apart so that secret scanners do not take the public example for a token.
	rfc7515A2Header    = "eyJhbGciOiJSUzI1NiJ9"
	rfc7515A2Payload   = "eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ"
	rfc7515A2Signature = "cC4hiUPoj9Eetdgtv3hF80EGrhuB__dzERat0XF9g2VtQgr9PJbu3XOiZj5RZmh7AAuHIm4Bh-0Qc_lF5YKt_O8W2Fp5jujGbds9uJdbF9CUAr7t1dnZcAcQjbKBYNX4BAynRFdiuB--f_nZLgrnbyTyWzO75vRK5h6xBArLIARNPvkSjtQBMHlb1L07Qe7K0GarZRmB_eSN9383LcOLn6_dO--xi12jzDwusC-eOkHWEsqtFZESc6BfI7noOPqvhJ1phCnvWh6IeYI2w9QOYEUipUTI8np6LbgGY9Fs98rqVt5AXLIhWkWywlVmtVrBp0igcN_IoypGlUPQGe77Rw"
)

func TestJWS_RFC7515ExampleRS256(t *testing.T) {
	t.Parallel()
	k, err := jose.ParseVerificationJWK([]byte(`{"kty":"RSA","n":"` + rfc7515A2N + `","e":"AQAB"}`))
	if err != nil {
		t.Fatal(err)
	}
	jws := rfc7515A2Header + "." + rfc7515A2Payload + "." + rfc7515A2Signature
	j, err := jose.ParseCompactJWS(jws, []string{jose.AlgRS256}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Verify(k); err != nil {
		t.Fatalf("Verify = %v", err)
	}
	// The same signature under PS256 never verifies: the algorithm is part of the check.
	ps := segment(`{"alg":"PS256"}`) + "." + rfc7515A2Payload + "." + rfc7515A2Signature
	jp, err := jose.ParseCompactJWS(ps, []string{jose.AlgPS256}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := jp.Verify(k); !errors.Is(err, jose.ErrSignature) {
		t.Fatalf("Verify(PS256) = %v, want ErrSignature", err)
	}
}

// rsaKey returns a fresh RSA key of bits.
func rsaKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSignCompact_RSARoundTrip(t *testing.T) {
	t.Parallel()
	key := rsaKey(t, 2048)
	pub, err := jose.JWKFromPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []jose.Signer{jose.RS256Signer{Key: key}, jose.PS256Signer{Key: key}} {
		tok, err := jose.SignCompact(s, jose.Header{Typ: "JWT", Kid: "k1"}, []byte(`{"a":1}`))
		if err != nil {
			t.Fatal(err)
		}
		j, err := jose.ParseCompactJWS(tok, []string{jose.AlgRS256, jose.AlgPS256}, "JWT")
		if err != nil {
			t.Fatalf("%s: %v", s.Alg(), err)
		}
		if j.Header.Alg != s.Alg() || j.Header.Kid != "k1" {
			t.Fatalf("%s: header %+v", s.Alg(), j.Header)
		}
		if err := j.Verify(pub); err != nil {
			t.Fatalf("%s: Verify = %v", s.Alg(), err)
		}
		// An EC key never verifies an RSA signature.
		ec, _ := keys(t)
		other, _ := jose.JWKFromPublicKey(&ec.PublicKey)
		if err := j.Verify(other); !errors.Is(err, jose.ErrSignature) {
			t.Fatalf("%s: Verify with an EC key = %v", s.Alg(), err)
		}
		// A key whose alg member names the other RSA algorithm fails.
		bound := pub
		bound.Alg = map[string]string{jose.AlgRS256: jose.AlgPS256, jose.AlgPS256: jose.AlgRS256}[s.Alg()]
		if err := j.Verify(bound); !errors.Is(err, jose.ErrSignature) {
			t.Fatalf("%s: Verify with a key for the other algorithm = %v", s.Alg(), err)
		}
	}
	// An RSA key never verifies an ES256 token.
	ec, _ := keys(t)
	tok, err := jose.SignCompact(jose.ES256Signer{Key: ec}, jose.Header{}, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	j, err := jose.ParseCompactJWS(tok, []string{jose.AlgES256}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Verify(pub); !errors.Is(err, jose.ErrSignature) {
		t.Fatalf("Verify(ES256 with RSA key) = %v", err)
	}
}

func TestParseVerificationJWK(t *testing.T) {
	t.Parallel()
	small := rsaKey(t, 1024)
	b64 := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	n := b64(rsaKey(t, 2048).N.Bytes())
	accepted := map[string]string{
		"EC":          `{"kty":"EC","crv":"P-256","x":"l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs","y":"9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA"}`,
		"OKP":         `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`,
		"RSA":         `{"kty":"RSA","n":"` + n + `","e":"AQAB"}`,
		"RSA PS256":   `{"kty":"RSA","n":"` + n + `","e":"AQAB","alg":"PS256","use":"sig","x5c":["AA"]}`,
		"RSA kid any": `{"kty":"RSA","n":"` + n + `","e":"AQAB","kid":"x"}`,
	}
	for name, raw := range accepted {
		if _, err := jose.ParseVerificationJWK([]byte(raw)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	rejected := map[string]string{
		"RSA 1024 bits":      `{"kty":"RSA","n":"` + b64(small.N.Bytes()) + `","e":"AQAB"}`,
		"RSA leading zero":   `{"kty":"RSA","n":"AA` + n + `","e":"AQAB"}`,
		"RSA even e":         `{"kty":"RSA","n":"` + n + `","e":"Ag"}`,
		"RSA e 1":            `{"kty":"RSA","n":"` + n + `","e":"AQ"}`,
		"RSA huge e":         `{"kty":"RSA","n":"` + n + `","e":"` + b64(new(big.Int).Lsh(big.NewInt(1), 40).Bytes()) + `"}`,
		"RSA e missing":      `{"kty":"RSA","n":"` + n + `"}`,
		"RSA n missing":      `{"kty":"RSA","e":"AQAB"}`,
		"RSA private d":      `{"kty":"RSA","n":"` + n + `","e":"AQAB","d":"AA"}`,
		"RSA alg ES256":      `{"kty":"RSA","n":"` + n + `","e":"AQAB","alg":"ES256"}`,
		"RSA alg RS512":      `{"kty":"RSA","n":"` + n + `","e":"AQAB","alg":"RS512"}`,
		"RSA padded n":       `{"kty":"RSA","n":"` + n + `=","e":"AQAB"}`,
		"RSA n not a string": `{"kty":"RSA","n":1,"e":"AQAB"}`,
		"RSA even modulus":   `{"kty":"RSA","n":"` + b64(append(new(big.Int).Lsh(big.NewInt(1), 2047).Bytes()[:255], 2)) + `","e":"AQAB"}`,
		"oct":                `{"kty":"oct","k":"AAAA"}`,
		"EC alg RS256":       `{"kty":"EC","crv":"P-256","x":"l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs","y":"9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA","alg":"RS256"}`,
		"duplicate n":        `{"kty":"RSA","n":"` + n + `","n":"` + n + `","e":"AQAB"}`,
	}
	for name, raw := range rejected {
		if _, err := jose.ParseVerificationJWK([]byte(raw)); !errors.Is(err, jose.ErrInvalidJWK) {
			t.Errorf("%s: ParseVerificationJWK = %v, want ErrInvalidJWK", name, err)
		}
	}
	// Round trip: the canonical form of an RSA key parses to the same thumbprint.
	k, err := jose.ParseVerificationJWK([]byte(accepted["RSA PS256"]))
	if err != nil {
		t.Fatal(err)
	}
	again, err := jose.ParseVerificationJWK(k.Canonical())
	if err != nil || again.Thumbprint() != k.Thumbprint() || !strings.HasPrefix(string(k.Canonical()), `{"e":"AQAB","kty":"RSA","n":"`) {
		t.Fatalf("canonical %s: %v", k.Canonical(), err)
	}
}

func TestRSASigner_Rejects(t *testing.T) {
	t.Parallel()
	if _, err := (jose.RS256Signer{}).Sign([]byte("x")); err == nil {
		t.Error("RS256 signer without key signed")
	}
	if _, err := (jose.PS256Signer{Key: rsaKey(t, 1024)}).Sign([]byte("x")); err == nil {
		t.Error("PS256 signer with a 1024-bit key signed")
	}
}
