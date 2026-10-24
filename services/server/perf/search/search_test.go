// SPDX-License-Identifier: Apache-2.0

//go:build perf

package search_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
	"github.com/OWNER/bothy/services/server/perf/internal/searchworld"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// test is the name of the results of this package in results.json (brief M3-16 design note 13).
const test = "perf/search"

// maxPrinted is how many failed samples the test prints; it counts all of them.
const maxPrinted = 10

// TestPerf_Search measures search.p95_target and search.prefix_p95_target (brief M3-16 design notes
// 7 to 13): seeding process with raised rate limits, Seed, stop, bulk load, measured process with
// the default configuration on the same durable database, product checks, phases messages, prefix
// and probe, the writing product checks, stop, phase rewrite. A p95 counts only when every answer
// of the run passed its checks (design note 10).
func TestPerf_Search(t *testing.T) {
	ctx := t.Context()
	env := perfkit.Setup(t)
	thMessages := env.Threshold(t, "search.p95_target")
	thPrefix := env.Threshold(t, "search.prefix_p95_target")
	n := env.Count(t, searchbench.KeyBenchmarkMessages)
	d, err := searchbench.LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := searchbench.NewPlan(searchbench.DefaultParams(), n, d)
	if err != nil {
		t.Fatal(err)
	}
	steps := map[string]float64{}
	step := func(name string, start time.Time) {
		steps[name+"_seconds"] = time.Since(start).Seconds()
		t.Logf("step %s: %s", name, time.Since(start).Round(time.Second))
	}
	failed := 0

	t0 := time.Now()
	p := env.StartSeeding(t, nil)
	db := p.AdminDB()
	w, err := searchworld.Seed(ctx, p.Owner, p.CreateUser, plan)
	if err != nil {
		t.Fatal(err)
	}
	stop(t, p)
	step("seed", t0)

	t0 = time.Now()
	var loadTime time.Time
	if err := db.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&loadTime); err != nil {
		t.Fatal(err)
	}
	corpus, err := searchbench.NewCorpus(plan, loadTime)
	if err != nil {
		t.Fatal(err)
	}
	base, err := searchbench.MainBase(ctx, db, w.IDs())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := searchbench.BuildRows(corpus, w.IDs(), base)
	if err != nil {
		t.Fatal(err)
	}
	load, err := searchbench.Load(ctx, db, rows)
	if err != nil {
		t.Fatal(err)
	}
	step("load", t0)

	t0 = time.Now()
	env.Measure(t, p, nil)
	everyone := append(append([]searchworld.Person{}, w.Authors...), w.Searchers...)
	if err := searchworld.Refresh(ctx, everyone); err != nil {
		t.Fatal(err)
	}
	if err := searchworld.CheckThreads(ctx, w, rows, plan.ConsistencySamples); err != nil {
		t.Fatal(err)
	}
	step("product_checks", t0)

	all := make([]int, plan.Searchers)
	for k := range all {
		all[k] = k
	}
	t0 = time.Now()
	msgGens := make([]*searchbench.Queries, plan.Searchers)
	for k := range msgGens {
		msgGens[k] = searchbench.NewQueries(corpus, k, false)
	}
	messages := runPhase(t, w, "messages", all, plan, func(k int) searchbench.Query { return msgGens[k].Next() },
		func(q searchbench.Query, a searchbench.Answer) (bool, error) {
			return searchbench.CheckSearch("messages", q, a, w.Visible(q.Searcher))
		})
	step("phase_messages", t0)

	t0 = time.Now()
	prefixGens := make([]*searchbench.Queries, plan.Searchers)
	for k := range prefixGens {
		prefixGens[k] = searchbench.NewQueries(corpus, k, true)
	}
	prefix := runPhase(t, w, "prefix", all, plan, func(k int) searchbench.Query {
		q, err := prefixGens[k].NextPrefix()
		if err != nil {
			t.Fatal(err)
		}
		return q
	}, func(q searchbench.Query, a searchbench.Answer) (bool, error) {
		return false, searchbench.CheckPrefix("prefix", q, a, w.Visible(q.Searcher))
	})
	step("phase_prefix", t0)

	t0 = time.Now()
	probe := runProbe(t, w, corpus)
	step("phase_probe", t0)

	t0 = time.Now()
	if _, err := searchworld.CheckWrites(ctx, w, rows, db); err != nil {
		t.Errorf("writing product checks: %v", err)
		failed++
	}
	stop(t, p)
	step("product_writes", t0)

	t0 = time.Now()
	sizes, err := searchbench.MeasureSizes(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	var shared, workMem string
	if err := db.QueryRow(ctx, `SELECT current_setting('shared_buffers'), current_setting('work_mem')`).Scan(&shared, &workMem); err != nil {
		t.Fatal(err)
	}
	rewrite, err := searchbench.MeasureRewrite(ctx, db, d.Languages, corpus.FixedPhrase())
	if err != nil {
		t.Errorf("phase rewrite: %v", err)
		failed++
	}
	step("phase_rewrite", t0)

	extra := map[string]any{
		"search.benchmark_messages": n, "corpus": plan.Map(), "load": load.Map(), "params": plan.Params.Map(), "defaults": d.Map(),
		"steps": steps, "gomaxprocs": env.ServerProcs(), "sizes": sizes.Map(n), "rewrite": rewrite.Map(), "probe": probe.extra,
		"postgres": map[string]any{"shared_buffers": shared, "work_mem": workMem, "maintenance_work_mem": rewrite.MaintenanceWorkMem},
	}
	var results []report.Result
	for _, ph := range []struct {
		phase *phaseResult
		th    report.Threshold
	}{{messages, thMessages}, {prefix, thPrefix}} {
		failed += ph.phase.failed
		extra["phase_"+ph.phase.name] = ph.phase.extra
		results = append(results, ph.phase.result(ph.th, n))
	}
	failed += probe.failed
	for i := range results {
		// A measurement counts only when the path behaved correctly while it was taken (design
		// note 10): one failure anywhere in the run fails both targets.
		results[i].Pass = results[i].Pass && failed == 0
	}
	if failed > 0 {
		t.Errorf("%d checks failed; neither target counts", failed)
	}
	env.Record(t, results, map[string]any{"search": extra})
}

