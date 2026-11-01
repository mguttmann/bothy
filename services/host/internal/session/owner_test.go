// SPDX-License-Identifier: Apache-2.0

package session

import "testing"

// TestVerifyOwnerAccountNeedsOwn: a token for the owner account (ADR 0073, D-OA-5) without
// own = true is refused as session_user_mismatch before step 11 binds the account, so a non-owner
// token never binds it; with own = true it passes and binds the owner.
func TestVerifyOwnerAccountNeedsOwn(t *testing.T) {
	f := newFixture()
	c := f.claims()
	c["own"] = false
	ck := f.check(f.signer.Sign(c))
	ck.OwnerAccount = alexOSU
	if d := f.verify(ck); d != DetailSessionUserMismatch {
		t.Fatalf("a non-owner token for the owner account: %q", d)
	}
	if _, bound := f.store.m[alexOSU]; bound || f.store.writes != 0 {
		t.Fatal("a non-owner token bound the owner account")
	}
	c = f.claims()
	c["own"] = true
	ck = f.check(f.signer.Sign(c))
	ck.OwnerAccount = alexOSU
	if d := f.verify(ck); d != "" {
		t.Fatalf("an owner token: %q", d)
	}
	if f.store.m[alexOSU] != alexSub {
		t.Fatal("the owner token did not bind the account")
	}
	// Other session users are unchanged: own = false still opens there.
	c = f.claims()
	c["own"], c["osu"], c["sub"], c["sid"] = false, samOSU, samSub, sessID
	ck = f.check(f.signer.Sign(c))
	ck.OwnerAccount = alexOSU
	if d := f.verify(ck); d != "" {
		t.Fatalf("a separate session user: %q", d)
	}
}
