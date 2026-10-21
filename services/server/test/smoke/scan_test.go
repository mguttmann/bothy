// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testCanary    = "smk0123456789abcdef01234567"
	testPrincipal = "0192f000-0000-7000-8000-00000000abcd"
)

func testCanaries() Canaries {
	return Canaries{
		{Label: "canary", Value: testCanary},
		{Label: "access_token", Value: "eyJhbGciOiJFUzI1NiJ9.eyJzdWIiOiJ4In0.c2lnbmF0dXJlc2lnbmF0dXJl"},
		{Label: "principal_id", Value: testPrincipal},
	}
}

// writeFile writes content to dir/name and creates the directories.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestScanFindsCanaryAndReportsLabelNotValue: every occurrence is a hit with file, line and label;
// neither the hit nor its text carries the value.
func TestScanFindsCanaryAndReportsLabelNotValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logs := writeFile(t, dir, "logs/compose.log", "api-1 | {\"msg\":\"start\"}\napi-1 | {\"content\":\"x"+testCanary+"y\"}\n")
	writeFile(t, dir, "otel/traces.jsonl", `{"resourceSpans":[]}`+"\n"+`{"attr":"`+testPrincipal+`"}`+"\n")
	hits, err := Scan(testCanaries(), []string{filepath.Join(dir, "otel"), filepath.Join(dir, "logs")})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %v, want 2", hits)
	}
	want := map[string]bool{logs + ":2: canary": true, filepath.Join(dir, "otel/traces.jsonl") + ":2: principal_id": true}
	for _, h := range hits {
		if !want[h.String()] {
			t.Errorf("unexpected hit %q", h.String())
		}
		if strings.Contains(h.String(), testCanary) || strings.Contains(h.String(), testPrincipal) {
			t.Errorf("hit %q carries a value", h.String())
		}
	}
}

// TestScanPositiveControl: a scanner whose matcher finds nothing fails before it scans, so a
// broken scanner cannot pass.
func TestScanPositiveControl(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "logs/compose.log", "clean\n")
	old := match
	match = func(_, _ []byte) bool { return false }
	t.Cleanup(func() { match = old })
	if _, err := Scan(testCanaries(), []string{filepath.Join(dir, "logs")}); err == nil || !strings.Contains(err.Error(), "positive control") {
		t.Errorf("Scan with a broken matcher = %v, want the positive control error", err)
	}
}

// TestScanRejectsShortOrMissingCanaries: an empty list, a list without the label canary and a
// value shorter than minCanaryLen fail; so does a missing root.
func TestScanRejectsShortOrMissingCanaries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "logs/compose.log", "clean\n")
	roots := []string{filepath.Join(dir, "logs")}
	for name, c := range map[string]Canaries{
		"empty":     nil,
		"no canary": {{Label: "access_token", Value: strings.Repeat("a", minCanaryLen)}},
		"short":     {{Label: "canary", Value: testCanary}, {Label: "principal_id", Value: "abc"}},
		"blank":     {{Label: "canary", Value: ""}},
	} {
		if _, err := Scan(c, roots); err == nil {
			t.Errorf("%s: Scan passed", name)
		}
	}
	if _, err := Scan(testCanaries(), []string{filepath.Join(dir, "missing")}); err == nil {
		t.Error("a missing root must fail")
	}
}

// TestScanIgnoresProbeOutputDir: Roots names otel and logs below the input directory only; the
// probe's own output (setup.out, canaries.txt) is never scanned.
func TestScanIgnoresProbeOutputDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := testCanaries()
	writeFile(t, dir, "out/canaries.txt", string(c.Marshal()))
	writeFile(t, dir, "out/setup.out", testCanary+"\n")
	writeFile(t, dir, "otel/logs.jsonl", "{}\n")
	writeFile(t, dir, "logs/compose.log", "clean\n")
	hits, err := Scan(c, Roots(dir))
	if err != nil || len(hits) != 0 {
		t.Errorf("Scan(Roots) = %v, %v; want no hit", hits, err)
	}
}

// TestCanariesRoundTrip: Marshal writes label<TAB>value lines that ParseCanaries reads back; a
// line without a tab is an error.
func TestCanariesRoundTrip(t *testing.T) {
	t.Parallel()
	c := testCanaries()
	got, err := ParseCanaries(c.Marshal())
	if err != nil || len(got) != len(c) || got[0] != c[0] || got[2] != c[2] {
		t.Errorf("round trip = %v, %v", got, err)
	}
	if !bytes.HasPrefix(c.Marshal(), []byte("canary\t")) {
		t.Errorf("Marshal = %q", c.Marshal())
	}
	if _, err := ParseCanaries([]byte("canary " + testCanary + "\n")); err == nil {
		t.Error("a line without a tab must fail")
	}
}

// OTLP JSON lines as the file exporter of the collector writes them.
const (
	spanLine = `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"bothy-server"}}]},` +
		`"scopeSpans":[{"spans":[{"name":"POST","attributes":[{"key":"http.route","value":{"stringValue":"/api/v1/threads/{thread_id}/messages"}}]}]}]}]}`
	logLine = `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"bothy-server"}}]},` +
		`"scopeLogs":[{"logRecords":[{"severityText":"INFO","body":{"stringValue":"request"}}]}]}]}`
	metricLine = `{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"bothy-server"}}]},` +
		`"scopeMetrics":[{"metrics":[{"name":"http.server.request.duration","histogram":{"dataPoints":[{"count":"1"}]}}]}]}]}`
)

// TestCheckTelemetry: a complete export passes; no span of the route, no log record, no metric
// point and another service.name each fail.
func TestCheckTelemetry(t *testing.T) {
	t.Parallel()
	const route = "/api/v1/threads/{thread_id}/messages"
	write := func(traces, logs, metrics string) string {
		dir := t.TempDir()
		writeFile(t, dir, "traces.jsonl", traces)
		writeFile(t, dir, "logs.jsonl", logs)
		writeFile(t, dir, "metrics.jsonl", metrics)
		return dir
	}
	if err := CheckTelemetry(write(spanLine+"\n", logLine+"\n", metricLine+"\n"), "bothy-server", route); err != nil {
		t.Errorf("complete export: %v", err)
	}
	otherRoute := strings.Replace(spanLine, "/messages", "/reactions", 1)
	noLog := strings.Replace(logLine, `{"severityText":"INFO","body":{"stringValue":"request"}}`, "", 1)
	noPoint := strings.Replace(metricLine, `{"count":"1"}`, "", 1)
	for name, dir := range map[string]string{
		"no span of the route": write(otherRoute+"\n", logLine+"\n", metricLine+"\n"),
		"no log record":        write(spanLine+"\n", noLog+"\n", metricLine+"\n"),
		"no metric point":      write(spanLine+"\n", logLine+"\n", noPoint+"\n"),
		"empty files":          write("", "", ""),
		"another service": write(strings.ReplaceAll(spanLine, "bothy-server", "other"), strings.ReplaceAll(logLine, "bothy-server", "other"),
			strings.ReplaceAll(metricLine, "bothy-server", "other")),
	} {
		if err := CheckTelemetry(dir, "bothy-server", route); err == nil {
			t.Errorf("%s: CheckTelemetry passed", name)
		}
	}
	if err := CheckTelemetry(t.TempDir(), "bothy-server", route); err == nil {
		t.Error("missing files must fail")
	}
}
