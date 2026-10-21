// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// chatConfigKeys returns the keys the module chat reads from docs/spec/03-data-model.md
// "Configuration values of this chapter": the space name limit (M1-52), the channel and tag text
// limits and the history backfill limits of a channel (M1-53), the tag limit of a thread (M1-55),
// the prompt limit of a channel (M2-02);
// and from docs/spec/09-agents.md the placeholder titles (M1-55). thread.title_max_chars is
// declared with messaging (configkeys_messaging.go), the page sizes with the API keys.
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
		{
			Name: "channel.prompt_max_bytes", Kind: config.KindInt, Default: "32768", Min: "1",
			Doc:     "Longest channel prompt in bytes of UTF-8 (putChannelBehavior, M2-02).",
			Chapter: chapter03,
		},
		{
			Name: "thread.max_tags", Kind: config.KindInt, Default: "5", Min: "1",
			Doc:     "Most forum tags of one thread.",
			Chapter: chapter03,
		},
		{
			// The empty title is a placeholder by the rule itself (09 section 12), and a list
			// element is never empty, so the default has no empty entry (docs/DECISIONS.md M1-55/2).
			Name: "thread.placeholder_titles", Kind: config.KindStringList,
			Default: `["Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"]`,
			Doc:     "Placeholder titles: a new thread with such a title keeps title_source auto.",
			Chapter: chapter09,
		},
	}
}
