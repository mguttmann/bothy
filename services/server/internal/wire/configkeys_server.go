// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// serverConfigKeys returns the keys of the process itself from docs/spec/16-betrieb.md
// "Konfigurationswerte dieses Kapitels": the role, which the flag --role sets, and the two
// listeners, which the cross-check listeners_distinct compares.
func serverConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "server.role", Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"},
			Doc:     "Role of the process: api serves REST and the gateway, worker runs jobs and consumers, all does both.",
			Chapter: chapter16,
		},
		{
			Name: "server.listen_addr", Kind: config.KindListenAddr, Default: "127.0.0.1:8080",
			Doc:     "Address of the public listener that serves REST and the gateway.",
			Chapter: chapter16,
		},
		{
			Name: "server.internal_listen_addr", Kind: config.KindListenAddr, Default: "127.0.0.1:8081",
			Doc:     "Address of the internal listener for health, readiness and incoming webhooks from services in the stack.",
			Chapter: chapter16,
		},
	}
}
