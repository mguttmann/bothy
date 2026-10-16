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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
)

// Names of the instruments (signals.md 4.6).
const (
	instrumentation   = "github.com/OWNER/bothy/services/server/internal/audit"
	metricFailures    = "bothy.audit.verify.failures"
	metricLastSuccess = "bothy.audit.verify.last_success.timestamp"
)

// Metrics are the two instruments of the chain verification.
type Metrics struct {
	failures    metric.Int64Counter
	lastSuccess metric.Float64Gauge
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}
	meter := mp.Meter(instrumentation)
	failures, err := meter.Int64Counter(metricFailures, metric.WithUnit("{run}"),
		metric.WithDescription("Runs of the periodic audit chain verification that found a broken chain."))
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %s: %w", metricFailures, err)
	}
	last, err := meter.Float64Gauge(metricLastSuccess, metric.WithUnit("s"),
		metric.WithDescription("Unix time of the last successful audit chain verification."))
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %s: %w", metricLastSuccess, err)
	}
	return &Metrics{failures: failures, lastSuccess: last}, nil
}

// VerifyFailed adds one run to bothy.audit.verify.failures.
func (m *Metrics) VerifyFailed(ctx context.Context) { m.failures.Add(ctx, 1) }

// VerifySucceeded sets bothy.audit.verify.last_success.timestamp to at in Unix seconds.
func (m *Metrics) VerifySucceeded(ctx context.Context, at time.Time) {
	m.lastSuccess.Record(ctx, float64(at.UnixNano())/float64(time.Second))
}
