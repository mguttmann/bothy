// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// statsWorld is the test world of 07 sections 10 and 11.1 for the statistics (M6-05): the Owner
// Alex, space Support with the channels Allgemein and Eskalation, space Engineering with the
// channel server; Sam with manage on Support, Robin with read on Support/Allgemein only; the
// agents support-agent and dev-runner. The process runs in role api with a shift clock, so the
// cache test passes stats.cache_ttl without waiting.
type statsWorld struct {
	*dlWorld
	clk                           *shiftClock
	sam, robin                    gwPerson
	support, engineering          string
	allgemein, eskalation, server channel
	supportAgent, devRunner       ids.ID
	samGrant                      string
	minGroup                      int
	people                        []string
	runSeq                        int
	bodies                        []string
	tz                            *time.Location
	statsTTL                      time.Duration
	webhookKid                    string
	stepTitle, stepDetail, prompt string
}

// statsEnv are the configuration entries of the world: day boundaries of Europe/Berlin, the
// largest cache lifetime (the shift clock passes it), a minimum group of 3, statistics per person
// switched on (it must change nothing) and the personal amounts shown.
var statsEnv = []string{
	"BOTHY_STATS__TIME_ZONE=Europe/Berlin", "BOTHY_STATS__CACHE_TTL=1h", "BOTHY_PRIVACY__STATS_MIN_GROUP_SIZE=3",
	"BOTHY_PRIVACY__PER_PERSON_STATS_ENABLED=true", "BOTHY_BILLING__SHOW_PERSONAL_ESTIMATES=true",
}

// newStatsWorld starts the world with the environment entries extra after statsEnv.
func newStatsWorld(t *testing.T, extra ...string) *statsWorld {
	t.Helper()
	clk := &shiftClock{}
	gw := newGatewayWorldRole(t, wire.RoleAPI, wire.Deps{Clock: clk}, append(append([]string{}, statsEnv...), extra...)...)
	gw.partitions(t)
	owner := gw.newPerson("alex", identity.DeviceStateActive)
	gw.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'test')`, ids.New().String(), owner.user.String())
	w := &statsWorld{dlWorld: &dlWorld{gwWorld: gw, owner: owner}, clk: clk,
		stepTitle: "Title of a hidden step", stepDetail: "detail-of-a-hidden-step", prompt: "Content of a message"}
	cfg, err := wire.StatsConfig(loadEnv(t, gw.env))
	if err != nil {
		t.Fatal(err)
	}
	w.minGroup, w.statsTTL = cfg.MinGroupSize, cfg.CacheTTL
	if w.tz, err = time.LoadLocation(cfg.TimeZone); err != nil {
		t.Fatal(err)
	}
	w.support, w.engineering = w.newSpace("Support"), w.newSpace("Engineering")
	w.allgemein, w.eskalation = w.newChannel(w.support, "allgemein"), w.newChannel(w.support, "eskalation")
	w.server = w.newChannel(w.engineering, "server")
	// The main threads exist before every fixture row of the world (runs cannot predate them).
	w.sql(`UPDATE threads SET created_at = '2026-09-01T00:00:00Z'`)
	w.sam = w.newPerson("sam", identity.DeviceStateActive)
	w.robin = w.newPerson("robin", identity.DeviceStateActive)
	w.samGrant = w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{"subject_kind": "user", "subject_id": w.sam.user.String(),
		"target_kind": "space", "target_id": w.support, "level": "write", "capabilities": []string{"manage"}, "reason": "Support lead"})
	w.grant(w.robin.user, "user", "channel", w.allgemein.id)
	w.supportAgent, w.devRunner = w.newAgent(), w.newAgent()
	w.people = []string{owner.user.String(), w.sam.user.String(), w.robin.user.String()}
	if err := gw.owner.QueryRow(context.Background(), `SELECT kid FROM server_signing_keys ORDER BY kid LIMIT 1`).Scan(&w.webhookKid); err != nil {
		t.Fatal(err)
	}
	return w
}

// message writes a message as the Owner into the main thread of c through the API and then moves
// it to author, origin and at, as a fixture row: the statistics count authors and times the API
// does not let a test choose.
func (w *statsWorld) message(c channel, author ids.ID, origin string, at time.Time) {
	w.t.Helper()
	cm := w.post(c.thread, w.prompt)
	w.sql(`UPDATE messages SET author_principal_id = $1, origin = $2, created_at = $3 WHERE client_msg_id = $4`, author.String(), origin, at, cm)
}

// run inserts an ended run of agent in the main thread of c as a fixture row; started is zero for
// a run that never started. It returns the run.
func (w *statsWorld) run(c channel, agent ids.ID, state, reason string, started time.Time, d time.Duration) string {
	w.t.Helper()
	id := ids.New().String()
	var startedAt, reasonArg any
	finished := started.Add(d)
	if !started.IsZero() {
		startedAt = started
	} else {
		finished = time.Date(2026, 10, 2, 13, 0, 0, 0, time.UTC)
	}
	if reason != "" {
		reasonArg = reason
	}
	w.sql(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, state_reason, billing_mode_expected, invocation_jti,
		queued_at, started_at, finished_at) VALUES ($1, $2, $3, 'message', $4, $5, $6, 'organization', $7, $8, $9, $10)`,
		id, c.thread, agent.String(), w.owner.user.String(), state, reasonArg, ids.New().String(), finished.Add(-time.Hour), startedAt, finished)
	return id
}

