// SPDX-License-Identifier: Apache-2.0

package reconcile_test

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	billingjobs "github.com/OWNER/bothy/services/server/internal/billing/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestReconcile runs the integration cases of brief M2-28 ("Integration") on one server, one after
// the other, each on its own runs: the job gate of the world is shared, so the cases never run in
// parallel.
func TestReconcile(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	app, token := e.installApp()
	for _, c := range []struct {
		name string
		run  func(e *env)
	}{
		{"TestReconcileCosts_OwnerWithStepUp", ownerWithStepUp},
		{"TestReconcileCosts_StepUpRequired", stepUpRequired},
		{"TestReconcileCosts_TokensRejected", func(e *env) { tokensRejected(e, token) }},
		{"TestSubmitCostReconciliation_App", func(e *env) { submitByApp(e, token) }},
		{"TestReconcile_BothPathsUseSameJob", func(e *env) { bothPathsUseSameJob(e, token) }},
		{"TestReconcile_BR10_EstimatesSuperseded", func(e *env) { estimatesSuperseded(e, token) }},
		{"TestReconcile_OnlyAddsReconciledEntries", func(e *env) { onlyAddsReconciledEntries(e, token) }},
		{"TestReconcile_Unmatched", func(e *env) { unmatched(e, token) }},
		{"TestReconcile_ResubmitSameAmountIsNoop", func(e *env) { resubmitIsNoop(e, token) }},
		{"TestReconcile_ConcurrentSubmissions", func(e *env) { concurrentSubmissions(e, token) }},
		{"TestReconcile_JobUpdatesRunsAndSessions", func(e *env) { jobUpdatesRunsAndSessions(e, token) }},
		{"TestReconcile_JobIsIdempotent", func(e *env) { jobIsIdempotent(e, token) }},
		{"TestReconcile_JobAgainstConcurrentBooking", func(e *env) { jobAgainstConcurrentBooking(e, token) }},
		{"TestReconcile_Validation", func(e *env) { validation(e, token) }},
		{"TestReconcile_IdempotencyKey", func(e *env) { idempotencyKey(e, token) }},
		{"TestReconcile_UnknownMemberCanary", func(e *env) { unknownMemberCanary(e, token) }},
		{"TestCostEstimates_Read", func(e *env) { estimatesRead(e, token) }},
		{"TestSubmitCostReconciliation_ScopeMissing", func(e *env) { scopeMissing(e, app, token) }},
		{"TestSubmitCostReconciliation_PeopleAndAgentsRejected", func(e *env) { peopleAndAgentsRejected(e, app, token) }},
	} {
		if !t.Run(c.name, func(t *testing.T) { c.run(e.in(t)) }) {
			return
		}
	}
}

