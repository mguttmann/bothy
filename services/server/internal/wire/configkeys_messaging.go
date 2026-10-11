// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// messagingConfigKeys returns the keys owned by the module messaging from
// docs/spec/03-datenmodell.md "Konfigurationswerte dieses Kapitels". It holds the message size
// now because the cross-check json_limit_above_message reads it.
func messagingConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "message.max_content_bytes", Kind: config.KindInt, Default: "262144", Min: "1",
			Doc:     "Largest content of one message in bytes.",
			Chapter: chapter03,
		},
	}
}
