// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	runsmetrics "github.com/OWNER/bothy/services/server/internal/runs/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// addSteps inserts n steps of the run as fixtures (steps of adapters come with M2-10).
func (w *world) addSteps(run string, n int) []string {
	w.t.Helper()
	var out []string
	for i := 1; i <= n; i++ {
		id := ids.New().String()
		w.sql(`INSERT INTO run_steps (id, run_id, ordinal, kind, name, title, status, detail) VALUES ($1, $2, $3, 'tool_call', 'read_file', 'Reading', 'completed', '{"lines": 3}')`,
			id, run, i)
		out = append(out, id)
	}
	return out
}

// TestGetRun_Document: every member of Run, steps keyed by ID, followups {}, ETag equal to the
// version, no JSON null.
func TestGetRun_Document(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, msg := w.openRun()
	steps := w.addSteps(r.ID, 2)
	resp := w.do("alex", http.MethodGet, "/api/v1/runs/"+r.ID, "")
	doc := w.wantOK(resp)
	if resp.Header.Get("ETag") != `"1"` {
		t.Fatalf("ETag %q", resp.Header.Get("ETag"))
	}
	for k, want := range map[string]any{
		"id": r.ID, "thread_id": r.Thread, "channel_id": w.allgemein.String(), "agent_id": w.lotse.String(), "triggered_by": w.alex.String(),
		"state": "queued", "billing_mode_expected": "organization", "trigger_kind": "message", "triggering_message_id": msg,
	} {
		if doc[k] != want {
			t.Fatalf("%s = %v, want %v (%v)", k, doc[k], want, doc)
		}
	}
	if doc["version"].(float64) != 1 || doc["queued_at"] == nil || doc["started_at"] != nil || doc["plan"] != nil {
		t.Fatalf("document %v", doc)
	}
	if f, ok := doc["followups"].(map[string]any); !ok || len(f) != 0 {
		t.Fatalf("followups %v", doc["followups"])
	}
	if m, ok := doc["metrics"].(map[string]any); !ok || len(m) != 0 {
		t.Fatalf("metrics %v", doc["metrics"])
	}
	cost := doc["cost"].(map[string]any)
	if cost["currency"] != "USD" || cost["is_estimate"] != true || cost["amount_micros"].(float64) != 0 {
		t.Fatalf("cost %v", cost)
	}
	got := doc["steps"].(map[string]any)
	if len(got) != 2 {
		t.Fatalf("steps %v", got)
	}
	for i, id := range steps {
		s := got[id].(map[string]any)
		if s["ordinal"].(float64) != float64(i+1) || s["kind"] != "tool_call" || s["detail"].(map[string]any)["lines"].(float64) != 3 {
			t.Fatalf("step %v", s)
		}
	}
}

// TestListRunSteps_Order: keyset by ordinal, limit capped at api.page_size_max, a foreign cursor is
// 400 cursor_invalid.
func TestListRunSteps_Order(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	other, _ := w.openRun()
	steps := w.addSteps(r.ID, 7)
	page := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/runs/"+r.ID+"/steps?limit=50", ""))
	items := page["items"].([]any)
	if len(items) != 5 || page["next_cursor"] == nil {
		t.Fatalf("page %v", page)
	}
	next := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/runs/"+r.ID+"/steps?cursor="+page["next_cursor"].(string), ""))
	items = append(items, next["items"].([]any)...)
	if next["next_cursor"] != nil || len(items) != 7 {
		t.Fatalf("second page %v", next)
	}
	for i, it := range items {
		if it.(map[string]any)["id"] != steps[i] {
			t.Fatalf("order at %d: %v", i, it)
		}
	}
	w.want(w.do("alex", http.MethodGet, "/api/v1/runs/"+other.ID+"/steps?cursor="+page["next_cursor"].(string), ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.do("sam", http.MethodGet, "/api/v1/runs/"+r.ID+"/steps?cursor="+page["next_cursor"].(string), ""), http.StatusBadRequest, "cursor_invalid")
}

// TestListThreadRuns_NewestFirst: the runs of one thread newest first, capped pages, a foreign
// cursor is 400 cursor_invalid.
func TestListThreadRuns_NewestFirst(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.allgemein)
	var want []string
	for i := range 6 {
		m := w.post("alex", th, "Run "+strconv.Itoa(i))["id"].(string)
		r, ok := w.runOf(m)
		if !ok {
			t.Fatalf("message %d opened no run", i)
		}
		w.mustTransition(r.ID, domain.StateQueued, domain.StateCancelled, 1, "user_cancelled", domain.SetterServer)
		want = append([]string{r.ID}, want...)
		w.clk.Advance(time.Second)
	}
	page := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/threads/"+th.String()+"/runs?limit=50", ""))
	items := page["items"].([]any)
	if len(items) != 5 || page["next_cursor"] == nil || page["total"] != nil {
		t.Fatalf("page %v", page)
	}
	next := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/threads/"+th.String()+"/runs?limit=5&cursor="+page["next_cursor"].(string), ""))
	items = append(items, next["items"].([]any)...)
	var got []string
	for _, it := range items {
		got = append(got, it.(map[string]any)["id"].(string))
	}
	if !slices.Equal(got, want) || next["next_cursor"] != nil {
		t.Fatalf("order %v, want %v", got, want)
	}
	w.want(w.do("alex", http.MethodGet, "/api/v1/threads/"+w.newThread(w.allgemein).String()+"/runs?cursor="+page["next_cursor"].(string), ""),
		http.StatusBadRequest, "cursor_invalid")
}

