// SPDX-License-Identifier: Apache-2.0

package load

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var testTargets = Targets{MessageCreateP99: 150 * time.Millisecond, FanoutP99: 200 * time.Millisecond, MessagesPerMinute: 60}

// holding returns a step result that meets every target.
func holding(i int, rate float64) StepResult {
	return StepResult{
		Index: i, Rate: rate, Senders: 2 * int(rate), Connections: 2 * int(rate), Groups: 1,
		Start: time.Date(2026, 10, 26, 12, i, 0, 0, time.UTC), WindowFrom: time.Date(2026, 10, 26, 12, i, 15, 0, time.UTC),
		WindowTo: time.Date(2026, 10, 26, 12, i, 45, 0, time.UTC),
		Sends:    SendSummary{Scheduled: 100, OK: 100, WithinTarget: 100, AchievedPerS: rate, GoodputPerS: rate, P50: 10 * time.Millisecond, P99: 50 * time.Millisecond, Max: 60 * time.Millisecond},
		Fanout:   FanoutSummary{Samples: 100, Expected: 100, P50: 5 * time.Millisecond, P99: 40 * time.Millisecond, Max: 50 * time.Millisecond},
	}
}

func TestResult_ViolationsNameTheMissedTarget(t *testing.T) {
	t.Parallel()
	if v := holding(0, 50).Violations(testTargets); len(v) != 0 {
		t.Fatalf("a holding step has violations %v", v)
	}
	cases := map[string]func(*StepResult){
		"perf.message_create_p99": func(s *StepResult) { s.Sends.P99 = 151 * time.Millisecond },
		"perf.fanout_local_p99":   func(s *StepResult) { s.Fanout.P99 = 201 * time.Millisecond },
		"failed sends":            func(s *StepResult) { s.Sends.Failed, s.Sends.OK = 1, 99 },
		"missing dispatches":      func(s *StepResult) { s.Fanout.Samples = 99 },
		"connection errors":       func(s *StepResult) { s.ConnectionErrors = 1 },
		"no samples":              func(s *StepResult) { s.Sends = SendSummary{} },
	}
	for want, breakIt := range cases {
		s := holding(0, 50)
		breakIt(&s)
		v := s.Violations(testTargets)
		if len(v) == 0 || !strings.Contains(strings.Join(v, "; "), want) {
			t.Errorf("%s: violations %v", want, v)
		}
	}
}

func TestResult_CapacityIsTheLastHoldingStep(t *testing.T) {
	t.Parallel()
	r := Result{Steps: []StepResult{holding(0, 50), holding(1, 100), holding(2, 200)}}
	r.Steps[2].Sends.P99 = time.Second
	last, first, ok := r.Capacity(testTargets)
	if !ok || last.Rate != 100 || first == nil || first.Rate != 200 {
		t.Fatalf("capacity: last %+v first %+v ok %v", last, first, ok)
	}
	all := Result{Steps: []StepResult{holding(0, 50), holding(1, 100)}}
	last, first, ok = all.Capacity(testTargets)
	if !ok || last.Rate != 100 || first != nil {
		t.Errorf("a ramp without a miss: last %+v first %+v", last, first)
	}
	none := Result{Steps: []StepResult{holding(0, 50)}}
	none.Steps[0].Sends.P99 = time.Second
	if _, first, ok = none.Capacity(testTargets); ok || first == nil {
		t.Error("a ramp that misses at its first step has a capacity")
	}
}

func TestResult_RoundTripAndParse(t *testing.T) {
	t.Parallel()
	r := Result{Seed: Seed, Targets: testTargets, Params: CurrentParams(), Steps: []StepResult{holding(0, 50)}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseResult(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Steps[0].Sends.P99 != r.Steps[0].Sends.P99 || got.Targets != r.Targets || got.Seed != Seed {
		t.Errorf("round trip changed the result: %+v", got)
	}
	for name, bad := range map[string]string{
		"not JSON":        `{`,
		"unknown field":   `{"seed":1,"steps":[],"extra":1}`,
		"no steps":        `{"seed":1,"targets":{"message_create_p99_ms":150,"fanout_p99_ms":200,"messages_per_minute":60},"steps":[]}`,
		"no targets":      strings.Replace(string(b), `"message_create_p99_ms":150`, `"message_create_p99_ms":0`, 1),
		"falling rates":   strings.Replace(strings.Replace(string(b), `"steps":[`, `"steps":[`+stepJSON(t, holding(0, 100))+`,`, 1), `"index":0,"rate":50`, `"index":1,"rate":50`, 1),
		"window reversed": strings.Replace(string(b), `"window_to":"2026-10-26T12:00:45Z"`, `"window_to":"2026-10-26T12:00:00Z"`, 1),
	} {
		if _, err := ParseResult([]byte(bad)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

func stepJSON(t *testing.T, s StepResult) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGate_FailsBelowTheMinimum(t *testing.T) {
	t.Parallel()
	if err := Gate(299.9, 300); err == nil {
		t.Error("a throughput below the minimum passed the gate")
	}
	for _, v := range []float64{300, 450} {
		if err := Gate(v, 300); err != nil {
			t.Errorf("throughput %g failed the gate: %v", v, err)
		}
	}
	if err := Gate(500, 0); err == nil {
		t.Error("a minimum of zero passed; the gate needs a named value")
	}
}
