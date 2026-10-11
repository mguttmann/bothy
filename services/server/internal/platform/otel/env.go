// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The OpenTelemetry variables platform/otel interprets itself. The OTLP exporter packages read
// endpoint, headers, timeout and TLS variables, and package resource reads
// OTEL_RESOURCE_ATTRIBUTES, from the process environment on their own
// (docs/operations/configuration.md "OpenTelemetry-Variablen").
const (
	envProtocol        = "OTEL_EXPORTER_OTLP_PROTOCOL"
	envTracesProtocol  = "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"
	envMetricsProtocol = "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"
	envLogsProtocol    = "OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"
	envTracesExporter  = "OTEL_TRACES_EXPORTER"
	envMetricsExporter = "OTEL_METRICS_EXPORTER"
	envLogsExporter    = "OTEL_LOGS_EXPORTER"
	envSpanQueueSize   = "OTEL_BSP_MAX_QUEUE_SIZE"
	envLogQueueSize    = "OTEL_BLRP_MAX_QUEUE_SIZE"
)

// OTLP protocols of the Go exporters. http/json has no Go exporter.
const (
	protocolGRPC = "grpc"
	protocolHTTP = "http/protobuf"
)

// defaultQueueSize is the default of OTEL_BSP_MAX_QUEUE_SIZE and OTEL_BLRP_MAX_QUEUE_SIZE in the
// OpenTelemetry specification and in the Go SDK.
const defaultQueueSize = 2048

// ErrInvalidEnv reports an OpenTelemetry variable with a value platform/otel cannot use. The
// message names the variable, never its value.
var ErrInvalidEnv = errors.New("otel: invalid OpenTelemetry variable")

// signalEnv is the export setting of one signal.
type signalEnv struct {
	off      bool   // OTEL_<SIGNAL>_EXPORTER=none
	protocol string // protocolGRPC or protocolHTTP
}

// otelEnv is what Setup reads from the OpenTelemetry variables.
type otelEnv struct {
	traces, metrics, logs signalEnv
	spanQueue, logQueue   int
}

// parseEnv reads the variables above from env ("NAME=value" entries, the first entry of a name
// wins like in the process environment). An empty value counts as unset.
func parseEnv(env []string) (otelEnv, error) {
	vars := make(map[string]string)
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, "OTEL_") {
			continue
		}
		if _, seen := vars[name]; !seen {
			vars[name] = value
		}
	}
	var e otelEnv
	var errs []error
	signal := func(exporter, protocol string) signalEnv {
		s, err := parseSignal(vars, exporter, protocol)
		if err != nil {
			errs = append(errs, err)
		}
		return s
	}
	queue := func(name string) int {
		n, err := parseQueueSize(vars, name)
		if err != nil {
			errs = append(errs, err)
		}
		return n
	}
	e.traces = signal(envTracesExporter, envTracesProtocol)
	e.metrics = signal(envMetricsExporter, envMetricsProtocol)
	e.logs = signal(envLogsExporter, envLogsProtocol)
	e.spanQueue = queue(envSpanQueueSize)
	e.logQueue = queue(envLogQueueSize)
	return e, errors.Join(errs...)
}

func parseSignal(vars map[string]string, exporterVar, protocolVar string) (signalEnv, error) {
	var s signalEnv
	switch strings.TrimSpace(vars[exporterVar]) {
	case "", "otlp":
	case "none":
		s.off = true
	default:
		return s, fmt.Errorf("%w %s: want otlp or none", ErrInvalidEnv, exporterVar)
	}
	name, value := protocolVar, strings.TrimSpace(vars[protocolVar])
	if value == "" {
		name, value = envProtocol, strings.TrimSpace(vars[envProtocol])
	}
	switch value {
	case "", protocolGRPC:
		s.protocol = protocolGRPC
	case protocolHTTP:
		s.protocol = protocolHTTP
	default:
		return s, fmt.Errorf("%w %s: want grpc or http/protobuf", ErrInvalidEnv, name)
	}
	return s, nil
}

func parseQueueSize(vars map[string]string, name string) (int, error) {
	value := strings.TrimSpace(vars[name])
	if value == "" {
		return defaultQueueSize, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w %s: want a whole number of at least 1", ErrInvalidEnv, name)
	}
	return n, nil
}
