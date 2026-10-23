// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The instrument names of signals.md 4.3.
const (
	startLatency = "bothy.run.start.latency"
	duration     = "bothy.run.duration"
	finished     = "bothy.run.finished"
	active       = "bothy.run.active"
)

// transition returns a committed transition of a run of the agent alex-bot.
func transition(from, to domain.State, reason string) app.Transitioned {
	return app.Transitioned{
		Run:         runs.RunSummary{State: string(to), StateReason: reason, BillingModeExpected: "organization"},
		From:        from,
		To:          to,
		AgentHandle: "alex-bot",
	}
}

// record records t on fresh instruments and returns what the reader collected by name.
func record(t *testing.T, ts ...app.Transitioned) map[string]metricdata.Metrics {
	t.Helper()
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range ts {
		m.RunTransitioned(context.Background(), tr)
	}
	return metricstest.Collect(t, reader)
}

// only returns the single point of name; t fails when there is not exactly one.
func only(t *testing.T, got map[string]metricdata.Metrics, name string) metricstest.Point {
	t.Helper()
	ps := metricstest.Points(got[name])
	if len(ps) != 1 {
		t.Fatalf("%s: %d points %v, want 1", name, len(ps), ps)
	}
	return ps[0]
}

// equalAttrs reports whether have holds exactly want.
func equalAttrs(have, want map[string]string) bool {
	if len(have) != len(want) {
		return false
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func TestRunTransitioned_StartLatency(t *testing.T) {
	cases := []struct {
		to      domain.State
		reason  string
		outcome string
	}{
		{domain.StateRunning, "", "started"},
		{domain.StateFailed, domain.RunReasonStale, "failed"},
		{domain.StateCancelled, domain.RunReasonUserCancelled, "cancelled"},
	}
	for _, c := range cases {
		t.Run(string(c.to), func(t *testing.T) {
			tr := transition(domain.StateQueued, c.to, c.reason)
			tr.QueuedFor = 1500 * time.Millisecond
			got := record(t, tr)
			p := only(t, got, startLatency)
			want := map[string]string{"bothy.agent.handle": "alex-bot", "bothy.outcome": c.outcome, "bothy.billing_mode": "organization"}
			if !equalAttrs(p.Attrs, want) || p.Count != 1 || p.Sum != 1.5 {
				t.Errorf("start latency %+v, want attrs %v, count 1, sum 1.5", p, want)
			}
			if !slices.Equal(p.Bounds, metrics.BucketsSeconds) {
				t.Errorf("bounds %v, want BucketsSeconds", p.Bounds)
			}
			// A run that never started has no duration.
			if _, has := got[duration]; has {
				t.Errorf("queued -> %s recorded %s", c.to, duration)
			}
		})
	}
}

func TestRunTransitioned_BillingModeUsedWins(t *testing.T) {
	tr := transition(domain.StateQueued, domain.StateRunning, "")
	tr.Run.BillingModeUsed = "personal"
	p := only(t, record(t, tr), startLatency)
	if p.Attrs["bothy.billing_mode"] != "personal" {
		t.Errorf("billing mode %q, want the reported personal", p.Attrs["bothy.billing_mode"])
	}
}

func TestRunTransitioned_EndStates(t *testing.T) {
	cases := []struct {
		from   domain.State
		to     domain.State
		reason string
	}{
		{domain.StateRunning, domain.StateCompleted, ""},
		{domain.StateRunning, domain.StateFailed, domain.RunReasonRuntimeError},
		{domain.StateRunning, domain.StateCancelled, domain.RunReasonUserDisabled},
		{domain.StateWaitingInput, domain.StateFailed, domain.RunReasonInterrupted},
		{domain.StateWaitingInput, domain.StateCancelled, domain.RunReasonUserCancelled},
	}
	for _, c := range cases {
		t.Run(string(c.from)+"->"+string(c.to), func(t *testing.T) {
			tr := transition(c.from, c.to, c.reason)
			tr.Started = true
			tr.Ran = 90 * time.Second
			got := record(t, tr)
			if _, has := got[startLatency]; has {
				t.Errorf("%s -> %s recorded %s", c.from, c.to, startLatency)
			}
			d := only(t, got, duration)
			if want := map[string]string{"bothy.agent.handle": "alex-bot", "bothy.run.state": string(c.to)}; !equalAttrs(d.Attrs, want) || d.Count != 1 || d.Sum != 90 {
				t.Errorf("duration %+v, want attrs %v, count 1, sum 90", d, want)
			}
			if !slices.Equal(d.Bounds, metrics.BucketsLong) {
				t.Errorf("bounds %v, want BucketsLong", d.Bounds)
			}
			f := only(t, got, finished)
			want := map[string]string{"bothy.agent.handle": "alex-bot", "bothy.run.state": string(c.to)}
			if c.reason != "" {
				want["bothy.reason"] = c.reason
			}
			if !equalAttrs(f.Attrs, want) || f.Value != 1 {
				t.Errorf("finished %+v, want attrs %v and 1", f, want)
			}
		})
	}
}

func TestRunTransitioned_QueuedToEndCountsFinishedWithoutDuration(t *testing.T) {
	tr := transition(domain.StateQueued, domain.StateFailed, domain.RunReasonAgentUnavailable)
	tr.QueuedFor = 2 * time.Second
	got := record(t, tr)
	f := only(t, got, finished)
	if want := map[string]string{"bothy.agent.handle": "alex-bot", "bothy.run.state": "failed", "bothy.reason": "agent_unavailable"}; !equalAttrs(f.Attrs, want) {
		t.Errorf("finished %v, want %v", f.Attrs, want)
	}
	if _, has := got[duration]; has {
		t.Error("a run that never started recorded a duration")
	}
}

func TestRunTransitioned_NonEndTransitionRecordsNothing(t *testing.T) {
	got := record(t,
		transition(domain.StateRunning, domain.StateWaitingInput, ""),
		transition(domain.StateWaitingInput, domain.StateRunning, ""),
	)
	for name, m := range got {
		if n := len(metricstest.Points(m)); n != 0 {
			t.Errorf("%s: %d points, want none", name, n)
		}
	}
}

func TestRunTransitioned_HandleOutsideThePatternIsOther(t *testing.T) {
	restore := metrics.SetStrict(nil) // production behaviour: no strict report
	defer restore()
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Sam Example", ""} {
		tr := transition(domain.StateRunning, domain.StateCompleted, "")
		tr.AgentHandle = h
		m.RunTransitioned(context.Background(), tr)
	}
	f := metricstest.Find(t, reader, finished)
	if got := metricstest.Sum(f, map[string]string{"bothy.agent.handle": "other"}); got != 2 {
		t.Errorf("other = %v, want 2", got)
	}
}

func TestRunFinished_ReasonsMatchTheDomain(t *testing.T) {
	var codes []string
	for _, r := range domain.ReasonTable() {
		codes = append(codes, r.Code)
	}
	var declared []string
	for _, a := range metrics.RunFinished.Attrs {
		if a.Key == "bothy.reason" {
			declared = a.Values
		}
	}
	slices.Sort(codes)
	declared = slices.Clone(declared)
	slices.Sort(declared)
	if !slices.Equal(codes, declared) {
		t.Errorf("bothy.reason values %v, domain reason table %v", declared, codes)
	}
}

// fakeActive is an ActiveSource answering from a list.
type fakeActive struct {
	groups []app.ActiveGroup
	err    error
}

func (f *fakeActive) ActiveGroups(context.Context) ([]app.ActiveGroup, error) { return f.groups, f.err }

func TestRunActiveGauge(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	src := &fakeActive{groups: []app.ActiveGroup{
		{AgentHandle: "alex-bot", State: domain.StateQueued, Count: 2},
		{AgentHandle: "alex-bot", State: domain.StateRunning, Count: 1},
		{AgentHandle: "sam-bot", State: domain.StateWaitingInput, Count: 3},
	}}
	reg, err := otelmetrics.RegisterActiveGauge(mp, src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reg.Unregister() }()
	m := metricstest.Find(t, reader, active)
	if m.Unit != "{run}" {
		t.Errorf("unit %q, want {run}", m.Unit)
	}
	ps := metricstest.Points(m)
	if len(ps) != 3 {
		t.Fatalf("%d points %v, want 3", len(ps), ps)
	}
	for _, g := range src.groups {
		want := map[string]string{"bothy.agent.handle": g.AgentHandle, "bothy.run.state": string(g.State)}
		if !slices.ContainsFunc(ps, func(p metricstest.Point) bool { return equalAttrs(p.Attrs, want) && p.Value == float64(g.Count) }) {
			t.Errorf("no point %v = %d in %v", want, g.Count, ps)
		}
	}

	// A failed read drops the points of that collection and never fails it (17 rule 4).
	src.err = errors.New("statement timeout")
	if got := metricstest.Points(metricstest.Collect(t, reader)[active]); len(got) != 0 {
		t.Errorf("a failed read observed %v, want no points", got)
	}
}

// TestCostBooked_RunCost: one booked entry is one data point of bothy.run.cost with the four
// attributes of signals.md 4.3 and the amount in currency units (Design note 5 h of M2-22).
func TestCostBooked_RunCost(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	m.CostBooked(context.Background(), app.BookedCost{
		AgentHandle: "alex-bot", BillingMode: "organization", Currency: "USD", AmountMicros: 1_250_000, IsEstimate: true,
	})
	p := only(t, metricstest.Collect(t, reader), "bothy.run.cost")
	want := map[string]string{
		"bothy.agent.handle": "alex-bot", "bothy.billing_mode": "organization",
		"bothy.currency": "USD", "bothy.cost.estimate": "true",
	}
	if !equalAttrs(p.Attrs, want) || p.Value != 1.25 {
		t.Errorf("point %v = %v, want %v = 1.25", p.Attrs, p.Value, want)
	}
}
