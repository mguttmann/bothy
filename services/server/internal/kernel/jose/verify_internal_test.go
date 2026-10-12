// SPDX-License-Identifier: Apache-2.0

package jose

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestVerify_RFC8037ExampleEdDSA checks the Ed25519 example of RFC 8037 appendix A.4. Its payload
// is plain text, which the profile of ParseCompactJWS refuses (payloads are JSON objects), so the
// test calls the signature check directly.
func TestVerify_RFC8037ExampleEdDSA(t *testing.T) {
	t.Parallel()
	const (
		header  = "eyJhbGciOiJFZERTQSJ9"
		payload = "RXhhbXBsZSBvZiBFZDI1NTE5IHNpZ25pbmc"
		sigText = "hgyY0il_MGCjP0JzlnLWG1PPOt7-09PGcvMg3AIbQR6dWbhijcNR4ki4iylGjg5BhVsPt9g7sVvpAr_MuM0KAg"
		jws     = header + "." + payload + "." + sigText
	)
	k, err := ParseJWK([]byte(`{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(jws, ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	input := []byte(parts[0] + "." + parts[1])
	if err := verifySignature(AlgEdDSA, k, input, sig); err != nil {
		t.Fatalf("verifySignature = %v", err)
	}
	input[0] ^= 1
	if err := verifySignature(AlgEdDSA, k, input, sig); err == nil {
		t.Fatal("changed input verified")
	}
}
