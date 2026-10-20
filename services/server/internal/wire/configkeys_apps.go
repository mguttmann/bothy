// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// appsConfigKeys returns the keys the module apps reads first (M2-24): app.manifest_max_bytes of
// docs/spec/11-extensions.md section 16, webhook.allowed_hosts of 03 section 9 (R-DM-17) and the
// command limits of 09. Defaults as the chapter tables write them.
func appsConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "app.manifest_max_bytes", Kind: config.KindInt, Default: "65536", Min: "1",
			Doc:     "Largest app manifest in bytes.",
			Chapter: chapter11,
		},
		{
			Name: "webhook.allowed_hosts", Kind: config.KindStringList, Default: "",
			Doc:     "Hosts an app manifest may name in its delivery addresses and outgoing webhooks may reach; exact host names, compared without regard to case.",
			Chapter: chapter03,
		},
		{
			Name: "command.max_per_principal", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Most commands of one agent or app.",
			Chapter: chapter09,
		},
		{
			Name: "command.description_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest command description in characters.",
			Chapter: chapter09,
		},
	}
}
