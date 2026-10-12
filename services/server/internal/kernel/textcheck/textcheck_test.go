// SPDX-License-Identifier: Apache-2.0

package textcheck_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

func TestDisplayText(t *testing.T) {
	t.Parallel()
	bidi := []rune{0x061C, 0x200E, 0x200F, 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069}
	type tc struct {
		name   string
		in     string
		rule   textcheck.Rule // "" means the text passes
		offset int
	}
	cases := []tc{
		{name: "plain", in: "Alex"},
		{name: "accent", in: "Zoë"},
		{name: "emoji", in: "Sam 🙂"},
		{name: "zero width joiner", in: "Robin 👩\u200D💻"},
		{name: "empty", in: ""},
		{name: "space only", in: "   "},
		{name: "NUL", in: "Al\x00ex", rule: textcheck.RuleControlCharacter, offset: 2},
		{name: "tab", in: "\tAlex", rule: textcheck.RuleControlCharacter, offset: 0},
		{name: "newline", in: "Alex\n", rule: textcheck.RuleControlCharacter, offset: 4},
		{name: "DEL", in: "A\x7f", rule: textcheck.RuleControlCharacter, offset: 1},
		{name: "NEL", in: "Zoë\u0085", rule: textcheck.RuleControlCharacter, offset: 3},
		{name: "C1 end", in: "\u009f", rule: textcheck.RuleControlCharacter, offset: 0},
		{name: "invalid UTF-8", in: "Al\xffex", rule: textcheck.RuleInvalidUTF8, offset: 2},
		{name: "truncated UTF-8", in: "Zo\xc3", rule: textcheck.RuleInvalidUTF8, offset: 2},
	}
	for _, r := range bidi {
		cases = append(cases, tc{
			name: fmt.Sprintf("bidi U+%04X", r), in: "ab" + string(r) + "c",
			rule: textcheck.RuleBidiControl, offset: 2,
		})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := textcheck.DisplayText(c.in)
			if c.rule == "" {
				if err != nil {
					t.Fatalf("DisplayText(%q) = %v, want nil", c.in, err)
				}
				return
			}
			var te *textcheck.Error
			if !errors.As(err, &te) {
				t.Fatalf("DisplayText(%q) = %v, want *textcheck.Error", c.in, err)
			}
			if te.Rule != c.rule || te.Offset != c.offset {
				t.Fatalf("DisplayText(%q) = rule %q offset %d, want %q at %d", c.in, te.Rule, te.Offset, c.rule, c.offset)
			}
			if !errors.Is(err, textcheck.ErrRejected) {
				t.Errorf("error does not wrap ErrRejected")
			}
			if strings.Contains(err.Error(), c.in) && c.in != "" {
				t.Errorf("error text %q echoes the input", err.Error())
			}
		})
	}
}

// TestDisplayTextErrorNeverEchoesText checks that the message names rule and offset only.
func TestDisplayTextErrorNeverEchoesText(t *testing.T) {
	t.Parallel()
	err := textcheck.DisplayText("secret-name\u202E")
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error text %q echoes the input", err.Error())
	}
	if want := "textcheck: bidi_control at rune 11"; err.Error() != want {
		t.Fatalf("error text %q, want %q", err.Error(), want)
	}
}
