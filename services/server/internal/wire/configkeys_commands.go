// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// commandsConfigKeys returns the keys of the command register of docs/spec/09-agents.md section
// 21 that no earlier ticket registered (M2-16): the most static choices of one command option, the
// same bound as the suggestions of an answer (09 section 10.5 point 3). command.max_per_principal
// and command.description_max_chars are registered by appsConfigKeys (M2-24).
func commandsConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "command.autocomplete_max_choices", Kind: config.KindInt, Default: "25", Min: "1",
			Doc:     "Most suggestions of one autocomplete answer and most static choices of one command option.",
			Chapter: chapter09,
		},
	}
}
