// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestFinishRunBooks_BR09: a finish of Lotse with costs for model-large (complete price set, an
// adapter amount that the price overrides), model-unpriced with an amount and model-none without
// one books computed, adapter_reported and unpriced entries with the copies of the run, the run
// columns of design note 5 f in the one update of the transition, exactly one run.state_changed
// with the booked cost and one bothy.run.cost per entry after the commit (BR-09).
func TestFinishRunBooks_BR09(t *testing.T) {
	w := costsWorld(t)
	thread, run := w.started()
	before := w.runCostOf(run)
	seq := w.maxSeq()
	w.wantOK(w.costs(run, report("model-large", 1000, "USD", `"amount_micros":999`),
		`{"provider":"example","model":"model-unpriced","tokens_in":10,"amount_micros":4321,"currency":"EUR"}`,
		`{"provider":"example","model":"model-none","tokens_out":7,"currency":"USD"}`))
	es := byModel(w.entries(run))
	if len(es) != 3 {
		t.Fatalf("entries %+v", es)
	}
	after := w.runCostOf(run)
	want := map[string]struct {
		source, currency string
		amount           int64
	}{
		"model-large": {"computed", "USD", large(1000)}, "model-unpriced": {"adapter_reported", "EUR", 4321},
		"model-none": {"unpriced", "USD", 0},
	}
	space := w.support.String()
	for m, x := range want {
		e := es[m]
		if e.Source != x.source || e.Currency != x.currency || e.Amount != x.amount || !e.Estimate || e.Mode != "organization" {
			t.Fatalf("%s: %+v, want %+v", m, e, x)
		}
		if e.Agent != w.lotse.String() || e.Thread != thread.String() || e.Channel != w.allgemein.String() || e.Space != space ||
			!e.Occurred.Equal(after.Finished) {
			t.Fatalf("%s copies: %+v (finished %v)", m, e, after.Finished)
		}
	}
	// The entries follow the reports: the first is model-large in USD; model-none adds 0 USD.
	if after.Micros != large(1000) || after.Currency != "USD" || !after.Estimate || after.Version != before.Version+1 {
		t.Fatalf("run columns %+v, before %+v", after, before)
	}
	changed := w.eventsOf("run.state_changed", run)
	evs := 0
	for _, e := range w.eventsAfter(seq) {
		if e.Type == "run.state_changed" {
			evs++
		}
	}
	last := changed[len(changed)-1]
	cost, _ := last["run"].(map[string]any)["cost"].(map[string]any)
	if evs != 1 || cost == nil || int64(cost["amount_micros"].(float64)) != large(1000) || cost["currency"] != "USD" {
		t.Fatalf("%d run.state_changed, last cost %v", evs, cost)
	}
	booked := w.metrics.booked()
	if len(booked) != 3 {
		t.Fatalf("bothy.run.cost points %+v", booked)
	}
	for _, c := range booked {
		if c.AgentHandle != "lotse" || c.BillingMode != "organization" || !c.IsEstimate || c.Currency == "" {
			t.Fatalf("point %+v", c)
		}
	}
	// The own measurement books cost next to bothy.run.cost (M6-13): one call per entry with the
	// mode, the currency, the amount and the channel and agent of the run.
	costs := w.measure.Calls(measure.Cost)
	sums := map[string]int64{}
	for _, c := range costs {
		if c.Key.ChannelID != w.allgemein || c.Key.AgentID != w.lotse || c.Key.Dimension != "organization" || c.Value.Count != 1 {
			t.Fatalf("cost call %+v", c)
		}
		sums[c.Key.Currency] += c.Value.Sum
	}
	if len(costs) != 3 || sums["USD"] != large(1000) || sums["EUR"] != 4321 || len(w.measure.Invalid()) != 0 {
		t.Fatalf("cost calls %+v, invalid %v", costs, w.measure.Invalid())
	}
	w.recompute()
}

// TestFinishRunPartialSet: model-small has prices only for input and output; a report with cache
// counts and an amount is adapter_reported, without an amount unpriced.
func TestFinishRunPartialSet(t *testing.T) {
	w := costsWorld(t)
	_, run := w.started()
	w.wantOK(w.costs(run, report("model-small", 10, "USD", `"amount_micros":77`)))
	if es := w.entries(run); len(es) != 1 || es[0].Source != "adapter_reported" || es[0].Amount != 77 {
		t.Fatalf("with amount: %+v", es)
	}
	_, run2 := w.started()
	w.wantOK(w.costs(run2, report("model-small", 10, "USD", "")))
	if es := w.entries(run2); len(es) != 1 || es[0].Source != "unpriced" || es[0].Amount != 0 {
		t.Fatalf("without amount: %+v", es)
	}
	_, run3 := w.started()
	w.wantOK(w.costs(run3, `{"provider":"example","model":"model-small","tokens_in":1000000,"tokens_out":1000000,"currency":"USD"}`))
	if es := w.entries(run3); len(es) != 1 || es[0].Source != "computed" || es[0].Amount != 6_000_000 {
		t.Fatalf("only priced units: %+v", es)
	}
}

