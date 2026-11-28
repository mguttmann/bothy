// SPDX-License-Identifier: Apache-2.0

//go:build platform && darwin

package platform

import "testing"

// stageImpostor stages the fake helper of TM-T26 on macOS (stageUnixImpostor): launchd owns the
// endpoint through the Sockets entry of the helper job that bothy-host install writes, so the
// suite boots the job out and bootstraps it again in the cleanup. The folder of the impostor lies
// next to the program folder, on the volume of the endpoint.
func stageImpostor(t *testing.T, user string) func() {
	t.Helper()
	label := "bothy-host.helper." + user
	return stageUnixImpostor(t, user, "/Library/Application Support/termsuite-impostor-"+user,
		func() { sh(t, "/bin/launchctl", "bootout", "system/"+label) },
		func() { _, _ = try("/bin/launchctl", "bootstrap", "system", "/Library/LaunchDaemons/"+label+".plist") })
}