// step inserts a tool call of run as a fixture row; d is zero for a call that has not finished.
func (w *statsWorld) step(run, name, status string, started time.Time, d time.Duration) {
	w.t.Helper()
	w.runSeq++
	var finished any
	if d > 0 {
		finished = started.Add(d)
	}
	w.sql(`INSERT INTO run_steps (id, run_id, ordinal, kind, name, title, status, detail, started_at, finished_at)
		VALUES ($1, $2, $3, 'tool_call', $4, $5, $6, $7, $8, $9)`, ids.New().String(), run, w.runSeq, name, w.stepTitle, status,
		`{"input":"`+w.stepDetail+`"}`, started, finished)
}

// cost inserts a cost entry of run as a fixture row and returns it; supersedes names the entry it
// replaces ("" for none).
func (w *statsWorld) cost(run string, c channel, space string, agent ids.ID, mode string, amount int64, currency string, estimate bool,
	at time.Time, supersedes string) string {
	w.t.Helper()
	id := ids.New().String()
	var sup any
	source := "computed"
	if supersedes != "" {
		sup, source = supersedes, "reconciled"
	}
	w.sql(`INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, billing_mode, provider, model, amount_micros,
		currency, is_estimate, source, occurred_at, supersedes_id) VALUES ($1, $2, $3, $4, $5, $6, $7, 'example', 'model-large', $8, $9, $10, $11, $12, $13)`,
		id, run, agent.String(), c.thread, c.id, space, mode, amount, currency, estimate, source, at, sup)
	return id
}

// statsSeries is the part of a StatsSeries the tests read.
type statsSeries struct {
	Granularity string `json:"granularity"`
	Buckets     []struct {
		Start         time.Time          `json:"start"`
		GroupKey      string             `json:"group_key"`
		Metrics       map[string]float64 `json:"metrics"`
		BelowMinGroup []string           `json:"below_min_group"`
	} `json:"buckets"`
}

// costSeries is the part of a CostSeries the tests read.
type costSeries struct {
	Currency string `json:"currency"`
	Buckets  []struct {
		Start        time.Time `json:"start"`
		GroupKey     string    `json:"group_key"`
		BillingMode  string    `json:"billing_mode"`
		AmountMicros int64     `json:"amount_micros"`
		IsEstimate   bool      `json:"is_estimate"`
	} `json:"buckets"`
}

// get calls GET path with the query q as p, with a DPoP proof at the clock of the process, and
// returns status and body; every body is kept for the scan for person IDs.
func (w *statsWorld) get(p gwPerson, path string, q url.Values) (int, []byte) {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	target := path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.base+target, nil)
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+p.access)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: gwPublicURL + path,
		IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(p.access)}))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	w.bodies = append(w.bodies, string(b))
	return resp.StatusCode, b
}