// stop ends the server process p (SIGTERM by PID) and waits until it has exited, so no request
// runs while the database is written directly.
func stop(t *testing.T, p *harness.Process) {
	t.Helper()
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Exited():
	case <-t.Context().Done():
		t.Fatal("cancelled while the server stopped")
	}
}

// phaseResult is the outcome of phase messages or prefix.
type phaseResult struct {
	name   string
	stats  searchbench.Stats
	ok     bool
	failed int
	extra  map[string]any
}

// result returns the result of th for the phase; a phase without enough samples fails.
func (r *phaseResult) result(th report.Threshold, n int) report.Result {
	res := th.Result(test, r.stats.P95, r.stats.All.Count)
	res.Pass = res.Pass && r.ok
	res.Note = fmt.Sprintf("corpus %d messages (%s)", n, searchbench.KeyBenchmarkMessages)
	return res
}

// runPhase runs one phase of design note 7 for the searchers: open loop at plan.Rate scaled to
// the searchers, request i from searcher searchers[i mod len] with the next query of its
// generator, Warmup without samples, Window with samples, then up to Drain for open requests (a
// request still open then fails). Every answer goes through check; failures are printed and
// counted, never with content.
func runPhase(t *testing.T, w *searchworld.World, name string, searchers []int, plan *searchbench.Plan,
	next func(k int) searchbench.Query, check func(searchbench.Query, searchbench.Answer) (bool, error)) *phaseResult {
	t.Helper()
	if err := searchworld.Refresh(t.Context(), w.Searchers); err != nil {
		t.Fatal(err)
	}
	rate := plan.SearcherRate * float64(len(searchers))
	total := int(rate * (plan.Warmup + plan.Window).Seconds())
	queries := make([]searchbench.Query, total)
	for i := range queries {
		queries[i] = next(searchers[i%len(searchers)])
	}
	samples := make([]searchbench.Sample, total)
	ctx, cancel := context.WithTimeout(t.Context(), plan.Warmup+plan.Window+plan.Drain+time.Second)
	defer cancel()
	start, sent := perfkit.Schedule(ctx, rate, plan.Warmup, plan.Window, func(i int, due time.Time) {
		samples[i] = send(ctx, w, name, queries[i], due, check)
	})
	samples = samples[:sent]
	from, to := start.Add(plan.Warmup), start.Add(plan.Warmup+plan.Window)
	res := &phaseResult{name: name, ok: true}
	res.failed = printFailures(t, name, samples)
	in := searchbench.InWindow(samples, from, to)
	st, err := searchbench.StatsOf(in, plan.MinSamples)
	if err != nil {
		t.Errorf("phase %s: %v", name, err)
		res.ok = false
		res.failed++
	}
	res.stats = st
	res.extra = st.Map()
	res.extra["achieved_requests_per_s"] = float64(len(in)) / plan.Window.Seconds()
	res.extra["planned_requests_per_s"] = rate
	res.extra["sent"] = sent
	res.extra["failed"] = res.failed
	t.Logf("phase %s: %d samples, p95 %.1f ms, p99 %.1f ms, max %.1f ms, %d failed", name, st.All.Count, st.All.P95, st.All.P99, st.All.Max, res.failed)
	return res
}

