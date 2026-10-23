// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// clusterComponents returns the names of the start sequence of role with cluster.mode mode.
func clusterComponents(t *testing.T, role Role, mode string) []string {
	t.Helper()
	s := &server{role: role, values: loadValues(t, "BOTHY_CLUSTER__MODE="+mode), terminalState: terminalState{plan: TerminalClusterPlan(mode, role)}}
	var out []string
	for _, c := range s.components() {
		out = append(out, c.Name())
	}
	return out
}

// TestTerminalSignalsWiring (M2-47 Design note 2, shared listener since M7-17): single uses
// LocalSignals and gives the terminal no route; multi uses NotifySignals in every role, and api
// and all start the shared component ephemeral_listener after the public listener (its stop step
// lies in PhaseEventTail, startEphemeral); a worker with multi gets NotifySignals without a
// listener. Role api with single still holds the listener for the gateway (M7-17 Design note 3).
func TestTerminalSignalsWiring(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleAPI, RoleWorker, RoleAll} {
		single := TerminalClusterPlan("single", role)
		if single.Multi || single.Notify || single.Listener || slices.Contains(clusterComponents(t, role, "single"), pg.EphemeralComponent) != (role == RoleAPI) {
			t.Errorf("%s single: %+v", role, single)
		}
		sig, local := terminalSignals(single, nil, nil)
		if local == nil || sig != any(local) {
			t.Errorf("%s single: signals %T", role, sig)
		}
		multi := TerminalClusterPlan("multi", role)
		listener := role != RoleWorker
		names := clusterComponents(t, role, "multi")
		if !multi.Multi || !multi.Notify || multi.Listener != listener || slices.Contains(names, pg.EphemeralComponent) != listener {
			t.Errorf("%s multi: %+v %v", role, multi, names)
		}
		if listener && slices.Index(names, pg.EphemeralComponent) < slices.Index(names, "public_listener") {
			t.Errorf("%s multi: the listener starts before the HTTP listeners: %v", role, names)
		}
		sig, local = terminalSignals(multi, nil, nil)
		if local != nil || sig == nil {
			t.Errorf("%s multi: signals %T", role, sig)
		}
	}
}
