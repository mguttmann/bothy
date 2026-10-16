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
