// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"unicode/utf8"
)

// TextBuilder collects the text of one extraction up to search.extract.max_text_bytes bytes
// (brief M3-15 design note 5). It normalises what it receives: invalid UTF-8 becomes U+FFFD, NUL
// and every other C0 control character except tab, line feed and carriage return become a space
// (PostgreSQL text rejects NUL). Separators the adapters write between parts of a document are
// written at most once between two pieces of text and never at the start or the end. A cut never
// splits a rune; Truncated reports that something was dropped, and Full tells the adapter to stop
// reading. The zero value has a limit of 0 and takes nothing; it is not safe for concurrent use.
//
// Example:
//
//	b := domain.NewTextBuilder(maxTextBytes)
//	b.WriteText("Rechnung")
//	b.Separator('\n')
//	b.WriteText("fertig")
//	text, truncated := b.String(), b.Truncated() // "Rechnung\nfertig", false
type TextBuilder struct {
	max       int
	b         strings.Builder
	sep       rune
	truncated bool
}

// NewTextBuilder returns a builder that keeps at most maxBytes bytes (negative counts as 0).
func NewTextBuilder(maxBytes int64) *TextBuilder {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &TextBuilder{max: int(min(maxBytes, int64(maxInt)))}
}

// maxInt is the largest int, the cap of the limit on every platform.
const maxInt = int(^uint(0) >> 1)

// sepRank orders the separators: a pending line feed is not weakened by a later tab or space.
func sepRank(r rune) int {
	switch r {
	case '\n':
		return 3
	case '\t':
		return 2
	case ' ':
		return 1
	}
	return 0
}

// Separator records a separator (line feed, tab or space; anything else counts as a space) to be
// written before the next text. Runs of separators collapse to the strongest of them; none is
// written at the start of the text or after a white-space character.
func (t *TextBuilder) Separator(r rune) {
	if sepRank(r) == 0 {
		r = ' '
	}
	if sepRank(r) > sepRank(t.sep) {
		t.sep = r
	}
}

// WriteText appends s (normalised as the type describes), after a pending separator.
func (t *TextBuilder) WriteText(s string) {
	for len(s) > 0 && !t.Full() {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		t.WriteRune(r)
	}
	if len(s) > 0 {
		t.truncated = true
	}
}

// WriteRune appends one rune (normalised; utf8.RuneError stands for an invalid byte and becomes
// U+FFFD), after a pending separator. A rune that does not fit sets Truncated.
func (t *TextBuilder) WriteRune(r rune) {
	if t.truncated {
		return
	}
	r = normaliseRune(r)
	if t.sep != 0 {
		if t.b.Len() > 0 && !t.endsInSpace() {
			if t.b.Len()+1+utf8.RuneLen(r) > t.max {
				t.truncated = true
				return
			}
			t.b.WriteRune(t.sep)
		}
		t.sep = 0
	}
	if t.b.Len()+utf8.RuneLen(r) > t.max {
		t.truncated = true
		return
	}
	t.b.WriteRune(r)
}

// normaliseRune maps invalid runes to U+FFFD and C0 controls other than tab, line feed and
// carriage return (and DEL) to a space.
func normaliseRune(r rune) rune {
	switch {
	case r == '\t' || r == '\n' || r == '\r':
		return r
	case r < 0x20 || r == 0x7f:
		return ' '
	case !utf8.ValidRune(r):
		return utf8.RuneError
	}
	return r
}

// endsInSpace reports a last written byte that is white space, after which a separator adds
// nothing.
func (t *TextBuilder) endsInSpace() bool {
	s := t.b.String()
	switch s[len(s)-1] {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// MarkTruncated records that input was left unread after Full, so Truncated is true.
func (t *TextBuilder) MarkTruncated() { t.truncated = true }

// Full reports that the builder takes nothing more: the limit is reached or something was
// dropped. An adapter stops reading then; if input remains unread it calls MarkTruncated.
func (t *TextBuilder) Full() bool { return t.truncated || t.b.Len() >= t.max }

// Truncated reports that text was dropped at the limit.
func (t *TextBuilder) Truncated() bool { return t.truncated }

// Len returns the bytes collected so far.
func (t *TextBuilder) Len() int { return t.b.Len() }

// String returns the collected text: valid UTF-8 without NUL, at most the limit in bytes.
func (t *TextBuilder) String() string { return t.b.String() }
