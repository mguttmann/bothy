// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the audit module: bothy.audit.verify.failures and
// bothy.audit.verify.last_success.timestamp of the job audit.chain_verify, exactly as
// docs/observability/signals.md "4.6" lists them, without attributes (alert
// BothyAuditChainBroken). It implements app.Metrics.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics are the two instruments of the chain verification, declared in the metric registry
// (metrics.AuditVerifyFailures, metrics.AuditVerifyLastSuccess).
type Metrics struct {
	failures    *metrics.Counter
	lastSuccess *metrics.Gauge
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	failures, err := metrics.NewCounter(mp, metrics.AuditVerifyFailures, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	last, err := metrics.NewGauge(mp, metrics.AuditVerifyLastSuccess, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{failures: failures, lastSuccess: last}, nil
}

// VerifyFailed adds one run to bothy.audit.verify.failures.
func (m *Metrics) VerifyFailed(ctx context.Context) { m.failures.Add(ctx, 1) }

// VerifySucceeded sets bothy.audit.verify.last_success.timestamp to at in Unix seconds.
func (m *Metrics) VerifySucceeded(ctx context.Context, at time.Time) {
	m.lastSuccess.Record(ctx, float64(at.UnixNano())/float64(time.Second))
}
