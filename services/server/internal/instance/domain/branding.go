// SPDX-License-Identifier: Apache-2.0

package domain

import "slices"

// BrandingVersion returns the version of the branding: the sum of the versions of
// branding.product_name, branding.accent_color and branding.logo_file_id, 0 for a key without a
// row. Every write raises one of them by one, so the sum rises strictly and serves as the ETag
// and optimistic lock of updateBranding (06 section 8 item 1, docs/DECISIONS.md M1-46/2).
func BrandingVersion(productName, accentColor, logoFileID int64) int64 {
	return productName + accentColor + logoFileID
}

// BrandingValues are the two branding keys updateBranding writes.
type BrandingValues struct {
	// ProductName is branding.product_name.
	ProductName string
	// AccentColor is branding.accent_color.
	AccentColor string
}

// BrandingChanges returns the keys whose value differs between cur and next, sorted; the keys
// updateBranding writes and names in instance.updated.
func BrandingChanges(cur, next BrandingValues) []string {
	var out []string
	if cur.AccentColor != next.AccentColor {
		out = append(out, KeyAccentColor)
	}
	if cur.ProductName != next.ProductName {
		out = append(out, KeyProductName)
	}
	slices.Sort(out)
	return out
}
