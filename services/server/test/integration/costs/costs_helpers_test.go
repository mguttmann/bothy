// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/adapters/billingtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The prices of the world (M2-22 "Tests"): provider example, model-large with all five units in
// USD, model-small with input and output only, model-eur with all units in EUR, source on
// example.com. Prices are per million tokens.
const (
	provider  = "example"
	sourceURL = "https://provider.example.com/pricing"
)

// largePrices are the prices of model-large per unit; model-eur uses the same numbers in EUR.
var largePrices = map[string]int64{
	"input_mtok": 3_000_000, "output_mtok": 15_000_000, "cache_read_mtok": 300_000, "cache_write_mtok": 3_750_000,
	"cache_write_1h_mtok": 6_000_000,
}

// costsWorld is the world of 07 11.1 with the run extensions and the price table of the tests,
// valid from one hour before the start of the clock.
func costsWorld(t *testing.T) *world {
	t.Helper()
	w := runsWorld(t)
	from := w.clk.Now().Add(-time.Hour)
	for unit, p := range largePrices {
		w.price("model-large", unit, p, "USD", from, time.Time{})
		w.price("model-eur", unit, p, "EUR", from, time.Time{})
	}
	w.price("model-small", "input_mtok", 1_000_000, "USD", from, time.Time{})
	w.price("model-small", "output_mtok", 5_000_000, "USD", from, time.Time{})
	return w
}

// priceBody is a PriceCreate body retrieved at valid_from.
func priceBody(model, unit string, micros int64, currency string, from, to time.Time) string {
	return priceBodyRetrieved(model, unit, micros, currency, from, to, from)
}

// priceBodyRetrieved is a PriceCreate body with retrieved_at.
func priceBodyRetrieved(model, unit string, micros int64, currency string, from, to, retrieved time.Time) string {
	body := `{"provider":"` + provider + `","model":"` + model + `","unit":"` + unit + `","price_micros":` +
		strconv.FormatInt(micros, 10) + `,"currency":"` + currency + `","valid_from":"` + stamp(from) +
		`","source_url":"` + sourceURL + `","retrieved_at":"` + stamp(retrieved) + `"`
	if !to.IsZero() {
		body += `,"valid_to":"` + stamp(to) + `"`
	}
	return body + "}"
}

// stamp is t in the Timestamp form of the API.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// createPrice sends createPrice as name.
func (w *world) createPrice(name, body string, headers ...string) *httpservertest.Response {
	if len(headers) == 0 {
		headers = key()
	}
	return w.do(name, http.MethodPost, "/api/v1/prices", body, headers...)
}

// price creates a price row as the owner and returns its id; a row valid from the future was
// retrieved now.
func (w *world) price(model, unit string, micros int64, currency string, from, to time.Time) string {
	w.t.Helper()
	retrieved := from
	if now := w.clk.Now(); retrieved.After(now) {
		retrieved = now
	}
	m := w.want(w.createPrice("owner", priceBodyRetrieved(model, unit, micros, currency, from, to, retrieved)), http.StatusCreated, "")
	return m["id"].(string)
}

// closePrice sends closePrice as name.
func (w *world) closePrice(name, id string, to time.Time) *httpservertest.Response {
	return w.do(name, http.MethodPost, "/api/v1/prices/"+id+"/close", `{"valid_to":"`+stamp(to)+`"}`, key()...)
}

// priceID reads the id of the open row of model and unit.
func (w *world) priceID(model, unit string) string {
	return w.text(`SELECT id::text FROM price_table WHERE model = $1 AND unit = $2 AND valid_to IS NULL`, model, unit)
}

// started opens a run of Lotse in a new thread of Support/Allgemein (triggered by Alex), delivers
// it and starts it with mode organization; it returns the thread and the run.
func (w *world) started() (ids.ID, string) {
	w.t.Helper()
	thread := w.newThread(w.allgemein)
	return thread, w.startedIn(thread)
}

// startedIn opens and starts a run of Lotse in thread.
func (w *world) startedIn(thread ids.ID, extra ...string) string {
	w.t.Helper()
	run := w.trigger(thread)
	w.wantOK(w.start("lotse", run, w.deliver(run), extra...))
	return run
}

// report is a CostReport with every token count n and the extra members.
func report(model string, n int64, currency string, extra string) string {
	s := strconv.FormatInt(n, 10)
	body := `{"provider":"` + provider + `","model":"` + model + `","tokens_in":` + s + `,"tokens_out":` + s +
		`,"tokens_cache_read":` + s + `,"tokens_cache_write":` + s + `,"tokens_cache_write_1h":` + s + `,"currency":"` + currency + `"`
	if extra != "" {
		body += "," + extra
	}
	return body + "}"
}

// large is the computed amount of model-large (or model-eur) for n tokens of every unit.
func large(n int64) int64 {
	var sum int64
	for _, p := range largePrices {
		sum += n * p
	}
	return (sum + 500_000) / 1_000_000
}

