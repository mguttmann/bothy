// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// FallbackName replaces a name that is empty, "." or ".." after cleaning
// (docs/DECISIONS.md D-M3-03-9).
const FallbackName = "file"

// MinNameBytes is the smallest limit CleanName works with: FallbackName must fit. The
// configuration schema of file.name_max_bytes has this minimum.
const MinNameBytes = len(FallbackName)

// IsHiddenRune reports the characters CleanName removes besides the Cc controls: the bidi
// controls of SEC-IN-4 (kernel/textcheck) and the zero-width characters U+200B to U+200D, U+2060
// and U+FEFF, which can hide or reorder parts of a shown name.
func IsHiddenRune(r rune) bool {
	switch {
	case textcheck.IsBidiControl(r):
		return true
	case r >= 0x200B && r <= 0x200D, r == 0x2060, r == 0xFEFF:
		return true
	}
	return false
}

// CleanName cleans a file name that is already in NFC (docs/spec/12-dateien.md "6.2"; the
// application layer applies NFC through its NameNormalizer port first) and returns the value of
// files.original_name. It never rejects a name:
//
//  1. Only the last path segment stays: the name is split at "/" and "\".
//  2. Bytes that are not UTF-8, Cc control characters, bidi controls and zero-width characters
//     are removed; white space at both ends is trimmed.
//  3. An empty rest, "." and ".." become FallbackName.
//  4. A name longer than maxBytes UTF-8 bytes keeps its extension (the last "." and what
//     follows, when that part alone is shorter than maxBytes) and loses the end of its stem, cut
//     at a rune boundary; without such an extension the end of the name goes. The result is
//     trimmed again.
//
// maxBytes is file.name_max_bytes and at least MinNameBytes; a smaller value is raised to it.
// The result is valid UTF-8, never empty, at most maxBytes bytes and stable: cleaning it again
// returns it unchanged. The name is a canary value (SEC-T10): callers never log it.
//
// Example:
//
//	domain.CleanName("../invoice\u202Efdp.exe", 255) // "invoicefdp.exe"
func CleanName(nfc string, maxBytes int) string {
	maxBytes = max(maxBytes, MinNameBytes)
	if i := strings.LastIndexAny(nfc, `/\`); i >= 0 {
		nfc = nfc[i+1:]
	}
	var b strings.Builder
	for i := 0; i < len(nfc); {
		r, size := utf8.DecodeRuneInString(nfc[i:])
		i += size
		if (r == utf8.RuneError && size == 1) || unicode.Is(unicode.Cc, r) || IsHiddenRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	name := fallback(strings.TrimSpace(b.String()))
	if len(name) <= maxBytes {
		return name
	}
	ext := ""
	if dot := strings.LastIndexByte(name, '.'); dot > 0 && len(name)-dot < maxBytes {
		ext = name[dot:]
	}
	stem := cutRunes(name[:len(name)-len(ext)], maxBytes-len(ext))
	if ext == "" {
		return fallback(strings.TrimSpace(stem))
	}
	return fallback(strings.TrimLeftFunc(stem, unicode.IsSpace) + ext)
}

// fallback returns FallbackName for an empty name, "." and "..".
func fallback(name string) string {
	if name == "" || name == "." || name == ".." {
		return FallbackName
	}
	return name
}

// cutRunes returns the longest prefix of s with at most n bytes that ends at a rune boundary.
func cutRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
