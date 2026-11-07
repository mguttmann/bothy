// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
)

// TestDecodeHeartbeatTurnHandoverAndWakes (AR-04, 09 18.19.1): details.capabilities accepts
// turn_handover and wakes as booleans and keeps them in the stored form (reported_details); any
// other type is invalid, an unknown member stays refused, and a heartbeat without them keeps them
// absent.
func TestDecodeHeartbeatTurnHandoverAndWakes(t *testing.T) {
	t.Parallel()
	r, fes := domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"turn_handover":true,"wakes":true}}}`))
	if len(fes) > 0 {
		t.Fatalf("errors %+v", fes)
	}
	c := r.Details.Capabilities
	if c == nil || c.TurnHandover == nil || !*c.TurnHandover || c.Wakes == nil || !*c.Wakes {
		t.Fatalf("capabilities %+v", c)
	}
	stored := string(r.StoredDetails())
	if !strings.Contains(stored, `"turn_handover":true`) || !strings.Contains(stored, `"wakes":true`) {
		t.Fatalf("stored form %s", stored)
	}
	back, err := domain.ReportFromStore(domain.RuntimeOnline, "", "", "", r.StoredDetails())
	if err != nil || back.Details.Capabilities == nil || back.Details.Capabilities.Wakes == nil || !*back.Details.Capabilities.Wakes {
		t.Fatalf("read back %+v %v", back.Details.Capabilities, err)
	}
	_, fes = domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"wakes":"yes"}}}`))
	if !hasError(fes, "/details/capabilities/wakes", "") || len(fes) != 1 {
		t.Fatalf("errors %+v", fes)
	}
	_, fes = domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"timers":true}}}`))
	if len(fes) == 0 {
		t.Fatal("an unknown capability was accepted")
	}
	plain, _ := domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"title":true}}}`))
	if pc := plain.Details.Capabilities; pc.TurnHandover != nil || pc.Wakes != nil || strings.Contains(string(plain.StoredDetails()), "wakes") {
		t.Fatalf("capabilities without a report: %+v", pc)
	}
}
