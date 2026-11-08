// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/httpapi"
)

// TestRunREST_PassesATriggerKindItDoesNotKnowThrough holds the read path of getRun that a server
// rollback below a stage relies on (requirement 19 of AR-04, issue 567 item 11, ruling
// D-I567-3): RunREST copies the stored trigger kind as it is, without checking it against the
// enum it was built with, and the production build validates no response against the contract,
// so a server older than AR-04 still answers getRun of a wake run (trigger_kind wake) with 200
// and the kind unchanged. The older binary has this same copy in RunREST; a check added here
// would break the next rollback below a stage that adds a kind.
func TestRunREST_PassesATriggerKindItDoesNotKnowThrough(t *testing.T) {
	for _, kind := range []string{"wake", "a_later_kind"} {
		r := runs.Run{
			RunSummary:  runs.RunSummary{ID: ids.New(), ThreadID: ids.New(), AgentID: ids.New(), TriggeredBy: ids.New(), State: "completed"},
			TriggerKind: kind, Metrics: map[string]any{},
		}
		out, err := httpapi.RunREST(r)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if out.TriggerKind == nil || string(*out.TriggerKind) != kind {
			t.Fatalf("%s: trigger_kind %v", kind, out.TriggerKind)
		}
		body, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil || m["trigger_kind"] != kind {
			t.Fatalf("%s: encoded %s", kind, body)
		}
	}
}
