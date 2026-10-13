// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

// TestVerifyMentioned covers the branches the real catalogue cannot reach: the generator only lets
// known actions into MentionedActions and M1-02 forbids system actions in ops and channel ops.
func TestVerifyMentioned(t *testing.T) {
	got := verifyMentioned(Violation{Subject: "x"}, "op \"x\"", []string{"no.such.action", "instance.bootstrap", "message.read"}, true)
	if len(got) != 2 || got[0].Kind != ViolationUnknownMentionedAction || got[1].Kind != ViolationSystemMentionedAction ||
		got[1].Action != "instance.bootstrap" {
		t.Fatalf("got %+v", got)
	}
	// Event types may mention a system action (their receive action is checked, not run).
	if got := verifyMentioned(Violation{}, "event", []string{"instance.bootstrap"}, false); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
