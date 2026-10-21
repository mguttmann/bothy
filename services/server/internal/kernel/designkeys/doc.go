// SPDX-License-Identifier: Apache-2.0

// Package designkeys holds the allowed keys of the design system as data: category colours,
// accents, space icons and the semantic icon names (kernel-core, docs/spec/02-architektur.md 4.5).
// The category colours, the space icons and the icon names (the keys of ds.icons.glyphs) are
// generated from design/build/tokens.resolved.json by tools/designkeysgen
// (designkeys_gen.go, held by make codegen-check); the accents are an explicit list of ADR 0044
// (accent.go). instance checks branding.accent_color with it (M1-46); chat checks space colours,
// space icons and forum tag colours with it (M1-52, M1-53), and runs the icon of a button
// component (M2-15), so no free colour value, glyph name or emoji enters the product (R-PR-08).
//
// Keys are case sensitive and never trimmed: "Spruce", " spruce" and "#5865f2" are not keys.
//
// Example:
//
//	if !designkeys.IsAccentColor(in.AccentColor) {
//		return fieldError("/accent_color", "unknown_color_key")
//	}
package designkeys

//go:generate go run ../../../tools/designkeysgen -tokens ../../../../../design/build/tokens.resolved.json -out designkeys_gen.go
