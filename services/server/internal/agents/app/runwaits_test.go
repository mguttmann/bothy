// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeRunWaits records the calls of the run waits port.
type fakeRunWaits struct {
	mu    gosync.Mutex
	calls [][]agents.RunWait
	agent []ids.ID
}

func (f *fakeRunWaits) ApplyRunWaits(_ context.Context, agentID ids.ID, waits []agents.RunWait) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, slices.Clone(waits))
	f.agent = append(f.agent, agentID)
	return nil
}

func (f *fakeRunWaits) snapshot() [][]agents.RunWait {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// TestHeartbeat_RunWaitsAndSlots (F-WARTESCHLANGE-GRUND, 09 4.8 and 5.2.1): waiting_runs reaches
// the runs port when the list differs from the last one this process applied (the first
// heartbeat applies the empty list, so a stale reason is cleared), and again after the persist
// interval; run_slots of the last heartbeat is in the presence and cleared by a heartbeat without
// it.
func TestHeartbeat_RunWaitsAndSlots(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	waits := &fakeRunWaits{}
	w.tr.BindRunWaits(waits)
	run := ids.New()
	st := w.beat(`{"state":"online","run_slots":{"used":2,"total":2}}`)
	if st.RunSlots == nil || st.RunSlots.Used != 2 || st.RunSlots.Total != 2 {
		t.Fatalf("run slots in the answer %+v", st.RunSlots)
	}
	if got := w.status().RunSlots; got == nil || *got != (agents.RunSlots{Used: 2, Total: 2}) {
		t.Fatalf("run slots in the presence %+v", got)
	}
	if calls := waits.snapshot(); len(calls) != 1 || len(calls[0]) != 0 {
		t.Fatalf("first heartbeat calls %+v, want one empty list", calls)
	}
	body := `{"state":"online","run_slots":{"used":2,"total":2},"waiting_runs":[{"run_id":"` + run.String() + `","reason":"slots_full"}]}`
	w.beat(body)
	w.beat(body)
	calls := waits.snapshot()
	if len(calls) != 2 || len(calls[1]) != 1 || calls[1][0] != (agents.RunWait{RunID: run, Reason: "slots_full"}) {
		t.Fatalf("calls %+v, want the list once", calls)
	}
	w.tm.advance(tPersist)
	w.beat(body)
	if n := len(waits.snapshot()); n != 3 {
		t.Fatalf("%d calls after the persist interval, want 3", n)
	}
	st = w.beat(`{"state":"online"}`)
	if calls := waits.snapshot(); len(calls) != 4 || len(calls[3]) != 0 {
		t.Fatalf("an absent member must clear the waits: %+v", calls)
	}
	if st.RunSlots != nil || w.status().RunSlots != nil {
		t.Fatalf("run slots kept without a report: %+v", st.RunSlots)
	}
	if waits.agent[0] != w.agent.ID {
		t.Fatalf("port got agent %s", waits.agent[0])
	}
}
