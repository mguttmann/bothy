// SPDX-License-Identifier: Apache-2.0

package otel

import "time"

// Config is the configuration of the telemetry of a process. internal/wire binds it from the
// loaded configuration (config.Bind with the tags below); the keys and their defaults are in
// docs/spec/17-observability.md, 20-privacy.md and 16-operations.md "Configuration values
// of this chapter".
type Config struct {
	// Enabled is telemetry.enabled. False exports no signal over OTLP; logs then go to stdout and
	// every request still gets a trace id (17 "Signals").
	Enabled bool `config:"telemetry.enabled"`
	// SamplerRatio is telemetry.traces.sampler_ratio, 0 to 1: the share of new traces the SDK
	// records.
	SamplerRatio float64 `config:"telemetry.traces.sampler_ratio"`
	// TrustRemoteParent is telemetry.traces.trust_remote_parent: an incoming traceparent becomes
	// the parent instead of a span link.
	TrustRemoteParent bool `config:"telemetry.traces.trust_remote_parent"`
	// LogsExporter is telemetry.logs.exporter: otlp, stdout or both.
	LogsExporter string `config:"telemetry.logs.exporter"`
	// LogsLevel is telemetry.logs.level: debug, info, warn or error.
	LogsLevel string `config:"telemetry.logs.level"`
	// PseudonymKeyRef is telemetry.pseudonym_key_ref: the secret reference of the master key of
	// the pseudonyms, at least MinKeyBytes long.
	PseudonymKeyRef string `config:"telemetry.pseudonym_key_ref"`
	// PseudonymRotation is privacy.telemetry_pseudonym_rotation: the length of one pseudonym
	// period, at least one second.
	PseudonymRotation time.Duration `config:"privacy.telemetry_pseudonym_rotation"`
	// Role is server.role: api, worker or all; it becomes the resource attribute
	// bothy.server.role.
	Role string `config:"server.role"`
	// Environment is instance.environment: dev, staging or prod; it becomes
	// deployment.environment.name unless OTEL_RESOURCE_ATTRIBUTES sets that attribute.
	Environment string `config:"instance.environment"`
}
