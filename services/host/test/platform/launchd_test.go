// SPDX-License-Identifier: Apache-2.0

package platform

import "testing"

// TestLaunchdExitCode: the exit code of a stopped launchd job as "launchctl print" shows it. For a
// code with a name in sysexits.h launchctl prints the name after the number ("last exit code =
// %d: %s" in /bin/launchctl), so the refused start of the host service reads "78: EX_CONFIG";
// the suite read that as 0 and every refusal test failed on macOS (run 46103059338,
// docs/decisions.d/V1-13.md). It runs untagged, so make check holds the parser on every build host.
func TestLaunchdExitCode(t *testing.T) {
	for _, c := range []struct {
		out  string
		code int
		ok   bool
	}{
		{"\tstate = not running\n\truns = 2\n\tlast exit code = 78: EX_CONFIG\n", 78, true},
		{"\tstate = not running\n\tlast exit code = 1\n", 1, true},
		{"\tstate = not running\n\tlast exit code = 0\n", 0, true},
		{"\tstate = not running\n\tlast exit code = (never exited)\n", 0, false},
		{"\tstate = running\n\tpid = 412\n\tlast exit code = 78: EX_CONFIG\n", 0, false},
		{"Could not find service \"bothy-host\" in domain for system\n", 0, false},
	} {
		code, ok := launchdExitCode(c.out)
		if code != c.code || ok != c.ok {
			t.Errorf("launchdExitCode(%q) = %d, %v; want %d, %v", c.out, code, ok, c.code, c.ok)
		}
	}
}
