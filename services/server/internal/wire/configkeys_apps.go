// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// appsConfigKeys returns the keys the module apps reads first (M2-24): app.manifest_max_bytes of
// docs/spec/11-extensions.md section 16, webhook.allowed_hosts of 03 section 9 (R-DM-17) and the
// command limits of 09, and the keys of the incoming webhooks of 11 section 16 (M2-27). Defaults as
// the chapter tables write them.
func appsConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "app.manifest_max_bytes", Kind: config.KindInt, Default: "65536", Min: "1",
			Doc:     "Largest app manifest in bytes.",
			Chapter: chapter11,
		},
		{
			Name: "webhook.allowed_hosts", Kind: config.KindStringList, Default: "",
			Doc:     "Targets an app manifest may name in its delivery addresses and outgoing webhooks may reach: entries host[:port], the host compared without regard to case, without a port only 443.",
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
		{
			Name: keyWebhookInMaxBodyBytes, Kind: config.KindInt, Default: "1048576", Min: "1",
			Doc:     "Largest body of an incoming webhook call in bytes.",
			Chapter: chapter11,
		},
		{
			Name: keyWebhookInFormatsEnabled, Kind: config.KindStringList, Default: `["native", "alertmanager"]`,
			Doc:     "Formats of incoming webhooks that are switched on: native and alertmanager.",
			Chapter: chapter11,
		},
	}
}

// The keys of the incoming webhooks (M2-27).
const (
	keyWebhookInMaxBodyBytes   = "webhook_in.max_body_bytes"
	keyWebhookInFormatsEnabled = "webhook_in.formats_enabled"
)
