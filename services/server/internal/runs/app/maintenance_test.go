// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestPrepare_HoldsInMaintenance (M7-08, ADR 0065 item 4): in maintenance mode the delivery of
// an event whose run is not a running run of the agent is held: nothing is sent, checked or
// written, so the queued run waits unchanged; command.invoked is held as well. The delivery of a
// running run of the agent goes on.
func TestPrepare_HoldsInMaintenance(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	w.maintenance = true
	w.build()
	if err := w.svc.BindTokens(newInvEnv(t).svc); err != nil {
		t.Fatal(err)
	}
	before, writes, triggers := w.current(), len(w.j.list()), len(w.triggerStates(w.message))
	for _, in := range []runs.DeliveryInput{
		w.input(domain.EventAgentInvocation),
		{AgentID: w.lotse, EventID: ids.New(), Seq: 8, Type: domain.EventCommandInvoked, ResourceKind: "command", ResourceID: ids.New()},
		{AgentID: w.texter, EventID: ids.New(), Seq: 9, Type: domain.EventAgentInvocation, ResourceKind: "run", ResourceID: w.run.ID},
	} {
		out := w.prepare(in)
		if !out.Hold || out.Send {
			t.Fatalf("%s of %s: outcome %+v, want held", in.Type, in.AgentID, out)
		}
	}
	if !reflect.DeepEqual(w.current(), before) || len(w.j.list()) != writes || len(w.triggerStates(w.message)) != triggers {
		t.Fatalf("a held delivery wrote: run %+v -> %+v, journal %v", before, w.current(), w.j.whats()[writes:])
	}
	w.setState(domain.StateRunning)
	if out := w.prepare(w.input(domain.EventAgentInvocation)); out.Hold || !out.Send {
		t.Fatalf("delivery of the running run: %+v, want sent", out)
	}
}
