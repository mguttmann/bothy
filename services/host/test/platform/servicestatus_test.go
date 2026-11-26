// SPDX-License-Identifier: Apache-2.0

package platform

import "testing"

// TestLaunchdExit holds the reading of "launchctl print" to the form launchctl writes on macOS 15
// (the code followed by its name), which the suite read as no number before.
func TestLaunchdExit(t *testing.T) {
	cases := []struct {
		name, out string
		code      int
		stopped   bool
	}{
		{"refused start", "system/bothy-host = {\n\tstate = not running\n\truns = 2\n\tlast exit code = 78: EX_CONFIG\n}\n", 78, true},
		{"code without a name", "\tstate = not running\n\tlast exit code = 1\n", 1, true},
		{"running", "\tstate = running\n\tlast exit code = 78: EX_CONFIG\n", 0, false},
		{"never exited", "\tstate = not running\n\tlast exit code = (never exited)\n", 0, false},
		{"no exit line", "\tstate = not running\n", 0, false},
	}
	for _, c := range cases {
		code, stopped := launchdExit(c.out)
		if code != c.code || stopped != c.stopped {
			t.Errorf("%s: %d %v, want %d %v", c.name, code, stopped, c.code, c.stopped)
		}
	}
	if n := launchdRuns("\tstate = running\n\truns = 3\n"); n != 3 {
		t.Errorf("runs %d, want 3", n)
	}
}
