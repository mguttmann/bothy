// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// settleNow is the "now" of the settle table.
var settleNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// settleCfg are the settle values of the table: one busy emoji and a quiet time of one minute.
var settleCfg = SettleConfig{BusyReactions: []string{"\U0001F440"}, SettleAfter: time.Minute}

// ago formats a time before settleNow as a Discord timestamp.
func ago(d time.Duration) string { return settleNow.Add(-d).Format(time.RFC3339Nano) }

// msgAt is a settled message of type 0 written d ago.
func msgAt(id string, d time.Duration) discord.Message {
	return discord.Message{ID: id, Type: 0, Content: "text", Timestamp: ago(d)}
}

// busy adds a reaction of emoji with count n to m.
func busy(m discord.Message, emoji string, n int) discord.Message {
	m.Reactions = append(m.Reactions, discord.Reaction{Count: n, Emoji: discord.Emoji{Name: emoji}})
	return m
}

// TestSettleRule (plan acceptance, 22 16 row "Settle rule", 22 12.3): the copy is the beginning
// of the new messages up to before the first busy or unsettled message of a taken type; the
// lookback empties it.
func TestSettleRule(t *testing.T) {
	const eyes, thumbs = "\U0001F440", "\U0001F44D"
	old := 10 * time.Minute
	edited := msgAt("3", old)
	edited.EditedTimestamp = ago(10 * time.Second)
	starterMsg := discord.Message{ID: "3", Type: 21, Content: "start", Timestamp: ago(old),
		MessageReference: &discord.MessageRef{MessageID: "900", ChannelID: "800"}}
	system := discord.Message{ID: "2", Type: 18, Timestamp: ago(time.Second), Reactions: []discord.Reaction{{Count: 1, Emoji: discord.Emoji{Name: eyes}}}}
	starters := map[string]*discord.Message{"900": new(busy(msgAt("900", old), eyes, 1))}
	for _, tc := range []struct {
		name     string
		msgs     []discord.Message
		lookback []discord.Message
		want     []string
		held     int
		reason   HoldReason
	}{
		{name: "all settled", msgs: []discord.Message{msgAt("1", old), msgAt("2", old), msgAt("3", old)}, want: []string{"1", "2", "3"}},
		{name: "busy in the middle", msgs: []discord.Message{msgAt("1", old), busy(msgAt("2", old), eyes, 1), msgAt("3", old)},
			want: []string{"1"}, held: 2, reason: HoldBusy},
		{name: "unsettled", msgs: []discord.Message{msgAt("1", old), msgAt("2", 30*time.Second), msgAt("3", old)},
			want: []string{"1"}, held: 2, reason: HoldUnsettled},
		{name: "edit within settle_after", msgs: []discord.Message{msgAt("1", old), msgAt("2", old), edited},
			want: []string{"1", "2"}, held: 1, reason: HoldUnsettled},
		{name: "starter busy", msgs: []discord.Message{msgAt("1", old), msgAt("2", old), starterMsg},
			want: []string{"1", "2"}, held: 1, reason: HoldBusy},
		{name: "skipped type passes", msgs: []discord.Message{msgAt("1", old), system, msgAt("3", old)}, want: []string{"1", "2", "3"}},
		{name: "lookback busy", msgs: []discord.Message{msgAt("5", old), msgAt("6", old)},
			lookback: []discord.Message{msgAt("3", old), busy(msgAt("4", old), eyes, 2)}, held: 2, reason: HoldLookbackBusy},
		{name: "lookback settled", msgs: []discord.Message{msgAt("5", old)}, lookback: []discord.Message{msgAt("4", old)}, want: []string{"5"}},
		{name: "count zero", msgs: []discord.Message{busy(msgAt("1", old), eyes, 0), msgAt("2", old)}, want: []string{"1", "2"}},
		{name: "other emoji", msgs: []discord.Message{busy(msgAt("1", old), thumbs, 3), msgAt("2", old)}, want: []string{"1", "2"}},
		{name: "nothing new", want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, held, reason := Settle(settleNow, tc.msgs, tc.lookback, func(id string) *discord.Message { return starters[id] }, settleCfg)
			ids := make([]string, 0, len(got))
			for _, m := range got {
				ids = append(ids, m.ID)
			}
			if !slices.Equal(ids, tc.want) && (len(ids) != 0 || len(tc.want) != 0) {
				t.Errorf("copy %v, want %v", ids, tc.want)
			}
			if held != tc.held || reason != tc.reason {
				t.Errorf("held %d reason %q, want %d %q", held, reason, tc.held, tc.reason)
			}
		})
	}
}

// TestSettleNilStarter: a type 21 message whose starter is unknown is judged by itself.
func TestSettleNilStarter(t *testing.T) {
	m := msgAt("1", time.Hour)
	m.Type = 21
	m.MessageReference = &discord.MessageRef{MessageID: "9"}
	got, held, _ := Settle(settleNow, []discord.Message{m}, nil, func(string) *discord.Message { return nil }, settleCfg)
	if len(got) != 1 || held != 0 {
		t.Errorf("copy %d held %d, want 1 and 0", len(got), held)
	}
}
