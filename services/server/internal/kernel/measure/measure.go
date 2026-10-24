// SPDX-License-Identifier: Apache-2.0

package measure

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Metric is stats_minutes.metric: one of the 18 values of the closed list of
// docs/spec/25-visualization.md 3.2 and the CHECK of docs/spec/03-data-model.md 4.15.
type Metric string

// The metrics of 25 3.2, in the order of the table. TestClosedMetricsAndDimensions holds them
// equal to the CHECK of the migration of stats_minutes.
const (
	// MessagesCreated counts committed messages per origin (messaging, after commit).
	MessagesCreated Metric = "messages_created"
	// ThreadsCreated counts committed threads (messaging, after commit).
	ThreadsCreated Metric = "threads_created"
	// RunsStarted counts runs entering running per billing mode used (runs).
	RunsStarted Metric = "runs_started"
	// RunsFinished counts runs reaching an end state, with their duration (runs).
	RunsFinished Metric = "runs_finished"
	// ToolCalls counts finished tool_call steps, with their duration (runs).
	ToolCalls Metric = "tool_calls"
	// ApprovalCards counts approval cards: requested, approved, denied, expired (runs).
	ApprovalCards Metric = "approval_cards"
	// Cost counts valid cost bookings with their amount in micro-units and the currency.
	Cost Metric = "cost"
	// InvocationsDenied counts I2 denials before a delivery and runs failed with
	// billing_mode_mismatch (runs).
	InvocationsDenied Metric = "invocations_denied"
	// BillingModeSwitches counts switches of an agent's billing mode per new mode (agents).
	BillingModeSwitches Metric = "billing_mode_switches"
	// Catchups counts finished catch-ups with their duration (gateway).
	Catchups Metric = "catchups"
	// Resyncs counts resync_required frames per reason (gateway).
	Resyncs Metric = "resyncs"
	// SlowConsumerCloses counts closes with 4014 (gateway).
	SlowConsumerCloses Metric = "slow_consumer_closes"
	// SLIAPIAvailability is the SLI of the REST answers without 5xx (platform/httpserver).
	SLIAPIAvailability Metric = "sli.api_availability"
	// SLIDeliveryLatency is the SLI of the live deliveries of durable events (gateway).
	SLIDeliveryLatency Metric = "sli.delivery_latency"
	// SLIReconnect is the SLI of the connections from the upgrade to ready or resumed (gateway).
	SLIReconnect Metric = "sli.reconnect"
	// SLIPushAcceptance is the SLI of the push provider answers (notify).
	SLIPushAcceptance Metric = "sli.push_acceptance"
	// SLIRunStart is the SLI of the runs leaving queued (runs).
	SLIRunStart Metric = "sli.run_start"
	// SLITurnDuration is the SLI of the completed runs (runs).
	SLITurnDuration Metric = "sli.turn_duration"
)

// Scope says which of channel_id and agent_id a metric carries (column "Channel, agent" of 25
// 3.2). A zero ID is NULL in either case.
type Scope uint8

// The scopes of 25 3.2.
const (
	// ScopeNone is a value of the whole instance: no channel, no agent.
	ScopeNone Scope = iota
	// ScopeChannel carries the channel only.
	ScopeChannel
	// ScopeAgent carries the agent only.
	ScopeAgent
	// ScopeChannelAgent carries channel and agent.
	ScopeChannelAgent
)

// spec is one row of 25 3.2: the closed dimension codes ({""} for the empty dimension) and the
// scope.
type spec struct {
	dimensions []string
	scope      Scope
}

// The dimension lists several metrics share.
var (
	billingModes = []string{"personal", "organization"}
	noDimension  = []string{""}
)

