// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// viewsCanary is the canary of VZ-T07 for the views: the title of a thread with a run in the
// timeline range, allowed only as the thread title of the run's chat for a person who may read the
// thread (25 4 rule 6); the prompt, step title and step detail of statsWorld are the other
// canaries.
const viewsCanary = "canary-views-7e2a"

// viewsWorld is the statistics world of M6-05 with the fixtures of the views (M6-14): minute rows
// of stats_minutes in one hour three hours before now, the SLI rows of the traffic light, runs in
// that hour, and a heartbeat of support-agent with a rate limit. The clock of the process is the
// system clock, so every fixture is placed relative to now and stays inside
// retention.stats_minutes whatever day the test runs. stats.flush_interval is an hour, so the own
// measurement of the requests of the test never adds rows; stats.timeline_max_runs is three.
type viewsWorld struct {
	*statsWorld
	at       time.Time
	resetsAt time.Time
}

// viewsMaxRuns is stats.timeline_max_runs of the world.
const viewsMaxRuns = 3

// newViewsWorld starts the world and writes its fixtures.
func newViewsWorld(t *testing.T) *viewsWorld {
	t.Helper()
	w := &viewsWorld{statsWorld: newStatsWorld(t, "BOTHY_STATS__TIMELINE_MAX_RUNS=3", "BOTHY_STATS__FLUSH_INTERVAL=1h")}
	w.at = w.clk.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)
	w.sql(`SELECT stats_minutes_ensure_partition($1::date)`, w.at.Format(time.DateOnly))
	m := w.at.Add(10 * time.Minute)
	sa, dr := w.supportAgent.String(), w.devRunner.String()
	for _, r := range []struct {
		metric, channel, agent, dimension, currency string
		count, sum, good                            int64
	}{
		{"runs_finished", w.allgemein.id, sa, "completed", "", 2, 84000, 0},
		{"messages_created", w.eskalation.id, "", "human", "", 1, 0, 0},
		{"messages_created", w.server.id, "", "human", "", 5, 0, 0},
		{"cost", w.allgemein.id, sa, "organization", "USD", 1, 1000, 0},
		{"cost", w.server.id, dr, "organization", "USD", 1, 700, 0},
		{"invocations_denied", w.allgemein.id, sa, "i2_denied", "", 2, 0, 0},
		{"invocations_denied", w.allgemein.id, sa, "billing_mode_mismatch", "", 1, 0, 0},
		{"billing_mode_switches", "", sa, "organization", "", 1, 0, 0},
		{"billing_mode_switches", "", dr, "personal", "", 3, 0, 0},
		{"catchups", "", "", "", "", 4, 400, 0},
		// The traffic light: run start red (budget spent), delivery latency amber, turn duration
		// green, push acceptance green, reconnect below stats.slo_min_events.
		{"sli.run_start", w.allgemein.id, sa, "", "", 1000, 0, 990},
		{"sli.delivery_latency", "", "", "", "", 1000, 0, 996},
		{"sli.turn_duration", w.server.id, dr, "", "", 500, 0, 490},
		{"sli.push_acceptance", "", "", "apns", "", 200, 0, 200},
		{"sli.reconnect", "", "", "", "", 10, 0, 10},
	} {
		w.sql(`INSERT INTO stats_minutes (bucket_start, metric, channel_id, agent_id, dimension, currency, count, sum, good)
			VALUES ($1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, NULLIF($6, ''), $7, $8, $9)`,
			m, r.metric, r.channel, r.agent, r.dimension, r.currency, r.count, r.sum, r.good)
	}
	// Five runs end in the hour: three in Support, two in Engineering; one in a thread whose title
	// is the canary, with a step whose title and detail are the canaries of statsWorld.
	titled := ids.New().String()
	w.sql(`INSERT INTO threads (id, channel_id, kind, title, created_by, created_at) VALUES ($1, $2, 'thread', $3, $4, '2026-09-01T00:00:00Z')`,
		titled, w.allgemein.id, viewsCanary, w.owner.user.String())
	r := w.run(channel{id: w.allgemein.id, thread: titled}, w.supportAgent, "completed", "", w.at.Add(5*time.Minute), 10*time.Minute)
	w.step(r, "search", "completed", w.at.Add(6*time.Minute), time.Second)
	w.run(w.allgemein, w.supportAgent, "failed", "billing_mode_mismatch", w.at.Add(15*time.Minute), time.Minute)
	w.run(w.eskalation, w.supportAgent, "completed", "", w.at.Add(20*time.Minute), time.Minute)
	w.run(w.server, w.devRunner, "completed", "", w.at.Add(25*time.Minute), time.Minute)
	w.run(w.server, w.devRunner, "completed", "", w.at.Add(30*time.Minute), time.Minute)
	// The first heartbeat of an agent on a node is stored at once (D-M7-17-10).
	w.resetsAt = w.at.Add(5 * time.Hour)
	w.heartbeat(w.supportAgent, `{"state":"online","billing_modes_supported":["organization"],"billing_evidence":{"key_type":"api_key"},
		"details":{"adapter":{"name":"example-adapter","version":"1.0.0"},"runtime":{"name":"example-runtime"}},"runtime_version":"9.9.9",
		"rate_limit":{"state":"near_limit","resets_at":"`+rfc(w.resetsAt)+`"}}`)
	return w
}

