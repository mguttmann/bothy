// SPDX-License-Identifier: Apache-2.0

package domain

// RunActive reports whether a run in state is active: queued, running or waiting_input (03 runs,
// R-DM-14). The consumer push uses it on the last run.state_changed before an agent message to
// decide whether that message was written during the run (13 2.1, docs/DECISIONS.md D-M3-19-4).
// Any other value, the end states completed, failed and cancelled included, is not active.
func RunActive(state string) bool {
	switch state {
	case "queued", "running", "waiting_input":
		return true
	}
	return false
}
