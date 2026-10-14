// SPDX-License-Identifier: Apache-2.0

package jose_test

import (
	"bytes"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// FuzzParseJWK: no input panics, and an accepted key parses again from its own JSON to the same
// thumbprint. The seed corpus is testdata/fuzz/FuzzParseJWK (run in every go test).
func FuzzParseJWK(f *testing.F) {
	f.Add([]byte(`{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`))
	f.Add([]byte(`{"kty":"EC","crv":"P-256","x":"l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs","y":"9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA"}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		k, err := jose.ParseJWK(raw)
		if err != nil {
			return
		}
		again, err := jose.ParseJWK(k.Canonical())
		if err != nil {
			t.Fatalf("canonical form of an accepted key is refused: %v", err)
		}
		if again.Thumbprint() != k.Thumbprint() || !bytes.Equal(again.Canonical(), k.Canonical()) {
			t.Fatal("thumbprint changed on a second parse")
		}
	})
}

// FuzzParseCompactJWS: no input panics; an accepted token parses again to the same header and
// payload, and verifying it with the key in its header gives the same result twice. The seed
// corpus testdata/fuzz/FuzzParseCompactJWS holds valid ES256 and EdDSA DPoP proofs and the
// malformed cases of TestJWS_Rejects.
func FuzzParseCompactJWS(f *testing.F) {
	f.Add(rfc7515A3JWS)
	f.Add("e30.e30.AA")
	allowed := []string{jose.AlgES256, jose.AlgEdDSA}
	f.Fuzz(func(t *testing.T, s string) {
		j, err := jose.ParseCompactJWS(s, allowed, "")
		if err != nil {
			return
		}
		again, err := jose.ParseCompactJWS(s, allowed, "")
		if err != nil || again.Header.Alg != j.Header.Alg || !bytes.Equal(again.Payload, j.Payload) {
			t.Fatalf("second parse differs: %v", err)
		}
		k, err := jose.ParseJWK(j.Header.JWK)
		if err != nil {
			return
		}
		first, second := j.Verify(k), again.Verify(k)
		if (first == nil) != (second == nil) {
			t.Fatal("verification is not deterministic")
		}
	})
}