// series calls op and requires 200.
func (w *statsWorld) series(p gwPerson, op string, q url.Values) statsSeries {
	w.t.Helper()
	code, b := w.get(p, "/api/v1/stats/"+op, q)
	if code != http.StatusOK {
		w.t.Fatalf("%s %v = %d %s", op, q, code, b)
	}
	var s statsSeries
	if err := json.Unmarshal(b, &s); err != nil {
		w.t.Fatal(err)
	}
	return s
}

// status calls op and returns the status and the problem code.
func (w *statsWorld) status(p gwPerson, op string, q url.Values) (int, string) {
	w.t.Helper()
	code, b := w.get(p, "/api/v1/stats/"+op, q)
	var pr struct {
		Code   string `json:"code"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(b, &pr)
	if len(pr.Errors) > 0 {
		return code, pr.Code + "/" + pr.Errors[0].Code
	}
	return code, pr.Code
}

// params builds a query from pairs.
func params(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		q.Set(kv[i], kv[i+1])
	}
	return q
}

// rfc formats t for a query.
func rfc(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// metric returns the metric of the bucket of group starting at start; ok false when absent.
func (s statsSeries) metric(start time.Time, group, name string) (float64, bool) {
	for _, b := range s.Buckets {
		if b.Start.Equal(start) && b.GroupKey == group {
			v, ok := b.Metrics[name]
			return v, ok
		}
	}
	return 0, false
}

// groups returns the group keys of the series.
func (s statsSeries) groups() map[string]bool {
	out := map[string]bool{}
	for _, b := range s.Buckets {
		out[b.GroupKey] = true
	}
	return out
}

// statsDay is the day of the run, cost and error fixtures (UTC noon, far from any day boundary).
var statsDay = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

// TestStats is the statistics of M6-05 end to end through a started api process: authorization
// and channel set per scope, the minimum group, runs, tool calls, error rates, costs, time zone,
// ranges, the closed request schema and the cache with the permission version. The subtests share
// one world and run in order; each reads only the fixture rows of its own period.
func TestStats(t *testing.T) {
	t.Parallel()
	w := newStatsWorld(t)
	w.seedRuns()
	w.seedInstanceRows()

	berlinDay := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, w.tz) }
	dayRange := params("from", rfc(berlinDay(2026, 10, 2)), "to", rfc(berlinDay(2026, 10, 3)), "granularity", "day")
	day := berlinDay(2026, 10, 2).UTC()

	t.Run("SpaceManagerSeesOnlyOwnChannels", func(t *testing.T) {
		w.t = t
		at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
		w.message(w.allgemein, w.robin.user, "human", at)
		w.message(w.eskalation, w.sam.user, "agent", at)
		w.message(w.server, w.owner.user, "human", at)
		w.message(w.server, w.owner.user, "human", at.Add(time.Hour))
		q := params("from", rfc(berlinDay(2026, 10, 6)), "to", rfc(berlinDay(2026, 10, 7)), "space_id", w.support)
		s := w.series(w.sam, "usage", q)
		start := berlinDay(2026, 10, 6).UTC()
		if g := s.groups(); len(g) != 1 || !g[w.support] {
			t.Fatalf("Sam's groups %v", g)
		}
		if n, _ := s.metric(start, w.support, "messages"); n != 2 {
			t.Fatalf("Sam's messages %v, want the 2 of Allgemein and Eskalation: %+v", n, s)
		}
		if n, _ := s.metric(start, w.support, "messages_agent"); n != 1 {
			t.Fatalf("messages_agent %v", n)
		}
		if _, ok := s.metric(start, w.support, "active_people"); ok {
			t.Fatal("active_people in a day bucket")
		}
		q.Set("space_id", w.engineering)
		if code, pc := w.status(w.sam, "usage", q); code != http.StatusNotFound || pc != "not_found" {
			t.Fatalf("Sam on Engineering = %d %s", code, pc)
		}
		q.Del("space_id")
		if code, pc := w.status(w.sam, "usage", q); code != http.StatusForbidden || pc != "forbidden" {
			t.Fatalf("Sam without space = %d %s", code, pc)
		}
		owner := w.series(w.owner, "usage", q)
		if n, _ := owner.metric(start, w.engineering, "messages"); n != 2 {
			t.Fatalf("the Owner's Engineering messages %v: %+v", n, owner)
		}
	})

	t.Run("ReadOnlyMemberIsForbidden", func(t *testing.T) {
		w.t = t
		for _, op := range []string{"usage", "runs", "tool-calls", "errors", "costs"} {
			q := params("from", rfc(berlinDay(2026, 10, 2)), "to", rfc(berlinDay(2026, 10, 3)), "space_id", w.support)
			if code, pc := w.status(w.robin, op, q); code != http.StatusForbidden || pc != "forbidden" {
				t.Fatalf("Robin %s = %d %s", op, code, pc)
			}
		}
	})

	t.Run("RunsAndTools", func(t *testing.T) {
		w.t = t
		s := w.series(w.owner, "runs", dayRange)
		sa, dr := w.supportAgent.String(), w.devRunner.String()
		for name, want := range map[string]float64{"runs": 4, "completed": 2, "failed": 1, "cancelled": 1,
			"failed_reason_billing_mode_mismatch": 1, "duration_ms_p50": 2000, "duration_ms_p95": 2900} {
			if got, ok := s.metric(day, sa, name); !ok || got != want {
				t.Fatalf("support-agent %s = %v (%v), want %v: %+v", name, got, ok, want, s)
			}
		}
		if _, ok := s.metric(day, sa, "failed_reason_user_cancelled"); ok {
			t.Fatal("a failure reason of a cancelled run")
		}
		if n, _ := s.metric(day, dr, "completed"); n != 1 {
			t.Fatalf("dev-runner completed %v", n)
		}
		q := params("from", dayRange.Get("from"), "to", dayRange.Get("to"), "space_id", w.support)
		if g := w.series(w.sam, "runs", q).groups(); len(g) != 1 || !g[sa] {
			t.Fatalf("Sam's run groups %v", g)
		}
		q.Set("agent_id", dr)
		if g := w.series(w.owner, "runs", q).groups(); len(g) != 0 {
			t.Fatalf("dev-runner in Support %v", g)
		}
		tools := w.series(w.owner, "tool-calls", dayRange)
		for _, c := range []struct {
			tool, metric string
			want         float64
			present      bool
		}{{"search", "calls", 2, true}, {"search", "failed", 1, true}, {"search", "duration_ms_p50", 200, true},
			{"deploy", "calls", 1, true}, {"deploy", "duration_ms_p50", 0, false}} {
			got, ok := tools.metric(day, c.tool, c.metric)
			if ok != c.present || got != c.want {
				t.Fatalf("%s %s = %v (%v): %+v", c.tool, c.metric, got, ok, tools)
			}
		}
	})

	t.Run("ErrorRates", func(t *testing.T) {
		w.t = t
		owner := w.series(w.owner, "errors", dayRange)
		for name, want := range map[string]float64{"run_failure_rate": 0.2, "invocation_denials": 2, "webhook_failure_rate": 0.5,
			"push_failure_rate_apns": 0.5, "push_failure_rate_fcm": 0, "scan_errors": 1, "scan_infected": 1} {
			if got, ok := owner.metric(day, "", name); !ok || got != want {
				t.Fatalf("instance %s = %v (%v), want %v: %+v", name, got, ok, want, owner)
			}
		}
		q := params("from", dayRange.Get("from"), "to", dayRange.Get("to"), "space_id", w.support)
		for _, p := range []gwPerson{w.owner, w.sam} {
			s := w.series(p, "errors", q)
			if got, _ := s.metric(day, w.support, "run_failure_rate"); got != 0.25 {
				t.Fatalf("Support run_failure_rate %v: %+v", got, s)
			}
			if got, _ := s.metric(day, w.support, "invocation_denials"); got != 2 {
				t.Fatalf("Support invocation_denials %v", got)
			}
			for _, b := range s.Buckets {
				for name := range b.Metrics {
					if strings.HasPrefix(name, "webhook") || strings.HasPrefix(name, "push") || strings.HasPrefix(name, "scan") {
						t.Fatalf("an instance metric for a space: %s", name)
					}
				}
			}
		}
	})

	t.Run("Costs", func(t *testing.T) {
		w.t = t
		q := params("from", dayRange.Get("from"), "to", dayRange.Get("to"), "group_by", "space")
		code, b := w.get(w.owner, "/api/v1/stats/costs", q)
		var s costSeries
		if code != http.StatusOK || json.Unmarshal(b, &s) != nil {
			t.Fatalf("costs = %d %s", code, b)
		}
		type cell struct {
			group, mode string
			amount      int64
			estimate    bool
		}
		var got []cell
		for _, bk := range s.Buckets {
			if !bk.Start.Equal(day) {
				t.Fatalf("bucket start %v", bk.Start)
			}
			got = append(got, cell{bk.GroupKey, bk.BillingMode, bk.AmountMicros, bk.IsEstimate})
		}
		want := []cell{{w.engineering, "organization", 700, true}, {w.support, "organization", 3000, true}, {w.support, "personal", 400, false}}
		if w.engineering > w.support {
			want = []cell{want[1], want[2], want[0]}
		}
		if s.Currency != "USD" || len(got) != len(want) {
			t.Fatalf("costs %+v, want %+v", s, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("costs %+v, want %+v", got, want)
			}
		}
		q.Set("space_id", w.support)
		code, b = w.get(w.sam, "/api/v1/stats/costs", q)
		if code != http.StatusOK || json.Unmarshal(b, &s) != nil {
			t.Fatalf("Sam's costs = %d %s", code, b)
		}
		for _, bk := range s.Buckets {
			if bk.GroupKey != w.support {
				t.Fatalf("Sam sees costs of %s", bk.GroupKey)
			}
		}
		wide := params("from", rfc(berlinDay(2026, 10, 1)), "to", rfc(berlinDay(2026, 10, 10)), "group_by", "none")
		if code, pc := w.status(w.owner, "costs", wide); code != http.StatusBadRequest || pc != "validation_failed/currency_required" {
			t.Fatalf("several currencies = %d %s", code, pc)
		}
		wide.Set("currency", "EUR")
		code, b = w.get(w.owner, "/api/v1/stats/costs", wide)
		if code != http.StatusOK || json.Unmarshal(b, &s) != nil || s.Currency != "EUR" || len(s.Buckets) != 1 || s.Buckets[0].AmountMicros != 50 {
			t.Fatalf("EUR costs = %d %s", code, b)
		}
	})

	t.Run("MinGroup", func(t *testing.T) {
		w.t = t
		week := berlinDay(2026, 10, 12)
		at := week.Add(30 * time.Hour)
		// Support: minGroup - 1 distinct people; Engineering: exactly minGroup.
		support := []ids.ID{w.robin.user, w.sam.user}
		engineering := []ids.ID{w.owner.user, w.sam.user, w.robin.user}
		if w.minGroup != 3 {
			t.Fatalf("the world assumes privacy.stats_min_group_size 3, got %d", w.minGroup)
		}
		for _, p := range support[:w.minGroup-1] {
			w.message(w.allgemein, p, "human", at)
		}
		w.message(w.eskalation, w.supportAgent, "agent", at)
		for _, p := range engineering[:w.minGroup] {
			w.message(w.server, p, "human", at)
		}
		q := params("from", rfc(week), "to", rfc(week.AddDate(0, 0, 7)), "granularity", "week")
		s := w.series(w.owner, "usage", q)
		start := week.UTC()
		if _, ok := s.metric(start, w.support, "active_people"); ok {
			t.Fatalf("active_people below the minimum group: %+v", s)
		}
		found := false
		for _, b := range s.Buckets {
			if b.GroupKey == w.support && b.Start.Equal(start) {
				found = len(b.BelowMinGroup) == 1 && b.BelowMinGroup[0] == "active_people" && b.Metrics["messages"] == 3
			}
		}
		if !found {
			t.Fatalf("Support bucket without below_min_group: %+v", s)
		}
		if n, ok := s.metric(start, w.engineering, "active_people"); !ok || n != float64(w.minGroup) {
			t.Fatalf("Engineering active_people %v %v", n, ok)
		}
		q.Set("granularity", "month")
		month := w.series(w.owner, "usage", q)
		if n, ok := month.metric(berlinDay(2026, 10, 1).UTC(), w.engineering, "active_people"); !ok || n != float64(w.minGroup) {
			t.Fatalf("Engineering active_people per month %v %v: %+v", n, ok, month)
		}
	})

	t.Run("NoPersonGrouping", func(t *testing.T) {
		w.t = t
		// No operation has a parameter that groups by person. The chain closes request bodies
		// (M1-18), not query strings: an unknown query member is ignored, so the answer is the one
		// without it and never grouped by person; group_by of getCostStats is an enum without
		// person.
		for _, op := range []string{"usage", "runs", "tool-calls", "errors"} {
			plain := w.series(w.owner, op, dayRange)
			for _, extra := range []string{"group_by", "person_id", "user_id"} {
				q := params("from", dayRange.Get("from"), "to", dayRange.Get("to"), "granularity", "day", extra, w.sam.user.String())
				got := w.series(w.owner, op, q)
				if len(got.Buckets) != len(plain.Buckets) || len(got.groups()) != len(plain.groups()) || got.groups()[w.sam.user.String()] {
					t.Fatalf("%s with %s: %+v, want %+v", op, extra, got, plain)
				}
			}
		}
		q := params("from", dayRange.Get("from"), "to", dayRange.Get("to"), "group_by", "person")
		if code, _ := w.status(w.owner, "costs", q); code != http.StatusBadRequest {
			t.Fatalf("costs grouped by person = %d", code)
		}
	})

	t.Run("TimeZone", func(t *testing.T) {
		w.t = t
		for _, at := range []string{"2026-10-24T21:59:00Z", "2026-10-24T22:00:00Z", "2026-10-25T22:59:00Z", "2026-10-25T23:00:00Z"} {
			ts, _ := time.Parse(time.RFC3339, at)
			w.message(w.eskalation, w.sam.user, "human", ts)
		}
		q := params("from", "2026-10-24T00:00:00.000Z", "to", "2026-10-27T00:00:00.000Z", "granularity", "day", "space_id", w.support)
		s := w.series(w.owner, "usage", q)
		for at, want := range map[string]float64{"2026-10-23T22:00:00Z": 1, "2026-10-24T22:00:00Z": 2, "2026-10-25T23:00:00Z": 1} {
			start, _ := time.Parse(time.RFC3339, at)
			if got, _ := s.metric(start, w.support, "messages"); got != want {
				t.Fatalf("bucket %s messages %v, want %v: %+v", at, got, want, s)
			}
		}
		// The aligned range ends at local midnight after the 27th; its last bucket holds the
		// current storage of the space and no message.
		last := s.Buckets[len(s.Buckets)-1]
		if len(s.Buckets) != 4 || !last.Start.Equal(time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC)) || last.Metrics["storage_bytes"] != 1000 ||
			len(last.Metrics) != 1 {
			t.Fatalf("buckets %+v", s)
		}
	})

	t.Run("Range", func(t *testing.T) {
		w.t = t
		from := berlinDay(2026, 1, 1)
		for _, c := range []struct {
			to   time.Time
			gran string
			code string
		}{
			{from.AddDate(0, 0, 401), "month", "validation_failed/range_too_long"},
			{from.AddDate(0, 0, 50), "hour", "validation_failed/too_many_buckets"},
			{from, "day", "validation_failed/invalid_range"},
			{from.Add(-time.Hour), "day", "validation_failed/invalid_range"},
		} {
			q := params("from", rfc(from), "to", rfc(c.to), "granularity", c.gran)
			if code, pc := w.status(w.owner, "usage", q); code != http.StatusBadRequest || pc != c.code {
				t.Fatalf("%s to %v = %d %s, want %s", c.gran, c.to, code, pc, c.code)
			}
		}
	})

	t.Run("OwnerInstanceWide", func(t *testing.T) {
		w.t = t
		s := w.series(w.owner, "usage", dayRange)
		if g := s.groups(); !g[w.support] || !g[w.engineering] {
			t.Fatalf("the Owner's groups %v", g)
		}
		if n, _ := s.metric(day, w.support, "storage_bytes"); n != 1000 {
			t.Fatalf("Support storage %v: %+v", n, s)
		}
		if n, _ := s.metric(day, w.support, "threads_created"); n != 2 {
			t.Fatalf("Support threads_created %v", n)
		}
		if n, _ := s.metric(day, w.support, "runs"); n != 4 {
			t.Fatalf("Support runs %v", n)
		}
	})

	t.Run("CacheRespectsPermissionVersion", func(t *testing.T) {
		w.t = t
		q := params("from", rfc(berlinDay(2026, 10, 20)), "to", rfc(berlinDay(2026, 10, 21)), "space_id", w.support)
		start := berlinDay(2026, 10, 20).UTC()
		at := berlinDay(2026, 10, 20).Add(10 * time.Hour)
		w.message(w.allgemein, w.robin.user, "human", at)
		count := func(p gwPerson) float64 {
			n, _ := w.series(p, "usage", q).metric(start, w.support, "messages")
			return n
		}
		if n, m := count(w.sam), count(w.owner); n != 1 || m != 1 {
			t.Fatalf("first counts %v %v", n, m)
		}
		w.message(w.allgemein, w.robin.user, "human", at)
		if n, m := count(w.sam), count(w.owner); n != 1 || m != 1 {
			t.Fatalf("within stats.cache_ttl the counts changed: %v %v", n, m)
		}
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+w.samGrant+"/revoke", map[string]any{"reason": "left the team"})
		if code, pc := w.status(w.sam, "usage", q); code != http.StatusForbidden && code != http.StatusNotFound {
			t.Fatalf("Sam after the revocation = %d %s", code, pc)
		}
		w.clk.Advance(w.statsTTL + time.Second)
		if m := count(w.owner); m != 2 {
			t.Fatalf("after stats.cache_ttl the Owner's count %v, want 2", m)
		}
	})

	t.Run("NoPersonIDInAnyResponse", func(t *testing.T) {
		w.t = t
		for _, b := range w.bodies {
			for _, p := range w.people {
				if strings.Contains(b, p) {
					t.Fatalf("a response names the person %s: %s", p, b)
				}
			}
			if strings.Contains(b, w.stepTitle) || strings.Contains(b, w.stepDetail) || strings.Contains(b, w.prompt) {
				t.Fatalf("a response carries content: %s", b)
			}
		}
	})
}

// seedRuns writes the runs, tool calls and costs of statsDay: support-agent in Allgemein with two
// completed runs (1 s and 3 s), one failed with billing_mode_mismatch (2 s) and one cancelled that
// never started; dev-runner in server with one completed run (0.5 s); the tool calls search
// (completed 100 ms, failed 300 ms) and deploy (running); the costs of Support (organization 1000
// estimate, a superseded 9999 replaced by 2000 reconciled, personal 400) and Engineering
// (organization 700), and 50 EUR on another day; a thread created and one closed in Allgemein; a
// message of Eskalation whose trigger was refused with billing_mode_personal_not_holder.
func (w *statsWorld) seedRuns() {
	w.t.Helper()
	r1 := w.run(w.allgemein, w.supportAgent, "completed", "", statsDay, time.Second)
	w.run(w.allgemein, w.supportAgent, "completed", "", statsDay, 3*time.Second)
	w.run(w.allgemein, w.supportAgent, "failed", "billing_mode_mismatch", statsDay, 2*time.Second)
	w.run(w.allgemein, w.supportAgent, "cancelled", "user_cancelled", time.Time{}, 0)
	r5 := w.run(w.server, w.devRunner, "completed", "", statsDay, 500*time.Millisecond)
	w.step(r1, "search", "completed", statsDay, 100*time.Millisecond)
	w.step(r1, "search", "failed", statsDay, 300*time.Millisecond)
	w.step(r1, "deploy", "running", statsDay, 0)
	w.cost(r1, w.allgemein, w.support, w.supportAgent, "organization", 1000, "USD", true, statsDay, "")
	old := w.cost(r1, w.allgemein, w.support, w.supportAgent, "organization", 9999, "USD", true, statsDay, "")
	w.cost(r1, w.allgemein, w.support, w.supportAgent, "organization", 2000, "USD", false, statsDay, old)
	w.cost(r1, w.allgemein, w.support, w.supportAgent, "personal", 400, "USD", false, statsDay, "")
	w.cost(r5, w.server, w.engineering, w.devRunner, "organization", 700, "USD", true, statsDay, "")
	w.cost(r5, w.server, w.engineering, w.devRunner, "organization", 50, "EUR", true, statsDay.AddDate(0, 0, 3), "")
	w.sql(`INSERT INTO threads (id, channel_id, kind, created_by, created_at) VALUES ($1, $2, 'thread', $3, $4)`,
		ids.New().String(), w.allgemein.id, w.owner.user.String(), statsDay)
	w.sql(`INSERT INTO threads (id, channel_id, kind, created_by, created_at, state, closed_at) VALUES ($1, $2, 'thread', $3, $4, 'closed', $4)`,
		ids.New().String(), w.allgemein.id, w.owner.user.String(), statsDay)
	cm := w.post(w.eskalation.thread, w.prompt)
	w.sql(`UPDATE messages SET created_at = $1, trigger_state = 'failed', trigger_state_reason = 'billing_mode_personal_not_holder'
		WHERE client_msg_id = $2`, statsDay, cm)
}

// seedInstanceRows writes the rows without a channel of statsDay: webhook deliveries (2
// succeeded, 1 failed, 1 dead, 1 pending), push registrations (apns: one active, one disabled
// with token_invalid; fcm: one disabled by logout), files (an infected attachment of 1000 bytes in
// Allgemein, a scan error of 500 bytes in server) .
func (w *statsWorld) seedInstanceRows() {
	w.t.Helper()
	sub := ids.New().String()
	w.sql(`INSERT INTO webhook_subscriptions (id, agent_id, url, event_types, signing_key_kid, created_by)
		VALUES ($1, $2, 'https://hooks.example.com/in', '{message.created}', $3, $4)`, sub, w.supportAgent.String(), w.webhookKid, w.owner.user.String())
	for i, status := range []string{"succeeded", "succeeded", "failed", "dead", "pending"} {
		w.sql(`INSERT INTO webhook_deliveries (id, subscription_id, event_seq, event_id, status, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			ids.New().String(), sub, i+1, ids.New().String(), status, statsDay)
	}
	created := statsDay.AddDate(0, 0, -1)
	for i, r := range []struct {
		provider, reason string
		person           gwPerson
	}{{"apns", "", w.sam}, {"apns", "token_invalid", w.robin}, {"fcm", "logout", w.owner}} {
		var disabledAt, reason any
		if r.reason != "" {
			disabledAt, reason = statsDay, r.reason
		}
		w.sql(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, created_at, disabled_at, disabled_reason)
			VALUES ($1, $2, $3, $4, $5, $6, 'com.example.app', $7, $8, $9)`, ids.New().String(), r.person.user.String(), r.person.device.String(),
			r.person.set.Session.ID.String(), r.provider, "token-"+strconv.Itoa(i), created, disabledAt, reason)
	}
	for _, f := range []struct {
		c      channel
		size   int
		status string
	}{{w.allgemein, 1000, "infected"}, {w.server, 500, "error"}} {
		w.sql(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, scope_channel_id, upload_state,
			scan_status, scanned_at) VALUES ($1, $2, '\x00', $3, 'text/plain', 'report.txt', $4, $5, 'complete', $6, $7)`,
			ids.New().String(), "objects/"+ids.New().String(), f.size, w.owner.user.String(), f.c.id, f.status, statsDay)
	}
}
