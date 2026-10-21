// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// attrFields returns one field per attribute below prefix: the key as a field of its own (a
// canary used as an attribute key is found) and the value.
func attrFields(prefix string, kvs []attribute.KeyValue) []Field {
	out := make([]Field, 0, 2*len(kvs))
	for _, kv := range kvs {
		loc := prefix + "/" + string(kv.Key)
		out = append(out, Field{Location: loc + "#key", Text: string(kv.Key)}, Field{Location: loc, Text: kv.Value.String()})
	}
	return out
}

// resourceFields returns the attributes of r below /resource.
func resourceFields(r *sdkresource.Resource) []Field {
	if r == nil {
		return nil
	}
	return attrFields("/resource", r.Attributes())
}

// CollectTelemetry adds what the server exported into ts to s: every span (name, attributes,
// events with their attributes, status description, link attributes, resource) to the trace
// sink, every OTLP log record (body, severity text, event name, attributes, resource) to the log
// sink and every metric point (metric name, attributes, exemplar attributes, resource) collected
// from the manual reader to the metric sink. Call FlushTelemetry of the server first.
func CollectTelemetry(ctx context.Context, s *Sinks, ts *harness.TelemetrySinks) error {
	for i, sp := range ts.Spans.GetSpans() {
		f := []Field{{Location: "/name", Text: sp.Name}, {Location: "/status/description", Text: sp.Status.Description}}
		f = append(f, attrFields("/attributes", sp.Attributes)...)
		for j, ev := range sp.Events {
			p := "/events/" + strconv.Itoa(j)
			f = append(f, Field{Location: p + "/name", Text: ev.Name})
			f = append(f, attrFields(p+"/attributes", ev.Attributes)...)
		}
		for j, l := range sp.Links {
			f = append(f, attrFields("/links/"+strconv.Itoa(j)+"/attributes", l.Attributes)...)
		}
		f = append(f, resourceFields(sp.Resource)...)
		s.Add(Record{Sink: SinkTrace, Source: fmt.Sprintf("span %d %q", i, sp.Name), Fields: f})
	}
	for i, r := range ts.Logs.Records() {
		f := []Field{{Location: "/body", Text: r.Body().String()}, {Location: "/severity_text", Text: r.SeverityText()}, {Location: "/event_name", Text: r.EventName()}}
		var kvs []attribute.KeyValue
		r.WalkAttributes(func(kv attribute.KeyValue) bool { kvs = append(kvs, kv); return true })
		f = append(f, attrFields("/attributes", kvs)...)
		f = append(f, resourceFields(r.Resource())...)
		s.Add(Record{Sink: SinkLog, Source: "otlp record " + strconv.Itoa(i), Fields: f})
	}
	var rm metricdata.ResourceMetrics
	if err := ts.Metrics.Collect(ctx, &rm); err != nil {
		return err
	}
	s.Add(Record{Sink: SinkMetric, Source: "resource", Fields: resourceFields(rm.Resource)})
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			s.Add(Record{Sink: SinkMetric, Source: "metric " + m.Name, Fields: []Field{{Location: "/name", Text: m.Name}, {Location: "/description", Text: m.Description}}})
			for i, p := range points(m.Data) {
				src := fmt.Sprintf("metric %s point %d", m.Name, i)
				s.Add(Record{Sink: SinkMetric, Source: src, Fields: append(attrFields("/attributes", p.attrs), attrFields("/exemplar", p.exemplar)...)})
			}
		}
	}
	return nil
}

// point is the part of a metric data point Scan reads.
type point struct{ attrs, exemplar []attribute.KeyValue }

// points returns the attribute sets and exemplar attributes of every data point of data.
func points(data metricdata.Aggregation) []point {
	var out []point
	add := func(set attribute.Set, ex [][]attribute.KeyValue) {
		p := point{attrs: set.ToSlice()}
		for _, e := range ex {
			p.exemplar = append(p.exemplar, e...)
		}
		out = append(out, p)
	}
	switch d := data.(type) {
	case metricdata.Sum[int64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.Sum[float64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.Gauge[int64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.Gauge[float64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.Histogram[int64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.Histogram[float64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.ExponentialHistogram[int64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	case metricdata.ExponentialHistogram[float64]:
		for _, p := range d.DataPoints {
			add(p.Attributes, exemplars(p.Exemplars))
		}
	}
	return out
}

// exemplars returns the filtered attributes of each exemplar.
func exemplars[N int64 | float64](ex []metricdata.Exemplar[N]) [][]attribute.KeyValue {
	out := make([][]attribute.KeyValue, 0, len(ex))
	for _, e := range ex {
		out = append(out, e.FilteredAttributes)
	}
	return out
}

// CollectAudit adds every row of audit_events, all columns as JSON (details as its members, so
// the reserved member reason has the location /details/reason), to the audit sink.
func CollectAudit(ctx context.Context, s *Sinks, db *pgxpool.Pool) (int, error) {
	rows, err := db.Query(ctx, `SELECT row_to_json(a)::text FROM audit_events a ORDER BY 1`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return n, err
		}
		n++
		s.AddJSON(SinkAudit, "audit row "+strconv.Itoa(n), "", "", []byte(row))
	}
	return n, rows.Err()
}

// ErrorObserver returns a Client.Observer that adds every response with status 400 or higher
// (body fields and every header) to the error sink, and passes every request and response to
// next when it is not nil.
func ErrorObserver(s *Sinks, who string, next func(harness.Request, harness.Response)) func(harness.Request, harness.Response) {
	return func(r harness.Request, resp harness.Response) {
		if next != nil {
			next(r, resp)
		}
		if resp.Status < http.StatusBadRequest {
			return
		}
		src := fmt.Sprintf("%s %s %s -> %d", who, r.Method, routeOf(r.Method, r.Path), resp.Status)
		f := markEcho(JSONFields("/body", resp.Body), resp.Body)
		for name, vs := range resp.Header {
			for _, v := range vs {
				f = append(f, Field{Location: "/header/" + name, Text: v})
			}
		}
		s.Add(Record{Sink: SinkError, Source: src, Fields: f})
	}
}

// markEcho sets Field.Echo on the fields of an error body that the contract returns to the
// caller: everything below /body/current of version_conflict and base_version_conflict, and the
// pointer of every errors[] entry with code unknown_member.
func markEcho(fields []Field, body []byte) []Field {
	var p struct {
		Code   string `json:"code"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(body, &p)
	echo := map[string]bool{}
	for i, e := range p.Errors {
		if e.Code == "unknown_member" {
			echo["/body/errors/"+strconv.Itoa(i)+"/pointer"] = true
		}
	}
	current := p.Code == "version_conflict" || p.Code == "base_version_conflict"
	for i := range fields {
		loc := fields[i].Location
		if echo[loc] || current && (loc == "/body/current" || strings.HasPrefix(loc, "/body/current/") || strings.HasPrefix(loc, "/body/current#")) {
			fields[i].Echo = true
		}
	}
	return fields
}
