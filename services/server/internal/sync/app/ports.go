// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PendingEvent is a durable event buffered in a unit of work: the producer's event with the
// generated ID and the correlation of the append context. The sync hook adds seq and
// permission_version.
type PendingEvent struct {
	// Event is the event; its ID is set.
	Event eventlog.Event
	// CorrelationID is the W3C trace ID of the triggering request; zero for none.
	CorrelationID ids.ID
	// TraceContext is the W3C traceparent of a recorded (sampled) request; empty otherwise.
	TraceContext string
}

// Writer writes numbered events in the ambient transaction (adapters/postgres).
type Writer interface {
	// Write inserts rows with seq first, first+1, ... in the given order and permission_version
	// pv, then sends pg_notify('event_log', <last seq>) with only the number, both in one
	// statement, so the counter lock is held one round trip less (perf.event_commit_min_per_s);
	// PostgreSQL delivers the notification after the commit. occurred_at keeps the column
	// default (the transaction time). An insert without a partition for the rows is kind
	// unavailable; other errors are the database's.
	Write(ctx context.Context, first, pv int64, rows []PendingEvent) error
}

// Correlator reads the correlation of the context an event is appended in (adapters/telemetry).
type Correlator interface {
	// Correlation returns the trace ID of the triggering request as an ID (zero for none) and the
	// W3C traceparent when the request is recorded (empty otherwise).
	Correlation(ctx context.Context) (correlationID ids.ID, traceContext string)
}

// Partitions reads the head of event_log and runs the two partition functions
// (adapters/postgres). Every method runs in the ambient transaction of ctx.
type Partitions interface {
	// Head returns max(seq) of event_log, 0 when it is empty.
	Head(ctx context.Context) (int64, error)
	// Ensure creates partitions so that ahead partitions of size numbers stand ready above
	// head; it returns how many it created.
	Ensure(ctx context.Context, size int64, ahead int, head int64) (int, error)
	// DropExpired detaches and drops the partitions below the one holding head whose newest
	// event is older than cutoff, waiting at most lockTimeout for the lock; it returns how many
	// it dropped.
	DropExpired(ctx context.Context, cutoff time.Time, head int64, lockTimeout time.Duration) (int, error)
}
