// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Transitioned is the result of Transition: the run after the change, the pair and the times the
// run metrics need (signals.md 4.3).
type Transitioned struct {
	// Run is the run after the change.
	Run runs.RunSummary
	// From and To are the transition.
	From, To domain.State
	// AgentHandle is the handle of the run's agent for bothy.agent.handle; "" when unknown.
	AgentHandle string
	// QueuedFor is queued_at until the change, set when the run left queued.
	QueuedFor time.Duration
	// Ran is started_at until finished_at, set when a started run ended.
	Ran time.Duration
	// Started reports that the run has a started_at after the change.
	Started bool
}

// Metrics is the outgoing metrics port of the run state changes (02 section 5 rule 5);
// adapters/otelmetrics implements it with the instruments of signals.md 4.3. Transition registers
// the call with txn.AfterCommit, so a vetoed or retried unit of work records nothing twice
// (D-M2-05-15).
type Metrics interface {
	// RunTransitioned records one committed transition: bothy.run.start.latency when the run left
	// queued, bothy.run.duration when a started run ended and bothy.run.finished on every end
	// state.
	RunTransitioned(ctx context.Context, t Transitioned)
	// CostBooked records one booked cost entry as bothy.run.cost. The caller registers it with
	// txn.AfterCommit, never inside the unit of work, so a retried unit counts nothing twice.
	CostBooked(ctx context.Context, c BookedCost)
}

// BookedCost is one booked cost entry as bothy.run.cost needs it (signals.md 4.3): recorded after
// the commit of the unit of work that booked it, never inside, so a retried unit counts nothing
// twice. It carries no ID (signals.md section 2).
type BookedCost struct {
	// AgentHandle is the handle of the run's agent for bothy.agent.handle; "" when unknown.
	AgentHandle string
	// BillingMode is the entry's billing_mode, personal or organization.
	BillingMode string
	// Currency is the entry's ISO 4217 currency code.
	Currency string
	// AmountMicros is the entry's amount in millionths of a currency unit.
	AmountMicros int64
	// IsEstimate is the entry's is_estimate, bothy.cost.estimate.
	IsEstimate bool
}

// noMetrics records nothing; it is the Metrics of a Service built without one.
type noMetrics struct{}

// RunTransitioned implements Metrics without effect.
func (noMetrics) RunTransitioned(context.Context, Transitioned) {}

// CostBooked implements Metrics without effect.
func (noMetrics) CostBooked(context.Context, BookedCost) {}

// ActiveGroup is one data point of the gauge bothy.run.active: the active runs of one agent handle
// in one state.
type ActiveGroup struct {
	AgentHandle string
	State       domain.State
	Count       int64
}
