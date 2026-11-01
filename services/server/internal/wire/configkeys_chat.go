// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// chatConfigKeys returns the keys the module chat reads from docs/spec/03-data-model.md
// "Configuration values of this chapter": the space name limit (M1-52), the channel and tag text
// limits and the history backfill limits of a channel (M1-53), the tag limit of a thread (M1-55),
// the prompt limit of a channel (M2-02), the length of a personal order list of the sidebar (M4-51),
// the undo window and the purge interval of deletions (FEAT-delete-spaces-chats);
// and from docs/spec/09-agents.md the placeholder titles (M1-55) and the text limit of a project
// binding (M2-03). thread.title_max_chars is
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
			// 512 characters of at most 4 bytes in UTF-8, also after lower(), stay below the
			// PostgreSQL B-tree entry limit of the unique index on lower(workspace_ref); the
			// maximum keeps that true for every configured value (docs/DECISIONS.md D-M2-03-2).
			Name: "project.binding_text_max_chars", Kind: config.KindInt, Default: "512", Min: "1", Max: "600",
			Doc:     "Longest workspace_ref, repository_url, default_branch, model and tool name of a project binding in Unicode characters (M2-03).",
			Chapter: chapter09,
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
			// The maxItems of the lists in docs/api/openapi.yaml is this maximum (M4-51), so an answer
			// never leaves its schema.
			Name: "sidebar.order_max_items", Kind: config.KindInt, Default: "1000", Min: "1", Max: "1000",
			Doc:     "Most items of a personal order list of the sidebar: channel_order of a space and thread_order of a channel (M4-51).",
			Chapter: chapter03,
		},
		{
			Name: "chat.delete_undo_window", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s",
			Doc:     "How long a deleted space, channel or chat can be restored before the job messaging.purge_deleted removes its content (01 6.18, D-DEL-1).",
			Chapter: chapter03,
		},
		{
			Name: "chat.delete_purge_interval", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s",
			Doc:     "Interval of the job messaging.purge_deleted, which also runs once at the start of the worker (D-DEL-1).",
			Chapter: chapter03,
		},
		{
			// The default is rendered from the word lists of package title, so the words exist
			// once (docs/DECISIONS.md D-M2-18-1). The empty title is a placeholder by the rule
			// itself (09 section 12), and a list element is never empty, so the default has no
			// empty entry (D-M2-18-2).
			Name: "thread.placeholder_titles", Kind: config.KindStringList,
			Default: title.DefaultConfigText(),
			Doc:     "Placeholder titles: a person's new thread with such a title, also with an appended number, keeps title_source auto; the configured list replaces the default.",
			Chapter: chapter09,
		},
	}
}
