// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// chatConfigKeys returns the keys the module chat reads from docs/spec/03-datenmodell.md
// "Konfigurationswerte dieses Kapitels": the space name limit (M1-52), the channel and tag text
// limits and the history backfill limits of a channel (M1-53).
func chatConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "space.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest space name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "channel.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest channel name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "channel.topic_max_chars", Kind: config.KindInt, Default: "1024", Min: "1",
			Doc:     "Longest channel topic in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "forum_tag.name_max_chars", Kind: config.KindInt, Default: "50", Min: "1",
			Doc:     "Longest forum tag name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "channel.history_backfill_limit_default", Kind: config.KindInt, Default: "20", Min: "0",
			Doc:     "history_backfill_limit of a new channel; at most channel.history_backfill_limit_max.",
			Chapter: chapter03,
		},
		{
			Name: "channel.history_backfill_limit_max", Kind: config.KindInt, Default: "200", Min: "0",
			Doc:     "Upper bound of history_backfill_limit.",
			Chapter: chapter03,
		},
	}
}
