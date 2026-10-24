// SPDX-License-Identifier: Apache-2.0

package textextract_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The tests of this file run with in-process telemetry sinks and therefore never in parallel with
// another harness start of this binary (harness.Options.Telemetry).

// telemetryText returns everything the server exported into ts: span names, attributes, events
// and status; log records with body and attributes; metric names and data points.
func telemetryText(t *testing.T, e *env, ts *harness.TelemetrySinks) string {
	t.Helper()
	if err := e.s.FlushTelemetry(context.Background()); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, sp := range ts.Spans.GetSpans() {
		fmt.Fprintf(&b, "span %s %v %v %v\n", sp.Name, sp.Attributes, sp.Events, sp.Status)
	}
	for _, r := range ts.Logs.Records() {
		fmt.Fprintf(&b, "log %s", r.Body().String())
		r.WalkAttributes(func(kv attribute.KeyValue) bool {
			fmt.Fprintf(&b, " %s=%s", kv.Key, kv.Value.String())
			return true
		})
		b.WriteString("\n")
	}
	var rm metricdata.ResourceMetrics
	if err := ts.Metrics.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			fmt.Fprintf(&b, "metric %s %+v\n", m.Name, m.Data)
		}
	}
	return b.String()
}

// TestXXEFailedWithoutFetching (plan row, 14 12): a DOCX and an ODT whose XML part declares an
// external entity pointing at a loopback HTTP server started by the test, and at a file URL of a
// file holding a canary value, both used in the text, end failed with xml_doctype; the HTTP server
// counted zero requests; the canary appears in no column of file_text_extracts, no log line and no
// span.
func TestXXEFailedWithoutFetching(t *testing.T) {
	var requests atomic.Int64
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("canary-http-7f3a"))
	}), ReadHeaderTimeout: time.Minute}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	dir, err := os.MkdirTemp("", "xxe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	const canary = "canary-file-91c2"
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte(canary), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := harness.NewTelemetrySinks()
	e := start(t, options{telemetry: ts})
	var files []string
	for i, url := range []string{"http://" + l.Addr().String() + "/x", "file://" + secret} {
		v := extracttest.XXESystem(url)
		d, _ := e.attached(t, fmt.Sprintf("entity %d.docx", i), extract.TypeDOCX, v.DOCX())
		o, _ := e.attached(t, fmt.Sprintf("entity %d.odt", i), extract.TypeODT, v.ODT())
		files = append(files, d, o)
	}
	for _, id := range files {
		if r := e.extracted(t, id); r.State != "failed" || r.Error != "xml_doctype" || r.HasText {
			t.Fatalf("file %s: %+v", id, r)
		}
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("the loopback server got %d requests", n)
	}
	if n := e.count(t, `SELECT count(*) FROM file_text_extracts WHERE row_to_json(file_text_extracts)::text LIKE '%canary%'`); n != 0 {
		t.Fatalf("%d rows carry a canary", n)
	}
	for _, where := range []string{e.logs.text(), telemetryText(t, e, ts)} {
		if strings.Contains(where, "canary") {
			t.Fatal("a canary reached the logs or the telemetry")
		}
	}
}

// TestExtractNoContentInTelemetry (SEC-LOG-4, SEC-T10 for this path): a document whose text and
// file name carry canary values, one run ending done and one ending failed; the logs, spans and
// metric attributes hold neither canary; the failure log has bothy.file.id and bothy.reason and
// nothing of the file; the arguments of the jobs hold only the file id.
func TestExtractNoContentInTelemetry(t *testing.T) {
	const textCanary, nameCanary = "Textkanarienvogel4711", "Namenskanarie0815"
	ts := harness.NewTelemetrySinks()
	e := start(t, options{telemetry: ts})
	done, _ := e.attached(t, nameCanary+" a.docx", extract.TypeDOCX, extracttest.DOCX("Inhalt "+textCanary))
	failed, _ := e.attached(t, nameCanary+" b.docx", extract.TypeDOCX, extracttest.XXESystem("http://127.0.0.1:1/x").DOCX())
	if r := e.extracted(t, done); r.State != "done" || !strings.Contains(r.Text, textCanary) {
		t.Fatalf("done: %+v", r)
	}
	if r := e.extracted(t, failed); r.State != "failed" {
		t.Fatalf("failed: %+v", r)
	}
	for _, where := range []string{e.logs.text(), telemetryText(t, e, ts)} {
		if strings.Contains(where, textCanary) || strings.Contains(where, nameCanary) {
			t.Fatal("a canary reached the logs or the telemetry")
		}
	}
	lines := e.logs.lines("file text extraction failed")
	if len(lines) != 1 {
		t.Fatalf("failure log lines: %q", lines)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatal(err)
	}
	if line["bothy.file.id"] != failed || line["bothy.reason"] != "xml_doctype" {
		t.Fatalf("failure log %v", line)
	}
	for k := range line {
		if strings.HasPrefix(k, "bothy.") && k != "bothy.file.id" && k != "bothy.reason" && !strings.HasPrefix(k, "bothy.job") &&
			!strings.HasPrefix(k, "bothy.correlation") && !strings.HasPrefix(k, "bothy.role") {
			t.Errorf("failure log carries %s", k)
		}
	}
	rows, err := e.db.Query(t.Context(), `SELECT args::text FROM jobs.river_job WHERE kind = 'files.extract_text'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var args string
		if err := rows.Scan(&args); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(args), &m); err != nil || len(m) != 1 || m["file_id"] == nil {
			t.Fatalf("job arguments %s", args)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
