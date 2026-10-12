// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// groupValuer is a LogValuer like secrets.Value or errs.Error: the filter resolves it before it
// checks the keys. Logged under the key "bothy", its members become bothy.message (dropped) and
// bothy.reason (kept).
type groupValuer struct{}

func (groupValuer) LogValue() slog.Value {
	return slog.GroupValue(slog.String("message", "canary-valuer"), slog.String("reason", "kept"))
}

// decodeLines parses one JSON object per line.
func decodeLines(t *testing.T, out string) []map[string]any {
	t.Helper()
	var recs []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %q is no JSON object: %v", line, err)
		}
		recs = append(recs, m)
	}
	return recs
}

func sampledContext() (context.Context, trace.SpanContext) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), sc), sc
}

// TestAllowlistFilterDropsUnknownAttributes: logs and spans keep only attr.Allowed keys.
func TestAllowlistFilterDropsUnknownAttributes(t *testing.T) {
	t.Parallel()
	t.Run("logs", testLogAllowlist)
	t.Run("spans", testSpanAllowlist)
}

// testLogAllowlist: every attribute outside attr.Allowed is dropped, also inside groups, behind
// WithGroup and WithAttrs and inside a LogValuer; groups are flattened to dotted keys; the record
// itself is never dropped.
func testLogAllowlist(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := NewCommandLogger(&buf, slog.LevelDebug)
	ctx := context.Background()

	logger.LogAttrs(ctx, slog.LevelInfo, "only unknown keys", slog.String("url.query", "canary-1"), slog.String("password", "canary-2"))
	logger.LogAttrs(ctx, slog.LevelInfo, "group",
		slog.Group("http", slog.String("route", "/a/{id}"), slog.String("target", "canary-3")),
		slog.Group("", slog.String(attr.Outcome, "ok"), slog.String("raw", "canary-4")),
		slog.Any("bothy", groupValuer{}))
	logger.With(slog.String(attr.Action, "thread.read"), slog.String("cookie", "canary-5")).
		WithGroup("bothy").With(slog.String("thread.id", "t-1"), slog.String("secret", "canary-6")).
		LogAttrs(ctx, slog.LevelInfo, "with group", slog.String("run.id", "r-1"), slog.String("body", "canary-7"))

	out := buf.String()
	if strings.Contains(out, "canary") {
		t.Fatalf("output contains a dropped value:\n%s", out)
	}
	recs := decodeLines(t, out)
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (the filter never drops a record):\n%s", len(recs), out)
	}
	wantAttrs := []map[string]any{
		{},
		{attr.HTTPRoute: "/a/{id}", attr.Outcome: "ok", attr.Reason: "kept"},
		{attr.Action: "thread.read", attr.ThreadID: "t-1", attr.RunID: "r-1"},
	}
	for i, rec := range recs {
		for _, k := range []string{"time", "level", "msg"} {
			delete(rec, k)
		}
		if len(rec) != len(wantAttrs[i]) {
			t.Errorf("record %d: attributes %v, want %v", i, rec, wantAttrs[i])
			continue
		}
		for k, v := range wantAttrs[i] {
			if rec[k] != v {
				t.Errorf("record %d: %s = %v, want %v", i, k, rec[k], v)
			}
		}
	}
}

// TestCommandLoggerJSON: level filter, JSON lines, and the trace fields from the context span.
func TestCommandLoggerJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := NewCommandLogger(&buf, slog.LevelWarn)
	logger.InfoContext(context.Background(), "below the level")
	ctx, sc := sampledContext()
	logger.WarnContext(ctx, "with span")
	logger.WarnContext(context.Background(), "without span")

	recs := decodeLines(t, buf.String())
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2:\n%s", len(recs), buf.String())
	}
	if recs[0]["msg"] != "with span" || recs[0]["trace_id"] != sc.TraceID().String() ||
		recs[0]["span_id"] != sc.SpanID().String() || recs[0][attr.CorrelationID] != "01020304-0506-0708-090a-0b0c0d0e0f10" {
		t.Errorf("record with span = %v", recs[0])
	}
	for _, k := range []string{"trace_id", "span_id", attr.CorrelationID} {
		if _, ok := recs[1][k]; ok {
			t.Errorf("record without span has %s: %v", k, recs[1])
		}
	}
}

