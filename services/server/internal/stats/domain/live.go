// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The kinds of the member scope of a live frame (schema StatsScope).
const (
	ScopeInstance = "instance"
	ScopeSpace    = "space"
)

// MetricCost is the metric whose rows carry a currency and are cost members (25 3.2, 4 rule 2).
const MetricCost = "cost"

// sliPrefix marks the SLI metrics of 25 3.2: never in rates, only in sli of the instance scope.
const sliPrefix = "sli."

// ErrFrameTooLarge reports a frame that reaches sync.event_payload_max_bytes even without runs:
// only runs may be shortened (25 5.2), so nothing is sent.
var ErrFrameTooLarge = errors.New("stats: the live frame reaches sync.event_payload_max_bytes without runs")

// BaseFrame is the base frame of one node and one stats.live_interval (M6-15 design note 2): the
// state of the whole instance with the attribution the filter needs (channel and space per run,
// rates per channel, operators per channel). It is never sent unfiltered.
type BaseFrame struct {
	// At is the build time.
	At time.Time
	// Runs are the active runs, newest first (QueuedAt, then RunID, descending); at most
	// stats.live_max_runs per channel.
	Runs []LiveRun
	// Agents are the agents with an active run or an operator role, by ID.
	Agents map[ids.ID]LiveAgent
	// Channels maps every live channel to its space and effective operator (zero for none).
	Channels map[ids.ID]ChannelPlace
	// ChannelRates are the rate cells of each channel; InstanceRates the cells without a channel.
	ChannelRates  map[ids.ID][]RateCell
	InstanceRates []RateCell
	// SLI are the totals of the sli.* metrics over stats.live_sli_window.
	SLI []SLICell
	// Connections are the gateway connections of the node; nil with cluster.mode = multi.
	Connections *Connections
}

// ChannelPlace is the space and the effective operator of a channel.
type ChannelPlace struct {
	SpaceID, OperatorID ids.ID
}

// LiveRun is one active run with its place (schema StatsLiveRun without the cost filter).
type LiveRun struct {
	RunID, ThreadID, ChannelID, SpaceID, AgentID ids.ID
	// State is queued, running or waiting_input.
	State string
	// QueuedAt is never zero; StartedAt is zero before the start.
	QueuedAt, StartedAt time.Time
	// BillingMode is the reported mode of the run, else the configured mode.
	BillingMode string
	// StepKind and StepName are the current step; "" for none.
	StepKind, StepName string
	// ApprovalPending reports an approval card of the run waiting for a decision.
	ApprovalPending bool
	// CostMicros, Currency and CostIsEstimate are the cost so far; Currency "" for none.
	CostMicros     int64
	Currency       string
	CostIsEstimate bool
}

// LiveAgent is the state of one agent (schema StatsLiveAgent).
type LiveAgent struct {
	AgentID ids.ID
	// Presence is online, degraded, offline or unknown.
	Presence string
	// BillingMode is the configured mode; ReportedBillingMode the reported one, "" when the
	// adapter follows the configured mode per run.
	BillingMode, ReportedBillingMode string
}

// RateCell is one key of stats_minutes summed over agents for the last full minute and the
// current minute: count of each, sum of the last.
type RateCell struct {
	Metric, Dimension, Currency string
	Last, Current, LastSum      int64
}

// SLICell is the total and the good count of one SLI metric over the window.
type SLICell struct {
	Metric      string
	Total, Good int64
}

// Connections are the gateway connections of a node by kind (schema StatsLiveConnections).
type Connections struct {
	User  int `json:"user"`
	Agent int `json:"agent"`
	App   int `json:"app"`
}

// Receiver is what the filter knows about one receiver (M6-15 design note 4).
type Receiver struct {
	// SpaceID is the subscribed space; zero for the instance.
	SpaceID ids.ID
	// Visible are the channels the receiver may read (Scope of channel.read).
	Visible map[ids.ID]struct{}
	// Costs reports cost.read.aggregate on the target.
	Costs bool
}

// Limits are the configuration values of the filter.
type Limits struct {
	// MaxRuns is stats.live_max_runs.
	MaxRuns int
	// PayloadMax is sync.event_payload_max_bytes; the payload must stay below it.
	PayloadMax int
	// Interval is stats.live_interval (member interval_ms).
	Interval time.Duration
}

