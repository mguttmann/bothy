// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"
)

// Metrics is the outgoing port of the sync metrics (02 "Layers within a module" rule 5):
// bothy.event.appended and bothy.db.event_seq.lock_wait of signals.md 4.2. internal/wire passes
// the implementation of adapters/otelmetrics; the application layer knows no metrics library.
// Implementations are safe for concurrent use and never block.
type Metrics interface {
	// EventsAppended counts n committed durable events of eventType; the phase hook calls it
	// after the commit of the unit only.
	EventsAppended(ctx context.Context, eventType string, n int)
	// EventSeqLockWait records the duration d of the statement that locks and advances the
	// event_seq row.
	EventSeqLockWait(ctx context.Context, d time.Duration)
}

// noMetrics is the Metrics of a phase hook without one, for unit tests.
type noMetrics struct{}

// EventsAppended implements Metrics and counts nothing.
func (noMetrics) EventsAppended(context.Context, string, int) {}

// EventSeqLockWait implements Metrics and records nothing.
func (noMetrics) EventSeqLockWait(context.Context, time.Duration) {}
