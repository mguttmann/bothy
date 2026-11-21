// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestValidHandle holds the handle rule of 09 3.9.1 and 03 team_members.
func TestValidHandle(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		h    string
		want bool
	}{
		{"coding", true}, {"qa-2", true}, {"a_b", true}, {"7", true},
		{"", false}, {"-a", false}, {"_a", false}, {"Coding", false}, {"a b", false}, {"a.b", false}, {"a+b", false},
		{strings.Repeat("a", 32), true}, {strings.Repeat("a", 33), false},
	} {
		if got := ValidHandle(tc.h, 32); got != tc.want {
			t.Errorf("ValidHandle(%q) = %v, want %v", tc.h, got, tc.want)
		}
	}
}

// TestMentionedHandles holds the mention rule of 09 3.9.4: order of first mention, lower case, no
// duplicates, word boundaries, nothing inside code.
func TestMentionedHandles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"none", "no mention here", nil},
		{"one", "@review I built X, please take over", []string{"review"}},
		{"order and duplicates", "@qa and @Review, then @review again and @qa", []string{"qa", "review"}},
		{"punctuation", "(@coding), @qa: done. @ops!", []string{"coding", "qa", "ops"}},
		{"email is no mention", "mail alex@example.com or @@qa", nil},
		{"bare at", "@ and @ alone", nil},
		{"code span", "run `@coding fix` then @qa", []string{"qa"}},
		{"double backtick span", "see ``a ` @coding`` and @qa", []string{"qa"}},
		{"unclosed backtick", "a ` @coding", []string{"coding"}},
		{"fenced block", "before @qa\n```\n@coding inside\n```\nafter @ops", []string{"qa", "ops"}},
		{"tilde fence", "~~~go\n@coding\n~~~\n@qa", []string{"qa"}},
		{"unclosed fence hides the rest", "@qa\n```\n@coding", []string{"qa"}},
	} {
		if got := MentionedHandles(tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: MentionedHandles(%q) = %q, want %q", tc.name, tc.text, got, tc.want)
		}
	}
}

// TestNextBrake holds the turn budget, the ping-pong rule and the pause of 09 3.9.6.
func TestNextBrake(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	lim := TeamLimits{TurnBudget: 5, PingpongMax: 3}

	t.Run("alternation pauses above the limit", func(t *testing.T) {
		t.Parallel()
		s := ResetBrake()
		var reason string
		for i, e := range [][2]ids.ID{{a, b}, {b, a}, {a, b}} {
			if s, reason = NextBrake(s, e[0], e[1], lim); reason != "" {
				t.Fatalf("input %d refused: %s", i, reason)
			}
		}
		if s.PingpongCount != 3 || s.MemberTurns != 3 {
			t.Fatalf("state %+v, want count 3 and 3 turns", s)
		}
		s, reason = NextBrake(s, b, a, lim)
		if reason != TeamPausePingpong || s.PauseReason != TeamPausePingpong || s.MemberTurns != 3 {
			t.Fatalf("fourth alternation: reason %q state %+v", reason, s)
		}
		if _, reason = NextBrake(s, c, a, lim); reason != TeamPausePingpong {
			t.Fatalf("a paused channel refuses every member input, got %q", reason)
		}
	})

	t.Run("another pair or the same direction starts a new alternation", func(t *testing.T) {
		t.Parallel()
		s := ResetBrake()
		s, _ = NextBrake(s, a, b, lim)
		s, _ = NextBrake(s, b, a, lim)
		s, _ = NextBrake(s, b, c, lim)
		if s.PingpongCount != 1 {
			t.Fatalf("a new pair starts at 1, got %+v", s)
		}
		s, _ = NextBrake(s, b, c, lim)
		if s.PingpongCount != 1 {
			t.Fatalf("the same direction again starts at 1, got %+v", s)
		}
	})

	t.Run("turn budget", func(t *testing.T) {
		t.Parallel()
		s := ResetBrake()
		var reason string
		for i := range 5 {
			from, to := a, b
			if i%2 == 1 {
				from, to = a, c
			}
			if s, reason = NextBrake(s, from, to, lim); reason != "" {
				t.Fatalf("input %d refused: %s", i, reason)
			}
		}
		s, reason = NextBrake(s, a, b, lim)
		if reason != TeamPauseTurnBudget || s.PauseReason != TeamPauseTurnBudget || s.MemberTurns != 5 {
			t.Fatalf("over the budget: reason %q state %+v", reason, s)
		}
		if r := ResetBrake(); r != (TeamBrake{}) {
			t.Fatalf("an owner message resets everything, got %+v", r)
		}
	})

	t.Run("stopped", func(t *testing.T) {
		t.Parallel()
		s := TeamBrake{PauseReason: TeamPauseStopped}
		if _, reason := NextBrake(s, a, b, lim); reason != TeamPauseStopped {
			t.Fatalf("a stopped channel refuses with stopped, got %q", reason)
		}
	})
}
