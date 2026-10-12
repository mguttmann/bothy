// SPDX-License-Identifier: Apache-2.0

// Package telemetry is the adapter of the sync module to the trace context of OpenTelemetry: it
// reads correlation_id and trace_context of a durable event from the context it is appended in
// (docs/spec/03-datenmodell.md event_log, docs/spec/17-observability.md "Signale", R-OB-07).
package telemetry

import (
	"context"
	"encoding/hex"

	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// Correlator implements app.Correlator. The zero value is ready for use.
type Correlator struct{}

// Compile-time check: Correlator is the outgoing port of app.
var _ app.Correlator = Correlator{}

// Correlation returns the correlation of ctx: with a valid span context the 16 bytes of its trace
// ID as correlation_id, and its W3C traceparent as trace_context only when the span is sampled
// (the request was recorded). Without a valid span context the correlation_id of the request in
// kernel/reqmeta (the same ID the response and the audit entry carry; random when no tracer
// issued a trace ID) and no trace_context; outside a request neither.
func (Correlator) Correlation(ctx context.Context) (ids.ID, string) {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		id := ids.ID(sc.TraceID())
		if !sc.IsSampled() {
			return id, ""
		}
		return id, "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-" + sc.TraceFlags().String()
	}
	return parseUUID(reqmeta.CorrelationID(ctx)), ""
}

// parseUUID reads the canonical 8-4-4-4-12 form of any UUID version (a trace ID is no UUIDv7, so
// kernel/ids.Parse does not apply); anything else is the zero ID.
func parseUUID(s string) ids.ID {
	var id ids.ID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return ids.ID{}
	}
	compact := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:36]
	if _, err := hex.Decode(id[:], []byte(compact)); err != nil {
		return ids.ID{}
	}
	return id
}
