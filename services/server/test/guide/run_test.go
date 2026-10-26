// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestServerConfig_NoStepIsBoundByTheServersSpeed holds the red team finding of M2-37: a step
// whose answer the server ends after a short timer of its own fails on a slow machine. The one such
// timer of the page is command.autocomplete_timeout (the suggestions of step 8, whose default is
// shorter than a reply over the gateway on a loaded machine may take); it lies above the patience
// of the test, and http.request_timeout above it. The other timers the steps meet
// (agent.ack_timeout, agent.cancel_grace, agent.invocation_token_ttl) only lead to a redelivery or
// a renewal, or are far longer than one step.
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
