// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// queueLogProcessor puts every emitted log record, with its attributes filtered to the catalog,
// into a front queue and forwards it to an SDK BatchProcessor whose queue is as large as the
// budget (D3). OnEmit never blocks; a record beyond the budget is counted in
// bothy.telemetry.dropped{bothy.signal=logs}.
//
// The SDK batch processor has no blocking option: on a full queue it drops its oldest record
// without telling anyone. The budget keeps it from ever holding more records than its queue
// size, so it never drops.
type queueLogProcessor struct {
	q     *frontQueue[sdklog.Record]
	batch sdklog.Processor
}

func newQueueLogProcessor(b budget, batch sdklog.Processor, dropped func(int64)) *queueLogProcessor {
	forward := func(r sdklog.Record) {
		// The batch processor copies what it keeps; the context only carries cancellation.
		if err := batch.OnEmit(context.Background(), &r); err != nil {
			b.release(1)
			dropped(1)
		}
	}
	return &queueLogProcessor{q: newFrontQueue(b, forward, dropped), batch: batch}
}

// Enabled reports true: the level filter of the slog chain already decided.
func (*queueLogProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

// OnEmit queues a filtered copy of the record without blocking.
func (p *queueLogProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	rec := r.Clone()
	filterRecord(&rec)
	p.q.offer(rec)
	return nil
}

// ForceFlush forwards the queued records and flushes the batch processor within ctx.
func (p *queueLogProcessor) ForceFlush(ctx context.Context) error {
	if err := p.q.flush(ctx); err != nil {
		return err
	}
	return p.batch.ForceFlush(ctx)
}

// Shutdown forwards the queued records, then shuts the batch processor and its exporter down,
// all within ctx.
func (p *queueLogProcessor) Shutdown(ctx context.Context) error {
	return errors.Join(p.q.close(ctx), p.batch.Shutdown(ctx))
}

// filterRecord keeps only attributes with an allowed key and a catalog value. The slog chain
// filters already; this covers records that reach the global logger provider another way.
func filterRecord(r *sdklog.Record) {
	kept := make([]attribute.KeyValue, 0, r.AttributesLen())
	dropped := false
	r.WalkAttributes(func(kv attribute.KeyValue) bool {
		if attr.Allowed(string(kv.Key)) && scalarValue(kv.Value) {
			kept = append(kept, kv)
		} else {
			dropped = true
		}
		return true
	})
	if dropped {
		r.SetAttributes(kept...)
	}
}

// countingLogExporter counts every record of a failed export in bothy.telemetry.dropped and
// returns the budget slots of the exported records.
type countingLogExporter struct {
	next    sdklog.Exporter
	dropped func(int64)
	budget  budget
}

// Export exports, counts the records of a failed export and frees their slots.
func (e countingLogExporter) Export(ctx context.Context, records []sdklog.Record) error {
	err := e.next.Export(ctx, records)
	if err != nil {
		e.dropped(int64(len(records)))
	}
	e.budget.release(len(records))
	return err
}

// ForceFlush flushes the next exporter.
func (e countingLogExporter) ForceFlush(ctx context.Context) error {
	return e.next.ForceFlush(ctx)
}

// Shutdown shuts the next exporter down.
func (e countingLogExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}
