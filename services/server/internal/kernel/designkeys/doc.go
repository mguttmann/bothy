// SPDX-License-Identifier: Apache-2.0

// Package designkeys holds the allowed keys of the design system as data: category colours,
// accents and space icons (kernel-core, docs/spec/02-architecture.md 4.5). The category colours and
// the space icons are generated from design/build/tokens.resolved.json by tools/designkeysgen
// (designkeys_gen.go, held by make codegen-check); the accents are an explicit list of ADR 0044
// (accent.go). instance checks branding.accent_color with it (M1-46); chat checks space colours,
// space icons and forum tag colours with it (M1-52, M1-53), so no free colour value or emoji
// enters the product (R-PR-08).
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
