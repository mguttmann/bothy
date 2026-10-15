// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Field codes and pointers of reactions and pins (brief M1-59 design notes 3 and 6).
const (
	// FieldInvalidEmoji marks a symbol that is not exactly one permitted grapheme cluster.
	FieldInvalidEmoji = "invalid_emoji"
	// PointerPathEmoji and PointerQueryEmoji are the origins of a symbol: the path of addReaction
	// and removeReaction, the filter of listReactions.
	PointerPathEmoji  = "#path/emoji"
	PointerQueryEmoji = "#query/emoji"
)

// InvalidEmoji is the 400 of a refused symbol at pointer; it names no part of the symbol.
func InvalidEmoji(pointer string) error {
	return fieldError(pointer, FieldInvalidEmoji, "the emoji must be exactly one permitted Unicode grapheme cluster")
}

// ReactionLimitReached is the 400 of a new symbol on a message that already has
// reaction.max_distinct_per_message distinct symbols.
func ReactionLimitReached() error {
	return fieldError(PointerPathEmoji, FieldLimitReached, "the message already has reaction.max_distinct_per_message distinct emoji")
}

// PinLimitReached is the 400 of a pin in a thread that already holds pin.max_per_thread pins
// (empty pointer).
func PinLimitReached() error {
	return fieldError("", FieldLimitReached, "the thread already holds pin.max_per_thread pins")
}

// ReactionGroup is one symbol of the reactions of a message: how many principals reacted and
// when the first did.
type ReactionGroup struct {
	Emoji   string
	Count   int
	FirstAt time.Time
}

// OrderSummary returns groups ordered by the earliest reaction of each symbol, then by symbol
// (bytewise), the order of Message.reactions (brief M1-59 design note 7). groups stays
// unchanged.
func OrderSummary(groups []ReactionGroup) []ReactionGroup {
	out := slices.Clone(groups)
	slices.SortStableFunc(out, func(a, b ReactionGroup) int {
		if c := a.FirstAt.Compare(b.FirstAt); c != 0 {
			return c
		}
		return strings.Compare(a.Emoji, b.Emoji)
	})
	return out
}

// OwnInOrder returns the symbols of ordered that are in own, in the order of ordered
// (Message.my_reactions); never nil.
func OwnInOrder(ordered []ReactionGroup, own map[string]bool) []string {
	out := []string{}
	for _, g := range ordered {
		if own[g.Emoji] {
			out = append(out, g.Emoji)
		}
	}
	return out
}

// ReactionKey is the keyset position of listReactions (brief M1-59 design note 8): creation
// time, principal and symbol, all ascending.
type ReactionKey struct {
	At          time.Time `json:"at"`
	PrincipalID ids.ID    `json:"p"`
	Emoji       string    `json:"e"`
}

// Valid reports a complete position; a decoded cursor without one is cursor_invalid.
func (k ReactionKey) Valid() bool { return !k.At.IsZero() && !k.PrincipalID.IsZero() && k.Emoji != "" }

// CursorInvalid is the 400 of a cursor that decodes but holds no valid position.
func CursorInvalid() error { return errs.New(errs.CodeCursorInvalid, "") }
