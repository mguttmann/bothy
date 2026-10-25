// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// RateLimitState is the rate-limit state an adapter reported in its last stored heartbeat
// (agents.reported_details.rate_limit, schema AgentRateLimitState): only the state and the reset
// time, never a quota, key or subscription name (09 section 4.8, I5).
type RateLimitState struct {
	// State is ok, near_limit, limited or unknown.
	State string
	// ResetsAt is zero when not reported.
	ResetsAt time.Time
}

// BackendState is what the backend view of stats shows of one agent (docs/spec/25-visualization.md
// 6.2, ruling T8): the configured and reported billing mode, the supported modes and the stored
// rate-limit state. It carries no adapter kind, runtime name or version, evidence, credential
// holder or other credential data (I5).
type BackendState struct {
	AgentID ids.ID
	// BillingMode is agents.billing_mode; ReportedBillingMode agents.reported_billing_mode ("" for
	// none).
	BillingMode, ReportedBillingMode string
	// BillingModesSupported is reported_details.billing_modes_supported; nil when not reported.
	BillingModesSupported []string
	// RateLimit is reported_details.rate_limit; nil when not reported.
	RateLimit *RateLimitState
}

// BackendSource is the source port of the backend view of stats (M6-14). It reads the stored rows
// (not the in-memory presence), which lag the live heartbeat by up to
// agent.heartbeat_persist_interval. It never authorizes: stats decides with stats.read and passes
// only agents of the channels the caller may read.
type BackendSource interface {
	// BackendStates returns the state of the agents among agentIDs, ascending by ID; people, apps
	// and unknown IDs are absent and an empty list reads nothing. Errors: the database error, or
	// stored reported_details outside its schema (internal_error).
	//
	// Example:
	//
	//	states, err := src.BackendStates(ctx, []ids.ID{supportAgent})
	BackendStates(ctx context.Context, agentIDs []ids.ID) ([]BackendState, error)
}
