// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"net/url"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// keyTelemetryDashboardURL is the link of the administration to the telemetry backend (17 8).
const keyTelemetryDashboardURL = "telemetry.dashboard_url"

// telemetryConfigKeys returns the keys platform/otel reads (brief M1-10): the telemetry keys of
// docs/spec/17-observability.md, the pseudonym period of 20-privacy.md and the environment of
// 16-operations.md, which becomes deployment.environment.name, and telemetry.dashboard_url, the
// link of the administration to the telemetry backend (M6-06). The link is a string key, because
// the schema allows the empty default ("no link") only for string kinds; the cross-check
// telemetry_dashboard_url requires an http or https URL with a host when one is set.
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
			Name: "telemetry.traces.trust_remote_parent", Kind: config.KindBool, Default: defaultFalse,
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
			Name: keyTelemetryDashboardURL, Kind: config.KindString, Default: "",
			Doc:     "Link of the administration to the telemetry backend, shown to owners only; empty means no link.",
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

// telemetryDashboardURL requires telemetry.dashboard_url to be empty or an http or https URL
// with a host and without user information, because the clients open it in the system browser
// or a new tab (brief M6-06 Design note 4); the error names the key, never the value.
func telemetryDashboardURL(v *config.Values) error {
	s := v.String(keyTelemetryDashboardURL)
	if s == "" {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "" || u.Hostname() == "" || u.User != nil {
		return errors.New(keyTelemetryDashboardURL + " must be empty or an http or https URL with a host and without user information")
	}
	return nil
}
