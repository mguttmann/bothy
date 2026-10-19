// SPDX-License-Identifier: Apache-2.0

package textcheck

import (
	"errors"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Rule names the SEC-IN-4 rule a display text violates. The values are stable machine codes that
// callers may copy into a field error.
type Rule string

// The rules DisplayText checks, in the order it reports them for one rune.
const (
	// RuleInvalidUTF8 means the bytes are not valid UTF-8.
	RuleInvalidUTF8 Rule = "invalid_utf8"
	// RuleControlCharacter means a rune of Unicode category Cc: C0 controls (including tab and
	// newline), DEL and the C1 controls.
	RuleControlCharacter Rule = "control_character"
	// RuleBidiControl means one of the bidi control characters U+061C, U+200E, U+200F, U+202A to
	// U+202E and U+2066 to U+2069.
	RuleBidiControl Rule = "bidi_control"
)

// ErrRejected is wrapped by every *Error, so callers can test with errors.Is without caring
// about the rule.
var ErrRejected = errors.New("textcheck: display text rejected")

// Error reports the first violation in a display text. It names the rule and the rune offset and
// never the text itself, because the text is input from a client and may end up in a log.
type Error struct {
	// Rule is the violated rule.
	Rule Rule
	// Offset is the 0-based index of the offending rune; for invalid UTF-8 it is the number of
	// valid runes before the first invalid byte.
	Offset int
}

// Error returns "textcheck: <rule> at rune <offset>".
func (e *Error) Error() string {
	return "textcheck: " + string(e.Rule) + " at rune " + strconv.Itoa(e.Offset)
}

// Unwrap returns ErrRejected.
func (e *Error) Unwrap() error { return ErrRejected }

// DisplayText checks s against SEC-IN-4 (docs/spec/19-sicherheit.md): it rejects invalid UTF-8,
// every rune of Unicode category Cc (C0 including tab and newline, DEL, C1) and the bidi
// controls U+061C, U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069. Letters with accents,
// emoji and U+200D (zero width joiner, part of many emoji sequences) pass.
//
// It judges neither length nor emptiness: the callers own those rules, and some display texts
// (thread titles) may be empty.
//
// It returns nil or an *Error for the first violation; the error wraps ErrRejected.
//
// Example:
//
//	if err := textcheck.DisplayText(name); err != nil {
//		var te *textcheck.Error
//		_ = errors.As(err, &te) // te.Rule == textcheck.RuleBidiControl, te.Offset == 3
//	}
func DisplayText(s string) error {
	offset := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			return &Error{Rule: RuleInvalidUTF8, Offset: offset}
		}
		if unicode.Is(unicode.Cc, r) {
			return &Error{Rule: RuleControlCharacter, Offset: offset}
		}
		if isBidiControl(r) {
			return &Error{Rule: RuleBidiControl, Offset: offset}
		}
		i += size
		offset++
	}
	return nil
}

// IsBidiControl reports whether r is one of the bidi controls of SEC-IN-4 that DisplayText
// rejects (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069). Callers that clean a
// text instead of rejecting it, such as the file names of docs/spec/12-dateien.md "6.2", use it so
// that the set stays in one place.
//
// Example:
//
//	textcheck.IsBidiControl('\u202E') // true
func IsBidiControl(r rune) bool { return isBidiControl(r) }

// isBidiControl reports the bidi controls of SEC-IN-4: the Arabic letter mark, the left-to-right
// and right-to-left marks, the embeddings and overrides with their pop, and the isolates. They
// can reorder how a name is shown and so make one person look like another.
func isBidiControl(r rune) bool {
	switch {
	case r == 0x061C, r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}
