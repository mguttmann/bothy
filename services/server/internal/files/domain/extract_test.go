// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

func TestExtractApplies(t *testing.T) {
	t.Parallel()
	type file struct {
		upload, scan string
		deleted      bool
	}
	states := []file{
		{"complete", "clean", false}, {"complete", "skipped", false}, {"complete", "infected", false},
		{"complete", "error", false}, {"pending", "pending", false}, {"failed", "pending", false},
		{"complete", "clean", true},
	}
	for _, purpose := range []string{"attachment", "avatar", "branding", "gdpr_export"} {
		for _, st := range states {
			for _, supported := range []bool{true, false} {
				deliverable := domain.IsDeliverable(st.upload, st.scan, st.deleted)
				want := purpose == "attachment" && supported && !st.deleted && st.upload == "complete" &&
					(st.scan == "clean" || st.scan == "skipped")
				if got := domain.ExtractApplies(purpose, deliverable, supported); got != want {
					t.Errorf("%s %+v supported=%v = %v; want %v", purpose, st, supported, got, want)
				}
			}
		}
	}
}

func TestRefreshDecision(t *testing.T) {
	t.Parallel()
	const cur, old, limit = "builtin-ooxml/2", "builtin-ooxml/1", int64(1000)
	cases := []struct {
		name      string
		row       domain.RefreshRow
		supported bool
		want      domain.RefreshAction
	}{
		{"none supported", domain.RefreshRow{}, true, domain.RefreshCreate},
		{"none unsupported", domain.RefreshRow{}, false, domain.RefreshNothing},
		{"pending", domain.RefreshRow{Exists: true, State: "pending", Extractor: cur}, true, domain.RefreshEnqueue},
		{"pending unsupported", domain.RefreshRow{Exists: true, State: "pending"}, false, domain.RefreshEnqueue},
		{"done old", domain.RefreshRow{Exists: true, State: "done", Extractor: old}, true, domain.RefreshReset},
		{"failed old", domain.RefreshRow{Exists: true, State: "failed", Extractor: old, Error: "xml_invalid"}, true, domain.RefreshReset},
		{"unsupported null", domain.RefreshRow{Exists: true, State: "unsupported"}, true, domain.RefreshReset},
		{"done old unsupported now", domain.RefreshRow{Exists: true, State: "done", Extractor: old}, false, domain.RefreshNothing},
		{"too large now fits", domain.RefreshRow{Exists: true, State: "failed", Extractor: cur, Error: "input_too_large", SizeBytes: limit}, true, domain.RefreshReset},
		{"too large still", domain.RefreshRow{Exists: true, State: "failed", Extractor: cur, Error: "input_too_large", SizeBytes: limit + 1}, true, domain.RefreshNothing},
		{"done current", domain.RefreshRow{Exists: true, State: "done", Extractor: cur}, true, domain.RefreshNothing},
		{"failed current other code", domain.RefreshRow{Exists: true, State: "failed", Extractor: cur, Error: "xml_doctype", SizeBytes: 1}, true, domain.RefreshNothing},
	}
	for _, c := range cases {
		if got := domain.RefreshDecision(c.row, cur, c.supported, limit); got != c.want {
			t.Errorf("%s: %v; want %v", c.name, got, c.want)
		}
	}
}

func TestTextBuilder(t *testing.T) {
	t.Parallel()
	t.Run("cut at a rune boundary", func(t *testing.T) {
		t.Parallel()
		b := domain.NewTextBuilder(5)
		// language-check: off
		b.WriteText("abcü") // 3 + 2 bytes: fits exactly
		if b.String() != "abcü" || b.Truncated() || !b.Full() {
			// language-check: on
			t.Fatalf("exact: %q truncated=%v full=%v", b.String(), b.Truncated(), b.Full())
		}
		b = domain.NewTextBuilder(4)
		// language-check: off
		b.WriteText("abcü")
		// language-check: on
		if b.String() != "abc" || !b.Truncated() || !b.Full() {
			t.Fatalf("cut: %q truncated=%v", b.String(), b.Truncated())
		}
	})
	t.Run("nothing dropped", func(t *testing.T) {
		t.Parallel()
		b := domain.NewTextBuilder(10)
		b.WriteText("abc")
		b.Separator('\n')
		if b.String() != "abc" || b.Truncated() || b.Full() {
			t.Fatalf("%q truncated=%v full=%v", b.String(), b.Truncated(), b.Full())
		}
	})
	t.Run("normalisation", func(t *testing.T) {
		t.Parallel()
		b := domain.NewTextBuilder(100)
		b.WriteText("a\xffb\x00c\x01d\te\nf\rg\x7fh")
		want := "a�b c d\te\nf\rg h"
		if got := b.String(); got != want || !utf8.ValidString(got) {
			t.Fatalf("%q; want %q", got, want)
		}
	})
	t.Run("separators once", func(t *testing.T) {
		t.Parallel()
		b := domain.NewTextBuilder(100)
		b.Separator('\n')
		b.WriteText("a")
		b.Separator(' ')
		b.Separator('\n')
		b.Separator('\t')
		b.WriteText("b")
		b.Separator('\t')
		b.WriteText("c ")
		b.Separator('\n')
		b.WriteText("d")
		b.Separator('\n')
		if got := b.String(); got != "a\nb\tc d" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("separator that does not fit", func(t *testing.T) {
		t.Parallel()
		b := domain.NewTextBuilder(2)
		b.WriteText("a")
		b.Separator('\n')
		b.WriteText("b")
		if b.String() != "a" || !b.Truncated() {
			t.Fatalf("%q truncated=%v", b.String(), b.Truncated())
		}
	})
	t.Run("full and mark", func(t *testing.T) {
		t.Parallel()
		b := domain.NewTextBuilder(3)
		b.WriteText("abc")
		if !b.Full() || b.Truncated() {
			t.Fatal("full without truncation expected")
		}
		b.MarkTruncated()
		if !b.Truncated() {
			t.Fatal("MarkTruncated")
		}
		var zero domain.TextBuilder
		zero.WriteText("x")
		if zero.String() != "" || !zero.Truncated() {
			t.Fatal("zero value takes nothing")
		}
	})
}

func TestHalveText(t *testing.T) {
	t.Parallel()
	if got := domain.HalveText(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := domain.HalveText("abcd"); got != "ab" {
		t.Fatalf("ascii: %q", got)
	}
	// language-check: off
	if got := domain.HalveText("aüü"); got != "a" { // 5 bytes, half 2 is inside the first ü
		// language-check: on
		t.Fatalf("rune boundary: %q", got)
	}
	s := strings.Repeat("ü€😀x", 50)
	for i := 0; s != ""; i++ {
		next := domain.HalveText(s)
		if !utf8.ValidString(next) || len(next) >= len(s) || !strings.HasPrefix(s, next) {
			t.Fatalf("step %d: %q from %q", i, next, s)
		}
		s = next
		if i > 64 {
			t.Fatal("halving does not reach the empty string")
		}
	}
}

func TestExtractError(t *testing.T) {
	t.Parallel()
	err := domain.NewExtractError(domain.CodeXMLDoctype)
	if err.Error() != "files: text extraction failed: xml_doctype" || err.Code != domain.CodeXMLDoctype {
		t.Fatal(err)
	}
}
