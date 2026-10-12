// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// queueSpanProcessor puts ended, sampled spans into a front queue and forwards them to an SDK
// BatchSpanProcessor built WithBlocking and with a queue as large as the budget (D3). OnEnd never
// blocks; a span beyond the budget is counted in bothy.telemetry.dropped{bothy.signal=traces}.
// The batch processor never holds more spans than the budget, so it neither waits nor drops
// (it would wait rather than drop, so the SDK never loses a span silently).
type queueSpanProcessor struct {
	q     *frontQueue[sdktrace.ReadOnlySpan]
	batch sdktrace.SpanProcessor
}

func newQueueSpanProcessor(b budget, batch sdktrace.SpanProcessor, dropped func(int64)) *queueSpanProcessor {
	return &queueSpanProcessor{q: newFrontQueue(b, batch.OnEnd, dropped), batch: batch}
}

// OnStart does nothing; the processor only sees ended spans.
func (*queueSpanProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

// OnEnd queues a sampled span without blocking.
func (p *queueSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if !s.SpanContext().IsSampled() {
		return
	}
	p.q.offer(s)
}

// ForceFlush forwards the queued spans and flushes the batch processor within ctx.
func (p *queueSpanProcessor) ForceFlush(ctx context.Context) error {
	if err := p.q.flush(ctx); err != nil {
		return err
	}
	return p.batch.ForceFlush(ctx)
}

// Shutdown forwards the queued spans, then shuts the batch processor and its exporter down,
// all within ctx.
func (p *queueSpanProcessor) Shutdown(ctx context.Context) error {
	return errors.Join(p.q.close(ctx), p.batch.Shutdown(ctx))
}

// filterSpanExporter keeps only attr.Allowed keys with catalog values on every exported span,
// its events and its links, and clears the status description and the trace states, which are
// free text (D5). It
// removes url.path, url.query, client.address, network.peer.* and every attribute a library adds
// later. Server spans also lose server.address and server.port, because otelhttp takes them from
// the Host header of the client. The pinned SDK has no supported attribute filter for spans, so
// the exporter rebuilds each span with tracetest.SpanStub.
type filterSpanExporter struct {
	next sdktrace.SpanExporter
}

// ExportSpans filters the spans and hands them to the next exporter.
func (e filterSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	out := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		out[i] = filterSpan(s)
	}
	return e.next.ExportSpans(ctx, out)
}

// Shutdown shuts the next exporter down.
func (e filterSpanExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}

func filterSpan(s sdktrace.ReadOnlySpan) sdktrace.ReadOnlySpan {
	stub := tracetest.SpanStubFromReadOnlySpan(s)
	server := stub.SpanKind == trace.SpanKindServer
	stub.Attributes = filterAttributes(stub.Attributes, server)
	for i := range stub.Events {
		stub.Events[i].Attributes = filterAttributes(stub.Events[i].Attributes, false)
	}
	for i := range stub.Links {
		stub.Links[i].Attributes = filterAttributes(stub.Links[i].Attributes, false)
		stub.Links[i].SpanContext = stub.Links[i].SpanContext.WithTraceState(trace.TraceState{})
	}
	// The trace state comes from the tracestate header of the client: free text outside the
	// header allowlist. Propagation to outgoing calls keeps it; only the export drops it.
	stub.SpanContext = stub.SpanContext.WithTraceState(trace.TraceState{})
	stub.Parent = stub.Parent.WithTraceState(trace.TraceState{})
	stub.Status.Description = ""
	return stub.Snapshot()
}

// filterAttributes returns the attributes with an allowed key and a catalog value. server drops
// the two keys otelhttp fills from the Host header.
func filterAttributes(kvs []attribute.KeyValue, server bool) []attribute.KeyValue {
	kept := make([]attribute.KeyValue, 0, len(kvs))
	for _, kv := range kvs {
		key := string(kv.Key)
		if !attr.Allowed(key) || !scalarValue(kv.Value) {
			continue
		}
		if server && (key == attr.ServerAddress || key == attr.ServerPort) {
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}

// scalarValue reports whether v is a value a catalog attribute may carry: a bool, number or
// string, or a list of them. Bytes, maps and mixed lists are dropped.
func scalarValue(v attribute.Value) bool {
	switch v.Type() {
	case attribute.BOOL, attribute.INT64, attribute.FLOAT64, attribute.STRING,
		attribute.BOOLSLICE, attribute.INT64SLICE, attribute.FLOAT64SLICE, attribute.STRINGSLICE:
		return true
	case attribute.EMPTY, attribute.BYTESLICE, attribute.SLICE, attribute.MAP:
		return false
	default:
		return false
	}
}

// countingSpanExporter counts every span of a failed export in bothy.telemetry.dropped and
// returns the budget slots of the exported spans.
type countingSpanExporter struct {
	next    sdktrace.SpanExporter
	dropped func(int64)
	budget  budget
}

// ExportSpans exports, counts the spans of a failed export and frees their slots.
func (e countingSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.next.ExportSpans(ctx, spans)
	if err != nil {
		e.dropped(int64(len(spans)))
	}
	e.budget.release(len(spans))
	return err
}

// Shutdown shuts the next exporter down.
func (e countingSpanExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}
