// SPDX-License-Identifier: Apache-2.0

package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// Metrics are the instruments of the sync phase hook: bothy.event.appended and
// bothy.db.event_seq.lock_wait (signals.md 4.2), declared in the metric registry. It implements
// app.Metrics.
type Metrics struct {
	appended *metrics.Counter
	lockWait *metrics.Histogram
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. bothy.event.type takes its values from the durable event types of the action
// catalogue (authorize.AllEventTypes). The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	types := authorize.AllEventTypes()
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	appended, err := metrics.NewCounter(mp, metrics.EventAppended, metrics.Values{metrics.SourceEventTypes: names})
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	wait, err := metrics.NewHistogram(mp, metrics.EventSeqLockWait, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{appended: appended, lockWait: wait}, nil
}

// EventsAppended implements app.Metrics.
func (m *Metrics) EventsAppended(ctx context.Context, eventType string, n int) {
	m.appended.Add(ctx, int64(n), eventType)
}

// EventSeqLockWait implements app.Metrics.
func (m *Metrics) EventSeqLockWait(ctx context.Context, d time.Duration) {
	m.lockWait.RecordDuration(ctx, d)
}
