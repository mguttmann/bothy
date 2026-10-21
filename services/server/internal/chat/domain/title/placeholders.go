// SPDX-License-Identifier: Apache-2.0

package title

import (
	"strings"
	"unicode"
)

// Placeholders is the effective list of thread.placeholder_titles in normal form. The zero value
// has no entries: only the empty title is a placeholder.
type Placeholders struct {
	// entries are the normalised non-blank entries in the configured order.
	entries []string
}

// NewPlaceholders returns the placeholders of entries, the effective value of
// thread.placeholder_titles (brief M2-18 note 3c): every entry is normalised, and an entry that is
// blank after normalisation is dropped, because the empty title is a placeholder anyway.
//
// Example:
//
//	p := title.NewPlaceholders([]string{"Draft"})
//	p.IsPlaceholder("draft 2") // true
func NewPlaceholders(entries []string) Placeholders {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := Normalize(e); n != "" {
			out = append(out, n)
		}
	}
	return Placeholders{entries: out}
}

// Normalize removes leading and trailing white space (unicode.IsSpace) and replaces every inner
// run of it by one U+0020 (09 section 12 point 2, "whitespace normalized"). It does not change
// case and applies no Unicode normalisation (docs/DECISIONS.md, no NFKC).
//
// Example:
//
//	title.Normalize(" Neuer   Chat ") // "Neuer Chat"
func Normalize(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

// IsPlaceholder reports whether title is a placeholder (09 section 12 point 2): its normal form is
// empty, equals an entry under strings.EqualFold (Unicode simple case folding), or is an entry, a
// space and one or more ASCII digits ("Neuer Chat 3"). Everything else is a real title, short
// names such as "AI" or "C#" and punctuation included.
func (p Placeholders) IsPlaceholder(title string) bool {
	n := Normalize(title)
	if n == "" {
		return true
	}
	prefix, numbered := "", false
	if i := strings.LastIndexByte(n, ' '); i > 0 && asciiDigits(n[i+1:]) {
		prefix, numbered = n[:i], true
	}
	// A linear scan with EqualFold: lower-case map keys would not be case folding.
	for _, e := range p.entries {
		if strings.EqualFold(n, e) || (numbered && strings.EqualFold(prefix, e)) {
			return true
		}
	}
	return false
}

// asciiDigits reports a non-empty string of the digits 0 to 9 only.
func asciiDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
