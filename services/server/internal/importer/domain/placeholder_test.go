// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/importer/domain"
)

// handleAlphabet is the handle rule of 03 principals.handle.
var handleAlphabet = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// TestPlaceholderHandle checks design note 8: prefix, alphabet, collapsed runs, the counter on
// collision, the cut at principal.handle_max_chars and emoji-only names.
func TestPlaceholderHandle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		n    int
		max  int
		want string
	}{
		{"Sam", 1, 32, "imported-sam"},
		{"Sam Example", 1, 32, "imported-sam-example"},
		{"Sam Example", 2, 32, "imported-sam-example-2"},
		{"Sam Example", 3, 32, "imported-sam-example-3"},
		{"  Robin!!__x  ", 1, 32, "imported-robin-__x"},
		{"Ünïcødé", 1, 32, "imported-n-c-d"},
		{"😀😀", 1, 32, "imported"},
		{"😀😀", 2, 32, "imported-2"},
		{"alex.example", 1, 32, "imported-alex.example"},
		{"abcdefghijklmnopqrstuvwxyz", 1, 20, "imported-abcdefghijk"},
		{"abcdefghijklmnopqrstuvwxyz", 12, 20, "imported-abcdefgh-12"},
		{"ab cdefghij", 1, 12, "imported-ab"},
	}
	for _, c := range cases {
		got := domain.PlaceholderHandle(c.name, c.n, c.max)
		if got != c.want {
			t.Errorf("PlaceholderHandle(%q, %d, %d) = %q, want %q", c.name, c.n, c.max, got, c.want)
		}
		if len(got) > c.max || !handleAlphabet.MatchString(got) {
			t.Errorf("%q breaks the handle rule", got)
		}
	}
}

// TestPlaceholderDisplayName prefers global_name, falls back to username and to the handle, and
// removes what SEC-IN-4 rejects.
func TestPlaceholderDisplayName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		global, user, fallback string
		max                    int
		want                   string
	}{
		{"Sam E.", "sam", "imported-sam", 64, "Sam E."},
		{"", "sam", "imported-sam", 64, "sam"},
		{"  ", "sam", "imported-sam", 64, "sam"},
		{"Ro\u202Ebin\x07", "robin", "x", 64, "Robin"},
		{"\u200B", "\u202E", "imported", 64, "imported"},
		{"abcdef", "", "", 3, "abc"},
	}
	for _, c := range cases {
		got := domain.PlaceholderDisplayName(c.global, c.user, c.fallback, c.max)
		if got != c.want || utf8.RuneCountInString(got) > c.max || strings.ContainsRune(got, '\u202E') {
			t.Errorf("PlaceholderDisplayName(%q, %q) = %q, want %q", c.global, c.user, got, c.want)
		}
	}
}