// heartbeat sends sendHeartbeat as agent with a fresh token and requires 200.
func (w *viewsWorld) heartbeat(agent ids.ID, body string) {
	w.t.Helper()
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	tok := w.agentToken(agent, key, time.Now().Add(time.Hour))
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	const path = "/api/v1/agents/me/heartbeat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.base+path, bytes.NewReader([]byte(body)))
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DPoP", identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: http.MethodPost, HTU: gwPublicURL + path,
		IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(tok)}))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		w.t.Fatalf("sendHeartbeat = %d %s", resp.StatusCode, b)
	}
}

// hour is the range of the fixtures.
func (w *viewsWorld) hour(kv ...string) url.Values {
	q := params(append([]string{"from", rfc(w.at), "to", rfc(w.at.Add(time.Hour))}, kv...)...)
	return q
}

// view calls GET /api/v1/stats/<op> as p, requires 200 and decodes the body into out.
func (w *viewsWorld) view(p gwPerson, op string, q url.Values, out any) {
	w.t.Helper()
	code, b := w.get(p, "/api/v1/stats/"+op, q)
	if code != http.StatusOK {
		w.t.Fatalf("%s %v = %d %s", op, q, code, b)
	}
	if err := json.Unmarshal(b, out); err != nil {
		w.t.Fatal(err)
	}
}

// timelineDoc is a StatsTimeline as the client reads it.
type timelineDoc struct {
	Buckets []struct {
		Metric    string `json:"metric"`
		ChannelID string `json:"channel_id"`
		AgentID   string `json:"agent_id"`
		Dimension string `json:"dimension"`
		Currency  string `json:"currency"`
		Count     int64  `json:"count"`
		Sum       int64  `json:"sum"`
	} `json:"buckets"`
	Runs []struct {
		RunID      string `json:"run_id"`
		ChannelID  string `json:"channel_id"`
		AgentID    string `json:"agent_id"`
		QueuedAt   string `json:"queued_at"`
		FinishedAt string `json:"finished_at"`
		State      string `json:"state"`
	} `json:"runs"`
	RunsTruncated bool `json:"runs_truncated"`
}

// backendDoc is a StatsBackendReport as the client reads it.
type backendDoc struct {
	Items []struct {
		AgentID               string   `json:"agent_id"`
		BillingMode           string   `json:"billing_mode"`
		BillingModesSupported []string `json:"billing_modes_supported"`
		Switches              int64    `json:"switches"`
		InvocationsDenied     struct {
			I2Denied            int64 `json:"i2_denied"`
			BillingModeMismatch int64 `json:"billing_mode_mismatch"`
		} `json:"invocations_denied"`
		Costs *[]struct {
			BillingMode  string `json:"billing_mode"`
			Currency     string `json:"currency"`
			AmountMicros int64  `json:"amount_micros"`
			IsEstimate   bool   `json:"is_estimate"`
		} `json:"costs"`
		RateLimit *struct {
			State    string    `json:"state"`
			ResetsAt time.Time `json:"resets_at"`
		} `json:"rate_limit"`
	} `json:"items"`
}

