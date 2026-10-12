// SPDX-License-Identifier: Apache-2.0

// Package telemetry is the metrics adapter of the authz module: it counts programming errors of
// the rights check in bothy.authz.errors (docs/observability/signals.md, alarm BothyAuthzError)
// for app.Catalog.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Names of the instrument (signals.md "REST und Gateway").
const (
	instrumentation   = "github.com/OWNER/bothy/services/server/internal/authz"
	metricAuthzErrors = "bothy.authz.errors"
	attrReason        = "bothy.reason"
)

// ErrorCounter is bothy.authz.errors of the authz module. It implements app.ErrorCounter.
type ErrorCounter struct {
	counter metric.Int64Counter
}

// NewErrorCounter creates the counter on mp; nil means the global provider that platform/otel
// installs in otel.Setup (internal/wire passes nil after that step). The error comes from the
// meter provider.
func NewErrorCounter(mp metric.MeterProvider) (*ErrorCounter, error) {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}
	c, err := mp.Meter(instrumentation).Int64Counter(metricAuthzErrors,
		metric.WithUnit("{check}"),
		metric.WithDescription("Programming errors of the authorization check."))
	if err != nil {
		return nil, fmt.Errorf("telemetry: %s: %w", metricAuthzErrors, err)
	}
	return &ErrorCounter{counter: c}, nil
}

// CountError adds one error with bothy.reason = reason (a value of the fixed list of signals.md).
func (e *ErrorCounter) CountError(ctx context.Context, reason string) {
	e.counter.Add(ctx, 1, metric.WithAttributes(attribute.String(attrReason, reason)))
}
