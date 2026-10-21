// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the authz module: bothy.authz.decisions
// (docs/observability/signals.md 4.1) through the declaration metrics.AuthzDecisions of the
// metric registry. It implements app.Metrics.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
//	az, err := app.NewAuthorizer(cfg, app.AuthorizerDeps{..., Metrics: m})
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics is the counter of the decisions.
type Metrics struct {
	decisions *metrics.Counter
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the counter on mp; nil means the global provider that platform/otel installs in
// otel.Setup. bothy.action takes its values from the action catalogue (authorize.AllActions).
// The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	acts := authorize.AllActions()
	names := make([]string, len(acts))
	for i, a := range acts {
		names[i] = string(a)
	}
	c, err := metrics.NewCounter(mp, metrics.AuthzDecisions, metrics.Values{metrics.SourceActions: names})
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{decisions: c}, nil
}

// Decision implements app.Metrics.
func (m *Metrics) Decision(ctx context.Context, act authorize.Action, o authorize.Outcome) {
	m.decisions.Add(ctx, 1, string(act), string(o))
}
