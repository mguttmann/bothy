// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// identityConfigKeys returns the keys owned by the module identity from
// docs/spec/03-data-model.md "Configuration values of this chapter": the limits of handles and
// display names that the domain of identity checks (M1-26).
func identityConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "principal.handle_max_chars", Kind: config.KindInt, Default: "32", Min: "1",
			Doc:     "Longest handle of a principal in characters.",
			Chapter: chapter03,
		},
		{
			Name: "principal.display_name_max_chars", Kind: config.KindInt, Default: "64", Min: "1",
			Doc:     "Longest display name of a principal in Unicode characters.",
			Chapter: chapter03,
		},
	}
}
