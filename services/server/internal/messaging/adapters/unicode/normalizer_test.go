// SPDX-License-Identifier: Apache-2.0

package unicode_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/unicode"
)

// maxBytes is reaction.emoji_max_bytes of the tests (the default of 03 section 9).
const maxBytes = 64

// TestSymbolNormalizer holds the symbol rule of brief M1-59 design note 3: valid UTF-8, NFC,
// exactly one extended grapheme cluster, at most maxBytes bytes, no control, surrogate,
// separator or Bidi control, and not invisible only.
func TestSymbolNormalizer(t *testing.T) {
	t.Parallel()
	n := unicode.Normalizer{}
	accept := map[string]struct{ in, want string }{
		"thumbs up":                 {"\U0001F44D", "\U0001F44D"},
		"thumbs up with skin tone":  {"\U0001F44D\U0001F3FD", "\U0001F44D\U0001F3FD"},
		"ZWJ family of four":        {"\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466", "\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466"},
		"kiss with two skin tones":  {"\U0001F9D1\U0001F3FB\u200D\u2764\uFE0F\u200D\U0001F48B\u200D\U0001F9D1\U0001F3FF", "\U0001F9D1\U0001F3FB\u200D\u2764\uFE0F\u200D\U0001F48B\u200D\U0001F9D1\U0001F3FF"},
		"England subdivision flag":  {"\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F", "\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F"},
		"keycap one":                {"1\uFE0F\u20E3", "1\uFE0F\u20E3"},
		"single letter":             {"a", "a"},
		"e acute composed":          {"\u00E9", "\u00E9"},
		"e acute decomposed to NFC": {"e\u0301", "\u00E9"},
	}
	for name, c := range accept {
		got, err := n.Normalize(c.in, maxBytes)
		if err != nil || got != c.want {
			t.Errorf("%s: Normalize(%q) = %q, %v; want %q", name, c.in, got, err, c.want)
		}
	}
	reject := map[string]string{
		"empty":                      "",
		"two thumbs up":              "\U0001F44D\U0001F44D",
		"two letters":                "ab",
		"space":                      " ",
		"right-to-left mark alone":   "\u200F",
		"right-to-left override":     "\u202E",
		"bell":                       "\u0007",
		"zero width space alone":     "\u200B",
		"lone combining mark":        "\u0301",
		"letter with too many marks": "x" + strings.Repeat("\u0301", maxBytes/2),
		"invalid UTF-8":              "\xff",
	}
	for name, in := range reject {
		if got, err := n.Normalize(in, maxBytes); err == nil {
			t.Errorf("%s: Normalize(%q) = %q, want an error", name, in, got)
		}
	}
}
