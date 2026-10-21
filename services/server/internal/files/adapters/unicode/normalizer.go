// SPDX-License-Identifier: Apache-2.0

// Package unicode is the adapter of the files module for Unicode normalization: NFC of file
// names with golang.org/x/text/unicode/norm (docs/spec/12-files.md 6.2), because the
// architecture gate allows the library only in adapters (02 7.2; the same pattern as messaging
// adapters/unicode). It implements app.NameNormalizer.
//
// Example:
//
//	name := unicode.Normalizer{}.NFC("A\u0308.txt") // "Ä.txt"
package unicode

import (
	"golang.org/x/text/unicode/norm"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// Normalizer brings file names into NFC. The zero value is ready to use.
type Normalizer struct{}

// Compile-time check: Normalizer is the outgoing port of app.
var _ app.NameNormalizer = Normalizer{}

// NFC implements app.NameNormalizer; invalid UTF-8 passes through for CleanName to drop.
func (Normalizer) NFC(s string) string { return norm.NFC.String(s) }
