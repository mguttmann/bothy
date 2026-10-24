// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// SettleConfig are the values of the settle rule (22 12.3, 19).
type SettleConfig struct {
	// BusyReactions is import.follow.busy_reactions: emoji names that mean "an agent is still
	// working".
	BusyReactions []string
	// SettleAfter is import.follow.settle_after: the quiet time after the last change.
	SettleAfter time.Duration
}

// HoldReason says why the settle rule held a thread back; "" when nothing was held.
type HoldReason string

// The reasons of Settle.
const (
	// HoldBusy: a message of a taken type (or the starter of a type 21 message) carries a busy
	// reaction.
	HoldBusy HoldReason = "busy"
	// HoldUnsettled: a message of a taken type changed less than import.follow.settle_after ago.
	HoldUnsettled HoldReason = "unsettled"
	// HoldLookbackBusy: a message of the lookback before the first new one is busy, so the whole
	// thread waits.
	HoldLookbackBusy HoldReason = "lookback_busy"
)

// Settle applies the settle rule of 22 12.3 to the new messages of one thread (newMsgs ascending
// by ID) and returns the beginning that may be copied, the number of messages held back and the
// reason. A message is busy when it, or for type 21 its starter message (starter returns nil when
// unknown), has a reaction whose emoji name is in cfg.BusyReactions with a count above zero; it is
// unsettled when the later of its timestamp and edited_timestamp lies less than cfg.SettleAfter
// before now. The copy ends before the first busy or unsettled message of a type that is taken
// over (22 6.1); messages of other types inside it pass through (the caller counts them as
// skipped) and never stop it. When any message of lookback (the import.follow.busy_lookback
// messages before the first new one) is busy, the copy is empty. Pure: no I/O, no clock.
//
// Example: with the defaults a reply written 30 seconds ago stops the copy before it, and every
// message after it waits for the next round.
func Settle(now time.Time, newMsgs []discord.Message, lookback []discord.Message, starter func(id string) *discord.Message,
	cfg SettleConfig) ([]discord.Message, int, HoldReason) {
	if len(newMsgs) == 0 {
		return nil, 0, ""
	}
	for _, m := range lookback {
		if isBusy(m, starter, cfg) {
			return nil, len(newMsgs), HoldLookbackBusy
		}
	}
	for i, m := range newMsgs {
		if !takenTypes[m.Type] {
			continue
		}
		if isBusy(m, starter, cfg) {
			return newMsgs[:i], len(newMsgs) - i, HoldBusy
		}
		if now.Sub(lastChange(m)) < cfg.SettleAfter {
			return newMsgs[:i], len(newMsgs) - i, HoldUnsettled
		}
	}
	return newMsgs, 0, ""
}

// isBusy reports a busy reaction on m or, for type 21, on its starter message.
func isBusy(m discord.Message, starter func(id string) *discord.Message, cfg SettleConfig) bool {
	if hasBusyReaction(m, cfg.BusyReactions) {
		return true
	}
	if m.Type != msgThreadStarter || m.MessageReference == nil || starter == nil {
		return false
	}
	s := starter(m.MessageReference.MessageID)
	return s != nil && hasBusyReaction(*s, cfg.BusyReactions)
}

// hasBusyReaction reports a reaction of names with a count above zero.
func hasBusyReaction(m discord.Message, names []string) bool {
	for _, r := range m.Reactions {
		if r.Count > 0 && slices.Contains(names, r.Emoji.Name) {
			return true
		}
	}
	return false
}

// lastChange is the later of timestamp and edited_timestamp; a timestamp Discord did not send or
// that does not parse falls back to the time in the message ID.
func lastChange(m discord.Message) time.Time {
	t := messageTime(m)
	if m.EditedTimestamp != "" {
		if e, err := ParseDiscordTime(m.EditedTimestamp); err == nil && e.After(t) {
			t = e
		}
	}
	return t
}
