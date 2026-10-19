// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"crypto/ed25519"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// FuzzCheckHostProof (M2-43, PLAN section 5 "Security"): arbitrary proofs give a verdict, never a
// panic, and never ProofValid unless the signature verifies with the stored key.
func FuzzCheckHostProof(f *testing.F) {
	k := newProofKey(f, "fuzz/host")
	host := ids.MustParse("0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f")
	f.Add(rawProof(k.priv, header(k.jkt), claimsAt(host, proofNow)))
	f.Add(rawProof(k.priv, `{"alg":"none"}`, `{}`))
	f.Add("a.b.c")
	f.Add("")
	stored := domain.HostLoginKey{Found: true, State: domain.StateActive, JKT: k.jkt, JWK: k.jwk}
	pub := k.priv.Public().(ed25519.PublicKey)
	f.Fuzz(func(t *testing.T, proof string) {
		got := domain.CheckHostProof(domain.ProofCheck{Proof: proof, HostID: host, Challenge: challenge, HelloAt: helloAt, Now: proofNow,
			Leeway: leeway}, stored)
		if got != domain.ProofValid {
			return
		}
		// A valid verdict needs a signature of the stored key over the first two parts.
		parts := splitProof(proof)
		if len(parts) != 3 {
			t.Fatalf("valid verdict for a proof without three parts")
		}
		sig, err := b64Decode(parts[2])
		if err != nil || !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
			t.Fatalf("valid verdict without a verifying signature")
		}
	})
}