// activeIDs lists the ids of listActiveRuns for name with the query, following every cursor.
func (w *world) activeIDs(name, query string) []string {
	w.t.Helper()
	var out []string
	cursor := ""
	for {
		q := query
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		page := w.wantOK(w.do(name, http.MethodGet, "/api/v1/runs?"+q, ""))
		for _, it := range page["items"].([]any) {
			out = append(out, it.(map[string]any)["id"].(string))
		}
		next, _ := page["next_cursor"].(string)
		if next == "" {
			return out
		}
		cursor = next
	}
}

// sameSet reports whether a and b hold the same strings.
func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// TestListActiveRuns_Visibility: Alex sees the active runs of Support only; Sam those of Marketing
// and of Support/Allgemein (G-4) but not of Support/Eskalation; the owner sees all; end states are
// absent; agent_id filters; an unknown agent_id gives an empty page.
func TestListActiveRuns_Visibility(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	run := func(name string, channel ids.ID) string {
		m := w.post(name, w.newThread(channel), "Please help")["id"].(string)
		r, ok := w.runOf(m)
		if !ok {
			t.Fatalf("%s in %v opened no run", name, channel)
		}
		return r.ID
	}
	alg, esk := run("alex", w.allgemein), run("alex", w.eskalation)
	kam := run("sam", w.kampagnen)
	notes := run("owner", w.notizen)
	ended := run("alex", w.allgemein)
	w.mustTransition(ended, domain.StateQueued, domain.StateFailed, 1, "stale", domain.SetterServer)
	for _, c := range []struct {
		name string
		want []string
	}{
		{"alex", []string{alg, esk}},
		{"sam", []string{alg, kam}},
		{"owner", []string{alg, esk, kam, notes}},
	} {
		if got := w.activeIDs(c.name, "limit=2"); !sameSet(got, c.want) {
			t.Fatalf("%s sees %v, want %v", c.name, got, c.want)
		}
	}
	if got := w.activeIDs("owner", "agent_id="+w.texter.String()); !sameSet(got, []string{kam}) {
		t.Fatalf("agent filter %v", got)
	}
	if got := w.activeIDs("owner", "agent_id="+ids.New().String()); len(got) != 0 {
		t.Fatalf("unknown agent %v", got)
	}
}