// The wire form of the frame (schema StatsLiveFrame): optional members are pointers with
// omitempty, so no JSON null is ever written.
type (
	frameJSON struct {
		At            string            `json:"at"`
		IntervalMS    int64             `json:"interval_ms"`
		Scope         scopeJSON         `json:"scope"`
		Runs          []json.RawMessage `json:"runs"`
		RunsTruncated bool              `json:"runs_truncated"`
		Agents        []agentJSON       `json:"agents"`
		Rates         []rateJSON        `json:"rates"`
		SLI           []sliJSON         `json:"sli,omitempty"`
		Connections   *Connections      `json:"connections,omitempty"`
	}
	scopeJSON struct {
		Kind    string `json:"kind"`
		SpaceID string `json:"space_id,omitempty"`
	}
	stepJSON struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	runJSON struct {
		RunID           string    `json:"run_id"`
		ThreadID        string    `json:"thread_id"`
		ChannelID       string    `json:"channel_id"`
		SpaceID         string    `json:"space_id"`
		AgentID         string    `json:"agent_id"`
		State           string    `json:"state"`
		QueuedAt        string    `json:"queued_at"`
		StartedAt       string    `json:"started_at,omitempty"`
		BillingMode     string    `json:"billing_mode"`
		Step            *stepJSON `json:"step,omitempty"`
		ApprovalPending bool      `json:"approval_pending"`
		CostMicros      *int64    `json:"cost_micros,omitempty"`
		Currency        string    `json:"currency,omitempty"`
		CostIsEstimate  *bool     `json:"cost_is_estimate,omitempty"`
	}
	agentJSON struct {
		AgentID             string `json:"agent_id"`
		Presence            string `json:"presence"`
		BillingMode         string `json:"billing_mode"`
		ReportedBillingMode string `json:"reported_billing_mode,omitempty"`
	}
	rateJSON struct {
		Metric        string `json:"metric"`
		Dimension     string `json:"dimension,omitempty"`
		LastMinute    int64  `json:"last_minute"`
		CurrentMinute int64  `json:"current_minute"`
		LastMinuteSum *int64 `json:"last_minute_sum,omitempty"`
		Currency      string `json:"currency,omitempty"`
	}
	sliJSON struct {
		Metric string   `json:"metric"`
		Total  int64    `json:"total"`
		Good   int64    `json:"good"`
		Ratio  *float64 `json:"ratio,omitempty"`
	}
)

// summedMetrics are the metrics whose sum is meaningful in a rate (25 3.2: durations in
// milliseconds and amounts in micro-units); the others count only.
var summedMetrics = map[string]bool{"runs_finished": true, "tool_calls": true, MetricCost: true}

// FilterLive returns the frame of base for r as the encoded JSON object StatsLiveFrame (25 4 and 5.2,
// M6-15 design notes 3 and 4) and whether runs were cut:
//
//   - the channel set is r.Visible, for a space only the channels of that space;
//   - runs outside the set drop out, the newest stats.live_max_runs stay (runs_truncated when more
//     were visible);
//   - agents stay when they have a run or are the effective operator of a channel of the set;
//   - rates sum the channel cells of the set, plus the cells without a channel for the instance;
//     cost cells and the cost members of runs only with r.Costs;
//   - sli and connections only for the instance;
//   - when the payload reaches limits.PayloadMax the oldest runs drop out until it fits, with
//     runs_truncated; the envelope's truncated is never involved.
//
// Errors: ErrFrameTooLarge when even no run fits, or an encoding error.
//
// Example:
//
//	b, cut, err := domain.FilterLive(base, domain.Receiver{SpaceID: support, Visible: set}, limits)
func FilterLive(base *BaseFrame, r Receiver, limits Limits) ([]byte, bool, error) {
	inSet := func(channel ids.ID) bool {
		if _, ok := r.Visible[channel]; !ok {
			return false
		}
		return r.SpaceID.IsZero() || base.Channels[channel].SpaceID == r.SpaceID
	}
	instance := r.SpaceID.IsZero()
	f := frameJSON{At: clock.FormatAPI(base.At), IntervalMS: limits.Interval.Milliseconds(), Scope: scopeJSON{Kind: ScopeInstance},
		Runs: []json.RawMessage{}, Agents: []agentJSON{}, Rates: []rateJSON{}}
	if !instance {
		f.Scope = scopeJSON{Kind: ScopeSpace, SpaceID: r.SpaceID.String()}
	}
	agentIDs := map[ids.ID]struct{}{}
	var runs []json.RawMessage
	visibleRuns := 0
	for _, run := range base.Runs {
		if !inSet(run.ChannelID) || (!instance && run.SpaceID != r.SpaceID) {
			continue
		}
		visibleRuns++
		agentIDs[run.AgentID] = struct{}{}
		if len(runs) >= limits.MaxRuns {
			continue
		}
		b, err := json.Marshal(encodeRun(run, r.Costs))
		if err != nil {
			return nil, false, err
		}
		runs = append(runs, b)
	}
	f.RunsTruncated = visibleRuns > len(runs)
	for ch, place := range base.Channels {
		if !place.OperatorID.IsZero() && inSet(ch) {
			agentIDs[place.OperatorID] = struct{}{}
		}
	}
	f.Agents = agents(base, agentIDs)
	f.Rates = rates(base, inSet, instance, r.Costs)
	if instance {
		f.SLI = slis(base.SLI)
		f.Connections = base.Connections
	}
	return cut(f, runs, limits.PayloadMax)
}

