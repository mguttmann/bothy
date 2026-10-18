// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// collectWait bounds the flush and collection at the end of a run.
const collectWait = 2 * time.Minute

// serverLog returns a logger through the OTLP log pipeline of the running server (the global
// logger provider otel.Setup installed): its records pass the catalogue filter and the bounded
// queue of the server like every record of the server's own logger.
func serverLog() *slog.Logger {
	return slog.New(otelslog.NewHandler("canary-meta", otelslog.WithLoggerProvider(global.GetLoggerProvider())))
}

// findingsIn returns the findings of sink.
func findingsIn(f []Finding, sink Sink) []Finding {
	var out []Finding
	for _, x := range f {
		if x.Sink == sink {
			out = append(out, x)
		}
	}
	return out
}

// TestSEC_T10_DetectorFindsInjectedLog: a canary logged through the server's log pipeline as an
// attribute is exactly one finding in the log sink, at that attribute key.
func TestSEC_T10_DetectorFindsInjectedLog(t *testing.T) {
	r := StartRun(t, RunOptions{})
	v := r.Reg.New(ClassContent, "", "injected log attribute")
	serverLog().LogAttrs(t.Context(), slog.LevelInfo, "canary meta", slog.String("error.type", v))
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	got := findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkLog)
	if len(got) != 1 || !strings.Contains(got[0].Location, "error.type") || got[0].Label != "injected log attribute" {
		t.Fatalf("log findings:\n%s\nwant exactly one at error.type", Report(got))
	}
}

// TestSEC_T10_DetectorFindsInjectedSpanAttribute: a canary set as a span attribute through the
// server's tracer provider is a finding in the trace sink at that key.
func TestSEC_T10_DetectorFindsInjectedSpanAttribute(t *testing.T) {
	r := StartRun(t, RunOptions{})
	v := r.Reg.New(ClassSecret, "", "injected span attribute")
	_, span := otelapi.Tracer("canary-meta").Start(t.Context(), "canary meta")
	span.SetAttributes(attribute.String("bothy.thread.id", v))
	span.End()
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	got := findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkTrace)
	if len(got) != 1 || !strings.Contains(got[0].Location, "bothy.thread.id") || got[0].Class != string(ClassSecret) {
		t.Fatalf("trace findings:\n%s\nwant exactly one at bothy.thread.id", Report(got))
	}
}

// TestSEC_T10_DetectorFindsClearPrincipalIDInLog: a human principal ID in clear in a log
// attribute is an identity finding in the log sink.
func TestSEC_T10_DetectorFindsClearPrincipalIDInLog(t *testing.T) {
	r := StartRun(t, RunOptions{})
	alex := r.Watch(r.Server.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"}), "alex")
	serverLog().LogAttrs(t.Context(), slog.LevelInfo, "canary meta", slog.String("bothy.thread.id", alex.PrincipalID))
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	r.RegisterIdentities(ctx)
	var hits []Finding
	for _, f := range findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkLog) {
		if f.Class == string(ClassIdentity) && strings.Contains(f.Location, "bothy.thread.id") {
			hits = append(hits, f)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("identity findings at bothy.thread.id:\n%s\nwant exactly one", Report(hits))
	}
}
