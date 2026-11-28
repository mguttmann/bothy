// SPDX-License-Identifier: Apache-2.0

//go:build platform && linux

package platform

import "testing"

// stageImpostor stages the fake helper of TM-T26 on Linux (stageUnixImpostor): systemd owns the
// endpoint through the socket unit that bothy-host install writes, so the suite stops the socket
// unit and the helper service, and starts the socket unit again in the cleanup.
func stageImpostor(t *testing.T, user string) func() {
	t.Helper()
	socketUnit := "bothy-host-helper-" + user + ".socket"
	return stageUnixImpostor(t, user, "/run/termsuite-impostor-"+user,
		func() { sh(t, "/usr/bin/systemctl", "stop", socketUnit, "bothy-host-helper-"+user+".service") },
		func() { _, _ = try("/usr/bin/systemctl", "start", socketUnit) })
}
