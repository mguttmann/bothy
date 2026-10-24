// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSessionsAddCost: the session sum of a booking (M2-22 design note 5 g). Inside the caller's
// unit of work the row is locked, takes the booking's currency while its cost and baseline are 0,
// adds an amount of its currency with version + 1 and skips another currency without a change. No
// event, no audit entry and no authorization.
func TestSessionsAddCost(t *testing.T) {
	t.Parallel()
	add := func(e *sessEnv, cmd agents.AddSessionCost) error {
		return e.inUnit(func(ctx context.Context) error { return e.svc.AddCost(ctx, cmd) })
	}
	// fixture stores a current row with the cost columns given.
	fixture := func(e *sessEnv, cumulative, baseline int64, currency string) agents.Session {
		r := e.row(e.thread, e.lotse.ID, "sess-a", true, true, 0)
		// A booked row is an estimate (the column default) until a reconciliation (M2-28).
		r.CumulativeCostMicros, r.CostBaselineMicros, r.Currency, r.CostIsEstimate = cumulative, baseline, currency, true
		return e.store.put(r)
	}
	// wantQuiet checks that AddCost wrote no event, audit entry or SessionAttached and never
	// authorized.
	wantQuiet := func(t *testing.T, e *sessEnv) {
		t.Helper()
		if n, a, p, c := len(e.eventsSnapshot()), len(e.auditSnapshot()), len(e.attachedSnapshot()), len(e.checksSnapshot()); n+a+p+c != 0 {
			t.Fatalf("events %d, audit %d, attached %d, authorizer calls %d, want none", n, a, p, c)
		}
	}
	t.Run("two adds", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, 100, 0, "EUR")
		for _, amount := range []int64{1500, 2500} {
			if err := add(e, agents.AddSessionCost{SessionID: r.ID, AmountMicros: amount, Currency: "EUR"}); err != nil {
				t.Fatal(err)
			}
		}
		got := e.stored(r.ID)
		want := r
		want.CumulativeCostMicros, want.Version = 4100, r.Version+2
		if got != want {
			t.Fatalf("row %+v, want %+v", got, want)
		}
		if n := len(e.store.locked); n != 2 || len(e.store.locked[0]) != 1 || e.store.locked[0][0] != r.ID {
			t.Fatalf("locks %v, want the row once per add", e.store.locked)
		}
		wantQuiet(t, e)
	})
	t.Run("another currency is skipped", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, 100, 0, "EUR")
		if err := add(e, agents.AddSessionCost{SessionID: r.ID, AmountMicros: 1500, Currency: "USD"}); err != nil {
			t.Fatal(err)
		}
		e.wantNothingChanged([]agents.Session{r})
		wantQuiet(t, e)
	})
	t.Run("a baseline keeps the currency", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, 0, 700, "EUR")
		if err := add(e, agents.AddSessionCost{SessionID: r.ID, AmountMicros: 1500, Currency: "USD"}); err != nil {
			t.Fatal(err)
		}
		e.wantNothingChanged([]agents.Session{r})
	})
	t.Run("a fresh row takes the currency", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, 0, 0, sessDefaultCurrency)
		if err := add(e, agents.AddSessionCost{SessionID: r.ID, AmountMicros: 1500, Currency: "EUR"}); err != nil {
			t.Fatal(err)
		}
		got := e.stored(r.ID)
		want := r
		want.CumulativeCostMicros, want.Currency, want.Version = 1500, "EUR", r.Version+1
		if got != want {
			t.Fatalf("row %+v, want %+v", got, want)
		}
		wantQuiet(t, e)
	})
	t.Run("a fresh row takes the currency of a zero amount", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, 0, 0, sessDefaultCurrency)
		if err := add(e, agents.AddSessionCost{SessionID: r.ID, Currency: "EUR"}); err != nil {
			t.Fatal(err)
		}
		got := e.stored(r.ID)
		want := r
		want.Currency, want.Version = "EUR", r.Version+1
		if got != want {
			t.Fatalf("row %+v, want %+v", got, want)
		}
	})
	t.Run("nothing to change", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		fresh := fixture(e, 0, 0, sessDefaultCurrency)
		r := e.store.put(agents.Session{ID: ids.New(), ThreadID: e.other, AgentID: e.lotse.ID, ExternalSessionRef: "sess-b", IsCurrent: true,
			CumulativeCostMicros: 900, Currency: "EUR", CostIsEstimate: true, CreatedAt: start})
		before := e.store.all()
		for _, cmd := range []agents.AddSessionCost{
			{SessionID: fresh.ID, Currency: sessDefaultCurrency},
			{SessionID: r.ID, Currency: "EUR"},
		} {
			if err := add(e, cmd); err != nil {
				t.Fatal(err)
			}
		}
		e.wantNothingChanged(before)
	})
	t.Run("a booking after a reconciliation is an estimate again", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, 3500, 0, "USD")
		r.CostIsEstimate = false
		r = e.store.put(r)
		if err := add(e, agents.AddSessionCost{SessionID: r.ID, AmountMicros: 1000, Currency: "USD"}); err != nil {
			t.Fatal(err)
		}
		want := r
		want.CumulativeCostMicros, want.CostIsEstimate, want.Version = 4500, true, r.Version+1
		if got := e.stored(r.ID); got != want {
			t.Fatalf("row %+v, want %+v", got, want)
		}
		// A zero amount of an estimate turns the flag back too.
		z := e.store.put(agents.Session{ID: ids.New(), ThreadID: e.other, AgentID: e.lotse.ID, ExternalSessionRef: "sess-z", IsCurrent: true,
			CumulativeCostMicros: 900, Currency: "USD", CreatedAt: start})
		if err := add(e, agents.AddSessionCost{SessionID: z.ID, Currency: "USD"}); err != nil {
			t.Fatal(err)
		}
		if got := e.stored(z.ID); !got.CostIsEstimate || got.Version != z.Version+1 || got.CumulativeCostMicros != 900 {
			t.Fatalf("row %+v", got)
		}
	})
	t.Run("internal errors", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		r := fixture(e, maxSafeInteger-10, 0, "EUR")
		before := e.store.all()
		err := e.svc.AddCost(context.Background(), agents.AddSessionCost{SessionID: r.ID, AmountMicros: 1, Currency: "EUR"})
		wantCode(t, err, errs.CodeInternalError)
		for _, cmd := range []agents.AddSessionCost{
			{SessionID: ids.New(), AmountMicros: 1, Currency: "EUR"},
			{SessionID: r.ID, AmountMicros: -1, Currency: "EUR"},
			{SessionID: r.ID, AmountMicros: 1, Currency: ""},
			{SessionID: r.ID, AmountMicros: 10, Currency: "EUR"},
		} {
			wantCode(t, add(e, cmd), errs.CodeInternalError)
		}
		e.wantNothingChanged(before)
		if err := add(e, agents.AddSessionCost{SessionID: r.ID, AmountMicros: 9, Currency: "EUR"}); err != nil {
			t.Fatalf("a sum just below 2^53: %v", err)
		}
		if got := e.stored(r.ID); got.CumulativeCostMicros != maxSafeInteger-1 {
			t.Fatalf("sum %d", got.CumulativeCostMicros)
		}
	})
}

