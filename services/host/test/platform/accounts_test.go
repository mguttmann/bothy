// SPDX-License-Identifier: Apache-2.0

package platform

import "testing"

// TestAccountNamesFitWindows: every account the suite creates has a name Windows accepts and no
// two share one. "net user <name> <password> /add" refuses a name of more than
// winAccountNameMax characters with its usage text; the person of TM-T24 had 21, so the test
// failed on Windows before it checked anything (docs/decisions.d/AUD-misc-misc-01.md). It runs
// untagged, so make check holds the names on every build host.
func TestAccountNamesFitWindows(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range []string{userA, userB, userC, personUser} {
		if len(name) == 0 || len(name) > winAccountNameMax {
			t.Errorf("account %q has %d characters, Windows accepts 1 to %d", name, len(name), winAccountNameMax)
		}
		if seen[name] {
			t.Errorf("account %q is used twice", name)
		}
		seen[name] = true
	}
}
