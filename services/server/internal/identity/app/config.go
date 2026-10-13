// SPDX-License-Identifier: Apache-2.0

package app

// Config holds the limits of the identity module, bound in internal/wire from the keys of
// docs/spec/03-data-model.md section 9 (wire.IdentityConfig).
type Config struct {
	// HandleMaxChars is principal.handle_max_chars, the longest handle in characters.
	HandleMaxChars int `config:"principal.handle_max_chars"`
	// DisplayNameMaxChars is principal.display_name_max_chars, the longest display name in
	// Unicode characters (runes).
	DisplayNameMaxChars int `config:"principal.display_name_max_chars"`
}
