// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the instance module: the counter
// bothy.console.csp_violations of submitCspReport, exactly as docs/observability/signals.md 4.1
// lists it (M6-03). It implements app.CSPViolationCounter.
//
// Example:
//
//	c, err := otelmetrics.NewCSPViolations(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// CSPViolations is the counter of accepted CSP violation reports (metrics.ConsoleCSPViolations).
type CSPViolations struct {
	c *metrics.Counter
}

// Compile-time check: CSPViolations is the outgoing port of app.
var _ app.CSPViolationCounter = (*CSPViolations)(nil)

// NewCSPViolations creates the counter on mp; nil means the global provider that platform/otel
// installs in otel.Setup. The error comes from the meter provider.
func NewCSPViolations(mp metric.MeterProvider) (*CSPViolations, error) {
	c, err := metrics.NewCounter(mp, metrics.ConsoleCSPViolations, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &CSPViolations{c: c}, nil
}

// Count adds one report; a directive or disposition outside the closed sets records as other.
func (m *CSPViolations) Count(ctx context.Context, directive, disposition string) {
	m.c.Add(ctx, 1, directive, disposition)
}
