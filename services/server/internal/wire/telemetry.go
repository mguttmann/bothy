// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
)

// TelemetryConfig binds the configuration of platform/otel from the loaded values. bothy serve
// passes it to otel.Setup (M1-11), and bothy telemetry pseudonym builds its Pseudonymizer from
// it. The error reports a schema without one of the keys (config.Bind); it names keys, never
// values.
func TelemetryConfig(v *config.Values) (otel.Config, error) {
	var c otel.Config
	if err := config.Bind(v, &c); err != nil {
		return otel.Config{}, err
	}
	return c, nil
}
