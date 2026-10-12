// SPDX-License-Identifier: Apache-2.0

package routestest_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes/routestest"
)

func TestFixtureSurface(t *testing.T) {
	ops := routestest.Operations()
	if len(ops) != 3 {
		t.Fatalf("Operations() = %d, want the three fixture operations", len(ops))
	}
	rs, err := routestest.Build(routestest.Catalog())
	if err != nil {
		t.Fatalf("Build with the fixture catalogue: %v", err)
	}
	for i, r := range rs {
		if r.Operation.ID != ops[i].ID || r.Handler == nil {
			t.Errorf("route %d = %s (handler %v), want %s with a handler", i, r.Operation.ID, r.Handler != nil, ops[i].ID)
		}
	}
}
