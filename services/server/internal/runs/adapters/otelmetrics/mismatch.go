// SPDX-License-Identifier: Apache-2.0

package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// Mismatches implements app.MismatchCounter: the counter bothy.agent.billing_mode_mismatch through
// the declaration metrics.AgentBillingModeMismatch, with bothy.agent.handle only.
type Mismatches struct {
	c *metrics.Counter
}

// Compile-time check: Mismatches is the outgoing port of app.
var _ app.MismatchCounter = (*Mismatches)(nil)

// NewMismatches creates the counter on mp; nil means the global provider of platform/otel. The
// error comes from the meter provider.
func NewMismatches(mp metric.MeterProvider) (*Mismatches, error) {
	c, err := metrics.NewCounter(mp, metrics.AgentBillingModeMismatch, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Mismatches{c: c}, nil
}

// BillingModeMismatch implements app.MismatchCounter: one count with the agent handle.
func (m *Mismatches) BillingModeMismatch(ctx context.Context, agentHandle string) {
	m.c.Add(ctx, 1, agentHandle)
}