// specs is the table of 25 3.2.
var specs = map[Metric]spec{
	MessagesCreated:     {dimensions: []string{"human", "agent", "app", "import"}, scope: ScopeChannel},
	ThreadsCreated:      {dimensions: noDimension, scope: ScopeChannel},
	RunsStarted:         {dimensions: billingModes, scope: ScopeChannelAgent},
	RunsFinished:        {dimensions: []string{"completed", "failed", "cancelled"}, scope: ScopeChannelAgent},
	ToolCalls:           {dimensions: []string{"completed", "failed"}, scope: ScopeChannelAgent},
	ApprovalCards:       {dimensions: []string{"requested", "approved", "denied", "expired"}, scope: ScopeChannelAgent},
	Cost:                {dimensions: billingModes, scope: ScopeChannelAgent},
	InvocationsDenied:   {dimensions: []string{"i2_denied", "billing_mode_mismatch"}, scope: ScopeChannelAgent},
	BillingModeSwitches: {dimensions: billingModes, scope: ScopeAgent},
	Catchups:            {dimensions: noDimension, scope: ScopeNone},
	// The reason codes of resync_required (04 7.3).
	Resyncs:            {dimensions: []string{"epoch_changed", "retention", "too_far_behind", "cursor_ahead"}, scope: ScopeNone},
	SlowConsumerCloses: {dimensions: noDimension, scope: ScopeNone},
	SLIAPIAvailability: {dimensions: noDimension, scope: ScopeNone},
	SLIDeliveryLatency: {dimensions: noDimension, scope: ScopeNone},
	SLIReconnect:       {dimensions: noDimension, scope: ScopeNone},
	// The values of bothy.push.provider (signals.md 5.3).
	SLIPushAcceptance: {dimensions: []string{"apns", "fcm", "unifiedpush", "wns"}, scope: ScopeNone},
	SLIRunStart:       {dimensions: noDimension, scope: ScopeChannelAgent},
	SLITurnDuration:   {dimensions: noDimension, scope: ScopeChannelAgent},
}

// Metrics returns the 18 metrics of 25 3.2 in the order of the table; a fresh slice per call.
func Metrics() []Metric {
	return []Metric{
		MessagesCreated, ThreadsCreated, RunsStarted, RunsFinished, ToolCalls, ApprovalCards, Cost, InvocationsDenied,
		BillingModeSwitches, Catchups, Resyncs, SlowConsumerCloses, SLIAPIAvailability, SLIDeliveryLatency, SLIReconnect,
		SLIPushAcceptance, SLIRunStart, SLITurnDuration,
	}
}

// Dimensions returns the closed dimension codes of m ({""} for a metric without a dimension); nil
// for an unknown metric. A fresh slice per call.
func Dimensions(m Metric) []string {
	s, ok := specs[m]
	if !ok {
		return nil
	}
	return slices.Clone(s.dimensions)
}

// ScopeOf returns the scope of m and whether m is a metric of the list.
func ScopeOf(m Metric) (Scope, bool) {
	s, ok := specs[m]
	return s.scope, ok
}

// IsSLI reports whether m is one of the six sli.* metrics, whose good column counts.
func (m Metric) IsSLI() bool {
	return len(m) > 4 && m[:4] == "sli."
}

// Key is the part of the key of stats_minutes the measuring point chooses; the minute comes from
// the clock of the recorder. It holds no person and no content (25 4 rule 3).
type Key struct {
	// ChannelID is stats_minutes.channel_id; zero means NULL.
	ChannelID ids.ID
	// AgentID is stats_minutes.agent_id; zero means NULL.
	AgentID ids.ID
	// Dimension is a code of the closed list of the metric ("" where it has none).
	Dimension string
	// Currency is the ISO 4217 code of a cost; empty for every other metric.
	Currency string
}

// Value is what one Record call adds. Build it with One, Timed, Amount, Outcome or SLI.
type Value struct {
	// Count is the number of events, at least 1.
	Count int64
	// Sum is added to stats_minutes.sum as it is: the micro-units of a cost.
	Sum int64
	// Elapsed is a duration; the recorder adds its whole milliseconds to stats_minutes.sum and
	// compares it, for an SLI with a threshold, with the threshold. A negative value counts as 0.
	Elapsed time.Duration
	// OK is the outcome of an SLI event: the event is good when OK holds and, for an SLI with a
	// threshold, Elapsed is at most the threshold. Ignored for other metrics.
	OK bool
}

// One is a single event without a sum.
func One() Value { return Value{Count: 1} }

// Timed is a single event that took d.
func Timed(d time.Duration) Value { return Value{Count: 1, Elapsed: d} }

// Amount is a single event with micros added to the sum (a cost in micro-units).
func Amount(micros int64) Value { return Value{Count: 1, Sum: micros} }

// Outcome is a single SLI event without a duration: good when ok (sli.api_availability,
// sli.push_acceptance).
func Outcome(ok bool) Value { return Value{Count: 1, OK: ok} }

