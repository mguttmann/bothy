// SPDX-License-Identifier: Apache-2.0

// Package unicode is the adapter of the reaction symbol rule of the messaging module (brief M1-59
// design note 3): Normalizer implements app.SymbolNormalizer with github.com/rivo/uniseg
// (extended grapheme clusters after UAX #29) and golang.org/x/text/unicode/norm (NFC). The
// application layer knows no third-party library (02 "Layers within a module"), so the rule that
// needs them lives here.
//
// Example:
//
//	sym, err := unicode.Normalizer{}.Normalize("é", 64) // "é", nil
package unicode

import (
	"errors"
	stdunicode "unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"

	"github.com/OWNER/bothy/services/server/internal/messaging/app"
)

// Normalizer implements app.SymbolNormalizer. The zero value is ready and safe for concurrent
// use.
type Normalizer struct{}

// Compile-time check: Normalizer is the outgoing port of the reaction use cases.
var _ app.SymbolNormalizer = Normalizer{}

// errInvalid is the one error of every refused symbol; it names no part of the input, because
// logs never carry the symbol (SEC-LOG-4).
var errInvalid = errors.New("messaging: not exactly one permitted symbol")

// Normalize implements app.SymbolNormalizer: raw must be valid UTF-8; its NFC form must be
// exactly one extended grapheme cluster of at most maxBytes bytes, contain no code point of the
// categories Cc, Cs, Zs, Zl or Zp and no Bidi control (U+061C, U+200E, U+200F, U+202A to U+202E,
// U+2066 to U+2069), and not consist only of format characters (Cf) and nonspacing marks (Mn),
// which would be invisible. Cf is otherwise allowed, because emoji ZWJ and tag sequences need it.
// It returns the NFC form. Errors: one error for every refusal, without the input.
func (Normalizer) Normalize(raw string, maxBytes int) (string, error) {
	if raw == "" || !utf8.ValidString(raw) {
		return "", errInvalid
	}
	s := norm.NFC.String(raw)
	if len(s) > maxBytes || uniseg.GraphemeClusterCount(s) != 1 {
		return "", errInvalid
	}
	visible := false
	for _, r := range s {
		if forbidden(r) {
			return "", errInvalid
		}
		if !stdunicode.In(r, stdunicode.Cf, stdunicode.Mn) {
			visible = true
		}
	}
	if !visible {
		return "", errInvalid
	}
	return s, nil
}

// forbidden reports a code point a symbol may never hold: a control, a surrogate, a separator or
// a Bidi control.
func forbidden(r rune) bool {
	if stdunicode.In(r, stdunicode.Cc, stdunicode.Cs, stdunicode.Zs, stdunicode.Zl, stdunicode.Zp) {
		return true
	}
	switch {
	case r == 0x061C, r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}
