// SPDX-License-Identifier: Apache-2.0

package title_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
)

// TestNormalize (brief M2-18 note 4a): leading and trailing white space disappears, every inner
// run of white space, Unicode spaces included, becomes one U+0020; normalised text is unchanged.
func TestNormalize(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":                     "",
		"   ":                  "",
		"  Neuer   Chat  ":     "Neuer Chat",
		"\tNew\nchat\r\n":      "New chat",
		" Ohne　Titel ":         "Ohne Titel",
		"Neuer  Chat":          "Neuer Chat",
		"Invoice export fails": "Invoice export fails",
	} {
		if got := title.Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsPlaceholder_DefaultList (09 section 12 point 2): every entry with case, white space and
// number variants, the empty title and blanks are placeholders; numbers without a space, other
// characters, entries inside longer titles and punctuation are not.
func TestIsPlaceholder_DefaultList(t *testing.T) {
	t.Parallel()
	p := title.NewPlaceholders(title.DefaultEntries())
	yes := []string{"", " ", "   ", "\t", "NEUER CHAT", "new CHAT", "  neuer   chat ", "Neuer Chat 3", "New chat 2", "untitled 12",
		"Ohne Titel 1", "Test 007", "Neuer Chat   3", "Chat 3"}
	for _, e := range title.DefaultEntries() {
		yes = append(yes, e, strings.ToUpper(e), strings.ToLower(e), "  "+e+"\t", strings.ReplaceAll(e, " ", "   "), e+" 3", e+" 042")
	}
	for _, v := range yes {
		if !p.IsPlaceholder(v) {
			t.Errorf("%q is not a placeholder", v)
		}
	}
	// Cases moved from TestPlaceholderTitle of M1-55 included ("3", "Chat3", "Chat -3", "Chat ٣").
	for _, v := range []string{"Test2", "Neuer Chat #3", "Neuer Chat (3)", "Neuer Chat 3.", "Neuer Chat 3a", "2", "2026", "3", " 3",
		"Chatbot", "Testrun", "New chat about invoices", "Untitled draft", "?", "...", "\U0001F600", "Chat3", "Chat 3a", "Chat -3",
		"Chat 3 4", "Neuer Chatbot", "Invoice export fails", "Chat ٣"} {
		if p.IsPlaceholder(v) {
			t.Errorf("%q is a placeholder", v)
		}
	}
}

// protectedShortNames are the examples of docs/REUSE.md L23 and 5.1 plus the short real names of
// the brief.
var protectedShortNames = []string{"KI", "M365", "C#", "PC 2", "AI", "UX", "Q3", "2FA"}

// TestIsPlaceholder_ProtectedShortNames (docs/REUSE.md L23): short real names are no placeholders
// under the default list and under each language list alone, and no entry equals one of them.
func TestIsPlaceholder_ProtectedShortNames(t *testing.T) {
	t.Parallel()
	lists := [][]string{title.DefaultEntries()}
	for _, lang := range title.Languages() {
		l, _ := title.WordList(lang)
		lists = append(lists, l)
	}
	for _, l := range lists {
		p := title.NewPlaceholders(l)
		for _, n := range protectedShortNames {
			if p.IsPlaceholder(n) {
				t.Errorf("%q is a placeholder under %v", n, l)
			}
			for _, e := range l {
				if strings.EqualFold(e, n) {
					t.Errorf("entry %q equals the short name %q", e, n)
				}
			}
		}
	}
}

// TestIsPlaceholder_ConfiguredListReplacesDefault (brief note 3c): a configured list replaces the
// default; blank entries are ignored and the empty title stays a placeholder.
func TestIsPlaceholder_ConfiguredListReplacesDefault(t *testing.T) {
	t.Parallel()
	p := title.NewPlaceholders([]string{"Draft", " ", ""})
	for _, v := range []string{"Draft", "draft 2", "", "  "} {
		if !p.IsPlaceholder(v) {
			t.Errorf("%q is not a placeholder", v)
		}
	}
	for _, v := range []string{"Neuer Chat", "7", " 7", "Draft2"} {
		if p.IsPlaceholder(v) {
			t.Errorf("%q is a placeholder", v)
		}
	}
	// The zero value has no entries: only the empty title is a placeholder.
	var zero title.Placeholders
	if !zero.IsPlaceholder("") || zero.IsPlaceholder("Chat") {
		t.Error("the zero value matches more than the empty title")
	}
}