// SLI is a single SLI event that took d with the outcome ok: good when ok and d is at most the
// threshold of the SLO (sli.delivery_latency, sli.reconnect, sli.run_start, sli.turn_duration).
func SLI(d time.Duration, ok bool) Value { return Value{Count: 1, Elapsed: d, OK: ok} }

// Recorder is the cross-cutting port of the own measurement (25 3.1, 02 4.5): every measuring
// point calls Record next to its OpenTelemetry instrument. The stats module implements it with
// counters in memory; a call never touches the database or the network and never waits longer
// than a map update. Implementations are safe for concurrent use.
//
// Example:
//
//	rec.Record(measure.MessagesCreated, measure.Key{ChannelID: ch, Dimension: "human"}, measure.One())
type Recorder interface {
	// Record adds v to the counters of metric and key in the current minute. A key outside the
	// closed lists (Validate) is a programming error: the stats recorder panics in a test binary
	// and drops and counts it in production.
	Record(metric Metric, key Key, value Value)
}

// Nop is the Recorder of tests and of code built without the stats module: it records nothing.
type Nop struct{}

// Record implements Recorder without effect.
func (Nop) Record(Metric, Key, Value) {}

// OrNop returns r, or Nop when r is nil, so a collaborator without a recorder records nothing.
func OrNop(r Recorder) Recorder {
	if r == nil {
		return Nop{}
	}
	return r
}

// ErrInvalid is wrapped by every error of Validate.
var ErrInvalid = errors.New("measure: invalid key")

// Validate checks a Record call against the closed lists of 25 3.2: a known metric, a dimension
// of its list, channel and agent only where its scope has them, a currency of three upper-case
// letters exactly for cost, a Count of at least 1 and a Sum that is not negative. The error wraps
// ErrInvalid and names the metric, never an ID.
func Validate(m Metric, k Key, v Value) error {
	s, ok := specs[m]
	if !ok {
		return fmt.Errorf("%w: unknown metric %q", ErrInvalid, string(m))
	}
	if !slices.Contains(s.dimensions, k.Dimension) {
		return fmt.Errorf("%w: dimension %q is not in the list of %s", ErrInvalid, k.Dimension, m)
	}
	if (s.scope == ScopeNone || s.scope == ScopeAgent) && !k.ChannelID.IsZero() {
		return fmt.Errorf("%w: %s carries no channel", ErrInvalid, m)
	}
	if (s.scope == ScopeNone || s.scope == ScopeChannel) && !k.AgentID.IsZero() {
		return fmt.Errorf("%w: %s carries no agent", ErrInvalid, m)
	}
	if (m == Cost) != (k.Currency != "") || k.Currency != "" && !currencyCode(k.Currency) {
		return fmt.Errorf("%w: %s has a currency exactly when it is cost, as an ISO 4217 code", ErrInvalid, m)
	}
	if v.Count < 1 || v.Sum < 0 {
		return fmt.Errorf("%w: %s needs a count of at least 1 and a sum that is not negative", ErrInvalid, m)
	}
	return nil
}

// currencyCode reports whether c has the form of an ISO 4217 code: three upper-case letters.
func currencyCode(c string) bool {
	if len(c) != 3 {
		return false
	}
	for i := range len(c) {
		if c[i] < 'A' || c[i] > 'Z' {
			return false
		}
	}
	return true
}

// Call is one Record call as Recording keeps it.
type Call struct {
	Metric Metric
	Key    Key
	Value  Value
}

// Recording is a Recorder for tests: it validates every call like the stats recorder and keeps
// the valid ones in order; an invalid call is kept in Invalid. It is safe for concurrent use.
type Recording struct {
	mu      sync.Mutex
	calls   []Call
	invalid []error
}

// Record implements Recorder.
func (r *Recording) Record(metric Metric, key Key, value Value) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := Validate(metric, key, value); err != nil {
		r.invalid = append(r.invalid, err)
		return
	}
	r.calls = append(r.calls, Call{Metric: metric, Key: key, Value: value})
}

// Calls returns a copy of the valid calls of metric in order; every valid call when metric is "".
func (r *Recording) Calls(metric Metric) []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Call
	for _, c := range r.calls {
		if metric == "" || c.Metric == metric {
			out = append(out, c)
		}
	}
	return out
}

// Invalid returns the errors of the invalid calls.
func (r *Recording) Invalid() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.invalid)
}
