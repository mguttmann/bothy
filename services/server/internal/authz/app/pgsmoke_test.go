// SPDX-License-Identifier: Apache-2.0

package app_test

import "testing"

// TestPGWorldSeeds: the seed runs through the real rights path: one change for the group and its
// member and one per grant.
func TestPGWorldSeeds(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	if v := w.version(t); v != 7 {
		t.Fatalf("permission_version %d after the seed", v)
	}
}
