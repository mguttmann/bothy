// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"slices"
	"testing"
)

// TestStartSequenceModuleSteps: every role starts the module steps 6a to 6e in the order Serve
// documents (audit, sync, authz, settings, identity; identity reads locale.default through the
// settings, M1-46), then the signing keys (M1-32) and the routes of step
// 7, so the device handlers of identity (M1-33) and the key handlers exist when the routes are
// mounted (docs/DECISIONS.md D-M2-01-11), and the keys before the jobs that rotate them. The session
// part of identity follows the signing keys, which sign its access tokens, and precedes the routes,
// whose chain authenticates with it (M1-34).
func TestStartSequenceModuleSteps(t *testing.T) {
	t.Parallel()
	want := []string{"audit", "sync", "authz", "settings", "identity", "signing_keys", "audit_log", "sessions", "routes", "jobs"}
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
		if got := slices.Index(names, "settings"); got != slices.Index(names, "authz")+1 {
			t.Errorf("%s: settings is not right after authz: %v", role, names)
		}
		if got := slices.Index(names, "identity"); got != slices.Index(names, "settings")+1 {
			t.Errorf("%s: identity is not right after settings: %v", role, names)
		}
	}
}