// TestFinishRunRepeatBooksNothing: the same finishRun again answers 200 and adds no entry, and
// parallel identical finishes book once.
func TestFinishRunRepeatBooksNothing(t *testing.T) {
	w := costsWorld(t)
	_, run := w.started()
	body := report("model-large", 5, "USD", "")
	w.wantOK(w.costs(run, body))
	w.wantOK(w.costs(run, body))
	if n := len(w.entries(run)); n != 1 {
		t.Fatalf("%d entries after a repeated finish", n)
	}
	_, run2 := w.started()
	var wg sync.WaitGroup
	rs := make([]*httpservertest.Response, 4)
	for i := range rs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rs[i] = w.costs(run2, body)
		}(i)
	}
	wg.Wait()
	for _, r := range rs {
		if r.StatusCode != http.StatusOK {
			t.Fatalf("parallel finish: %d %s", r.StatusCode, r.Body)
		}
	}
	if n := len(w.entries(run2)); n != 1 {
		t.Fatalf("%d entries after parallel finishes", n)
	}
}

// TestFinishRunCostsRules: invalid reports give 400 with pointers under /costs and change
// nothing; a non-empty costs on a run that never started gives 400 (invalid, /costs) and the run
// stays queued; a finish without costs or with costs [] books nothing and keeps the cost columns.
func TestFinishRunCostsRules(t *testing.T) {
	w := costsWorld(t)
	_, run := w.started()
	before := w.runCostOf(run)
	cases := map[string]struct{ reports, pointer, code string }{
		"empty provider":   {`{"provider":" ","model":"m","currency":"USD"}`, "/costs/0/provider", "required"},
		"negative tokens":  {`{"provider":"example","model":"m","tokens_out":-1,"currency":"USD"}`, "/costs/0/tokens_out", "too_small"},
		"too large":        {`{"provider":"example","model":"m","amount_micros":9007199254740992,"currency":"USD"}`, "/costs/0/amount_micros", "too_large"},
		"lower currency":   {`{"provider":"example","model":"m","currency":"usd"}`, "/costs/0/currency", "invalid_format"},
		"duplicate":        {`{"provider":"example","model":"m","currency":"USD"},{"provider":"example","model":"m","currency":"USD"}`, "/costs/1/model", "invalid"},
		"control in model": {`{"provider":"example","model":"m\u0007","currency":"USD"}`, "/costs/0/model", "invalid"},
	}
	for name, c := range cases {
		p := w.want(w.finish("lotse", run, `"state":"completed","costs":[`+c.reports+`]`), http.StatusBadRequest, "validation_failed")
		if got := fieldCodes(p); got[c.pointer] != c.code {
			t.Fatalf("%s: errors %v, want %s %s", name, got, c.pointer, c.code)
		}
	}
	if after := w.runCostOf(run); after != before || len(w.entries(run)) != 0 || w.lifecycle(run).State != "running" {
		t.Fatalf("an invalid report changed the run: %+v", after)
	}
	queued := w.trigger(w.newThread(w.allgemein))
	p := w.want(w.finish("lotse", queued, `"state":"failed","reason":"runtime_error","costs":[`+report("model-large", 1, "USD", "")+`]`),
		http.StatusBadRequest, "validation_failed")
	if got := fieldCodes(p); got["/costs"] != "invalid" || w.lifecycle(queued).State != "queued" || len(w.entries(queued)) != 0 {
		t.Fatalf("a run that never started: %v, state %s", got, w.lifecycle(queued).State)
	}
	// Non-default cost columns, as the live /cost patches of M2-10 would leave them.
	_, keep := w.started()
	w.sql(`UPDATE runs SET cost_micros = 1234, currency = 'EUR', cost_is_estimate = false WHERE id = $1`, keep)
	w.wantOK(w.finish("lotse", keep, `"state":"completed"`))
	_, empty := w.started()
	w.sql(`UPDATE runs SET cost_micros = 1234, currency = 'EUR', cost_is_estimate = false WHERE id = $1`, empty)
	w.wantOK(w.finish("lotse", empty, `"state":"completed","costs":[]`))
	for _, r := range []string{keep, empty} {
		if c := w.runCostOf(r); c.Micros != 1234 || c.Currency != "EUR" || c.Estimate || len(w.entries(r)) != 0 {
			t.Fatalf("a finish without costs changed the cost columns: %+v", c)
		}
	}
}

// TestFinishRunTwoCurrencies: model-large and model-eur give two entries; the run columns hold
// the currency of the first entry only, and getThreadCosts shows both currencies unconverted.
func TestFinishRunTwoCurrencies(t *testing.T) {
	w := costsWorld(t)
	thread, run := w.started()
	w.wantOK(w.costs(run, report("model-eur", 100, "EUR", ""), report("model-large", 200, "USD", "")))
	if c := w.runCostOf(run); c.Currency != "EUR" || c.Micros != large(100) {
		t.Fatalf("run columns %+v", c)
	}
	b := w.costsOf(w.h, "alex", thread)
	if len(b.Totals) != 2 || b.Totals[0].Currency != "EUR" || b.Totals[0].AmountMicros != large(100) ||
		b.Totals[1].Currency != "USD" || b.Totals[1].AmountMicros != large(200) {
		t.Fatalf("totals %+v", b.Totals)
	}
	if len(b.Runs) != 1 || b.Runs[0].Costs == nil || len(*b.Runs[0].Costs) != 2 {
		t.Fatalf("runs %+v", b.Runs)
	}
}
