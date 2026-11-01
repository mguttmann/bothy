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
	// stats.live_max_runs + 1 per channel.
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
	// Tasks are the nodes of the agent tree (TaskTree); TasksTruncated reports a limit cut.
	Tasks          []LiveTask
	TasksTruncated bool
	// ChannelName and ThreadTitle name the chat of the run as the sidebar does; "" when unknown
	// (no chat without a channel name) and ThreadTitle "" for a main thread or an empty title.
	// The filter sends them only to a receiver who may read the thread (Receiver.Named).
	ChannelName, ThreadTitle string
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
	// Readable are the channels whose runs the receiver may read (Scope of run.read): only their
	// task nodes carry a label (docs/design/topology-agent-tree.md 5).
	Readable map[ids.ID]struct{}
	// Costs reports cost.read.aggregate on the target.
	Costs bool
	// Named are the channels whose threads the receiver may read (Scope of thread.read, the rule
	// of the sidebar): only runs there carry their chat (25 4 rule 6). nil names nothing.
	Named map[ids.ID]struct{}
}

// LiveLimits are the configuration values of the filter.
type LiveLimits struct {
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
		RunID           string     `json:"run_id"`
		ThreadID        string     `json:"thread_id"`
		ChannelID       string     `json:"channel_id"`
		SpaceID         string     `json:"space_id"`
		AgentID         string     `json:"agent_id"`
		State           string     `json:"state"`
		QueuedAt        string     `json:"queued_at"`
		StartedAt       string     `json:"started_at,omitempty"`
		BillingMode     string     `json:"billing_mode"`
		Step            *stepJSON  `json:"step,omitempty"`
		ApprovalPending bool       `json:"approval_pending"`
		CostMicros      *int64     `json:"cost_micros,omitempty"`
		Currency        string     `json:"currency,omitempty"`
		CostIsEstimate  *bool      `json:"cost_is_estimate,omitempty"`
		Tasks           []taskJSON `json:"tasks,omitempty"`
		TasksTruncated  bool       `json:"tasks_truncated,omitempty"`
		Chat            *chatJSON  `json:"chat,omitempty"`
	}
	taskJSON struct {
		ID         string `json:"id"`
		ParentID   string `json:"parent_id,omitempty"`
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		State      string `json:"state"`
		StartedAt  string `json:"started_at"`
		FinishedAt string `json:"finished_at,omitempty"`
		Tool       string `json:"tool,omitempty"`
		Label      string `json:"label,omitempty"`
	}
	chatJSON struct {
		ThreadID    string `json:"thread_id"`
		ChannelName string `json:"channel_name"`
		ThreadTitle string `json:"thread_title,omitempty"`
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
//     were visible); a run carries its chat (channel name, thread title) only when its channel is
//     in r.Named;
//   - agents stay when they have a run or are the effective operator of a channel of the set;
//   - rates sum the channel cells of the set, plus the cells without a channel for the instance;
//     cost cells and the cost members of runs only with r.Costs;
//   - sli and connections only for the instance;
//   - every run carries its task nodes; their labels only when the run's channel is in r.Readable;
//   - when the payload reaches limits.PayloadMax the task nodes of the oldest runs drop out first
//     (tasks_truncated on each), then the oldest runs until it fits, with runs_truncated; the
//     envelope's truncated is never involved.
//
// Errors: ErrFrameTooLarge when even no run fits, or an encoding error.
//
// Example:
//
//	b, cut, err := domain.FilterLive(base, domain.Receiver{SpaceID: support, Visible: set}, limits)
func FilterLive(base *BaseFrame, r Receiver, limits LiveLimits) ([]byte, bool, error) {
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
	runs, light, truncated, agentIDs, err := visibleRuns(base, r, inSet, limits.MaxRuns)
	if err != nil {
		return nil, false, err
	}
	f.RunsTruncated = truncated
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
	return cut(f, runs, light, limits.PayloadMax)
}

// visibleRuns encodes the newest maxRuns runs of base in the channel set (and the subscribed
// space) and returns them, the same runs without their task nodes (the light form the size cut
// falls back to), whether more were visible, and the agents of every visible run.
func visibleRuns(base *BaseFrame, r Receiver, inSet func(ids.ID) bool, maxRuns int) (full, light []json.RawMessage, more bool,
	agentIDs map[ids.ID]struct{}, err error) {
	agentIDs = map[ids.ID]struct{}{}
	full, light = []json.RawMessage{}, []json.RawMessage{}
	visible := 0
	for _, run := range base.Runs {
		if !inSet(run.ChannelID) || (!r.SpaceID.IsZero() && run.SpaceID != r.SpaceID) {
			continue
		}
		visible++
		agentIDs[run.AgentID] = struct{}{}
		if len(full) >= maxRuns {
			continue
		}
		_, labels := r.Readable[run.ChannelID]
		_, named := r.Named[run.ChannelID]
		enc := encodeRun(run, r.Costs, labels, named)
		b, err := json.Marshal(enc)
		if err != nil {
			return nil, nil, false, nil, err
		}
		full = append(full, b)
		if len(enc.Tasks) > 0 {
			enc.Tasks, enc.TasksTruncated = nil, true
			if b, err = json.Marshal(enc); err != nil {
				return nil, nil, false, nil, err
			}
		}
		light = append(light, b)
	}
	return full, light, visible > len(full), agentIDs, nil
}

// encodeTasks returns the wire form of the task nodes, the labels only when labels.
func encodeTasks(tasks []LiveTask, labels bool) []taskJSON {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]taskJSON, len(tasks))
	for i, x := range tasks {
		out[i] = taskJSON{ID: x.ID.String(), Kind: x.Kind, Name: x.Name, State: x.State, StartedAt: clock.FormatAPI(x.StartedAt),
			Tool: x.Tool}
		if !x.ParentID.IsZero() {
			out[i].ParentID = x.ParentID.String()
		}
		if !x.FinishedAt.IsZero() {
			out[i].FinishedAt = clock.FormatAPI(x.FinishedAt)
		}
		if labels {
			out[i].Label = x.Label
		}
	}
	return out
}

