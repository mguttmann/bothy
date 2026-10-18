// SPDX-License-Identifier: Apache-2.0

// Package telemetry is the metrics adapter of the authz module: it counts programming errors of
// the rights check in bothy.authz.errors (docs/observability/signals.md, alarm BothyAuthzError)
// for app.Catalog, through the declaration metrics.AuthzErrors of the metric registry.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// ErrorCounter is bothy.authz.errors of the authz module. It implements app.ErrorCounter.
type ErrorCounter struct {
	counter *metrics.Counter
}

// NewErrorCounter creates the counter on mp; nil means the global provider that platform/otel
// installs in otel.Setup (internal/wire passes nil after that step). The error comes from the
// meter provider.
func NewErrorCounter(mp metric.MeterProvider) (*ErrorCounter, error) {
	c, err := metrics.NewCounter(mp, metrics.AuthzErrors, nil)
	if err != nil {
		return nil, fmt.Errorf("telemetry: %w", err)
	}
	return &ErrorCounter{counter: c}, nil
}

// CountError adds one error with bothy.reason = reason (a value of the fixed list of signals.md;
// any other value counts as other).
func (e *ErrorCounter) CountError(ctx context.Context, reason string) {
	e.counter.Add(ctx, 1, reason)
}