// TestRunReads_NotFoundEquality (AZ-P4 for getRun, listRunSteps and listThreadRuns): a foreign
// run or thread and an unknown id give the same status, body and headers, and the medians of the
// response times differ by at most authz.not_found_timing_tolerance.
func TestRunReads_NotFoundEquality(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	foreign := w.post("sam", w.newThread(w.kampagnen), "Marketing question")["id"].(string)
	r, ok := w.runOf(foreign)
	if !ok {
		t.Fatal("no Marketing run")
	}
	unknown := ids.New().String()
	const tolerance = 20 * time.Millisecond // authz.not_found_timing_tolerance
	for _, c := range []struct{ name, a, b string }{
		{"getRun", "/api/v1/runs/" + r.ID, "/api/v1/runs/" + unknown},
		{"listRunSteps", "/api/v1/runs/" + r.ID + "/steps", "/api/v1/runs/" + unknown + "/steps"},
		{"listThreadRuns", "/api/v1/threads/" + r.Thread + "/runs", "/api/v1/threads/" + unknown + "/runs"},
	} {
		var ta, tb []time.Duration
		var bodyA, bodyB string
		for i := range 21 {
			for _, side := range []int{i % 2, 1 - i%2} {
				path := c.a
				if side == 1 {
					path = c.b
				}
				t0 := time.Now()
				resp := w.do("alex", http.MethodGet, path, "")
				d := time.Since(t0)
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("%s %s: %d %s", c.name, path, resp.StatusCode, resp.Body)
				}
				body := normalized(resp.Body)
				if side == 0 {
					ta, bodyA = append(ta, d), body
				} else {
					tb, bodyB = append(tb, d), body
				}
			}
		}
		if bodyA != bodyB {
			t.Fatalf("%s: bodies differ:\n%s\n%s", c.name, bodyA, bodyB)
		}
		if diff := median(ta) - median(tb); diff > tolerance || -diff > tolerance {
			t.Fatalf("%s: medians %v and %v", c.name, median(ta), median(tb))
		}
	}
}

// normalized drops the members of a problem body that differ per request (instance, correlation).
func normalized(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return string(body)
	}
	delete(m, "instance")
	delete(m, "correlation_id")
	b, _ := json.Marshal(m)
	return string(b)
}

// median returns the median of d.
func median(d []time.Duration) time.Duration {
	s := slices.Clone(d)
	slices.Sort(s)
	return s[len(s)/2]
}

// TestAZP5_RunsOverviewAndSnapshot: property test over random worlds of grants with active runs
// (seed logged, a fixed number of rounds): no item of listActiveRuns and no entry of
// Snapshot.active_runs refers to a channel without read right, and every active run of a readable
// channel is listed.
func TestAZP5_RunsOverviewAndSnapshot(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	channels := []ids.ID{w.allgemein, w.eskalation, w.kampagnen, w.werkzeuge, w.notizen}
	runsOf := map[ids.ID]string{}
	for _, ch := range channels {
		m := w.post("owner", w.newThread(ch), "Start")["id"].(string)
		r, ok := w.runOf(m)
		if !ok {
			t.Fatalf("no run in %v", ch)
		}
		runsOf[ch] = r.ID
	}
	seed := time.Now().UnixNano()
	t.Logf("seed %d", seed)
	rnd := rand.New(rand.NewPCG(uint64(seed), 7)) // #nosec G404 -- a seeded property test must be reproducible from its logged seed
	for round := range 4 {
		name := fmt.Sprintf("p%d", round)
		p := w.person(name, false)
		var readable []string
		for _, ch := range channels {
			if rnd.IntN(2) == 0 {
				continue
			}
			w.grant(`"subject_kind":"user","subject_id":"` + p.String() + `","target_kind":"channel","target_id":"` + ch.String() + `","level":"read","reason":"AZ-P5"`)
			readable = append(readable, runsOf[ch])
		}
		if got := w.activeIDs(name, "limit=2"); !sameSet(got, readable) {
			t.Fatalf("seed %d round %d: listActiveRuns %v, want %v", seed, round, got, readable)
		}
		snap := w.wantOK(w.do(name, http.MethodGet, "/api/v1/sync/snapshot", ""))
		var got []string
		for _, it := range snap["active_runs"].([]any) {
			got = append(got, it.(map[string]any)["id"].(string))
		}
		if !sameSet(got, readable) {
			t.Fatalf("seed %d round %d: snapshot %v, want %v", seed, round, got, readable)
		}
	}
}

