// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// chatConfigKeys returns the keys the module chat reads: the space name limit (M1-52) from
// docs/spec/03-datenmodell.md "Konfigurationswerte dieses Kapitels".
func chatConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "space.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest space name in Unicode characters.",
			Chapter: chapter03,
		},
	}
}
