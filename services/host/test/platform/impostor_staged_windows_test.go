// SPDX-License-Identifier: Apache-2.0

//go:build platform && windows

package platform

import "testing"

// stageImpostor exists only on Linux and macOS, where the service manager owns the endpoint of
// the helper (impostor_staged_test.go); startImpostor calls it only there.
func stageImpostor(t *testing.T, _ string) func() {
	t.Helper()
	t.Fatal("stageImpostor runs only on Linux and macOS")
	return func() {}
}
