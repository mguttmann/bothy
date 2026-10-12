// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// authzConfigKeys returns the keys the Authorizer of the module authz reads (M1-28): the bound of
// the rights snapshot cache from docs/spec/07-rechte.md and the age of a fresh step-up from
// 03-datenmodell.md "Konfigurationswerte dieses Kapitels".
func authzConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "authz.snapshot_cache_max_entries", Kind: config.KindInt, Default: "10000", Min: "1",
			Doc:     "Most rights snapshots a process keeps in memory (least recently used go first).",
			Chapter: chapter07,
		},
		{
			// At least 1s: a shorter window would make every step-up stale before it is used.
			Name: "auth.step_up_max_age", Kind: config.KindDuration, Default: "5m", Min: "1s",
			Doc:     "How long a step-up confirmation counts as fresh for actions that need one.",
			Chapter: chapter03,
		},
	}
}