// costs sends finishRun completed with the reports as Lotse.
func (w *world) costs(run string, reports ...string) *httpservertest.Response {
	body := `"state":"completed","costs":[`
	for i, r := range reports {
		if i > 0 {
			body += ","
		}
		body += r
	}
	return w.finish("lotse", run, body+"]")
}

// entryRow is the part of a cost_entries row the tests read.
type entryRow struct {
	ID, Run, Agent, Thread, Channel, Space, Session, Mode, Model, Currency, Source string
	Amount                                                                         int64
	Estimate                                                                       bool
	Occurred                                                                       time.Time
}

// entries reads the cost entries of run in ascending id.
func (w *world) entries(run string) []entryRow {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id::text, run_id::text, agent_id::text, thread_id::text, channel_id::text,
		coalesce(space_id::text, ''), coalesce(agent_session_id::text, ''), billing_mode, model, currency, source, amount_micros,
		is_estimate, occurred_at FROM cost_entries WHERE run_id = $1 ORDER BY id`, run)
	must(w.t, err)
	defer rs.Close()
	var out []entryRow
	for rs.Next() {
		var e entryRow
		must(w.t, rs.Scan(&e.ID, &e.Run, &e.Agent, &e.Thread, &e.Channel, &e.Space, &e.Session, &e.Mode, &e.Model, &e.Currency, &e.Source,
			&e.Amount, &e.Estimate, &e.Occurred))
		out = append(out, e)
	}
	return out
}

// byModel indexes entries by model.
func byModel(es []entryRow) map[string]entryRow {
	out := map[string]entryRow{}
	for _, e := range es {
		out[e.Model] = e
	}
	return out
}

// runCost is the cost columns of a run.
type runCost struct {
	Micros   int64
	Currency string
	Estimate bool
	Version  int64
	Finished time.Time
}

// runCostOf reads the cost columns of run.
func (w *world) runCostOf(run string) runCost {
	w.t.Helper()
	var c runCost
	var finished *time.Time
	must(w.t, w.db.QueryRow(context.Background(), `SELECT cost_micros, currency, cost_is_estimate, version, finished_at FROM runs WHERE id = $1`, run).
		Scan(&c.Micros, &c.Currency, &c.Estimate, &c.Version, &finished))
	if finished != nil {
		c.Finished = *finished
	}
	return c
}

// threadCosts sends getThreadCosts as name on the harness h.
func (w *world) threadCosts(h *httpservertest.Harness, name string, thread ids.ID) *httpservertest.Response {
	headers := []string{}
	if name != "" {
		headers = append(headers, "Authorization", "Bearer "+name)
	}
	return h.Do(h.NewRequest(http.MethodGet, "/api/v1/threads/"+thread.String()+"/costs", "", headers...))
}

// breakdown is the decoded CostBreakdown.
type breakdown struct {
	ThreadID string `json:"thread_id"`
	Totals   []struct {
		BillingMode  string `json:"billing_mode"`
		AmountMicros int64  `json:"amount_micros"`
		Currency     string `json:"currency"`
		IsEstimate   bool   `json:"is_estimate"`
	} `json:"totals"`
	Runs []struct {
		RunID          string           `json:"run_id"`
		BillingMode    string           `json:"billing_mode"`
		AgentSessionID string           `json:"agent_session_id"`
		Tokens         map[string]int64 `json:"tokens"`
		Costs          *[]struct {
			AmountMicros int64  `json:"amount_micros"`
			Currency     string `json:"currency"`
			IsEstimate   bool   `json:"is_estimate"`
		} `json:"costs"`
		Unpriced bool `json:"unpriced"`
	} `json:"runs"`
}

// costsOf reads getThreadCosts of thread as name and decodes the 200 body.
func (w *world) costsOf(h *httpservertest.Harness, name string, thread ids.ID) breakdown {
	w.t.Helper()
	r := w.threadCosts(h, name, thread)
	w.want(r, http.StatusOK, "")
	var b breakdown
	must(w.t, json.Unmarshal(r.Body, &b))
	return b
}

// recompute runs billingtest.RecomputeComputed and fails on a difference.
func (w *world) recompute() {
	w.t.Helper()
	diffs, err := billingtest.New(w.db).RecomputeComputed(context.Background())
	must(w.t, err)
	if len(diffs) != 0 {
		w.t.Fatalf("computed entries no longer recompute: %+v", diffs)
	}
}

// fieldCodes returns the errors[] of a problem as pointer -> code.
func fieldCodes(p map[string]any) map[string]string {
	out := map[string]string{}
	list, _ := p["errors"].([]any)
	for _, e := range list {
		m, _ := e.(map[string]any)
		ptr, _ := m["pointer"].(string)
		code, _ := m["code"].(string)
		out[ptr] = code
	}
	return out
}