// ownerWithStepUp (TestReconcileCosts_OwnerWithStepUp): the owner with a fresh step-up reconciles
// two runs; 200 with reconciled 2, the new rows exactly as design note 3 f, one job with both run
// IDs; before the job the run totals are unchanged, after it they hold with cost_is_estimate false.
func ownerWithStepUp(e *env) {
	r1 := e.run(e.a1(), usd(1_000_000))
	r2 := e.run(e.a2(), usd(3_000_000))
	from, to := e.day(r1, r2)
	before := map[string][]entry{r1: e.entries(r1), r2: e.entries(r2)}
	e.gate.hold()
	res := e.decode(e.must(e.owner, http.MethodPost, "/costs/reconcile", body(from, to, r1, 1_500_000, "USD", r2, 2_000_000, "USD"),
		http.StatusOK))
	if res.Reconciled != 2 || len(res.Unmatched) != 0 {
		e.release()
		e.t.Fatalf("result %+v", res)
	}
	for run, amount := range map[string]int64{r1: 1_500_000, r2: 2_000_000} {
		old := before[run][0]
		after := e.entries(run)
		if len(after) != 2 {
			e.release()
			e.t.Fatalf("run %s has %d entries", run, len(after))
		}
		n := after[1]
		if n.Supersedes != old.ID || n.Source != "reconciled" || n.Estimate || n.Reconciled == nil || n.Amount != amount || n.Currency != "USD" ||
			n.Run != old.Run || n.Agent != old.Agent || n.Thread != old.Thread || n.Channel != old.Channel || n.Space != old.Space ||
			n.Session != old.Session || n.Mode != old.Mode || n.Provider != old.Provider || n.Model != old.Model || n.Tokens != old.Tokens ||
			!n.Occurred.Equal(old.Occurred) {
			e.release()
			e.t.Fatalf("reconciled entry %+v of %+v", n, old)
		}
		if c := e.runCostOf(run); c.Micros != old.Amount || !c.Estimate {
			e.release()
			e.t.Fatalf("run %s changed before the job: %+v", run, c)
		}
	}
	if n := e.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'billing.reconcile' AND args->'run_ids' ? $1 AND args->'run_ids' ? $2`,
		r1, r2); n != 1 {
		e.release()
		e.t.Fatalf("%d jobs with both runs", n)
	}
	e.release()
	e.settle()
	for run, amount := range map[string]int64{r1: 1_500_000, r2: 2_000_000} {
		if c := e.runCostOf(run); c.Micros != amount || c.Estimate || c.Currency != "USD" {
			e.t.Fatalf("run %s after the job: %+v", run, c)
		}
	}
}

// release lets held jobs run.
func (e *env) release() { e.gate.release() }

// stepUpRequired (TestReconcileCosts_StepUpRequired): a session of the owner without a fresh
// step-up gets 401 step_up_required and nothing is written; the same request with the same
// Idempotency-Key after the step-up gives 200.
func stepUpRequired(e *env) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	fresh, err := e.s.OwnerSession(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	key := ids.New().String()
	b := body(from, to, r, 900_000, "USD")
	e.wantProblem(e.do(fresh, http.MethodPost, "/costs/reconcile", b, "Idempotency-Key", key), http.StatusUnauthorized, "step_up_required", "")
	if n := len(e.entries(r)); n != 1 || e.jobsOf(r) != 0 {
		e.t.Fatalf("written before the step-up: %d entries, %d jobs", n, e.jobsOf(r))
	}
	if err := fresh.StepUp(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	res := e.decode(e.do(fresh, http.MethodPost, "/costs/reconcile", b, "Idempotency-Key", key))
	if res.Reconciled != 1 {
		e.t.Fatalf("after the step-up %+v", res)
	}
	e.settle()
}

// tokensRejected (TestReconcileCosts_TokensRejected, R-AZ-03): the app token with costs:reconcile,
// an agent token and the owner's personal token get 403 forbidden owner_only; no row, no job; one
// audit deny entry each.
func tokensRejected(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	b := body(from, to, r, 900_000, "USD")
	seq := e.auditSeq()
	e.wantProblem(e.bearer(token, http.MethodPost, "/costs/reconcile", b, "Idempotency-Key", ids.New().String()), http.StatusForbidden, "forbidden",
		"owner_only")
	e.wantProblem(e.do(e.lotse, http.MethodPost, "/costs/reconcile", b), http.StatusForbidden, "forbidden", "owner_only")
	personal := e.personal()
	e.wantProblem(e.do(personal, http.MethodPost, "/costs/reconcile", b), http.StatusForbidden, "forbidden", "owner_only")
	if n := len(e.entries(r)); n != 1 || e.jobsOf(r) != 0 {
		e.t.Fatalf("a refused token wrote: %d entries, %d jobs", n, e.jobsOf(r))
	}
	denials := e.waitAudits("cost.reconcile", "deny", seq, 3)
	actors := map[string]bool{}
	for _, d := range denials {
		if d.Reason != "owner_only" {
			e.t.Fatalf("denial %+v", d)
		}
		actors[d.Actor] = true
	}
	if len(denials) != 3 || len(actors) != 3 {
		e.t.Fatalf("denials %+v", denials)
	}
}

// personal returns a client with a personal API token of the owner (tokens.personal_enabled on).
func (e *env) personal() *harness.Client {
	e.t.Helper()
	if _, err := e.db.Exec(context.Background(), `INSERT INTO feature_flags (key, enabled) VALUES ('tokens.personal_enabled', true)
		ON CONFLICT (key) DO UPDATE SET enabled = true`); err != nil {
		e.t.Fatal(err)
	}
	return e.s.NewTokenClient(e.t, e.owner)
}

// submitByApp (TestSubmitCostReconciliation_App): the app with the scope gets 200; the audit allow
// entry has exactly the details of design note 3 h.
func submitByApp(e *env, token string) {
	r1 := e.run(e.a1(), usd(1_000_000))
	r2 := e.run(e.a4(), usd(2_000_000))
	from, to := e.day(r1, r2)
	seq := e.auditSeq()
	unknown := ids.New().String()
	res := e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r1, 1_250_000, "USD", unknown, 7, "USD", r2, 750_000,
		"USD"), "Idempotency-Key", ids.New().String()))
	if res.Reconciled != 2 || !slices.Equal(res.Unmatched, []string{unknown}) {
		e.t.Fatalf("result %+v", res)
	}
	allows := e.waitAudits("cost.reconcile.submit", "allow", seq, 1)
	d := allows[0].Details
	keys := detailKeys(d)
	slices.Sort(keys)
	want := []string{"entry_count", "period_end", "period_start", "reconciled_count", "total_micros_usd", "unmatched_count"}
	if !slices.Equal(keys, want) || num(d["entry_count"]) != 3 || num(d["reconciled_count"]) != 2 || num(d["unmatched_count"]) != 1 ||
		num(d["total_micros_usd"]) != 2_000_000 {
		e.t.Fatalf("audit details %v, want the keys %v", d, want)
	}
	e.settle()
}

// num reads a JSON number.
func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

// bothPathsUseSameJob (TestReconcile_BothPathsUseSameJob): one request per path inserts exactly
// one job billing.reconcile in its own transaction (a rolled back request inserts none, here a
// request that fails validation after its rows would have been decided); the totals change only
// after the job ran; both jobs run the same handler.
func bothPathsUseSameJob(e *env, token string) {
	r1 := e.run(e.a1(), usd(1_000_000))
	r2 := e.run(e.a2(), usd(1_000_000))
	from, to := e.day(r1, r2)
	calls := e.gate.callCount()
	e.gate.hold()
	e.decode(e.must(e.owner, http.MethodPost, "/costs/reconcile", body(from, to, r1, 400_000, "USD"), http.StatusOK))
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r2, 600_000, "USD"), "Idempotency-Key", ids.New().String()))
	// A request that is refused rolls back: no job.
	e.wantProblem(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r1, 1, "USD", r1, 2, "USD"), "Idempotency-Key",
		ids.New().String()), http.StatusBadRequest, "validation_failed", "")
	if a, b := e.jobsOf(r1), e.jobsOf(r2); a != 1 || b != 1 {
		e.release()
		e.t.Fatalf("jobs %d and %d, want one per request", a, b)
	}
	if c1, c2 := e.runCostOf(r1), e.runCostOf(r2); c1.Micros != 1_000_000 || c2.Micros != 1_000_000 {
		e.release()
		e.t.Fatalf("totals changed before the jobs: %+v %+v", c1, c2)
	}
	e.release()
	e.settle()
	if got := e.gate.callCount() - calls; got != 2 {
		e.t.Fatalf("%d jobs ran through the handler, want 2", got)
	}
	if c1, c2 := e.runCostOf(r1), e.runCostOf(r2); c1.Micros != 400_000 || c2.Micros != 600_000 {
		e.t.Fatalf("totals after the jobs: %+v %+v", c1, c2)
	}
}

// estimatesSuperseded (TestReconcile_BR10_EstimatesSuperseded): a run with two models and an
// entry on another day: every valid entry in the period gets exactly one reconciled successor with
// copied tokens; the valid sum in the period equals the amount, the remainder sits on the largest
// entry, the entry of the other day stays; getThreadCosts shows the new amount without estimate.
func estimatesSuperseded(e *env, token string) {
	r := e.run(e.a1(), usd(2_000_000), report{model: "model-small", amount: 1_000_000, currency: "USD", tokens: 7})
	from, to := e.day(r)
	es := e.entries(r)
	// The turn of another day: a copy of the first entry one day earlier.
	if _, err := e.db.Exec(context.Background(), `INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id,
		agent_session_id, billing_mode, provider, model, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, tokens_cache_write_1h,
		amount_micros, currency, is_estimate, source, occurred_at)
		SELECT $2, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id, billing_mode, provider, 'model-old', tokens_in, tokens_out,
		tokens_cache_read, tokens_cache_write, tokens_cache_write_1h, 500000, currency, true, source, occurred_at - interval '1 day'
		FROM cost_entries WHERE id = $1`, es[0].ID, ids.New().String()); err != nil {
		e.t.Fatal(err)
	}
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 1_000_001, "USD"), "Idempotency-Key", ids.New().String()))
	e.settle()
	all := e.entries(r)
	successors := map[string]entry{}
	for _, x := range all {
		if x.Supersedes != "" {
			if _, dup := successors[x.Supersedes]; dup {
				e.t.Fatalf("entry %s has two successors", x.Supersedes)
			}
			successors[x.Supersedes] = x
		}
	}
	var sum int64
	for _, old := range es {
		n, ok := successors[old.ID]
		if !ok || n.Source != "reconciled" || n.Estimate || n.Reconciled == nil || n.Tokens != old.Tokens || n.Model != old.Model {
			e.t.Fatalf("successor of %+v: %+v", old, n)
		}
		sum += n.Amount
	}
	// 1000001 over 2:1 is 666667 and 333333, the remainder 1 on the larger entry.
	if sum != 1_000_001 || successors[es[0].ID].Amount != 666_668 || successors[es[1].ID].Amount != 333_333 {
		e.t.Fatalf("shares %d and %d", successors[es[0].ID].Amount, successors[es[1].ID].Amount)
	}
	var outside int
	for _, x := range valid(all) {
		if x.Model == "model-old" {
			outside++
			if !x.Estimate || x.Occurred.After(from) {
				e.t.Fatalf("the entry of the other day changed: %+v", x)
			}
		}
	}
	if outside != 1 {
		e.t.Fatalf("%d valid entries of the other day", outside)
	}
	var costs struct {
		Runs []struct {
			RunID string `json:"run_id"`
			Costs []struct {
				AmountMicros int64 `json:"amount_micros"`
				IsEstimate   bool  `json:"is_estimate"`
			} `json:"costs"`
		} `json:"runs"`
	}
	e.must(e.alex, http.MethodGet, "/threads/"+es[0].Thread+"/costs", nil, http.StatusOK).JSON(e.t, &costs)
	if len(costs.Runs) != 1 || len(costs.Runs[0].Costs) != 1 || costs.Runs[0].Costs[0].AmountMicros != 1_500_001 || !costs.Runs[0].Costs[0].IsEstimate {
		e.t.Fatalf("thread costs %+v", costs)
	}
	if c := e.runCostOf(r); c.Micros != 1_500_001 || !c.Estimate {
		e.t.Fatalf("run %+v: the estimate of the other day stays", c)
	}
}

// onlyAddsReconciledEntries (TestReconcile_OnlyAddsReconciledEntries, ADR 0055 "Confirmation"):
// every row of cost_entries before the request is byte-equal afterwards; every new row is
// reconciled and points to an entry of a submitted run; bothy_app has no UPDATE or DELETE on
// cost_entries.
func onlyAddsReconciledEntries(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	snapshot := func() map[string]string {
		rows, err := e.db.Query(context.Background(), `SELECT id::text, to_jsonb(c)::text FROM cost_entries c`)
		if err != nil {
			e.t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var id, raw string
			if err := rows.Scan(&id, &raw); err != nil {
				e.t.Fatal(err)
			}
			out[id] = raw
		}
		return out
	}
	before := snapshot()
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 2_000_000, "USD"), "Idempotency-Key", ids.New().String()))
	e.settle()
	after := snapshot()
	for id, raw := range before {
		if after[id] != raw {
			e.t.Fatalf("entry %s changed:\n%s\n%s", id, raw, after[id])
		}
	}
	for id := range after {
		if _, ok := before[id]; ok {
			continue
		}
		var src, run, target string
		if err := e.db.QueryRow(context.Background(), `SELECT c.source, c.run_id::text, s.run_id::text FROM cost_entries c
			JOIN cost_entries s ON s.id = c.supersedes_id WHERE c.id = $1`, id).Scan(&src, &run, &target); err != nil {
			e.t.Fatal(err)
		}
		if src != "reconciled" || run != r || target != r {
			e.t.Fatalf("new entry %s: %s of %s superseding an entry of %s", id, src, run, target)
		}
	}
	var update, del, colUpdate bool
	if err := e.db.QueryRow(context.Background(), `SELECT has_table_privilege('bothy_app', 'cost_entries', 'UPDATE'),
		has_table_privilege('bothy_app', 'cost_entries', 'DELETE'), has_any_column_privilege('bothy_app', 'cost_entries', 'UPDATE')`).
		Scan(&update, &del, &colUpdate); err != nil {
		e.t.Fatal(err)
	}
	if update || del || colUpdate {
		e.t.Fatalf("bothy_app may update %v, delete %v, update a column %v", update, del, colUpdate)
	}
}

// unmatched (TestReconcile_Unmatched): an unknown run, a run without entries in the period, the
// personal run of A3, a run of another provider and an entry in another currency are listed in
// request order; reconciled plus unmatched equals the entries; nothing is written for them.
func unmatched(e *env, token string) {
	matched := e.run(e.a1(), usd(1_000_000))
	early := e.run(e.a2(), usd(1_000_000))
	personal := e.run(e.a3(), usd(1_000_000))
	other := e.run(e.a4(), usd(1_000_000))
	if _, err := e.db.Exec(context.Background(), `UPDATE cost_entries SET provider = 'other' WHERE run_id = $1`, other); err != nil {
		e.t.Fatal(err)
	}
	euro := e.run(e.a1(), report{model: "model-large", amount: 1_000_000, currency: "EUR", tokens: 1})
	from, to := e.day(matched, early, personal, other, euro)
	unknown := ids.New().String()
	// The run early has no entry in a period of the day before.
	res := e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from.Add(-24*time.Hour), from, early, 1, "USD"), "Idempotency-Key",
		ids.New().String()))
	if res.Reconciled != 0 || !slices.Equal(res.Unmatched, []string{early}) {
		e.t.Fatalf("result of the other period %+v", res)
	}
	res = e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, unknown, 1, "USD", personal, 2, "USD", matched, 3, "USD",
		other, 4, "USD", euro, 5, "USD"), "Idempotency-Key", ids.New().String()))
	if res.Reconciled != 1 || !slices.Equal(res.Unmatched, []string{unknown, personal, other, euro}) || res.Reconciled+len(res.Unmatched) != 5 {
		e.t.Fatalf("result %+v", res)
	}
	for _, run := range []string{early, personal, other, euro} {
		if n := len(e.entries(run)); n != 1 || e.jobsOf(run) != 0 {
			e.t.Fatalf("run %s: %d entries, %d jobs", run, n, e.jobsOf(run))
		}
	}
	e.settle()
}

// resubmitIsNoop (TestReconcile_ResubmitSameAmountIsNoop): the same body under a new key writes no
// row and no job and still counts the run; another amount supersedes the reconciled rows (chain
// estimate, reconciled, reconciled) and the job brings the totals to it.
func resubmitIsNoop(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	b := body(from, to, r, 800_000, "USD")
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", b, "Idempotency-Key", ids.New().String()))
	e.settle()
	jobs := e.jobsOf(r)
	res := e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", b, "Idempotency-Key", ids.New().String()))
	if res.Reconciled != 1 || len(e.entries(r)) != 2 || e.jobsOf(r) != jobs {
		e.t.Fatalf("the repeat wrote: %+v, %d entries, %d jobs", res, len(e.entries(r)), e.jobsOf(r))
	}
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 700_000, "USD"), "Idempotency-Key", ids.New().String()))
	e.settle()
	es := e.entries(r)
	if len(es) != 3 || es[1].Supersedes != es[0].ID || es[2].Supersedes != es[1].ID || es[2].Amount != 700_000 {
		e.t.Fatalf("chain %+v", es)
	}
	if c := e.runCostOf(r); c.Micros != 700_000 || c.Estimate {
		e.t.Fatalf("run %+v", c)
	}
}

// concurrentSubmissions (TestReconcile_ConcurrentSubmissions): two parallel requests for the same
// run with different amounts both end with 200; every replaced entry has at most one successor;
// the valid sum equals the amount of the request that committed last; after both jobs the run
// totals equal the valid sum.
func concurrentSubmissions(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000), report{model: "model-small", amount: 500_000, currency: "USD", tokens: 3})
	from, to := e.day(r)
	results := make(chan harness.Response, 2)
	for _, amount := range []int64{600_000, 900_000} {
		go func() {
			results <- e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, amount, "USD"), "Idempotency-Key", ids.New().String())
		}()
	}
	for range 2 {
		if resp := testkit.Receive(e.t, "a reconciliation", results); resp.Status != http.StatusOK {
			e.t.Fatalf("status %d %s", resp.Status, resp.Body)
		}
	}
	e.settle()
	es := e.entries(r)
	seen := map[string]bool{}
	for _, x := range es {
		if x.Supersedes != "" {
			if seen[x.Supersedes] {
				e.t.Fatalf("entry %s has two successors", x.Supersedes)
			}
			seen[x.Supersedes] = true
		}
	}
	var sum int64
	var last time.Time
	for _, x := range valid(es) {
		sum += x.Amount
		if x.Reconciled != nil && x.Reconciled.After(last) {
			last = *x.Reconciled
		}
	}
	if sum != 600_000 && sum != 900_000 {
		e.t.Fatalf("valid sum %d", sum)
	}
	// The later request superseded the rows of the earlier one, so its rows are the valid ones.
	if len(es) != 6 {
		e.t.Fatalf("%d entries, want 2 estimates and two generations of 2", len(es))
	}
	if c := e.runCostOf(r); c.Micros != sum || c.Estimate {
		e.t.Fatalf("run %+v, valid sum %d", c, sum)
	}
}

// jobUpdatesRunsAndSessions (TestReconcile_JobUpdatesRunsAndSessions): after the job the version
// of a changed run rose by one and that of an unchanged run not; the session sum equals the valid
// sum of the session, cost_is_estimate is false when all is reconciled, its version rose by one and
// cost_baseline_micros stayed.
func jobUpdatesRunsAndSessions(e *env, token string) {
	changed := e.run(e.a1().withSession(), usd(1_000_000))
	same := e.run(e.a2(), usd(1_000_000))
	from, to := e.day(changed, same)
	sBefore := e.sessionOf(changed)
	rcBefore, rsBefore := e.runCostOf(changed), e.runCostOf(same)
	if sBefore.Micros != 1_000_000 || !sBefore.Estimate {
		e.t.Fatalf("session before %+v", sBefore)
	}
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, changed, 1_200_000, "USD"), "Idempotency-Key",
		ids.New().String()))
	e.settle()
	// The same amount as the estimate: the run's totals only lose the estimate flag.
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, same, 1_000_000, "USD"), "Idempotency-Key", ids.New().String()))
	e.settle()
	if c := e.runCostOf(changed); c.Micros != 1_200_000 || c.Estimate || c.Version != rcBefore.Version+1 {
		e.t.Fatalf("changed run %+v, before %+v", c, rcBefore)
	}
	if c := e.runCostOf(same); c.Micros != 1_000_000 || c.Estimate || c.Version != rsBefore.Version+1 {
		e.t.Fatalf("run with the same amount %+v, before %+v", c, rsBefore)
	}
	s := e.sessionOf(changed)
	if s.Micros != 1_200_000 || s.Estimate || s.Version != sBefore.Version+1 || s.Baseline != sBefore.Baseline {
		e.t.Fatalf("session %+v, before %+v", s, sBefore)
	}
}

// jobIsIdempotent (TestReconcile_JobIsIdempotent): running the same job again changes no row and
// no version.
func jobIsIdempotent(e *env, token string) {
	r := e.run(e.a1().withSession(), usd(1_000_000))
	from, to := e.day(r)
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 1_100_000, "USD"), "Idempotency-Key", ids.New().String()))
	e.settle()
	rc := e.runCostOf(r)
	sc := e.sessionOf(r)
	// The handler of the job once more with the same arguments.
	run := ids.MustParse(r)
	if err := (billingjobs.Handler{Applier: e.gate.inner}).Work(context.Background(), jobs.Job[billingjobs.Reconcile]{
		Args: billingjobs.Reconcile{RunIDs: []ids.ID{run}}}); err != nil {
		e.t.Fatal(err)
	}
	e.settle()
	if got := e.runCostOf(r); got != rc {
		e.t.Fatalf("run %+v, before %+v", got, rc)
	}
	if got := e.sessionOf(r); got != sc {
		e.t.Fatalf("session %+v, before %+v", got, sc)
	}
}

// validation (TestReconcile_Validation): the 400 validation_failed cases of design notes 3 b and
// 10 with their pointers and codes; nothing written, no job.
func validation(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	cases := []struct {
		name string
		body map[string]any
		want map[string]string
	}{
		{"period end equal", body(from, from, r, 1, "USD"), map[string]string{"/period_end": "invalid"}},
		{"period end before", body(to, from, r, 1, "USD"), map[string]string{"/period_end": "invalid"}},
		{"repeated run", body(from, to, r, 1, "USD", r, 2, "USD"), map[string]string{"/entries/1/run_id": "invalid"}},
		{"negative amount", body(from, to, r, -1, "USD"), map[string]string{"/entries/0/amount_micros": "too_small"}},
		{"currency", body(from, to, r, 1, "usd"), map[string]string{"/entries/0/currency": "pattern_mismatch"}},
	}
	empty := body(from, to, r, 1, "USD")
	empty["provider"] = ""
	cases = append(cases, struct {
		name string
		body map[string]any
		want map[string]string
	}{"empty provider", empty, map[string]string{"/provider": "too_short"}})
	for _, c := range cases {
		p := e.wantProblem(e.bearer(token, http.MethodPost, "/costs/reconciliations", c.body, "Idempotency-Key", ids.New().String()),
			http.StatusBadRequest, "validation_failed", "")
		got := map[string]string{}
		for _, f := range p.Errors {
			got[f.Pointer] = f.Code
		}
		for ptr, code := range c.want {
			if got[ptr] != code {
				e.t.Fatalf("%s: errors %v, want %s %s", c.name, got, ptr, code)
			}
		}
	}
	if n := len(e.entries(r)); n != 1 || e.jobsOf(r) != 0 {
		e.t.Fatalf("a refused body wrote: %d entries, %d jobs", n, e.jobsOf(r))
	}
}

// idempotencyKey (TestReconcile_IdempotencyKey): a replay returns the stored answer with
// Idempotent-Replay true and no new rows or job; another body under the same key gives 422
// idempotency_key_reuse.
func idempotencyKey(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	key := ids.New().String()
	first := e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 500_000, "USD"), "Idempotency-Key", key)
	e.decode(first)
	e.settle()
	entries, jobs := len(e.entries(r)), e.jobsOf(r)
	replay := e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 500_000, "USD"), "Idempotency-Key", key)
	if replay.Status != http.StatusOK || replay.Header.Get("Idempotent-Replay") != "true" || !reflect.DeepEqual(e.decode(replay), e.decode(first)) {
		e.t.Fatalf("replay %d %v %s", replay.Status, replay.Header, replay.Body)
	}
	if len(e.entries(r)) != entries || e.jobsOf(r) != jobs {
		e.t.Fatal("the replay wrote")
	}
	e.wantProblem(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 400_000, "USD"), "Idempotency-Key", key),
		http.StatusUnprocessableEntity, "idempotency_key_reuse", "")
}

// unknownMemberCanary (TestReconcile_UnknownMemberCanary): a body with an extra member holding a
// canary gives 400 unknown_member; the canary appears in no table, no server log and no stored
// idempotency answer.
func unknownMemberCanary(e *env, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	canary := "canary" + randomAlnum(e.t, 24)
	b := body(from, to, r, 1, "USD")
	b["admin_key"] = canary
	p := e.wantProblem(e.bearer(token, http.MethodPost, "/costs/reconciliations", b, "Idempotency-Key", ids.New().String()), http.StatusBadRequest,
		"validation_failed", "")
	if len(p.Errors) == 0 || p.Errors[0].Code != "unknown_member" {
		e.t.Fatalf("errors %+v", p.Errors)
	}
	wantScanFinds(e.t, e.db, r)
	if found := scanDatabase(e.t, e.db, canary); len(found) > 0 {
		e.t.Fatalf("the canary is stored in %v", found)
	}
	if strings.Contains(e.logs.String(), canary) {
		e.t.Fatal("the canary is in the server log")
	}
}

// estimatesRead (TestCostEstimates_Read, R-AZ-28): the app with the scope and an owner without a
// fresh step-up read the estimates of A1 and A2 for one day with paging; personal runs, mixed
// currencies and runs outside the range are absent; no item names a channel or thread; the estimate
// stays the original after a reconciliation and reconciled turns true; allowed reads write no audit
// entry; an agent, the owner's personal token and Sam get 403 owner_only, each denial audited.
func estimatesRead(e *env, token string) {
	r1 := e.run(e.a1(), usd(1_000_000))
	r2 := e.run(e.a2(), usd(2_000_000))
	mixed := e.run(e.a1(), usd(1_000_000), report{model: "model-small", amount: 1, currency: "EUR", tokens: 1})
	personal := e.run(e.a3(), usd(5_000_000))
	// The runs of this case on a day of their own, ten days back, so no booking of another case
	// lies in it or in the day before it, also across midnight (test setup, as superuser).
	from := e.day1(r1).Add(-10 * 24 * time.Hour)
	to := from.Add(24 * time.Hour)
	if _, err := e.db.Exec(context.Background(), `UPDATE cost_entries SET occurred_at = $1 WHERE run_id = ANY ($2::uuid[])`,
		from.Add(12*time.Hour), []string{r1, r2, mixed, personal}); err != nil {
		e.t.Fatal(err)
	}
	agents := []string{e.lotse.PrincipalID, e.texter.PrincipalID, e.assistent.PrincipalID}
	query := func(extra string) string {
		q := "/costs/estimates?from=" + stamp(from) + "&to=" + stamp(to) + extra
		for _, a := range agents {
			q += "&agent_id=" + a
		}
		return q
	}
	seq := e.auditSeq()
	items := e.estimates(func(path string) harness.Response { return e.bearer(token, http.MethodGet, path, nil) }, query("&limit=1"))
	byRun := map[string]map[string]any{}
	for _, it := range items {
		for k := range it {
			if k == "channel_id" || k == "thread_id" || k == "space_id" || k == "content" {
				e.t.Fatalf("item names %s: %v", k, it)
			}
		}
		byRun[it["run_id"].(string)] = it
	}
	if byRun[r1] == nil || byRun[r2] == nil || byRun[mixed] != nil || byRun[personal] != nil {
		e.t.Fatalf("items %v", items)
	}
	if num(byRun[r2]["estimate_micros"]) != 2_000_000 || byRun[r2]["reconciled"] != false || byRun[r2]["currency"] != "USD" ||
		byRun[r2]["provider"] != provider || byRun[r2]["agent_id"] != e.texter.PrincipalID {
		e.t.Fatalf("item %v", byRun[r2])
	}
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r2, 3_000_000, "USD"), "Idempotency-Key", ids.New().String()))
	e.settle()
	fresh, err := e.s.OwnerSession(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	items = e.estimates(func(path string) harness.Response { return e.do(fresh, http.MethodGet, path, nil) }, query(""))
	for _, it := range items {
		if it["run_id"] == r2 && (num(it["estimate_micros"]) != 2_000_000 || it["reconciled"] != true) {
			e.t.Fatalf("after the reconciliation %v", it)
		}
	}
	// Outside the range: nothing of the day before.
	before := "/costs/estimates?from=" + stamp(from.Add(-24*time.Hour)) + "&to=" + stamp(from) + "&agent_id=" + e.texter.PrincipalID +
		"&agent_id=" + e.lotse.PrincipalID
	if got := e.estimates(func(path string) harness.Response { return e.bearer(token, http.MethodGet, path, nil) }, before); len(got) != 0 {
		e.t.Fatalf("items of the day before %v", got)
	}
	if allowed := e.audits("cost.reconcile.read", seq); len(allowed) != 0 {
		e.t.Fatalf("allowed reads audited: %+v", allowed)
	}
	e.wantProblem(e.do(e.lotse, http.MethodGet, query(""), nil), http.StatusForbidden, "forbidden", "owner_only")
	e.wantProblem(e.do(e.personal(), http.MethodGet, query(""), nil), http.StatusForbidden, "forbidden", "owner_only")
	e.wantProblem(e.do(e.sam, http.MethodGet, query(""), nil), http.StatusForbidden, "forbidden", "owner_only")
	e.waitAudits("cost.reconcile.read", "deny", seq, 3)
}

// estimates follows the cursor of listCostEstimates from path with get and returns every item.
func (e *env) estimates(get func(path string) harness.Response, path string) []map[string]any {
	e.t.Helper()
	var out []map[string]any
	next := path
	for range 100 {
		r := get(next)
		if r.Status != http.StatusOK {
			e.t.Fatalf("listCostEstimates %d %s", r.Status, r.Body)
		}
		var page struct {
			Items      []map[string]any `json:"items"`
			NextCursor *string          `json:"next_cursor"`
		}
		r.JSON(e.t, &page)
		out = append(out, page.Items...)
		if page.NextCursor == nil {
			return out
		}
		next = path + "&cursor=" + *page.NextCursor
	}
	e.t.Fatal("more than 100 pages")
	return nil
}

// scopeMissing (TestSubmitCostReconciliation_ScopeMissing): after setAppScopes without
// costs:reconcile the app gets 403 scope_missing and nothing is written; the estimates read too.
func scopeMissing(e *env, app, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	e.must(e.owner, http.MethodPut, "/apps/"+app+"/scopes", map[string]any{"granted_scopes": []string{}, "reason": "Pause the reconciliation"},
		http.StatusOK)
	e.wantProblem(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 1, "USD"), "Idempotency-Key", ids.New().String()),
		http.StatusForbidden, "scope_missing", "scope_missing")
	e.wantProblem(e.bearer(token, http.MethodGet, "/costs/estimates?from="+stamp(from)+"&to="+stamp(to)+"&agent_id="+e.lotse.PrincipalID, nil),
		http.StatusForbidden, "scope_missing", "scope_missing")
	if n := len(e.entries(r)); n != 1 || e.jobsOf(r) != 0 {
		e.t.Fatalf("written without the scope: %d entries", n)
	}
	e.must(e.owner, http.MethodPut, "/apps/"+app+"/scopes", map[string]any{"granted_scopes": []string{"costs:reconcile"}, "reason": "Resume"},
		http.StatusOK)
}

// peopleAndAgentsRejected (TestSubmitCostReconciliation_PeopleAndAgentsRejected): the owner's
// session and an agent token get 403 subject_kind_not_allowed; an uninstalled app gets 401.
func peopleAndAgentsRejected(e *env, app, token string) {
	r := e.run(e.a1(), usd(1_000_000))
	from, to := e.day(r)
	b := body(from, to, r, 1, "USD")
	e.wantProblem(e.do(e.owner, http.MethodPost, "/costs/reconciliations", b), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	e.wantProblem(e.do(e.lotse, http.MethodPost, "/costs/reconciliations", b), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	e.must(e.owner, http.MethodPost, "/apps/"+app+"/uninstall", map[string]any{"reason": "Done"}, http.StatusNoContent)
	if r := e.bearer(token, http.MethodPost, "/costs/reconciliations", b, "Idempotency-Key", ids.New().String()); r.Status != http.StatusUnauthorized {
		e.t.Fatalf("uninstalled app: %d %s", r.Status, r.Body)
	}
	if r := e.bearer(token, http.MethodGet, "/costs/estimates?from="+stamp(from)+"&to="+stamp(to)+"&agent_id="+e.lotse.PrincipalID, nil); r.Status !=
		http.StatusUnauthorized {
		e.t.Fatalf("uninstalled app reads: %d %s", r.Status, r.Body)
	}
	if n := len(e.entries(r)); n != 1 {
		e.t.Fatalf("written: %d entries", n)
	}
}

// jobAgainstConcurrentBooking (red team M2-28, the class "the job writes a stale session total"):
// a booking of the session that is in flight while the job runs holds the shared session lock, so
// the job waits for it and its sums include the booking; afterwards the run and session totals
// equal the valid sums. The booking is simulated in a transaction of the test that does what
// BookRun and AddCost do (the shared lock, an estimate entry of the run on another day, the session
// sum), because the real booking cannot be paused between its statements.
func jobAgainstConcurrentBooking(e *env, token string) {
	r := e.run(e.a1().withSession(), usd(1_000_000))
	from, to := e.day(r)
	session := e.entries(r)[0].Session
	e.gate.hold()
	e.decode(e.bearer(token, http.MethodPost, "/costs/reconciliations", body(from, to, r, 1_200_000, "USD"), "Idempotency-Key", ids.New().String()))
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	if err != nil {
		e.release()
		e.t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, stmt := range []string{
		`SELECT pg_advisory_xact_lock_shared(hashtextextended('billing.session:' || $1::uuid::text, 0))`,
		`INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id, billing_mode, provider, model,
			amount_micros, currency, is_estimate, source, occurred_at)
			SELECT $2, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id, billing_mode, provider, 'model-next', 300000, currency,
			true, 'adapter_reported', occurred_at + interval '1 day' FROM cost_entries WHERE agent_session_id = $1 ORDER BY id LIMIT 1`,
		`UPDATE agent_sessions SET cumulative_cost_micros = cumulative_cost_micros + 300000, cost_is_estimate = true, version = version + 1
			WHERE id = $1`,
	} {
		args := []any{session}
		if strings.Contains(stmt, "$2") {
			args = append(args, ids.New().String())
		}
		if _, err := tx.Exec(ctx, stmt, args...); err != nil {
			e.release()
			e.t.Fatal(err)
		}
	}
	e.release()
	// The job waits for the booking: on the session lock (or, without it, on the session row).
	testkit.Until(e.t, "the job waits for the booking", func() bool {
		return e.count(`SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
			WHERE NOT l.granted AND (a.query LIKE '%billing.session:%' OR a.query LIKE '%agent_sessions%')`) > 0
	})
	if err := tx.Commit(ctx); err != nil {
		e.t.Fatal(err)
	}
	e.settle()
	var sum int64
	for _, x := range valid(e.entries(r)) {
		sum += x.Amount
	}
	if sum != 1_500_000 {
		e.t.Fatalf("valid sum of the run %d", sum)
	}
	if c := e.runCostOf(r); c.Micros != sum || !c.Estimate {
		e.t.Fatalf("run %+v, valid sum %d with an estimate", c, sum)
	}
	if s := e.sessionOf(r); s.Micros != sum || !s.Estimate {
		e.t.Fatalf("session %+v, valid sum %d with an estimate", s, sum)
	}
}
