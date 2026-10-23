// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// names are the mentioned principals of the excerpt tests: only sam has a mentions row.
var names = map[string]string{"sam": "Sam"}

// TestExcerpt_Table holds each step of brief M3-22 design note 10.
func TestExcerpt_Table(t *testing.T) {
	t.Parallel()
	const limit = 240
	for _, tc := range []struct{ name, in, want string }{
		{"mention of a mentioned principal", "@sam can you check?", "Sam can you check?"},
		{"mention case-insensitive", "Hi @SAM.", "Hi Sam."},
		{"mention without a mentions row", "@robin and @sam", "@robin and Sam"},
		{"mention inside a word", "mail alex@sam.example.com", "mail alex@sam.example.com"},
		{"mention in code stays", "run `@sam` now", "run @sam now"},
		{"fence", "before\n```go\nx := 1\n```\nafter", "before x := 1 after"},
		{"tilde fence", "~~~\ncode\n~~~", "code"},
		{"inline code", "use `make check` first", "use make check first"},
		{"double backtick code", "``a ` b``", "a ` b"},
		{"emphasis", "this is *very* _really_ important", "this is very really important"},
		{"strong", "**bold** and __bold__", "bold and bold"},
		{"strike", "~~old~~ new", "old new"},
		{"nested", "***both*** done", "both done"},
		{"unpaired star stays", "2 * 3 = 6", "2 * 3 = 6"},
		{"intraword underscore stays", "snake_case_name", "snake_case_name"},
		{"link", "see [the docs](https://example.com/a_b) now", "see the docs now"},
		{"link with emphasis", "[**bold** text](https://example.com)", "bold text"},
		{"image", "![a diagram](https://example.com/x.png)", "a diagram"},
		{"autolink", "<https://example.com/x>", "https://example.com/x"},
		{"email autolink", "<alex@example.com>", "alex@example.com"},
		{"html literal", "<b>bold</b>", "<b>bold</b>"},
		{"heading", "# Title\n## Sub ##", "Title Sub"},
		{"quote", "> quoted\n>> deeper", "quoted deeper"},
		{"list", "- one\n* two\n+ three\n1. four\n2) five", "one two three four five"},
		{"task list", "- [ ] open\n- [x] done", "open done"},
		{"thematic break", "a\n---\nb", "a b"},
		{"table", "| a | b |\n|---|:-:|\n| 1 | 2 |", "a b 1 2"},
		{"escapes", `\*not emphasis\* and \# and \[x\]`, "*not emphasis* and # and [x]"},
		{"line breaks", "one\r\ntwo\n\n\nthree", "one two three"},
		{"white space runs", "a \t  b  c", "a b c"},
		{"bidi control removed", "abc\u202edef\u2066g", "abcdefg"},
		{"control removed", "a\x00b\x1bc\x7fd", "abcd"},
		{"empty", "", ""},
		{"only markup", "**", "**"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.Excerpt(tc.in, names, limit); got != tc.want {
				t.Fatalf("Excerpt(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExcerpt_Cut: exactly maxChars stays; one more is cut to maxChars - 1 code points and an
// ellipsis; multi-byte characters at the cut are never split.
func TestExcerpt_Cut(t *testing.T) {
	t.Parallel()
	const limit = 10
	exact := strings.Repeat("a", limit)
	if got := domain.Excerpt(exact, nil, limit); got != exact {
		t.Fatalf("exact: %q", got)
	}
	if got := domain.Excerpt(exact+"b", nil, limit); got != strings.Repeat("a", limit-1)+"…" {
		t.Fatalf("one more: %q", got)
	}
	multi := strings.Repeat("ä", limit+3)
	got := domain.Excerpt(multi, nil, limit)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != limit || !strings.HasSuffix(got, "…") {
		t.Fatalf("multi-byte: %q", got)
	}
	if got := domain.Excerpt("abc", nil, 0); got != "" {
		t.Fatalf("zero limit: %q", got)
	}
}

// FuzzExcerpt: the excerpt never panics, is valid UTF-8, has at most maxChars code points and no
// control or bidi control character.
func FuzzExcerpt(f *testing.F) {
	for _, s := range []string{"**@sam**", "```\n@sam\n```", "[a](b)", "<x:y>", "| a |", "\xff\xfe", "a\u202eb", "~~a~~"} {
		f.Add(s, uint8(8))
	}
	f.Fuzz(func(t *testing.T, s string, n uint8) {
		maxChars := int(n%64) + 1
		got := domain.Excerpt(s, names, maxChars)
		if !utf8.ValidString(got) {
			t.Fatalf("invalid UTF-8: %q", got)
		}
		if c := utf8.RuneCountInString(got); c > maxChars {
			t.Fatalf("%d code points, limit %d", c, maxChars)
		}
		for _, r := range got {
			if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
				t.Fatalf("control character %U in %q", r, got)
			}
		}
	})
}

// TestParsePreviewMode: the three values pass, anything else is the default.
func TestParsePreviewMode(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]domain.PreviewMode{
		"full": domain.PreviewFull, "names": domain.PreviewNames, "none": domain.PreviewNone,
		"": domain.PreviewNames, "FULL": domain.PreviewNames, "yes": domain.PreviewNames,
	} {
		if got := domain.ParsePreviewMode(in, domain.PreviewNames); got != want {
			t.Errorf("ParsePreviewMode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPreviewKindAllowed: the six kinds with a message are allowed; security, admin and sync not.
func TestPreviewKindAllowed(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]bool{
		"message": true, "mention": true, "dm": true, "run_done": true, "run_failed": true, "input_needed": true,
		"security": false, "admin": false, "sync": false, "": false,
	} {
		if got := domain.PreviewKindAllowed(kind); got != want {
			t.Errorf("PreviewKindAllowed(%q) = %v, want %v", kind, got, want)
		}
	}
}