// TestSnapshot_ActiveRuns: the snapshot of Alex lists the active Support runs as RunSummary; an
// ended run disappears; the M1 binding noruns no longer exists.
func TestSnapshot_ActiveRuns(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	other := w.post("sam", w.newThread(w.kampagnen), "Marketing")["id"].(string)
	if _, ok := w.runOf(other); !ok {
		t.Fatal("no Marketing run")
	}
	snap := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/sync/snapshot", ""))
	list := snap["active_runs"].([]any)
	if len(list) != 1 {
		t.Fatalf("active_runs %v", list)
	}
	s := list[0].(map[string]any)
	if s["id"] != r.ID || s["state"] != "queued" || s["channel_id"] != w.allgemein.String() || s["step_count"].(float64) != 0 {
		t.Fatalf("summary %v", s)
	}
	w.mustTransition(r.ID, domain.StateQueued, domain.StateCancelled, 1, "user_cancelled", domain.SetterServer)
	snap = w.wantOK(w.do("alex", http.MethodGet, "/api/v1/sync/snapshot", ""))
	if list := snap["active_runs"].([]any); len(list) != 0 {
		t.Fatalf("an ended run stays: %v", list)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", "internal", "sync", "adapters", "noruns")); !os.IsNotExist(err) {
		t.Fatalf("internal/sync/adapters/noruns still exists: %v", err)
	}
}

// TestRunStateChanged_CatchUpReceivers: listEvents gives run.state_changed of a run in
// Support/Eskalation to Alex and the owner and not to Sam; resource.version equals runs.version;
// the event and message.updated pass the frame schemas of asyncapi.yaml (TestRunSchemas).
func TestRunStateChanged_CatchUpReceivers(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	after := w.maxSeq()
	m := w.post("alex", w.newThread(w.eskalation), "Escalation")["id"].(string)
	r, ok := w.runOf(m)
	if !ok {
		t.Fatal("no run")
	}
	w.mustTransition(r.ID, domain.StateQueued, domain.StateRunning, 1, "", domain.SetterServer)
	contract, err := specschema.Load(filepath.Join("..", "..", "..", "..", ".."))
	must(t, err)
	for _, c := range []struct {
		name string
		want int
	}{{"alex", 2}, {"owner", 2}, {"sam", 0}} {
		page := w.wantOK(w.do(c.name, http.MethodGet, "/api/v1/events?limit=500&after="+strconv.FormatInt(after, 10), ""))
		n := 0
		for _, it := range page["events"].([]any) {
			ev := it.(map[string]any)
			switch ev["type"] {
			case "run.state_changed":
				res := ev["resource"].(map[string]any)
				if res["id"] != r.ID {
					continue
				}
				n++
				if want := w.runsWhere(`id = $1`, r.ID)[0].Version; n == 2 && res["version"].(float64) != float64(want) {
					t.Fatalf("resource.version %v, runs.version %d", res["version"], want)
				}
				validFrame(t, contract, ev)
			case "message.updated":
				validFrame(t, contract, ev)
			}
		}
		if n != c.want {
			t.Fatalf("%s got %d run.state_changed, want %d", c.name, n, c.want)
		}
	}
}

// validFrame checks one envelope as a dispatch frame of the gateway against asyncapi.yaml.
func validFrame(t *testing.T, c *specschema.Contract, ev map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"op": "dispatch", "d": ev})
	must(t, err)
	if strings.Contains(string(raw), "null") {
		t.Fatalf("null in %s", raw)
	}
	if err := c.ValidateServerFrame(raw); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
}

// TestRunActiveGauge: the worker gauge reports the active runs per agent handle and state.
func TestRunActiveGauge(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	a, _ := w.openRun()
	b, _ := w.openRun()
	w.mustTransition(b.ID, domain.StateQueued, domain.StateRunning, 1, "", domain.SetterServer)
	w.post("sam", w.newThread(w.kampagnen), "Marketing")
	_ = a
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	reg, err := runsmetrics.RegisterActiveGauge(mp, w.runs)
	must(t, err)
	defer func() { _ = reg.Unregister() }()
	var rm metricdata.ResourceMetrics
	must(t, reader.Collect(context.Background(), &rm))
	got := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bothy.run.active" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Gauge[int64]).DataPoints {
				h, _ := dp.Attributes.Value("bothy.agent.handle")
				s, _ := dp.Attributes.Value("bothy.run.state")
				got[h.AsString()+"/"+s.AsString()] += dp.Value
			}
		}
	}
	want := map[string]int64{"lotse/queued": 1, "lotse/running": 1, "texter/queued": 1}
	keys := func(m map[string]int64) []string {
		var k []string
		for x := range m {
			k = append(k, fmt.Sprintf("%s=%d", x, m[x]))
		}
		sort.Strings(k)
		return k
	}
	if !slices.Equal(keys(got), keys(want)) {
		t.Fatalf("gauge %v, want %v", keys(got), keys(want))
	}
}