// encodeRun returns the wire form of run, with the cost members only when costs, the labels of
// its task nodes only when labels and the chat only when named.
func encodeRun(run LiveRun, costs, labels, named bool) runJSON {
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
	out.Tasks, out.TasksTruncated = encodeTasks(run.Tasks, labels), run.TasksTruncated
	if named && run.ChannelName != "" {
		out.Chat = &chatJSON{ThreadID: run.ThreadID.String(), ChannelName: run.ChannelName, ThreadTitle: run.ThreadTitle}
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

// rateSums adds rate cells into rows per rateKey.
type rateSums map[rateKey]*RateCell

// add adds c to the summed row of its metric and currency and, with a dimension, to the row of
// that dimension. SLI cells never count; cost cells only with costs.
func (s rateSums) add(c RateCell, costs bool) {
	if strings.HasPrefix(c.Metric, sliPrefix) || (c.Metric == MetricCost && !costs) {
		return
	}
	keys := []rateKey{{c.Metric, "", c.Currency}}
	if c.Dimension != "" {
		keys = append(keys, rateKey{c.Metric, c.Dimension, c.Currency})
	}
	for _, k := range keys {
		row := s[k]
		if row == nil {
			row = &RateCell{Metric: k.metric, Dimension: k.dimension, Currency: k.currency}
			s[k] = row
		}
		row.Last += c.Last
		row.Current += c.Current
		row.LastSum += c.LastSum
	}
}

// rates sums the cells of the channels in the set and, for the instance, the cells without a
// channel: one row per metric (per currency for cost) with every dimension summed and one row per
// non-empty dimension, ordered by metric, currency and dimension.
func rates(base *BaseFrame, inSet func(ids.ID) bool, instance, costs bool) []rateJSON {
	sums := rateSums{}
	for ch, cells := range base.ChannelRates {
		if inSet(ch) {
			for _, c := range cells {
				sums.add(c, costs)
			}
		}
	}
	if instance {
		for _, c := range base.InstanceRates {
			sums.add(c, costs)
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

// cut encodes f with the newest runs that keep the payload below max (25 5.2 size rule): first
// every run with its task nodes; when that is too large, the light form of each run (without
// its task nodes) and back the full form for the newest runs while it fits; when even the light
// forms are too large, the newest light runs that fit. The length of every run is known, so each
// choice is computed once and checked by one encoding; runs_truncated is set when runs drop out.
// It returns the payload and whether the size bound cut runs.
func cut(f frameJSON, full, light []json.RawMessage, max int) ([]byte, bool, error) {
	if full == nil {
		// A nil list encodes as JSON null, which no frame may carry (ADR 0053) and the hub refuses.
		full, light = []json.RawMessage{}, []json.RawMessage{}
	}
	f.Runs = full
	b, err := json.Marshal(f)
	if err != nil {
		return nil, false, err
	}
	if len(b) < max {
		return b, false, nil
	}
	if b, ok, err := withoutTrees(f, full, light, max); err != nil || ok {
		return b, false, err
	}
	return cutRuns(f, light, max)
}

// withoutTrees encodes every run of f, the newest ones with their task nodes as long as the
// payload stays below max; false when even every run without its nodes reaches max.
func withoutTrees(f frameJSON, full, light []json.RawMessage, max int) ([]byte, bool, error) {
	f.Runs = slices.Clone(light)
	b, err := json.Marshal(f)
	if err != nil || len(b) >= max {
		return nil, false, err
	}
	size := len(b)
	for i := range full {
		grow := len(full[i]) - len(light[i])
		if grow == 0 {
			continue
		}
		if size+grow >= max {
			break
		}
		f.Runs[i], size = full[i], size+grow
	}
	if b, err = json.Marshal(f); err != nil {
		return nil, false, err
	}
	return b, len(b) < max, nil
}

// cutRuns encodes f with the newest of runs that keep the payload below max, with
// runs_truncated.
func cutRuns(f frameJSON, runs []json.RawMessage, max int) ([]byte, bool, error) {
	var b []byte
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
