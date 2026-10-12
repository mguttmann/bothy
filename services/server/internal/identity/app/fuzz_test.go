// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// fuzzTime and fuzzPepper fix the clock and the nonce key of FuzzVerifyDPoPProof, so the valid
// seeds of testdata/fuzz/FuzzVerifyDPoPProof stay valid.
var fuzzTime = time.Date(2026, 10, 12, 9, 2, 30, 0, time.UTC)

const fuzzPepper = "the fixed pepper of the dpop fuzz"

func fuzzVerifier(t testing.TB) *app.DPoPVerifier {
	clk := clock.NewFake(fuzzTime)
	v, err := app.NewDPoPVerifier(testDPoPConfig, []byte(fuzzPepper), replay.New(clk), clk, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// FuzzVerifyDPoPProof: no proof panics the verifier; an accepted proof is accepted with the same
// result by a fresh verifier with the same pepper, and refused by the same verifier a second time
// (replay). Seeds: valid ES256 and EdDSA proofs and the malformed cases of TestDPoP_IDT14.
func FuzzVerifyDPoPProof(f *testing.F) {
	v := fuzzVerifier(f)
	for _, alg := range []string{jose.AlgES256, jose.AlgEdDSA} {
		key, _ := identitytest.NewDeviceKey(alg)
		f.Add(identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: "GET", HTU: testURL, IAT: fuzzTime, Nonce: v.CurrentNonce()}))
		f.Add(identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: "GET", HTU: testURL, IAT: fuzzTime}))
		f.Add(identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: "GET", HTU: testURL, IAT: fuzzTime, Nonce: v.CurrentNonce(), Typ: "JWT"}))
	}
	f.Add("a.b.c")
	f.Fuzz(func(t *testing.T, proof string) {
		v := fuzzVerifier(t)
		req := identity.ProofRequest{Proofs: []string{proof}, Method: "GET", URL: testURL}
		got, err := v.Verify(context.Background(), req)
		if err != nil {
			return
		}
		again, err := fuzzVerifier(t).Verify(context.Background(), req)
		if err != nil || !reflect.DeepEqual(again, got) {
			t.Fatalf("a fresh verifier gives %+v, %v; first %+v", again, err, got)
		}
		if _, err := v.Verify(context.Background(), req); codeOf(err) != identity.DPoPInvalidProof {
			t.Fatalf("second use with the same verifier: %v", err)
		}
	})
}
