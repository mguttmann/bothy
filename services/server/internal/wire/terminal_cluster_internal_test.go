// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"slices"
	"testing"
)

// componentNames returns the names of the start sequence of role with cluster.mode mode.
func clusterComponents(role Role, mode string) []string {
	s := &server{role: role, plan: TerminalClusterPlan(mode, role)}
	var out []string
	for _, c := range s.components() {
		out = append(out, c.Name())
	}
	return out
}

// TestTerminalSignalsWiring (M2-47 Design note 2): single uses LocalSignals and has no component
// terminal_ephemeral; multi uses NotifySignals in every role, and api and all start the listener
// after the public listener (its stop step lies in PhaseEventTail, startTerminalEphemeral); a
// worker with multi gets NotifySignals without a listener.
func TestTerminalSignalsWiring(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleAPI, RoleWorker, RoleAll} {
		single := TerminalClusterPlan("single", role)
		if single.Multi || single.Notify || single.Listener || slices.Contains(clusterComponents(role, "single"), "terminal_ephemeral") {
			t.Errorf("%s single: %+v", role, single)
		}
		sig, local := terminalSignals(single, nil, nil)
		if local == nil || sig != any(local) {
			t.Errorf("%s single: signals %T", role, sig)
		}
		multi := TerminalClusterPlan("multi", role)
		listener := role != RoleWorker
		names := clusterComponents(role, "multi")
		if !multi.Multi || !multi.Notify || multi.Listener != listener || slices.Contains(names, "terminal_ephemeral") != listener {
			t.Errorf("%s multi: %+v %v", role, multi, names)
		}
		if listener && slices.Index(names, "terminal_ephemeral") < slices.Index(names, "public_listener") {
			t.Errorf("%s multi: the listener starts before the HTTP listeners: %v", role, names)
		}
		sig, local = terminalSignals(multi, nil, nil)
		if local != nil || sig == nil {
			t.Errorf("%s multi: signals %T", role, sig)
		}
	}
}