func TestCorrelationIDFormat(t *testing.T) {
	t.Parallel()
	ctx, _ := sampledContext()
	if got, want := CorrelationID(ctx), "01020304-0506-0708-090a-0b0c0d0e0f10"; got != want {
		t.Errorf("CorrelationID = %q, want %q", got, want)
	}
	if got := CorrelationID(context.Background()); got != "" {
		t.Errorf("CorrelationID without span = %q, want empty", got)
	}
}

func TestParseLevel(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError} {
		got, err := ParseLevel(text)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", text, got, err, want)
		}
	}
	for _, bad := range []string{"", "INFO", "warning", "trace"} {
		if _, err := ParseLevel(bad); err == nil {
			t.Errorf("ParseLevel(%q) succeeded, want an error", bad)
		}
	}
}

// TestServerErrorLogHasNoDetails holds SEC-LOG-5: the lines of http.Server.ErrorLog carry client
// addresses and foreign error texts; the log keeps only a constant message and a closed class.
func TestServerErrorLogHasNoDetails(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := ServerErrorLog(NewCommandLogger(&buf, slog.LevelDebug))
	lines := map[string]string{
		"http: TLS handshake error from 192.0.2.1:51234: remote error: canary-a":       "tls_handshake",
		"http: Accept error: accept tcp 192.0.2.1:8080: too many open files; canary-b": "accept",
		"http: panic serving 192.0.2.1:51234: canary-c\ngoroutine 1 [running]:":        "panic",
		"http2: server: error reading preface from client 192.0.2.1:51234: canary-d":   "other",
	}
	var want []string
	for line, class := range lines {
		l.Print(line)
		want = append(want, class)
	}
	out := buf.String()
	if strings.Contains(out, "canary") || strings.Contains(out, "192.0.2.1") {
		t.Fatalf("server error log keeps details:\n%s", out)
	}
	recs := decodeLines(t, out)
	if len(recs) != len(lines) {
		t.Fatalf("got %d records, want %d", len(recs), len(lines))
	}
	for i, rec := range recs {
		if rec["msg"] != "http server error" || rec["level"] != "WARN" || rec[attr.ErrorType] != want[i] {
			t.Errorf("record %d = %v, want message %q, level WARN, error.type %q", i, rec, "http server error", want[i])
		}
	}
}

// TestAllowlistFilterDropsNonScalarValues: an error or a struct under an allowed key would reach
// the output as its text; only scalars and string lists pass.
func TestAllowlistFilterDropsNonScalarValues(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	NewCommandLogger(&buf, slog.LevelInfo).LogAttrs(context.Background(), slog.LevelError, "non scalar",
		slog.Any(attr.ErrorType, errors.New("canary-error")),
		slog.Any(attr.Reason, struct{ S string }{"canary-struct"}),
		slog.Any("gen_ai.response.finish_reasons", []string{"stop"}),
		slog.Int(attr.HTTPResponseStatusCode, 500))
	out := buf.String()
	if strings.Contains(out, "canary") {
		t.Fatalf("output contains a non scalar value:\n%s", out)
	}
	rec := decodeLines(t, out)[0]
	if rec[attr.HTTPResponseStatusCode] != float64(500) {
		t.Errorf("status code = %v, want 500", rec[attr.HTTPResponseStatusCode])
	}
	if got, ok := rec["gen_ai.response.finish_reasons"].([]any); !ok || len(got) != 1 || got[0] != "stop" {
		t.Errorf("finish reasons = %v, want [stop]", rec["gen_ai.response.finish_reasons"])
	}
}
