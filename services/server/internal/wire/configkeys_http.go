// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// httpConfigKeys returns the keys of the HTTP chain from docs/spec/02-architektur.md
// "Konfigurationswerte dieses Kapitels". It holds the JSON body limit now because the
// cross-check json_limit_above_message reads it; the HTTP chain (M1-18) adds its other keys here.
func httpConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "http.max_json_body_bytes", Kind: config.KindInt, Default: "1048576", Min: "1",
			Doc:     "Largest JSON request body in bytes; it must be greater than message.max_content_bytes.",
			Chapter: chapter02,
		},
	}
}
