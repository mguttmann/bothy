// SPDX-License-Identifier: Apache-2.0

// Package locale is the adapter of the identity port app.Locales: it parses BCP 47 language tags
// with golang.org/x/text/language (M1-31), which only adapters may import (go-arch-lint vendor
// group xtext).
//
// Example:
//
//	tag, ok := locale.Parser{}.Canonical("en-us") // "en-US", true
package locale

import (
	"golang.org/x/text/language"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
)

// Parser implements app.Locales. The zero value is ready to use and safe for concurrent use.
type Parser struct{}

// Compile-time check: the locale port of the identity use cases.
var _ app.Locales = Parser{}

// Canonical implements app.Locales: language.Parse must accept the whole tag without error (an
// unknown but well-formed subtag is accepted, as BCP 47 allows), and the result is the canonical
// form of x/text (language lower case, script title case, region upper case).
func (Parser) Canonical(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	tag, err := language.Parse(s)
	if err != nil {
		return "", false
	}
	return tag.String(), true
}
