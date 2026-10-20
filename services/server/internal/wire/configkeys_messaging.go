// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// messagingConfigKeys returns the keys owned by the module messaging from
// docs/spec/03-data-model.md "Configuration values of this chapter". It holds the message size
// now because the cross-check json_limit_above_message reads it, and the attachment count and
// the thread title length because getInstance reports them as client limits (M1-46). Sending
// (M1-56) adds the mention limit, the templates (M1-60) their name length and count bound.
func messagingConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "message.max_content_bytes", Kind: config.KindInt, Default: "262144", Min: "1",
			Doc:     "Largest content of one message in bytes.",
			Chapter: chapter03,
		},
		{
			Name: "message.max_mentions", Kind: config.KindInt, Default: "50", Min: "1",
			Doc:     "Most distinct mention handles in one message.",
			Chapter: chapter03,
		},
		{
			Name: "message.max_attachments", Kind: config.KindInt, Default: "10", Min: "0",
			Doc:     "Most attachments of one message.",
			Chapter: chapter03,
		},
		{
			Name: "template.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest template name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "template.max_per_scope", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Most templates per person or per channel; listTemplates is not paginated.",
			Chapter: chapter03,
		},
		{
			Name: "thread.title_max_chars", Kind: config.KindInt, Default: "200", Min: "1",
			Doc:     "Longest thread title in Unicode characters.",
			Chapter: chapter03,
		},
	}
}
