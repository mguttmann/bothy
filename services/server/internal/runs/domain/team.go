// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Team channels (docs/spec/09-agents.md 3.9, FEAT-TEAM-CHANNEL-SERVER): the handle rule, the
// handles a reply mentions and the safety brake, as pure functions.

// TriggerTeam is the trigger kind of a run of a team channel member and of its token: the
// triggering object is the input message in the member's work thread (09 3.9.9).
const TriggerTeam TriggerKind = "team"

// The sources of a team input (03 team_inputs.source).
const (
	// TeamSourceOwner is an owner's message in the team channel.
	TeamSourceOwner = "owner"
	// TeamSourceMember is a member's final reply that mentions the receiving member.
	TeamSourceMember = "member"
)

// The pause reasons of a team channel (03 team_channels.pause_reason, 09 3.9.6 and 3.9.7) and the
// refusals of a team input (09 3.9.5); a refused input names one of them in its note and audit.
const (
	TeamPauseTurnBudget = "turn_budget"
	TeamPausePingpong   = "pingpong"
	TeamPauseStopped    = "stopped"
	// TeamRefusalSponsor: the authority of the input is no active owner now.
	TeamRefusalSponsor = "team_sponsor_not_owner"
	// TeamRefusalReaders: the readers of the team channel and the member's home channel differ.
	TeamRefusalReaders = "team_readers_differ"
)

// ValidHandle reports whether h is a handle of a team member (09 3.9.1, 03 team_members): lower
// case letters a to z, digits, '-' and '_', starting with a letter or digit, at most maxChars
// characters.
func ValidHandle(h string, maxChars int) bool {
	if h == "" || utf8.RuneCountInString(h) > maxChars {
		return false
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case (c == '-' || c == '_') && i > 0:
		default:
			return false
		}
	}
	return true
}

// handleChar reports whether c can be part of a handle in running text (either case; 09 3.9.4).
func handleChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// MentionedHandles returns the handles text mentions, lower case, in the order of their first
// mention and without duplicates (09 3.9.4): "@handle" at the start or after a character that is
// no letter, digit, '-', '_' or '@', running up to the first character that cannot be part of a
// handle; text inside a code span or a fenced code block does not count. The caller keeps only the
// handles of members.
func MentionedHandles(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range outsideCode(text) {
		for i := 0; i < len(part); i++ {
			if part[i] != '@' || (i > 0 && (handleChar(part[i-1]) || part[i-1] == '@')) {
				continue
			}
			j := i + 1
			for j < len(part) && handleChar(part[j]) {
				j++
			}
			if j == i+1 {
				continue
			}
			h := strings.ToLower(part[i+1 : j])
			if !seen[h] {
				seen[h] = true
				out = append(out, h)
			}
			i = j - 1
		}
	}
	return out
}

// outsideCode returns the parts of text outside fenced code blocks (lines that start with three
// or more backticks or tildes open and close one) and outside code spans (a run of backticks up
// to the next run of the same length on the same line part; an unclosed run is text).
func outsideCode(text string) []string {
	var parts []string
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if f := fenceOf(trimmed); f != "" {
			switch {
			case fence == "":
				fence = f
				continue
			case strings.HasPrefix(trimmed, fence) && strings.TrimSpace(strings.TrimLeft(trimmed, fence[:1])) == "":
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		parts = append(parts, outsideSpans(line)...)
	}
	return parts
}

// fenceOf returns the fence a line opens or closes: three or more backticks or tildes at its
// start; "" for none.
func fenceOf(line string) string {
	for _, c := range []byte{'`', '~'} {
		n := 0
		for n < len(line) && line[n] == c {
			n++
		}
		if n >= 3 {
			return line[:n]
		}
	}
	return ""
}

// outsideSpans returns the parts of one line outside its code spans: a run of backticks opens a
// span that the next run of the same length closes; an unclosed run is text.
func outsideSpans(line string) []string {
	var parts []string
	start, i := 0, 0
	for i < len(line) {
		if line[i] != '`' {
			i++
			continue
		}
		n := backticks(line, i)
		end := closingRun(line, i+n, n)
		if end < 0 {
			i += n
			continue
		}
		parts = append(parts, line[start:i])
		i = end + n
		start = i
	}
	return append(parts, line[start:])
}

// backticks returns the length of the run of backticks at i.
func backticks(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// closingRun returns the start of the first run of exactly n backticks at or after from; -1 for
// none.
func closingRun(s string, from, n int) int {
	for j := from; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		m := backticks(s, j)
		if m == n {
			return j
		}
		j += m
	}
	return -1
}

// TeamBrake is the brake state of a team channel (03 team_channels): the member inputs since an
// owner's last message, the last input's source and target and the length of the alternation
// between them, and the pause reason ("" while member inputs flow).
type TeamBrake struct {
	MemberTurns   int
	From, To      ids.ID
	PingpongCount int
	PauseReason   string
}

// TeamLimits are the limits of the brake: team.turn_budget and team.pingpong_max (09 21).
type TeamLimits struct {
	TurnBudget, PingpongMax int
}

// NextBrake decides one member input from the member from to the member to (09 3.9.6). It returns
// the brake state to store and, for a refused input, the reason: the pause reason of a paused
// channel; turn_budget when the channel reached the budget; pingpong when the input would make the
// alternation between the two longer than the limit. A refusal of a channel that was not paused
// pauses it; the counts of a refused input stay. An allowed input counts one more turn and continues
// the alternation when it reverses the last input, else starts a new one at 1.
func NextBrake(b TeamBrake, from, to ids.ID, lim TeamLimits) (TeamBrake, string) {
	if b.PauseReason != "" {
		return b, b.PauseReason
	}
	if b.MemberTurns >= lim.TurnBudget {
		b.PauseReason = TeamPauseTurnBudget
		return b, TeamPauseTurnBudget
	}
	count := 1
	if b.From == to && b.To == from {
		count = b.PingpongCount + 1
	}
	if count > lim.PingpongMax {
		b.PauseReason = TeamPausePingpong
		return b, TeamPausePingpong
	}
	b.MemberTurns++
	b.From, b.To, b.PingpongCount = from, to, count
	return b, ""
}

// ResetBrake is the brake state after an owner's message in the team channel (09 3.9.3): no turns,
// no alternation, no pause.
func ResetBrake() TeamBrake { return TeamBrake{} }
