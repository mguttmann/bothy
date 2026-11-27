// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"strings"
	"testing"
)

// TestComputerSection: the section computer of bothy-host doctor (docs/spec/27-computer-use.md
// 9.3): no HD check, one line per fact, no path and no user name.
func TestComputerSection(t *testing.T) {
	off := ComputerSection{}.Format()
	if off != "computer: off\n" {
		t.Errorf("off: %q", off)
	}
	on := ComputerSection{On: true, Installed: true, TrustedPath: true, Signed: true, Helper: "connected", Enabled: true, Generation: 3,
		Capture: "granted", Input: "denied"}.Format()
	want := "computer: on\n" +
		"computer helper installed: yes\n" +
		"computer helper path trusted: yes\n" +
		"computer build: signed\n" +
		"computer helper: connected\n" +
		"computer switched on: yes (generation 3)\n" +
		"computer permission capture: granted\n" +
		"computer permission input: denied\n"
	if on != want {
		t.Errorf("on:\n%s\nwant:\n%s", on, want)
	}
	dev := ComputerSection{On: true, Helper: "refused", Capture: "unknown", Input: "unknown", TestPeer: true}.Format()
	for _, line := range []string{"computer helper installed: no\n", "computer helper path trusted: no\n", "computer build: development\n",
		"computer helper: refused\n", "computer switched on: no\n", "computer test peer: set (test setting)\n"} {
		if !strings.Contains(dev, line) {
			t.Errorf("development section lacks %q:\n%s", line, dev)
		}
	}
	if strings.Contains(dev, "HD") {
		t.Error("the section is no HD check")
	}
}
