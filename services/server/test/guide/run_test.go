// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestServerConfig_NoStepIsBoundByTheServersSpeed holds the class of the red team finding of
// M2-37: a step that waits for an answer the server bounds with a short timer (the suggestions of
// step 8 and command.autocomplete_timeout) fails on a slow machine. Every such timer of the guide's
// server is longer than the patience of the test, and http.request_timeout above it.
func TestServerConfig_NoStepIsBoundByTheServersSpeed(t *testing.T) {
	cfg := ServerConfig("unix:///tmp/clamd.sock")
	parse := func(key string) time.Duration {
		d, err := time.ParseDuration(cfg[key])
		if err != nil {
			t.Fatalf("%s = %q: %v", key, cfg[key], err)
		}
		return d
	}
	if wait := parse("command.autocomplete_timeout"); wait <= testkit.Patience {
		t.Fatalf("command.autocomplete_timeout = %s, want more than testkit.Patience (%s)", wait, testkit.Patience)
	}
	if parse("http.request_timeout") <= parse("command.autocomplete_timeout") {
		t.Fatal("http.request_timeout must lie above command.autocomplete_timeout")
	}
	if cfg["file.scan.clamd_address"] == "" || cfg["ratelimit.rl.trigger.burst"] == "" {
		t.Fatalf("the clamd address or the raised rate limits are missing: %v", cfg)
	}
}