// encodeRun returns the wire form of run, with the cost members only when costs.
func encodeRun(run LiveRun, costs bool) runJSON {
	out := runJSON{RunID: run.RunID.String(), ThreadID: run.ThreadID.String(), ChannelID: run.ChannelID.String(),
		SpaceID: run.SpaceID.String(), AgentID: run.AgentID.String(), State: run.State, QueuedAt: clock.FormatAPI(run.QueuedAt),
		BillingMode: run.BillingMode, ApprovalPending: run.ApprovalPending}
	if !run.StartedAt.IsZero() {
		out.StartedAt = clock.FormatAPI(run.StartedAt)
	}
	if run.StepKind != "" {
		out.Step = &stepJSON{Kind: run.StepKind, Name: run.StepName}
	}
	if costs && run.Currency != "" {
		micros, estimate := run.CostMicros, run.CostIsEstimate
		out.CostMicros, out.Currency, out.CostIsEstimate = &micros, run.Currency, &estimate
	}
	return out
}

// agents returns the states of the agents among want, ascending by ID; an agent without a known
// state drops out.
func agents(base *BaseFrame, want map[ids.ID]struct{}) []agentJSON {
	out := make([]agentJSON, 0, len(want))
	for id := range want {
		a, ok := base.Agents[id]
		if !ok {
			continue
		}
		out = append(out, agentJSON{AgentID: id.String(), Presence: a.Presence, BillingMode: a.BillingMode,
			ReportedBillingMode: a.ReportedBillingMode})
	}
	slices.SortFunc(out, func(x, y agentJSON) int { return strings.Compare(x.AgentID, y.AgentID) })
	return out
}

// rateKey is one row of the rates: metric, dimension ("" is the sum of all dimensions) and
// currency.
type rateKey struct{ metric, dimension, currency string }

// rates sums the cells of the channels in the set and, for the instance, the cells without a
// channel: one row per metric (per currency for cost) with every dimension summed and one row per
// non-empty dimension. SLI cells never appear; cost cells only with costs.
func rates(base *BaseFrame, inSet func(ids.ID) bool, instance, costs bool) []rateJSON {
	sums := map[rateKey]*RateCell{}
	add := func(c RateCell) {
		if strings.HasPrefix(c.Metric, sliPrefix) || (c.Metric == MetricCost && !costs) {
			return
		}
		keys := []rateKey{{c.Metric, "", c.Currency}}
		if c.Dimension != "" {
			keys = append(keys, rateKey{c.Metric, c.Dimension, c.Currency})
		}
		for _, k := range keys {
			s := sums[k]
			if s == nil {
				s = &RateCell{Metric: k.metric, Dimension: k.dimension, Currency: k.currency}
				sums[k] = s
			}
			s.Last += c.Last
			s.Current += c.Current
			s.LastSum += c.LastSum
		}
	}
	for ch, cells := range base.ChannelRates {
		if !inSet(ch) {
			continue
		}
		for _, c := range cells {
			add(c)
		}
	}
	if instance {
		for _, c := range base.InstanceRates {
			add(c)
		}
	}
	out := make([]rateJSON, 0, len(sums))
	for _, s := range sums {
		r := rateJSON{Metric: s.Metric, Dimension: s.Dimension, LastMinute: s.Last, CurrentMinute: s.Current, Currency: s.Currency}
		if summedMetrics[s.Metric] {
			sum := s.LastSum
			r.LastMinuteSum = &sum
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(x, y rateJSON) int {
		return cmp.Or(strings.Compare(x.Metric, y.Metric), strings.Compare(x.Currency, y.Currency), strings.Compare(x.Dimension, y.Dimension))
	})
	return out
}

// slis returns the sli member: total, good and the ratio (absent when total is 0), by metric.
func slis(cells []SLICell) []sliJSON {
	out := make([]sliJSON, 0, len(cells))
	for _, c := range cells {
		s := sliJSON{Metric: c.Metric, Total: c.Total, Good: c.Good}
		if c.Total > 0 {
			ratio := float64(c.Good) / float64(c.Total)
			s.Ratio = &ratio
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(x, y sliJSON) int { return strings.Compare(x.Metric, y.Metric) })
	return out
}

// cut encodes f with the newest runs that keep the payload below max (25 5.2 size rule): the
// length of every run is known, so the number of runs is computed once and checked by one
// encoding; runs_truncated is set when runs drop out. It returns the payload and whether the
// size bound cut runs.
func cut(f frameJSON, runs []json.RawMessage, max int) ([]byte, bool, error) {
	f.Runs = runs
	b, err := json.Marshal(f)
	if err != nil {
		return nil, false, err
	}
	if len(b) < max {
		return b, false, nil
	}
	f.Runs, f.RunsTruncated = []json.RawMessage{}, true
	empty, err := json.Marshal(f)
	if err != nil {
		return nil, false, err
	}
	if len(empty) >= max {
		return nil, false, ErrFrameTooLarge
	}
	size, n := len(empty), 0
	for n < len(runs) {
		add := len(runs[n])
		if n > 0 {
			add++ // the comma between two runs
		}
		if size+add >= max {
			break
		}
		size += add
		n++
	}
	for ; n >= 0; n-- {
		f.Runs = runs[:n]
		if b, err = json.Marshal(f); err != nil {
			return nil, false, err
		}
		if len(b) < max {
			return b, true, nil
		}
	}
	return nil, false, ErrFrameTooLarge
}
