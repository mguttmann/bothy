// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-test-gateway-window (run by `make semgrep`).
package fixtures

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func windows() {
	_ = []string{
		// ruleid: bothy-go-test-gateway-window
		"BOTHY_SYNC__HEARTBEAT_INTERVAL=1s",
		// ruleid: bothy-go-test-gateway-window
		"BOTHY_SYNC__HEARTBEAT_MISS_LIMIT=1",
		// ruleid: bothy-go-test-gateway-window
		"BOTHY_SYNC__IDENTIFY_TIMEOUT=2s",
		// ruleid: bothy-go-test-gateway-window
		"BOTHY_SYNC__RESUME_WINDOW=3s",
		// A miss limit that keeps the deadline of the default interval beyond Patience.
		// ok: bothy-go-test-gateway-window
		"BOTHY_SYNC__HEARTBEAT_MISS_LIMIT=9",
		// Another window of the gateway.
		// ok: bothy-go-test-gateway-window
		"BOTHY_SYNC__WS_MAX_FRAME_BYTES=4096",
	}
	_ = app.Config{
		// ruleid: bothy-go-test-gateway-window
		IdentifyTimeout: 5 * time.Second,
		// ok: bothy-go-test-gateway-window
		HeartbeatInterval: 2 * time.Second,
		// ruleid: bothy-go-test-gateway-window
		HeartbeatMissLimit: 2,
	}
	_ = app.Config{
		// ok: bothy-go-test-gateway-window
		IdentifyTimeout: testkit.Patience,
		// ok: bothy-go-test-gateway-window
		HeartbeatMissLimit: testkit.HeartbeatMissLimit(2 * time.Second),
	}
	_ = testkit.PatientGateway(2 * time.Second)
}
