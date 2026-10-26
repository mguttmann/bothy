// SPDX-License-Identifier: Apache-2.0

package load

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Targets are the thresholds a step must hold and the limit the plan respects, read by the caller
// from the specification (02 section 22, configuration reference); this package holds none.
type Targets struct {
	// MessageCreateP99 is perf.message_create_p99.
	MessageCreateP99 time.Duration `json:"-"`
	// FanoutP99 is perf.fanout_local_p99.
	FanoutP99 time.Duration `json:"-"`
	// MessagesPerMinute is the default of ratelimit.rl.message.per_minute.
	MessagesPerMinute int `json:"messages_per_minute"`
}

// targetsJSON is the form of Targets in the result file, durations in milliseconds.
type targetsJSON struct {
	MessageCreateP99Ms float64 `json:"message_create_p99_ms"`
	FanoutP99Ms        float64 `json:"fanout_p99_ms"`
	MessagesPerMinute  int     `json:"messages_per_minute"`
}

// MarshalJSON writes the durations in milliseconds.
func (t Targets) MarshalJSON() ([]byte, error) {
	return json.Marshal(targetsJSON{float64(t.MessageCreateP99) / float64(time.Millisecond), float64(t.FanoutP99) / float64(time.Millisecond), t.MessagesPerMinute})
}

// UnmarshalJSON reads the form of MarshalJSON.
func (t *Targets) UnmarshalJSON(b []byte) error {
	var j targetsJSON
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&j); err != nil {
		return err
	}
	*t = Targets{time.Duration(j.MessageCreateP99Ms * float64(time.Millisecond)), time.Duration(j.FanoutP99Ms * float64(time.Millisecond)), j.MessagesPerMinute}
	return nil
}

// Valid reports an error when a target or the limit is not positive.
func (t Targets) Valid() error {
	if t.MessageCreateP99 <= 0 || t.FanoutP99 <= 0 || t.MessagesPerMinute <= 0 {
		return fmt.Errorf("load: every target must be positive: %+v", t)
	}
	return nil
}

// Params are the scenario parameters of a run (params.go), repeated in the result file.
type Params struct {
	GroupSize         int       `json:"group_size"`
	ObserversPerGroup int       `json:"observers_per_group"`
	Headroom          float64   `json:"headroom"`
	WarmupS           float64   `json:"warmup_s"`
	WindowS           float64   `json:"window_s"`
	DrainS            float64   `json:"drain_s"`
	MinContentRunes   int       `json:"min_content_runes"`
	MaxContentRunes   int       `json:"max_content_runes"`
	RampRates         []float64 `json:"ramp_rates"`
}

// CurrentParams returns the parameters of params.go.
func CurrentParams() Params {
	return Params{
		GroupSize: GroupSize, ObserversPerGroup: ObserversPerGroup, Headroom: Headroom, WarmupS: Warmup.Seconds(),
		WindowS: Window.Seconds(), DrainS: Drain.Seconds(), MinContentRunes: MinContentRunes, MaxContentRunes: MaxContentRunes,
		RampRates: RampRates,
	}
}

// StepResult is what one step measured on the client side.
type StepResult struct {
	Index       int     `json:"index"`
	Rate        float64 `json:"rate"`
	Senders     int     `json:"senders"`
	Connections int     `json:"connections"`
	Groups      int     `json:"groups"`
	// Start is when the connections of the step began to open; WindowFrom and WindowTo bound the
	// measured sends (after Warmup), the time range the stack's metrics are read for.
	Start      time.Time     `json:"start"`
	WindowFrom time.Time     `json:"window_from"`
	WindowTo   time.Time     `json:"window_to"`
	Sends      SendSummary   `json:"sends"`
	Fanout     FanoutSummary `json:"fanout"`
	// ConnectionErrors counts gateway connections that failed to open or ended during the step.
	ConnectionErrors int `json:"connection_errors"`
	// Errors holds the first errors of the step (never a token or a text).
	Errors []string `json:"errors,omitempty"`
	// Missed lists the violations of the step against the targets of the run (Violations).
	Missed []string `json:"missed,omitempty"`
}

// maxErrors bounds StepResult.Errors.
const maxErrors = 10

// Violations returns why the step misses the targets: perf.message_create_p99 or
// perf.fanout_local_p99 exceeded, a send that failed (a 429 means the plan broke the rate limit),
// a dispatch an observer did not receive, a gateway connection that failed, or no samples at all.
// An empty list means the step holds.
func (s StepResult) Violations(t Targets) []string {
	var v []string
	if s.Sends.Scheduled == 0 || s.Sends.OK == 0 {
		v = append(v, "no samples in the window")
		return v
	}
	if s.Sends.P99 > t.MessageCreateP99 {
		v = append(v, fmt.Sprintf("perf.message_create_p99: p99 %v above %v", s.Sends.P99, t.MessageCreateP99))
	}
	if s.Fanout.Samples > 0 && s.Fanout.P99 > t.FanoutP99 {
		v = append(v, fmt.Sprintf("perf.fanout_local_p99: p99 %v above %v", s.Fanout.P99, t.FanoutP99))
	}
	if s.Sends.Failed > 0 {
		v = append(v, fmt.Sprintf("failed sends: %d of %d (%v)", s.Sends.Failed, s.Sends.Scheduled, s.Sends.FailedStatuses))
	}
	if s.Fanout.Samples < s.Fanout.Expected || s.Fanout.Expected == 0 {
		v = append(v, fmt.Sprintf("missing dispatches: %d of %d arrived at the observers", s.Fanout.Samples, s.Fanout.Expected))
	}
	if s.ConnectionErrors > 0 {
		v = append(v, fmt.Sprintf("connection errors: %d", s.ConnectionErrors))
	}
	return v
}

// Result is the result file of a ramp (cmd/bothy-load writes it, tools/load reads it).
type Result struct {
	Seed    uint64       `json:"seed"`
	Targets Targets      `json:"targets"`
	Params  Params       `json:"params"`
	Steps   []StepResult `json:"steps"`
	// Stopped says why the ramp ended: at the first missed step or after the last rate.
	Stopped string `json:"stopped"`
}

// Capacity returns the last step that held and the first that missed (nil when every step held).
// ok is false when no step held.
func (r Result) Capacity(t Targets) (last StepResult, first *StepResult, ok bool) {
	for i := range r.Steps {
		if len(r.Steps[i].Violations(t)) > 0 {
			s := r.Steps[i]
			return last, &s, ok
		}
		last, ok = r.Steps[i], true
	}
	return last, nil, ok
}

// ParseResult reads a result file strictly: no unknown field, positive targets, at least one
// step, indexes from 0 in order, rising rates and every window with its end after its start.
func ParseResult(b []byte) (Result, error) {
	var r Result
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return Result{}, fmt.Errorf("load: result file: %w", err)
	}
	if err := r.Targets.Valid(); err != nil {
		return Result{}, err
	}
	if len(r.Steps) == 0 {
		return Result{}, errors.New("load: the result has no step")
	}
	for i, s := range r.Steps {
		if s.Index != i {
			return Result{}, fmt.Errorf("load: step %d has index %d", i, s.Index)
		}
		if i > 0 && s.Rate <= r.Steps[i-1].Rate {
			return Result{}, fmt.Errorf("load: the rate of step %d does not rise", i)
		}
		if !s.WindowTo.After(s.WindowFrom) || s.WindowFrom.Before(s.Start) {
			return Result{}, fmt.Errorf("load: the window of step %d is not after its start", i)
		}
	}
	return r, nil
}
