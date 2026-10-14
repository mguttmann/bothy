// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"slices"
	"testing"
)

// TestStartSequenceModuleSteps: every role starts the module steps 6a to 6d in the order Serve
// documents (audit, sync, authz, identity) and before the routes of step 7, so the device
// handlers of identity (M1-33) exist when the routes are mounted (docs/DECISIONS.md D-M2-01-11).
func TestStartSequenceModuleSteps(t *testing.T) {
	t.Parallel()
	want := []string{"audit", "sync", "authz", "identity", "routes"}
	for _, role := range []Role{RoleAPI, RoleWorker, RoleAll} {
		names := componentNames(role)
		last := -1
		for _, name := range want {
			i := slices.Index(names, name)
			if i < 0 || i <= last {
				t.Fatalf("%s: start sequence %v, want %v in this order", role, names, want)
			}
			last = i
		}
		if got := slices.Index(names, "identity"); got != slices.Index(names, "authz")+1 {
			t.Errorf("%s: identity is not right after authz: %v", role, names)
		}
	}
}
