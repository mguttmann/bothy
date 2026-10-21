// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"net/http"
	"slices"
	"testing"
)

// TestSEC_T10_CoverageReportsMissingOperation: the coverage record maps requests to operation
// IDs like the server routes and names every enabled operation without a call unless exempt, so
// TestSEC_T10_ScenarioCoversEveryEnabledOperation fails for a new operation.
func TestSEC_T10_CoverageReportsMissingOperation(t *testing.T) {
	c := NewCoverage()
	c.Observe(http.MethodGet, "/threads/0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b/messages?limit=5", http.StatusOK)
	c.Observe(http.MethodPost, "/spaces", 0) // a transport error is no call
	if got := OperationOf(http.MethodPut, "/me/settings/notify.default_level"); got != "putMySetting" {
		t.Errorf("OperationOf = %q, want putMySetting", got)
	}
	missing := c.Missing([]string{"listThreadMessages", "createSpace", "getMe"}, map[string]string{"getMe": "reason"})
	if !slices.Equal(missing, []string{"createSpace"}) {
		t.Errorf("Missing = %v, want [createSpace]", missing)
	}
	enabled, err := EnabledOperations()
	if err != nil || len(enabled) == 0 || !slices.IsSorted(enabled) {
		t.Fatalf("EnabledOperations = %d entries, %v", len(enabled), err)
	}
	for _, id := range enabled {
		if !c.known(id) {
			t.Errorf("enabled operation %s is not in the contract", id)
		}
	}
}
