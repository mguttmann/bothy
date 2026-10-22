// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// placeholderPrefix starts every placeholder handle (22 5.3).
const placeholderPrefix = "imported"

// PlaceholderHandle returns the n-th candidate handle of a placeholder for the source name (n
// starts at 1; M8-03 design note 8): "imported-" plus name in lower case, every character outside
// the handle alphabet of 03 (a-z, 0-9, '.', '_', '-') replaced by '-', runs of '-' collapsed and
// the ends trimmed; a name with nothing left gives "imported". From n = 2 on the suffix "-<n>" is
// added; the handle is cut so that it fits maxChars (principal.handle_max_chars) with the suffix.
//
// Example:
//
//	PlaceholderHandle("Sam Example", 1, 32) // "imported-sam-example"
//	PlaceholderHandle("Sam Example", 2, 32) // "imported-sam-example-2"
func PlaceholderHandle(name string, n, maxChars int) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r < utf8.RuneSelf && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	base := placeholderPrefix
	if part := strings.Trim(b.String(), "-._"); part != "" {
		base += "-" + part
	}
	suffix := ""
	if n > 1 {
		suffix = "-" + strconv.Itoa(n)
	}
	if room := maxChars - len(suffix); len(base) > room {
		if room < 1 {
			room = 1
		}
		base = strings.TrimRight(base[:room], "-._")
	}
	return base + suffix
}

// PlaceholderDisplayName returns the display name of a placeholder (22 5.3): globalName or else
// username, without control characters, bidi controls and invalid UTF-8 (SEC-IN-4), trimmed and cut
// to maxChars runes (principal.display_name_max_chars); fallback when nothing visible is left.
func PlaceholderDisplayName(globalName, username, fallback string, maxChars int) string {
	for _, s := range []string{globalName, username} {
		if c := cleanName(s, maxChars); c != "" {
			return c
		}
	}
	return cleanName(fallback, maxChars)
}

// cleanName removes what textcheck.DisplayText rejects, trims white space and cuts to maxChars
// runes; "" when no visible rune is left.
func cleanName(s string, maxChars int) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cc, r) || textcheck.IsBidiControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxChars {
		s = strings.TrimSpace(string([]rune(s)[:maxChars]))
	}
	visible := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) && !unicode.Is(unicode.Cf, r) }) >= 0
	if !visible {
		return ""
	}
	return s
}
