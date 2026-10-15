// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// telemetryConfigKeys returns the keys platform/otel reads (brief M1-10): the telemetry keys of
// docs/spec/17-observability.md, the pseudonym period of 20-privacy.md and the environment of
// 16-operations.md, which becomes deployment.environment.name. telemetry.dashboard_url comes with
// the admin console that links it.
func telemetryConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "telemetry.enabled", Kind: config.KindBool, Default: defaultTrue,
			Doc:     "Turns the OTLP export of traces, metrics and logs on or off; logs then go to stdout.",
			Chapter: chapter17,
		},
		{
			Name: "telemetry.traces.sampler_ratio", Kind: config.KindFloat, Default: "1.0", Min: "0", Max: "1",
			Doc:     "Share of new traces the SDK records; the collector samples again after that.",
			Chapter: chapter17,
		},
		{
			Name: "telemetry.traces.trust_remote_parent", Kind: config.KindBool, Default: "false",
			Doc:     "With true an incoming traceparent becomes the parent of the server span, otherwise only a span link.",
			Chapter: chapter17,
		},
		{
			Name: "telemetry.logs.exporter", Kind: config.KindEnum, Default: "both", Enum: []string{"otlp", "stdout", "both"},
			Doc:     "Where logs go: OTLP to the collector, JSON on stdout, or both.",
			Chapter: chapter17,
		},
		{
			Name: "telemetry.logs.level", Kind: config.KindEnum, Default: "info", Enum: []string{"debug", "info", "warn", "error"},
			Doc:     "Lowest level that is written.",
			Chapter: chapter17,
		},
		{
			Name: "telemetry.pseudonym_key_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/telemetry_pseudonym_key",
			Doc:     "Secret reference of the key for HMAC pseudonyms of people in telemetry; at least 32 bytes.",
			Chapter: chapter17,
		},
		{
			// At least 1s: the period index divides by the rotation in whole seconds.
			Name: "privacy.telemetry_pseudonym_rotation", Kind: config.KindDuration, Default: "30d", Min: "1s",
			Doc:     "Length of one period of the telemetry pseudonyms; each period derives its own key.",
			Chapter: chapter20,
		},
		{
			Name: "instance.environment", Kind: config.KindEnum, Default: "prod", Enum: []string{"dev", "staging", "prod"},
			Doc:     "Environment of the instance: dev, staging or prod.",
			Chapter: chapter16,
		},
	}
}
