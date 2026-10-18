// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// TestNewValidatorLeavesOutResponseSchemas: the production build validates requests only, so its
// validator keeps no responses and compiles no response schema at the start (perf.startup_ready_max
// and perf.idle_rss_max, M1-72); the test build with contract_validate keeps them for G8
// (TestG8_ChainInstallsTheCheck). The test runs in both builds and checks the one it is in.
func TestNewValidatorLeavesOutResponseSchemas(t *testing.T) {
	t.Parallel()
	v, err := NewValidator(rest.Spec, []string{"getMe", "createMessage"})
	if err != nil {
		t.Fatal(err)
	}
	for id, e := range v.ops {
		for _, op := range e.item.GetOperations().FromOldest() {
			if (op.Responses != nil) != keepResponses {
				t.Errorf("%s: responses kept %t, want %t in this build", id, op.Responses != nil, keepResponses)
			}
		}
	}
}