// maxSafeInteger is 2^53, the integer bound of 06 "Basic rules": every sum stays below it.
const maxSafeInteger = int64(1) << 53

// TestSessionsApplyCostTotals: the reconciled session totals (M2-28 design note 7). Inside the
// caller's unit of work the rows are locked once in ascending id; a row whose sum or estimate flag
// differs gets both with version + 1, an equal row, a row of another currency and an unknown
// session stay, cost_baseline_micros is never touched. No event, no audit entry, no authorization.
func TestSessionsApplyCostTotals(t *testing.T) {
	t.Parallel()
	e := newSessEnv(t)
	apply := func(totals ...agents.SessionCostTotal) error {
		return e.inUnit(func(ctx context.Context) error { return e.svc.ApplyCostTotals(ctx, totals) })
	}
	changed := e.row(e.thread, e.lotse.ID, "sess-a", true, true, 0)
	changed.CumulativeCostMicros, changed.CostBaselineMicros, changed.Currency, changed.CostIsEstimate = 4000, 700, "USD", true
	changed = e.store.put(changed)
	equal := e.store.put(agents.Session{ID: ids.New(), ThreadID: e.other, AgentID: e.lotse.ID, ExternalSessionRef: "sess-b", IsCurrent: true,
		CumulativeCostMicros: 900, Currency: "USD", CostIsEstimate: false, CreatedAt: start})
	foreign := e.store.put(agents.Session{ID: ids.New(), ThreadID: e.other, AgentID: e.lotse.ID, ExternalSessionRef: "sess-c",
		CumulativeCostMicros: 900, Currency: "EUR", CostIsEstimate: true, CreatedAt: start})
	if err := apply(
		agents.SessionCostTotal{SessionID: equal.ID, AmountMicros: 900, Currency: "USD"},
		agents.SessionCostTotal{SessionID: changed.ID, AmountMicros: 3500, Currency: "USD"},
		agents.SessionCostTotal{SessionID: foreign.ID, AmountMicros: 1, Currency: "USD"},
		agents.SessionCostTotal{SessionID: ids.New(), AmountMicros: 1, Currency: "USD"},
	); err != nil {
		t.Fatal(err)
	}
	want := changed
	want.CumulativeCostMicros, want.CostIsEstimate, want.Version = 3500, false, changed.Version+1
	if got := e.stored(changed.ID); got != want {
		t.Fatalf("changed row %+v, want %+v", got, want)
	}
	for _, r := range []agents.Session{equal, foreign} {
		if got := e.stored(r.ID); got != r {
			t.Fatalf("row %+v changed to %+v", r, got)
		}
	}
	if n := len(e.store.locked); n != 1 || len(e.store.locked[0]) != 4 {
		t.Fatalf("locks %v, want one call with every session", e.store.locked)
	}
	if n, a, p, c := len(e.eventsSnapshot()), len(e.auditSnapshot()), len(e.attachedSnapshot()), len(e.checksSnapshot()); n+a+p+c != 0 {
		t.Fatalf("events %d, audit %d, attached %d, authorizer calls %d, want none", n, a, p, c)
	}
	// Again with the same totals: nothing changes.
	before := e.store.all()
	if err := apply(agents.SessionCostTotal{SessionID: changed.ID, AmountMicros: 3500, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	e.wantNothingChanged(before)
	wantCode(t, e.svc.ApplyCostTotals(context.Background(), nil), errs.CodeInternalError)
	wantCode(t, apply(agents.SessionCostTotal{SessionID: changed.ID, AmountMicros: -1, Currency: "USD"}), errs.CodeInternalError)
	wantCode(t, apply(agents.SessionCostTotal{SessionID: changed.ID, AmountMicros: maxSafeInteger, Currency: "USD"}), errs.CodeInternalError)
}
