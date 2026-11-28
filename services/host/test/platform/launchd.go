// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"strconv"
	"strings"
)

// launchdExitCode returns the exit code of a job that "launchctl print system/<label>" shows as
// not running, and false while it runs or before it ever exited. launchctl prints a code with a
// name in sysexits.h as "<code>: <name>" (78 as "78: EX_CONFIG"); only the number counts. It lives
// in this untagged file so make check holds it (launchd_test.go).
func launchdExitCode(out string) (int, bool) {
	if !strings.Contains(out, "state = not running") {
		return 0, false
	}
	for _, l := range strings.Split(out, "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(l), "last exit code = ")
		if !ok {
			continue
		}
		num, _, _ := strings.Cut(v, ":")
		n, err := strconv.Atoi(strings.TrimSpace(num))
		return n, err == nil
	}
	return 0, false
}
