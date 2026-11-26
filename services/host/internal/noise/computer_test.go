// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"errors"
	"testing"
)

// TestComputerPrologue: a computer session binds the prologue "bothy-computer/1" || session ID ||
// host ID (docs/spec/27-computer-use.md 5.3 step 0); an initiator with the terminal prologue fails
// against a computer responder and the reverse, so a terminal stream never becomes a computer
// stream.
func TestComputerPrologue(t *testing.T) {
	if len(ComputerPrologueLabel) != len(PrologueLabel) {
		t.Fatalf("label length %d, want %d", len(ComputerPrologueLabel), len(PrologueLabel))
	}
	sid, hid := mustID(t, goSessionID), mustID(t, goHostID)
	p := PrologueWith(ComputerPrologueLabel, sid, hid)
	if string(p[:len(ComputerPrologueLabel)]) != "bothy-computer/1" || len(p) != 48 {
		t.Fatalf("prologue %q", p)
	}
	static := testKey(t, "test/static")
	run := func(initLabel, respLabel string) error {
		init, err := NewInitiator(InitiatorConfig{HostStatic: static.Public, SessionID: sid, HostID: hid, Ephemeral: testKey(t, "test/eph-i"), Label: initLabel})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := NewResponder(ResponderConfig{Static: static, SessionID: sid, HostID: hid, Label: respLabel})
		if err != nil {
			t.Fatal(err)
		}
		m1, err := init.WriteMessage1([]byte("token"))
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = resp.ReadMessage1(m1)
		return err
	}
	if err := run(ComputerPrologueLabel, ComputerPrologueLabel); err != nil {
		t.Errorf("computer to computer: %v", err)
	}
	if err := run("", ComputerPrologueLabel); !errors.Is(err, ErrHandshake) {
		t.Errorf("terminal initiator, computer responder: %v", err)
	}
	if err := run(ComputerPrologueLabel, ""); !errors.Is(err, ErrHandshake) {
		t.Errorf("computer initiator, terminal responder: %v", err)
	}
}
