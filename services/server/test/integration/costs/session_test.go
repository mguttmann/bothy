// SPDX-License-Identifier: Apache-2.0

package costs_test

import "testing"

// TestSessionCostSum: two runs in one session raise cumulative_cost_micros by their sums and the
// version by one each, write no agent_session.changed, and an entry of another currency is not
// added.
func TestSessionCostSum(t *testing.T) {
	w := costsWorld(t)
	thread := w.newThread(w.allgemein)
	r1 := w.startedIn(thread, `"external_session_ref":"session-a"`)
	row, ok := w.current(thread)
	if !ok || w.runSession(r1) != row.ID {
		t.Fatalf("no session row of the run: %+v", row)
	}
	events := len(w.sessionEvents(thread))
	w.wantOK(w.costs(r1, report("model-large", 10, "USD", "")))
	if n := len(w.sessionEvents(thread)); n != events {
		t.Fatalf("the first booking wrote %d agent_session.changed", n-events)
	}
	after1, _ := w.current(thread)
	if after1.Cumulative != row.Cumulative+large(10) || after1.Currency != "USD" || after1.Version != row.Version+1 {
		t.Fatalf("after the first run %+v, before %+v", after1, row)
	}
	r2 := w.startedIn(thread)
	if w.runSession(r2) != row.ID {
		t.Fatalf("the second run is not in the session: %s", w.runSession(r2))
	}
	before2, _ := w.current(thread)
	events2 := len(w.sessionEvents(thread))
	w.wantOK(w.costs(r2, report("model-large", 20, "USD", ""), report("model-eur", 30, "EUR", "")))
	after2, _ := w.current(thread)
	if after2.Cumulative != before2.Cumulative+large(20) || after2.Version != before2.Version+1 || after2.Currency != "USD" {
		t.Fatalf("after the second run %+v, before %+v", after2, before2)
	}
	if n := len(w.sessionEvents(thread)); n != events2 {
		t.Fatalf("the second booking wrote %d agent_session.changed", n-events2)
	}
}
