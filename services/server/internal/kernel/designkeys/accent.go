// SPDX-License-Identifier: Apache-2.0

package designkeys

import "slices"

// accentColors are the accent keys of ADR 0044 and docs/spec/01-produkt.md section 12, sorted.
// They are written by hand, not derived from the tokens: granite is the neutral palette of all
// surfaces and no accent, and a new category colour becomes an accent only after the contrast
// gate of ADR 0044 has passed for it. TestAccentColorsMatchADR0044 holds the list equal to the
// documents and every accent a generated category colour.
var accentColors = []string{"amber", "gorse", "heather", "loch", "moss", "rowan", "spruce"}

// AccentColors returns the accent keys, sorted; a copy.
func AccentColors() []string { return slices.Clone(accentColors) }

// IsAccentColor reports whether key is exactly an accent key. A hex value, a CSS colour, an
// emoji, a key in another case or with spaces is not.
func IsAccentColor(key string) bool {
	_, found := slices.BinarySearch(accentColors, key)
	return found
}
