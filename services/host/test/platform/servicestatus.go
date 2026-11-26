// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"strconv"
	"strings"
)

// launchdExit reads the output of "launchctl print system/<label>" and returns the last exit
// code of a job that is not running. launchctl writes the code with its name ("last exit code =
// 78: EX_CONFIG"), so the number ends at the colon; a job that never exited ("(never exited)")
// or still runs has none. It lives in this untagged file so make check holds it on every build
// host (servicestatus_test.go): the suite read "78:" as no number and took every refused start
// on macOS for another code (docs/decisions.d/AUD-PKG-R2-06-host-platform-ci.md).
func launchdExit(out string) (int, bool) {
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
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// launchdRuns reads the number of starts ("runs = N") from the output of launchctl print.
func launchdRuns(out string) int {
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "runs = "); ok {
			n, _ := strconv.Atoi(strings.TrimSpace(v))
			return n
		}
	}
	return 0
}
