// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// decoded is the frame as a client reads it.
type decoded struct {
	At         string `json:"at"`
	IntervalMS int64  `json:"interval_ms"`
	Scope      struct {
		Kind    string `json:"kind"`
		SpaceID string `json:"space_id"`
	} `json:"scope"`
	Runs []struct {
		RunID       string  `json:"run_id"`
		ChannelID   string  `json:"channel_id"`
		SpaceID     string  `json:"space_id"`
		AgentID     string  `json:"agent_id"`
		CostMicros  *int64  `json:"cost_micros"`
		Currency    *string `json:"currency"`
		CostEst     *bool   `json:"cost_is_estimate"`
		Approval    bool    `json:"approval_pending"`
		BillingMode string  `json:"billing_mode"`
		Step        *struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"step"`
	} `json:"runs"`
	RunsTruncated bool `json:"runs_truncated"`
	Agents        []struct {
		AgentID string `json:"agent_id"`
	} `json:"agents"`
	Rates []struct {
		Metric        string `json:"metric"`
		Dimension     string `json:"dimension"`
		Currency      string `json:"currency"`
		LastMinute    int64  `json:"last_minute"`
		CurrentMinute int64  `json:"current_minute"`
		LastMinuteSum *int64 `json:"last_minute_sum"`
	} `json:"rates"`
	SLI         []map[string]any `json:"sli"`
	Connections *map[string]int  `json:"connections"`
}

// liveWorld is the test world of 07 10 reduced to what the filter sees: the spaces Support and
// Engineering, the channels support and billing (Support) and dev (Engineering), the agents
// support-agent (operator of support), billing-agent (operator of billing, idle) and dev-runner
// (operator of dev).
type liveWorld struct {
	support, engineering                  ids.ID
	supportCh, billingCh, devCh           ids.ID
	supportAgent, billingAgent, devRunner ids.ID
	supportRun, devRun                    ids.ID
	base                                  *domain.BaseFrame
	at                                    time.Time
}

func newLiveWorld() *liveWorld {
	w := &liveWorld{support: ids.New(), engineering: ids.New(), supportCh: ids.New(), billingCh: ids.New(), devCh: ids.New(),
		supportAgent: ids.New(), billingAgent: ids.New(), devRunner: ids.New(), supportRun: ids.New(), devRun: ids.New(),
		at: time.Date(2026, 10, 10, 9, 30, 0, 0, time.UTC)}
	w.base = &domain.BaseFrame{
		At: w.at,
		Runs: []domain.LiveRun{
			{RunID: w.devRun, ThreadID: ids.New(), ChannelID: w.devCh, SpaceID: w.engineering, AgentID: w.devRunner, State: "running",
				QueuedAt: w.at.Add(-time.Second), BillingMode: "organization", CostMicros: 900, Currency: "USD", CostIsEstimate: true},
			{RunID: w.supportRun, ThreadID: ids.New(), ChannelID: w.supportCh, SpaceID: w.support, AgentID: w.supportAgent,
				State: "waiting_input", QueuedAt: w.at.Add(-time.Minute), StartedAt: w.at.Add(-50 * time.Second), BillingMode: "personal",
				StepKind: "tool_call", StepName: "Bash", ApprovalPending: true, CostMicros: 1200, Currency: "USD"},
		},
		Agents: map[ids.ID]domain.LiveAgent{
			w.supportAgent: {AgentID: w.supportAgent, Presence: "online", BillingMode: "personal"},
			w.billingAgent: {AgentID: w.billingAgent, Presence: "offline", BillingMode: "organization"},
			w.devRunner:    {AgentID: w.devRunner, Presence: "degraded", BillingMode: "organization", ReportedBillingMode: "organization"},
		},
		Channels: map[ids.ID]domain.ChannelPlace{
			w.supportCh: {SpaceID: w.support, OperatorID: w.supportAgent},
			w.billingCh: {SpaceID: w.support, OperatorID: w.billingAgent},
			w.devCh:     {SpaceID: w.engineering, OperatorID: w.devRunner},
		},
		ChannelRates: map[ids.ID][]domain.RateCell{
			w.supportCh: {
				{Metric: "messages_created", Dimension: "human", Last: 3, Current: 1},
				{Metric: "messages_created", Dimension: "agent", Last: 2},
				{Metric: "cost", Dimension: "personal", Currency: "USD", Last: 1, LastSum: 1200},
				{Metric: "sli.run_start", Last: 4},
			},
			w.devCh: {
				{Metric: "messages_created", Dimension: "human", Last: 10, Current: 5},
				{Metric: "cost", Dimension: "organization", Currency: "USD", Last: 1, LastSum: 900},
			},
		},
		InstanceRates: []domain.RateCell{{Metric: "catchups", Last: 7}, {Metric: "billing_mode_switches", Dimension: "personal", Current: 1}},
		SLI:           []domain.SLICell{{Metric: "sli.api_availability", Total: 100, Good: 99}, {Metric: "sli.reconnect"}},
		Connections:   &domain.Connections{User: 2, Agent: 3},
	}
	return w
}

// limits are generous limits: nothing is cut.
func limits() domain.LiveLimits {
	return domain.LiveLimits{MaxRuns: 200, PayloadMax: 1 << 20, Interval: time.Second}
}

// filter runs the filter and decodes the frame.
func filter(t *testing.T, base *domain.BaseFrame, r domain.Receiver, l domain.LiveLimits) (decoded, []byte) {
	t.Helper()
	b, _, err := domain.FilterLive(base, r, l)
	if err != nil {
		t.Fatalf("FilterLive: %v", err)
	}
	noNull(t, b)
	var d decoded
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return d, b
}

// noNull fails when the frame b contains a JSON null (ADR 0053; the hub refuses such a payload)
// or a required list is not an array.
func noNull(t *testing.T, b []byte) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if tok == nil {
			t.Fatalf("JSON null in the frame %s", b)
		}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"runs", "agents", "rates"} {
		if v := top[member]; len(v) == 0 || v[0] != '[' {
			t.Fatalf("member %s is %s, want an array", member, v)
		}
	}
}

// set returns the channel set of chans.
func set(chans ...ids.ID) map[ids.ID]struct{} {
	out := map[ids.ID]struct{}{}
	for _, c := range chans {
		out[c] = struct{}{}
	}
	return out
}

// rate returns the rate row of metric, dimension and currency; false when absent.
func rate(d decoded, metric, dimension, currency string) (int64, int64, bool) {
	for _, r := range d.Rates {
		if r.Metric == metric && r.Dimension == dimension && r.Currency == currency {
			return r.LastMinute, r.CurrentMinute, true
		}
	}
	return 0, 0, false
}

// agentIDs returns the agents of d.
func agentIDs(d decoded) []string {
	var out []string
	for _, a := range d.Agents {
		out = append(out, a.AgentID)
	}
	return out
}

// TestFilter (VZ-T05 in memory): a space manager of Support who also reads dev sees only the runs,
// channels, agents and rates of Support, without the rows without a channel, without sli and
// connections, and without any cost member when cost.read.aggregate is denied; the owner keeps
// everything; an agent without a run or an operator role in the set disappears.
func TestFilter(t *testing.T) {
	t.Parallel()
	w := newLiveWorld()

	t.Run("space manager with costs", func(t *testing.T) {
		t.Parallel()
		d, _ := filter(t, w.base, domain.Receiver{SpaceID: w.support, Visible: set(w.supportCh, w.billingCh, w.devCh), Costs: true}, limits())
		if d.Scope.Kind != "space" || d.Scope.SpaceID != w.support.String() || d.IntervalMS != 1000 || d.At != "2026-10-10T09:30:00.000Z" {
			t.Fatalf("scope, interval or at: %+v", d)
		}
		if len(d.Runs) != 1 || d.Runs[0].RunID != w.supportRun.String() || d.RunsTruncated {
			t.Fatalf("runs = %+v", d.Runs)
		}
		r := d.Runs[0]
		if r.CostMicros == nil || *r.CostMicros != 1200 || r.Currency == nil || r.CostEst == nil || !r.Approval || r.Step == nil ||
			r.Step.Kind != "tool_call" || r.Step.Name != "Bash" || r.BillingMode != "personal" {
			t.Fatalf("support run = %+v", r)
		}
		want := []string{w.supportAgent.String(), w.billingAgent.String()}
		slices.Sort(want)
		if got := agentIDs(d); !slices.Equal(got, want) {
			t.Fatalf("agents %v, want support-agent and billing-agent %v", got, want)
		}
		if last, cur, ok := rate(d, "messages_created", "", ""); !ok || last != 5 || cur != 1 {
			t.Fatalf("messages_created summed = %d %d %v (rates %+v)", last, cur, ok, d.Rates)
		}
		if last, _, ok := rate(d, "messages_created", "human", ""); !ok || last != 3 {
			t.Fatalf("messages_created human = %d %v", last, ok)
		}
		if _, _, ok := rate(d, "cost", "", "USD"); !ok {
			t.Fatal("cost rate missing with cost.read.aggregate")
		}
		for _, metric := range []string{"catchups", "billing_mode_switches", "sli.run_start"} {
			if _, _, ok := rate(d, metric, "", ""); ok {
				t.Errorf("%s in a space frame", metric)
			}
		}
		if d.SLI != nil || d.Connections != nil {
			t.Fatalf("sli %v or connections %v in a space frame", d.SLI, d.Connections)
		}
	})

	t.Run("space manager without costs", func(t *testing.T) {
		t.Parallel()
		_, raw := filter(t, w.base, domain.Receiver{SpaceID: w.support, Visible: set(w.supportCh, w.billingCh)}, limits())
		for _, member := range []string{"cost_micros", "currency", "cost_is_estimate", `"cost"`} {
			if strings.Contains(string(raw), member) {
				t.Errorf("%s in a frame without cost.read.aggregate: %s", member, raw)
			}
		}
	})

	t.Run("owner", func(t *testing.T) {
		t.Parallel()
		d, _ := filter(t, w.base, domain.Receiver{Visible: set(w.supportCh, w.billingCh, w.devCh), Costs: true}, limits())
		if d.Scope.Kind != "instance" || d.Scope.SpaceID != "" || len(d.Runs) != 2 || d.Runs[0].RunID != w.devRun.String() {
			t.Fatalf("owner frame %+v", d)
		}
		if len(d.Agents) != 3 {
			t.Fatalf("agents %v", agentIDs(d))
		}
		if last, cur, ok := rate(d, "messages_created", "", ""); !ok || last != 15 || cur != 6 {
			t.Fatalf("messages_created = %d %d", last, cur)
		}
		if last, _, ok := rate(d, "catchups", "", ""); !ok || last != 7 {
			t.Fatal("catchups missing for the owner")
		}
		if _, cur, ok := rate(d, "billing_mode_switches", "personal", ""); !ok || cur != 1 {
			t.Fatal("billing_mode_switches missing for the owner")
		}
		if len(d.SLI) != 2 || d.Connections == nil || (*d.Connections)["agent"] != 3 {
			t.Fatalf("sli %v connections %v", d.SLI, d.Connections)
		}
		for _, s := range d.SLI {
			if _, has := s["ratio"]; has != (s["metric"] == "sli.api_availability") {
				t.Errorf("ratio of %v", s)
			}
		}
	})

	t.Run("no visible run, no channel, empty base", func(t *testing.T) {
		t.Parallel()
		d, _ := filter(t, w.base, domain.Receiver{SpaceID: w.support, Visible: set(w.billingCh)}, limits())
		if len(d.Runs) != 0 || d.RunsTruncated {
			t.Fatalf("runs %+v", d.Runs)
		}
		filter(t, w.base, domain.Receiver{Visible: set()}, limits())
		filter(t, &domain.BaseFrame{At: w.at}, domain.Receiver{Visible: set(), Costs: true}, limits())
	})

	t.Run("agent without run or operator role in the set", func(t *testing.T) {
		t.Parallel()
		d, _ := filter(t, w.base, domain.Receiver{SpaceID: w.support, Visible: set(w.supportCh), Costs: true}, limits())
		if got := agentIDs(d); !slices.Equal(got, []string{w.supportAgent.String()}) {
			t.Fatalf("agents %v, want only support-agent", got)
		}
		if _, _, ok := rate(d, "messages_created", "", ""); !ok {
			t.Fatal("rates of support missing")
		}
	})
}

// TestSizeCut (VZ-T08): a frame with more runs than stats.live_max_runs and a payload over
// sync.event_payload_max_bytes is cut below the bound with runs_truncated; the newest runs stay,
// one more run would not fit, and the frame has no member truncated.
func TestSizeCut(t *testing.T) {
	t.Parallel()
	w := newLiveWorld()
	base := *w.base
	base.Runs = nil
	for i := range 300 {
		base.Runs = append(base.Runs, domain.LiveRun{RunID: ids.New(), ThreadID: ids.New(), ChannelID: w.supportCh, SpaceID: w.support,
			AgentID: w.supportAgent, State: "running", QueuedAt: w.at.Add(-time.Duration(i) * time.Second), BillingMode: "organization",
			StepKind: "tool_call", StepName: "Bash"})
	}
	l := domain.LiveLimits{MaxRuns: 250, PayloadMax: 8192, Interval: time.Second}
	r := domain.Receiver{Visible: set(w.supportCh, w.billingCh, w.devCh), Costs: true}
	b, cutBySize, err := domain.FilterLive(&base, r, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) >= l.PayloadMax || !cutBySize {
		t.Fatalf("payload %d bytes, cut %v; bound %d", len(b), cutBySize, l.PayloadMax)
	}
	var d decoded
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	if _, has := top["truncated"]; has || !d.RunsTruncated || len(d.Runs) == 0 {
		t.Fatalf("truncated member %v, runs_truncated %v, %d runs", has, d.RunsTruncated, len(d.Runs))
	}
	for i, run := range d.Runs {
		if run.RunID != base.Runs[i].RunID.String() {
			t.Fatalf("run %d is %s, want the newest runs in order", i, run.RunID)
		}
	}
	// One more run would reach the bound.
	bigger, _, err := domain.FilterLive(&base, r, domain.LiveLimits{MaxRuns: len(d.Runs) + 1, PayloadMax: 1 << 20, Interval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(bigger) < l.PayloadMax {
		t.Fatalf("%d runs plus one fit in %d bytes (%d): the cut kept too few", len(d.Runs), l.PayloadMax, len(bigger))
	}

	t.Run("only the run limit", func(t *testing.T) {
		t.Parallel()
		b, cutBySize, err := domain.FilterLive(&base, r, domain.LiveLimits{MaxRuns: 250, PayloadMax: 1 << 20, Interval: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		var d decoded
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		if cutBySize || len(d.Runs) != 250 || !d.RunsTruncated {
			t.Fatalf("cut %v, %d runs, truncated %v", cutBySize, len(d.Runs), d.RunsTruncated)
		}
	})

	t.Run("nothing fits", func(t *testing.T) {
		t.Parallel()
		if _, _, err := domain.FilterLive(&base, r, domain.LiveLimits{MaxRuns: 250, PayloadMax: 64, Interval: time.Second}); !errors.Is(err, domain.ErrFrameTooLarge) {
			t.Fatalf("err = %v, want ErrFrameTooLarge", err)
		}
	})
}

// BenchmarkFilterLive measures the filter of one receiver over a base frame with
// stats.live_max_runs (200) runs in 20 channels of two spaces, 40 agents and rate cells per
// channel (report of M6-15, 25 5.1 item 4).
func BenchmarkFilterLive(b *testing.B) {
	w := newLiveWorld()
	base := *w.base
	base.Runs, base.Channels, base.ChannelRates = nil, map[ids.ID]domain.ChannelPlace{}, map[ids.ID][]domain.RateCell{}
	base.Agents = map[ids.ID]domain.LiveAgent{}
	visible := map[ids.ID]struct{}{}
	var chans []ids.ID
	for i := range 20 {
		ch, op := ids.New(), ids.New()
		space := w.support
		if i%2 == 1 {
			space = w.engineering
		}
		chans = append(chans, ch)
		visible[ch] = struct{}{}
		base.Channels[ch] = domain.ChannelPlace{SpaceID: space, OperatorID: op}
		base.Agents[op] = domain.LiveAgent{AgentID: op, Presence: "online", BillingMode: "organization"}
		base.ChannelRates[ch] = []domain.RateCell{{Metric: "messages_created", Dimension: "human", Last: 3, Current: 1},
			{Metric: "tool_calls", Dimension: "completed", Last: 4, LastSum: 900}, {Metric: "cost", Dimension: "organization", Currency: "USD", Last: 1, LastSum: 10}}
	}
	for i := range 200 {
		ch := chans[i%len(chans)]
		base.Runs = append(base.Runs, domain.LiveRun{RunID: ids.New(), ThreadID: ids.New(), ChannelID: ch, SpaceID: base.Channels[ch].SpaceID,
			AgentID: base.Channels[ch].OperatorID, State: "running", QueuedAt: w.at.Add(-time.Duration(i) * time.Second), BillingMode: "organization",
			StepKind: "tool_call", StepName: "Bash", CostMicros: 10, Currency: "USD"})
	}
	l := domain.Limits{MaxRuns: 200, PayloadMax: 1 << 20, Interval: time.Second}
	r := domain.Receiver{SpaceID: w.support, Visible: visible, Costs: true}
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := domain.FilterLive(&base, r, l); err != nil {
			b.Fatal(err)
		}
	}
}
