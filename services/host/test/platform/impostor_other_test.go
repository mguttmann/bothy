// SPDX-License-Identifier: Apache-2.0

//go:build platform && !linux

package platform

import "testing"

// stageLinuxImpostor exists only on Linux, where systemd owns the endpoint of the helper
// (impostor_linux_test.go); startImpostor calls it only there.
func stageLinuxImpostor(t *testing.T, _ string) func() {
	t.Helper()
	t.Fatal("stageLinuxImpostor runs only on Linux")
	return func() {}
}