// TestStatsViews is M6-14 end to end through a started api process: the timeline of a space
// manager (VZ-T05 for REST) and of the Owner, the range rules (25 11), the traffic light (owner
// only, own measurement), the backend view and no person and no content in any answer (VZ-T07).
// The subtests share one world and run in order.
func TestStatsViews(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t)
	support := map[string]bool{w.allgemein.id: true, w.eskalation.id: true}

	t.Run("Timeline_SpaceManager", func(t *testing.T) {
		w.t = t
		var tl timelineDoc
		w.view(w.sam, "timeline", w.hour("space_id", w.support), &tl)
		var metrics []string
		for _, b := range tl.Buckets {
			if b.ChannelID == "" || !support[b.ChannelID] {
				t.Fatalf("Sam sees a row outside Support: %+v", b)
			}
			metrics = append(metrics, b.Metric)
		}
		slices.Sort(metrics)
		want := []string{"cost", "invocations_denied", "invocations_denied", "messages_created", "runs_finished", "sli.run_start"}
		if !slices.Equal(metrics, want) {
			t.Fatalf("Sam's metrics %v, want %v", metrics, want)
		}
		if len(tl.Runs) != 3 || tl.RunsTruncated {
			t.Fatalf("Sam's runs %+v truncated %v", tl.Runs, tl.RunsTruncated)
		}
		for i, r := range tl.Runs {
			if !support[r.ChannelID] || r.AgentID != w.supportAgent.String() || r.FinishedAt == "" {
				t.Fatalf("run %+v", r)
			}
			if i > 0 && tl.Runs[i-1].QueuedAt > r.QueuedAt {
				t.Fatalf("runs not ordered by queued_at: %+v", tl.Runs)
			}
		}
		if code, pc := w.status(w.sam, "timeline", w.hour()); code != http.StatusForbidden || pc != "forbidden" {
			t.Fatalf("Sam without space = %d %s", code, pc)
		}
		if code, pc := w.status(w.sam, "timeline", w.hour("space_id", w.engineering)); code != http.StatusNotFound || pc != "not_found" {
			t.Fatalf("Sam on Engineering = %d %s", code, pc)
		}
		if code, pc := w.status(w.robin, "timeline", w.hour("space_id", w.support)); code != http.StatusForbidden || pc != "forbidden" {
			t.Fatalf("Robin = %d %s", code, pc)
		}
		var only timelineDoc
		q := w.hour("space_id", w.support)
		q["metric"] = []string{"cost", "catchups"}
		w.view(w.sam, "timeline", q, &only)
		if len(only.Buckets) != 1 || only.Buckets[0].Metric != "cost" || only.Buckets[0].Sum != 1000 || only.Buckets[0].Currency != "USD" {
			t.Fatalf("Sam's cost rows %+v", only.Buckets)
		}
	})

	t.Run("Timeline_Owner", func(t *testing.T) {
		w.t = t
		var tl timelineDoc
		w.view(w.owner, "timeline", w.hour(), &tl)
		instance, costs, engineering := 0, 0, false
		for _, b := range tl.Buckets {
			switch b.ChannelID {
			case "":
				instance++
			case w.server.id:
				engineering = true
			}
			if b.Metric == "cost" {
				costs++
			}
		}
		// switches (2), catchups and the SLI rows without a channel (delivery, push, reconnect).
		if instance != 6 || costs != 2 || !engineering || len(tl.Buckets) != 15 {
			t.Fatalf("the Owner's rows: %d without a channel, %d cost, %d in all: %+v", instance, costs, len(tl.Buckets), tl.Buckets)
		}
		if len(tl.Runs) != viewsMaxRuns || !tl.RunsTruncated {
			t.Fatalf("the Owner's runs %d truncated %v, want %d and true", len(tl.Runs), tl.RunsTruncated, viewsMaxRuns)
		}
		var spaced timelineDoc
		w.view(w.owner, "timeline", w.hour("space_id", w.engineering), &spaced)
		for _, b := range spaced.Buckets {
			if b.ChannelID != w.server.id {
				t.Fatalf("the Owner's Engineering timeline has a row outside it: %+v", b)
			}
		}
		if len(spaced.Runs) != 2 || spaced.RunsTruncated {
			t.Fatalf("the Owner's Engineering runs %+v", spaced.Runs)
		}
	})

	t.Run("Timeline_Range", func(t *testing.T) {
		w.t = t
		now := w.clk.Now().UTC().Truncate(time.Minute)
		old := now.Add(-91 * 24 * time.Hour)
		for _, c := range []struct {
			from, to time.Time
			code     string
		}{
			{w.at, w.at.Add(24*time.Hour + time.Minute), "validation_failed/range_too_long"},
			{old, old.Add(time.Hour), "validation_failed/outside_retention"},
			{w.at.Add(30 * time.Second), w.at.Add(time.Hour), "validation_failed/not_full_minute"},
			{w.at, w.at, "validation_failed/invalid_range"},
		} {
			q := params("from", rfc(c.from), "to", rfc(c.to))
			if code, pc := w.status(w.owner, "timeline", q); code != http.StatusBadRequest || pc != c.code {
				t.Fatalf("%v to %v = %d %s, want %s", c.from, c.to, code, pc, c.code)
			}
		}
	})

	t.Run("Slo_OwnerOnly", func(t *testing.T) {
		w.t = t
		var r struct {
			MeasuredBy string `json:"measured_by"`
			Items      []struct {
				Slo         string   `json:"slo"`
				Metric      string   `json:"metric"`
				Objective   float64  `json:"objective"`
				ThresholdMs *int     `json:"threshold_ms"`
				PeriodDays  int      `json:"period_days"`
				Total       int64    `json:"total"`
				Good        int64    `json:"good"`
				Budget      *float64 `json:"budget_remaining"`
				State       string   `json:"state"`
			} `json:"items"`
		}
		w.view(w.owner, "slo", nil, &r)
		if r.MeasuredBy != "own" || len(r.Items) != 6 {
			t.Fatalf("report %+v", r)
		}
		want := map[string]string{"sli.run_start": "red", "sli.delivery_latency": "amber", "sli.turn_duration": "green",
			"sli.push_acceptance": "green", "sli.reconnect": "insufficient_data"}
		for _, e := range r.Items {
			if s, ok := want[e.Metric]; ok && e.State != s {
				t.Fatalf("%s = %s, want %s: %+v", e.Metric, e.State, s, e)
			}
			if e.PeriodDays != 30 || e.Slo == "" || e.Objective <= 0 {
				t.Fatalf("entry %+v", e)
			}
			if e.Metric == "sli.delivery_latency" && (e.Total != 1000 || e.Good != 996 || e.ThresholdMs == nil || e.Budget == nil) {
				t.Fatalf("delivery latency %+v", e)
			}
		}
		if code, pc := w.status(w.sam, "slo", nil); code != http.StatusForbidden || pc != "forbidden" {
			t.Fatalf("Sam = %d %s", code, pc)
		}
	})

	t.Run("Backends", func(t *testing.T) {
		w.t = t
		code, raw := w.get(w.sam, "/api/v1/stats/backends", w.hour("space_id", w.support))
		var r backendDoc
		if code != http.StatusOK || json.Unmarshal(raw, &r) != nil {
			t.Fatalf("Sam's backends = %d %s", code, raw)
		}
		if len(r.Items) != 1 || r.Items[0].AgentID != w.supportAgent.String() {
			t.Fatalf("Sam's agents %s", raw)
		}
		a := r.Items[0]
		if a.Switches != 1 || a.InvocationsDenied.I2Denied != 2 || a.InvocationsDenied.BillingModeMismatch != 1 ||
			!slices.Equal(a.BillingModesSupported, []string{"organization"}) || a.Costs == nil || len(*a.Costs) != 1 ||
			(*a.Costs)[0].AmountMicros != 1000 || (*a.Costs)[0].Currency != "USD" || !(*a.Costs)[0].IsEstimate ||
			a.RateLimit == nil || a.RateLimit.State != "near_limit" || !a.RateLimit.ResetsAt.Equal(w.resetsAt) {
			t.Fatalf("support-agent %s", raw)
		}
		// The schema has no member for a runtime, an adapter, a version or evidence (D-M6-14-4).
		for _, banned := range []string{"adapter", "runtime", "9.9.9", "example-", "evidence", "key_type", "api_key", "credential"} {
			if bytes.Contains(raw, []byte(banned)) {
				t.Fatalf("the backend view carries %q: %s", banned, raw)
			}
		}
		var item map[string]json.RawMessage
		var doc struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Items) != 1 || json.Unmarshal(doc.Items[0], &item) != nil {
			t.Fatal(err)
		}
		members := []string{"agent_id", "billing_mode", "reported_billing_mode", "billing_modes_supported", "switches", "invocations_denied",
			"costs", "rate_limit"}
		for k := range item {
			if !slices.Contains(members, k) {
				t.Fatalf("member %q outside the schema: %s", k, raw)
			}
		}
		var owner backendDoc
		w.view(w.owner, "backends", w.hour(), &owner)
		if len(owner.Items) != 2 {
			t.Fatalf("the Owner's agents %+v", owner.Items)
		}
		for _, it := range owner.Items {
			if it.AgentID == w.devRunner.String() && (it.Switches != 3 || it.Costs == nil || len(*it.Costs) != 1 || it.RateLimit != nil) {
				t.Fatalf("dev-runner %+v", it)
			}
		}
		if code, pc := w.status(w.sam, "backends", w.hour()); code != http.StatusForbidden || pc != "forbidden" {
			t.Fatalf("Sam without space = %d %s", code, pc)
		}
	})

	t.Run("NoPeopleNoContent", func(t *testing.T) {
		w.t = t
		if len(w.bodies) < 10 {
			t.Fatalf("only %d answers were read", len(w.bodies))
		}
		named := false
		for _, b := range w.bodies {
			for _, p := range w.people {
				if strings.Contains(b, p) {
					t.Fatalf("an answer names the person %s: %s", p, b)
				}
			}
			rest, titles := withoutChatTitles(t, []byte(b))
			named = named || slices.Contains(titles, viewsCanary)
			for _, c := range []string{viewsCanary, w.stepTitle, w.stepDetail, w.prompt} {
				if strings.Contains(string(rest), c) {
					t.Fatalf("an answer carries the content %q outside the chat of a run: %s", c, b)
				}
			}
		}
		if !named {
			t.Fatal("no timeline names the chat of the titled thread for a person who may read it")
		}
	})
}
