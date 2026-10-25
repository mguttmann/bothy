// SPDX-License-Identifier: Apache-2.0

// Package status_test holds the integration tests of brief M7-09: the real server of
// internal/wire in role all through test/harness on a fake clock, PostgreSQL from pgtest, and
// fakes for the checks of the status page (wire.Deps.StatusSources). The test takes the job
// status.probe and the participants of retention.ops from the job modules and runs them itself
// with a controlled clock; River runs a no-op in place of status.probe, so no round runs behind
// the test's back.
package status_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	statusjobs "github.com/OWNER/bothy/services/server/internal/status/adapters/jobs"
	statusapp "github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// interval is status.check_interval of the tests: its default, named by the key.
const interval = 30 * time.Second

// checkKeys are the health_source keys of 16 12 with a source of their own.
var checkKeys = []string{
	domain.SourceAPIHTTP, domain.SourceSyncFanout, domain.SourcePushAPNs, domain.SourcePushFCM,
	domain.SourceFilesStorage, domain.SourceFilesScan, domain.SourceSearchQuery,
}

// fakes are the simulated checks: one outcome per key, Pass until a test changes it.
type fakes struct {
	mu  sync.Mutex
	out map[string]domain.Outcome
}

func (f *fakes) set(key string, o domain.Outcome) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.out[key] = o
}

func (f *fakes) source(key string) statusapp.Source {
	return statusapp.SourceFunc(func(context.Context, statusapp.Window) domain.Outcome {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.out[key]
	})
}

// world is one test server with the status page.
type world struct {
	t     *testing.T
	s     *harness.Server
	clk   *clock.Fake
	db    *pgxpool.Pool
	fakes *fakes

	mu    sync.Mutex
	probe statusjobs.Runner
	reg   *retention.Registry
}

// noProbe is the River handler of status.probe in the tests: it does nothing.
type noProbe struct{}

func (noProbe) Run(context.Context) error { return nil }

// newWorld starts the server with cfg on a fake clock with fake checks that pass; the agent
// presence stays the real stored one.
func newWorld(t *testing.T, cfg map[string]string) *world { return start(t, cfg, true) }

// newRealWorld starts the server with the real checks of the process.
func newRealWorld(t *testing.T) *world { return start(t, nil, false) }

// start starts the server with cfg; fake replaces the checks with the fakes.
func start(t *testing.T, cfg map[string]string, fake bool) *world {
	t.Helper()
	all := harness.RaisedRateLimits()
	for k, v := range cfg {
		all[k] = v
	}
	all["auth.clock_leeway"], all["auth.dpop_proof_max_age"] = "96h", "96h"
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond)), fakes: &fakes{out: map[string]domain.Outcome{}}}
	for _, k := range checkKeys {
		w.fakes.out[k] = domain.Pass
	}
	opts := harness.Options{Config: all, Clock: w.clk, EgressDenyAll: true, JobModules: w.take}
	if fake {
		opts.StatusSources = func(real statusapp.Sources) statusapp.Sources {
			src := statusapp.Sources{Checks: map[string]statusapp.Source{}, Agents: real.Agents}
			for _, k := range checkKeys {
				src.Checks[k] = w.fakes.source(k)
			}
			return src
		}
	}
	w.s = harness.Start(t, opts)
	w.db = w.s.AdminDB()
	return w
}

// take keeps the job status.probe and the retention participants and gives River a no-op.
func (w *world) take(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.probe, w.reg = m.StatusProbe, m.RetentionParticipants
	m.StatusProbe = noProbe{}
}

// round moves the clock by one check interval and runs one round of status.probe.
func (w *world) round() {
	w.t.Helper()
	w.clk.Advance(interval)
	w.mu.Lock()
	p := w.probe
	w.mu.Unlock()
	if err := p.Run(w.t.Context()); err != nil {
		w.t.Fatalf("status.probe: %v", err)
	}
}

// rounds runs n rounds.
func (w *world) rounds(n int) {
	w.t.Helper()
	for range n {
		w.round()
	}
}

// page is the schema StatusPage as the tests read it.
type page struct {
	OverallState string      `json:"overall_state"`
	Components   []component `json:"components"`
	Incidents    []incident  `json:"incidents"`
}

type component struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SortKey      string `json:"sort_key"`
	State        string `json:"state"`
	HealthSource string `json:"health_source"`
	AgentID      string `json:"agent_id"`
}

type incident struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Kind         string   `json:"kind"`
	Impact       string   `json:"impact"`
	State        string   `json:"state"`
	ResolvedAt   string   `json:"resolved_at"`
	ComponentIDs []string `json:"component_ids"`
	Updates      []struct {
		State  string `json:"state"`
		BodyMD string `json:"body_md"`
	} `json:"updates"`
}

// read returns the page as c (getStatus).
func (w *world) read(c *harness.Client) page {
	w.t.Helper()
	var p page
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodGet, Path: "/status"}, http.StatusOK).JSON(w.t, &p)
	return p
}

// bySource returns the component with health_source hs on p.
func (p page) bySource(t *testing.T, hs string) component {
	t.Helper()
	for _, c := range p.Components {
		if c.HealthSource == hs {
			return c
		}
	}
	t.Fatalf("no component with health_source %s in %+v", hs, p.Components)
	return component{}
}

// byAgent returns the component of agent and whether p has it.
func (p page) byAgent(agent string) (component, bool) {
	for _, c := range p.Components {
		if c.AgentID == agent {
			return c, true
		}
	}
	return component{}, false
}

// incidentsOf returns the incidents on p that name component id.
func (p page) incidentsOf(id string) []incident {
	var out []incident
	for _, i := range p.Incidents {
		for _, c := range i.ComponentIDs {
			if c == id {
				out = append(out, i)
			}
		}
	}
	return out
}

// openIncidentsOf returns the incidents on p that name component id and are not resolved.
func (p page) openIncidentsOf(id string) []incident {
	var out []incident
	for _, i := range p.incidentsOf(id) {
		if i.State != "resolved" {
			out = append(out, i)
		}
	}
	return out
}

// incident returns the incident id on p.
func (p page) incident(t *testing.T, id string) incident {
	t.Helper()
	for _, i := range p.Incidents {
		if i.ID == id {
			return i
		}
	}
	t.Fatalf("no incident %s on the page", id)
	return incident{}
}

// updatedEvents returns the payloads of status.updated in the event log, oldest first.
func (w *world) updatedEvents() []map[string]any {
	w.t.Helper()
	rows, err := w.db.Query(w.t.Context(), `SELECT payload FROM event_log WHERE type = 'status.updated' ORDER BY seq`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			w.t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.db.Exec(w.t.Context(), sql, args...); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
}

func (w *world) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.db.QueryRow(w.t.Context(), sql, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// heartbeat stores the presence of agent as the heartbeat of an api node would, at the time of
// the clock.
func (w *world) heartbeat(agent, state string) {
	w.t.Helper()
	w.exec(`UPDATE agents SET reported_runtime_state = $2, last_heartbeat_at = $3 WHERE principal_id = $1`, agent, state, w.clk.Now())
}
