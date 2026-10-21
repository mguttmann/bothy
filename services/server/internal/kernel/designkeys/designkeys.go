// SPDX-License-Identifier: Apache-2.0

package designkeys

import "slices"

// CategoryColors returns the keys of the category colours of the design system, sorted; a copy
// of the generated list, so callers may change it.
func CategoryColors() []string { return slices.Clone(categoryColors) }

// IsCategoryColor reports whether key is exactly a category colour key.
func IsCategoryColor(key string) bool {
	_, found := slices.BinarySearch(categoryColors, key)
	return found
}

// SpaceIcons returns the names of the space icons of the design system, sorted; a copy.
func SpaceIcons() []string { return slices.Clone(spaceIcons) }

// IsSpaceIcon reports whether key is exactly the name of a space icon.
func IsSpaceIcon(key string) bool {
	_, found := slices.BinarySearch(spaceIcons, key)
	return found
}

// IconNames returns the semantic icon names of the design system (the keys of ds.icons.glyphs),
// sorted; a copy.
func IconNames() []string { return slices.Clone(iconNames) }

// IsIconName reports whether name is exactly a semantic icon name, for example "state.warning".
// A glyph file name ("warning"), an emoji or a free name is not (docs/design/icons.md "Rules"); the
// icon of a button component is checked with it (M2-15).
func IsIconName(name string) bool {
	_, found := slices.BinarySearch(iconNames, name)
	return found
}