// send sends q as its searcher at its due time and returns the sample: the latency runs from due
// to the fully read body (harness Client.Do reads it whole).
func send(ctx context.Context, w *searchworld.World, phase string, q searchbench.Query, due time.Time,
	check func(searchbench.Query, searchbench.Answer) (bool, error)) searchbench.Sample {
	path := searchworld.SearchPath(q.Text)
	if q.Class == searchbench.ClassPrefix {
		path = searchworld.PrefixPath(q.Text)
	}
	resp := w.Searchers[q.Searcher].Client.Do(ctx, harness.Request{Method: http.MethodGet, Path: path})
	s := searchbench.Sample{Phase: phase, Query: q, Scheduled: due, Done: time.Now()}
	s.Capped, s.Err = check(q, searchbench.Answer{Status: resp.Status, Body: resp.Body})
	return s
}

// printFailures prints up to maxPrinted failures of samples and returns their count.
func printFailures(t *testing.T, phase string, samples []searchbench.Sample) int {
	t.Helper()
	fails := searchbench.Failures(samples)
	for i, err := range fails {
		if i == maxPrinted {
			t.Errorf("phase %s: %d more failures", phase, len(fails)-maxPrinted)
			break
		}
		t.Error(err)
	}
	return len(fails)
}

// probeResult is the outcome of phase probe.
type probeResult struct {
	failed int
	extra  map[string]any
}

// runProbe runs phase probe (design note 11): the searchers without the probe space send, at the
// rate of phase messages, the probe term and the absent term alternating; ProbeWarmup of each term
// are unmeasured, then ProbeQueries of each are measured. Every answer must be the empty page;
// extra records p50 and p95 of both terms and their differences, without a threshold.
func runProbe(t *testing.T, w *searchworld.World, c *searchbench.Corpus) probeResult {
	t.Helper()
	plan := c.Plan
	if err := searchworld.Refresh(t.Context(), w.Searchers); err != nil {
		t.Fatal(err)
	}
	searchers := plan.ProbeSearchers
	var queries []searchbench.Query
	var measured []bool
	count := [2]int{}
	for i := 0; count[0] < plan.ProbeWarmup+plan.ProbeQueries || count[1] < plan.ProbeWarmup+plan.ProbeQueries; i++ {
		k := searchers[i%len(searchers)]
		term := (i/len(searchers) + i) % 2
		if count[term] >= plan.ProbeWarmup+plan.ProbeQueries {
			term = 1 - term
		}
		queries = append(queries, c.ProbeQuery(k, term == 1))
		measured = append(measured, count[term] >= plan.ProbeWarmup)
		count[term]++
	}
	rate := plan.SearcherRate * float64(len(searchers))
	window := time.Duration((float64(len(queries)) + 0.5) / rate * float64(time.Second))
	samples := make([]searchbench.Sample, len(queries))
	ctx, cancel := context.WithTimeout(t.Context(), window+plan.Drain+time.Second)
	defer cancel()
	_, sent := perfkit.Schedule(ctx, rate, 0, window, func(i int, due time.Time) {
		if i < len(queries) {
			samples[i] = send(ctx, w, "probe", queries[i], due, func(q searchbench.Query, a searchbench.Answer) (bool, error) {
				return false, searchbench.CheckProbe("probe", q, a)
			})
		}
	})
	samples = samples[:min(sent, len(queries))]
	res := probeResult{failed: printFailures(t, "probe", samples)}
	if len(samples) < len(queries) {
		t.Errorf("phase probe: %d of %d queries sent", len(samples), len(queries))
		res.failed++
	}
	byTerm := map[searchbench.Class][]time.Duration{}
	for i, s := range samples {
		if measured[i] {
			byTerm[s.Query.Class] = append(byTerm[s.Query.Class], s.Latency())
		}
	}
	ms := func(xs []time.Duration, p float64) float64 {
		v, err := report.DurationsP(xs, p)
		if err != nil {
			return 0
		}
		return v / float64(time.Millisecond)
	}
	probe, absent := byTerm[searchbench.ClassProbe], byTerm[searchbench.ClassAbsent]
	res.extra = map[string]any{
		"searchers": len(searchers), "measured_per_term": map[string]int{"probe": len(probe), "absent": len(absent)},
		"probe_p50_ms": ms(probe, 50), "probe_p95_ms": ms(probe, 95), "absent_p50_ms": ms(absent, 50), "absent_p95_ms": ms(absent, 95),
		"difference_p50_ms": ms(probe, 50) - ms(absent, 50), "difference_p95_ms": ms(probe, 95) - ms(absent, 95), "failed": res.failed,
	}
	t.Logf("phase probe: probe p50 %.1f ms p95 %.1f ms, absent p50 %.1f ms p95 %.1f ms", ms(probe, 50), ms(probe, 95), ms(absent, 50), ms(absent, 95))
	return res
}
