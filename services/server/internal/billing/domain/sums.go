// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// EntryRow is one stored cost entry as the sums read it.
type EntryRow struct {
	ID, RunID, AgentSessionID ids.ID
	BillingMode               string
	Provider, Model           string
	Tokens                    Tokens
	AmountMicros              int64
	Currency                  string
	IsEstimate                bool
	Source                    string
	OccurredAt                time.Time
	// SupersedesID is the entry this one corrects; zero for none.
	SupersedesID ids.ID
}

// Valid returns the entries no other entry of es supersedes, in their order (03 cost_entries:
// "Valid total = entries without a successor").
func Valid(es []EntryRow) []EntryRow {
	superseded := make(map[ids.ID]bool)
	for _, e := range es {
		if !e.SupersedesID.IsZero() {
			superseded[e.SupersedesID] = true
		}
	}
	out := make([]EntryRow, 0, len(es))
	for _, e := range es {
		if !superseded[e.ID] {
			out = append(out, e)
		}
	}
	return out
}

// Total is a valid sum of one billing mode and currency.
type Total struct {
	BillingMode, Currency string
	AmountMicros          int64
	// IsEstimate is true when any summed entry is an estimate.
	IsEstimate bool
	Tokens     Tokens
	Entries    int64
}

// Sum groups the valid entries of es by billing mode and currency, ordered by billing mode, then
// currency (M2-22 design note 7). Amounts are never converted; token counts are summed per field,
// and a correction entry repeats the counts of the entry it supersedes, so token sums do not drop
// after a reconciliation.
func Sum(es []EntryRow) []Total {
	idx := map[[2]string]int{}
	var out []Total
	for _, e := range Valid(es) {
		k := [2]string{e.BillingMode, e.Currency}
		i, ok := idx[k]
		if !ok {
			out = append(out, Total{BillingMode: e.BillingMode, Currency: e.Currency})
			i = len(out) - 1
			idx[k] = i
		}
		out[i].AmountMicros += e.AmountMicros
		out[i].IsEstimate = out[i].IsEstimate || e.IsEstimate
		out[i].Tokens = out[i].Tokens.Add(e.Tokens)
		out[i].Entries++
	}
	slices.SortFunc(out, func(a, b Total) int {
		return cmp.Or(cmp.Compare(a.BillingMode, b.BillingMode), cmp.Compare(a.Currency, b.Currency))
	})
	return out
}

// ShowAmounts is the display rule of 10 section 12.2 (BR-11, D-BR-01): amounts appear for mode
// organization, and for mode personal only when billing.show_personal_estimates is on.
func ShowAmounts(mode string, showPersonal bool) bool {
	return mode != "personal" || showPersonal
}

// Money is an amount of one currency with its estimate flag.
type Money struct {
	AmountMicros int64
	Currency     string
	IsEstimate   bool
}

// RunCost is the cost of one run in a thread breakdown.
type RunCost struct {
	RunID, AgentSessionID ids.ID
	BillingMode           string
	Tokens                Tokens
	// Costs are one Money per currency, ordered by currency; nil when the display rule hides
	// the amounts.
	Costs []Money
	// Unpriced reports a valid entry of source unpriced ("Price missing").
	Unpriced bool
}

// ThreadBreakdown is the cost breakdown of one thread (schema CostBreakdown).
type ThreadBreakdown struct {
	// Totals are the valid sums per billing mode and currency whose amounts the display rule
	// shows.
	Totals []Total
	// Runs are the runs with at least one valid entry, by run id ascending (UUIDv7, the order of
	// the runs).
	Runs []RunCost
}

// Breakdown builds the thread breakdown of the entries es (M2-22 design note 8 b to e): valid
// entries only, grouped per run and per currency; the display rule hides the amounts of personal
// runs (no costs, no total) unless showPersonal; billing mode and tokens are always present.
func Breakdown(es []EntryRow, showPersonal bool) ThreadBreakdown {
	valid := Valid(es)
	var out ThreadBreakdown
	for _, t := range Sum(valid) {
		if ShowAmounts(t.BillingMode, showPersonal) {
			out.Totals = append(out.Totals, t)
		}
	}
	byRun := map[ids.ID][]EntryRow{}
	var runIDs []ids.ID
	for _, e := range valid {
		if _, ok := byRun[e.RunID]; !ok {
			runIDs = append(runIDs, e.RunID)
		}
		byRun[e.RunID] = append(byRun[e.RunID], e)
	}
	slices.SortFunc(runIDs, func(a, b ids.ID) int { return cmp.Compare(a.String(), b.String()) })
	for _, id := range runIDs {
		out.Runs = append(out.Runs, runCost(id, byRun[id], showPersonal))
	}
	return out
}

// runCost sums the valid entries of one run.
func runCost(id ids.ID, es []EntryRow, showPersonal bool) RunCost {
	rc := RunCost{RunID: id, BillingMode: es[0].BillingMode}
	var costs []Money
	for _, e := range es {
		rc.Tokens = rc.Tokens.Add(e.Tokens)
		rc.Unpriced = rc.Unpriced || e.Source == SourceUnpriced
		if rc.AgentSessionID.IsZero() {
			rc.AgentSessionID = e.AgentSessionID
		}
		i := slices.IndexFunc(costs, func(m Money) bool { return m.Currency == e.Currency })
		if i < 0 {
			costs = append(costs, Money{Currency: e.Currency})
			i = len(costs) - 1
		}
		costs[i].AmountMicros += e.AmountMicros
		costs[i].IsEstimate = costs[i].IsEstimate || e.IsEstimate
	}
	slices.SortFunc(costs, func(a, b Money) int { return cmp.Compare(a.Currency, b.Currency) })
	if ShowAmounts(rc.BillingMode, showPersonal) {
		rc.Costs = costs
	}
	return rc
}
