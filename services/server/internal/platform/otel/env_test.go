// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"errors"
	"strings"
	"testing"
)

func TestOTELEnvParsing(t *testing.T) {
	t.Parallel()
	got, err := parseEnv(nil)
	if err != nil {
		t.Fatalf("parseEnv(nil): %v", err)
	}
	want := otelEnv{
		traces: signalEnv{protocol: protocolGRPC}, metrics: signalEnv{protocol: protocolGRPC},
		logs: signalEnv{protocol: protocolGRPC}, spanQueue: defaultQueueSize, logQueue: defaultQueueSize,
	}
	if got != want {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}

	got, err = parseEnv([]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL=grpc",
		"OTEL_TRACES_EXPORTER=none",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_BSP_MAX_QUEUE_SIZE=16",
		"OTEL_BSP_MAX_QUEUE_SIZE=32", // the first entry of a name wins
		"OTEL_BLRP_MAX_QUEUE_SIZE= 8 ",
		"PATH=/usr/bin",
	})
	if err != nil {
		t.Fatalf("parseEnv: %v", err)
	}
	want = otelEnv{
		traces: signalEnv{off: true, protocol: protocolHTTP}, metrics: signalEnv{protocol: protocolGRPC},
		logs: signalEnv{protocol: protocolHTTP}, spanQueue: 16, logQueue: 8,
	}
	if got != want {
		t.Errorf("parseEnv = %+v, want %+v", got, want)
	}

	for _, bad := range []struct{ entry, name string }{
		{"OTEL_LOGS_EXPORTER=console", envLogsExporter},
		{"OTEL_EXPORTER_OTLP_PROTOCOL=http/json", envProtocol},
		{"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=udp", envTracesProtocol},
		{"OTEL_BSP_MAX_QUEUE_SIZE=0", envSpanQueueSize},
		{"OTEL_BLRP_MAX_QUEUE_SIZE=many", envLogQueueSize},
	} {
		_, err := parseEnv([]string{bad.entry})
		if !errors.Is(err, ErrInvalidEnv) || !strings.Contains(err.Error(), bad.name) {
			t.Errorf("parseEnv(%q) = %v, want ErrInvalidEnv naming %s", bad.entry, err, bad.name)
		}
		if _, value, _ := strings.Cut(bad.entry, "="); err != nil && strings.Contains(err.Error(), value) {
			t.Errorf("parseEnv(%q): the error quotes the value: %v", bad.entry, err)
		}
	}
}
