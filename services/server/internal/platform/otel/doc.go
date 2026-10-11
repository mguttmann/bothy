// SPDX-License-Identifier: Apache-2.0

// Package otel is the telemetry of the server: structured log/slog logs, OpenTelemetry traces,
// metrics and logs exported over OTLP to the OpenTelemetry Collector, the server-side sampling
// decision and HMAC pseudonyms for people (docs/spec/17-observability.md,
// docs/observability/signals.md, docs/observability/privacy.md, 19-sicherheit.md SEC-LOG-1 to
// SEC-LOG-3, ADR 0034). Every package instruments through this one, so content and secrets stay
// out of telemetry by construction.
package otel
