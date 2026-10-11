// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestTelemetryConfigBinds: the defaults of chapters 16, 17 and 20 reach otel.Config, and an
// environment variable overrides one of them.
func TestTelemetryConfigBinds(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := wire.TelemetryConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	want := otel.Config{
		Enabled: true, SamplerRatio: 1, TrustRemoteParent: false, LogsExporter: "both",
		LogsLevel: "info", PseudonymKeyRef: "file:/run/bothy/secrets/telemetry_pseudonym_key",
		PseudonymRotation: 30 * 24 * time.Hour, Role: "all", Environment: "prod",
	}
	if got != want {
		t.Errorf("TelemetryConfig = %+v, want %+v", got, want)
	}

	v, err = config.Load(schema(t), config.Sources{Env: []string{
		"BOTHY_TELEMETRY__TRACES__SAMPLER_RATIO=0.25", "BOTHY_INSTANCE__ENVIRONMENT=staging",
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, err = wire.TelemetryConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.SamplerRatio != 0.25 || got.Environment != "staging" {
		t.Errorf("TelemetryConfig from env = %+v, want ratio 0.25 and environment staging", got)
	}
}

// TestSamplerRatioBounds: a ratio outside 0 to 1 is a load problem of the key.
func TestSamplerRatioBounds(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"-0.1", "1.5"} {
		_, err := config.Load(schema(t), config.Sources{Env: []string{"BOTHY_TELEMETRY__TRACES__SAMPLER_RATIO=" + bad}})
		if err == nil {
			t.Errorf("sampler ratio %s loaded, want a problem", bad)
		}
	}
}
