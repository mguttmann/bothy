// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestExcerpt_ParsesABoundedPrefix: the excerpt parses at most excerptInputFactor times maxChars
// code points of content_md, whatever its length, so the work per call has a bound that depends
// on notify.preview_max_chars only and never on the message (red team M3-22: a line of 256 KiB of
// "[" or backticks made the parser quadratic over the whole message). The worst cases of every
// construct the parser scans ahead for are checked.
func TestExcerpt_ParsesABoundedPrefix(t *testing.T) {
	const maxChars = 240
	bound := maxChars * excerptInputFactor
	for name, unit := range map[string]string{
		"brackets": "[", "images": "![", "backticks": "` `` ", "stars": "*a ", "underscores": "_a ", "tildes": "~~a ",
		"autolinks": "<", "mentions": "@a", "multibyte": "ä[", "invalid bytes": "\xff[",
	} {
		t.Run(name, func(t *testing.T) {
			huge := strings.Repeat(unit, (1<<18)/len(unit))
			src := excerptSource(huge, maxChars)
			if n := utf8.RuneCountInString(src); n > bound {
				t.Fatalf("the parser gets %d code points, want at most %d", n, bound)
			}
			if !strings.HasPrefix(huge, src) {
				t.Fatal("the parsed part is not a prefix of the content")
			}
			if got, want := Excerpt(huge, nil, maxChars), Excerpt(src, nil, maxChars); got != want {
				t.Errorf("Excerpt of the content %q differs from Excerpt of its prefix %q", got, want)
			}
		})
	}
	if short := "**Hi** @sam"; excerptSource(short, maxChars) != short {
		t.Error("a short content is not parsed whole")
	}
}
