// SPDX-License-Identifier: Apache-2.0

package session

import (
	"testing"
	"time"
)

// TestComputerTokenOnTerminalConnection: an attach token of a computer session
// (typ computer-session+jwt, docs/spec/27-computer-use.md 5.2) on a terminal data connection is
// token_invalid, with its own claims and with the claims of a terminal token (CU-T08).
func TestComputerTokenOnTerminalConnection(t *testing.T) {
	f := newFixture()
	hdr := map[string]any{"alg": "EdDSA", "kid": "terminal-key-1", "typ": "computer-session+jwt"}
	if d := f.verify(f.check(f.signer.SignRaw(hdr, f.claims()))); d != DetailTokenInvalid {
		t.Errorf("terminal claims with the computer typ: %q", d)
	}
	attach := map[string]any{"iss": "0192f4a0-7b1c-7b22-9d4f-0000000000e1", "aud": hostID, "sub": alexSub, "sid": sessID,
		"jti": "0192f4a0-7b1c-7b22-9d4f-0000000000f1", "iat": now.Unix(), "exp": now.Add(30 * time.Second).Unix(),
		"run": "0192f4a0-7b1c-7b22-9d4f-0000000000b1", "agt": "0192f4a0-7b1c-7b22-9d4f-0000000000c1", "cek": cek, "own": true, "gen": 1}
	if d := f.verify(f.check(f.signer.SignRaw(hdr, attach))); d != DetailTokenInvalid {
		t.Errorf("attach token: %q", d)
	}
	if d := f.verify(f.check(f.signer.Sign(attach))); d != DetailTokenInvalid {
		t.Errorf("attach claims with the terminal typ: %q", d)
	}
}
