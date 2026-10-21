// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// reserveCard is a card of two buttons and a select whose first label has n bytes.
func reserveCard(n int) []messaging.Component {
	button := func(custom, label string, pos int) messaging.Component {
		def, _ := json.Marshal(map[string]string{"label": label, "value": custom})
		return messaging.Component{CustomID: custom, Type: "button", Definition: def, Position: pos, SingleUse: true, RestrictedTo: "any", State: "active", Version: 1}
	}
	sel := messaging.Component{CustomID: "pick", Type: "select", Position: 3, RestrictedTo: "run_starter", RequiredCapability: "approve_actions",
		Definition: json.RawMessage(`{"options":[{"label":"One","value":"one"}],"min_values":1,"max_values":1}`), State: "active", Version: 1}
	return []messaging.Component{button("approve", strings.Repeat("l", n), 1), button("deny", "Deny", 2), sel}
}

// reserveRunes are the characters of a random display name or content, the longest JSON forms
// included (U+2028 is escaped to six bytes, a four byte emoji).
var reserveRunes = []rune{'a', 'Z', '<', '&', 'é', '€', '😀', ' '}

// randomText returns a text of at most maxRunes characters of reserveRunes, at least one.
func randomText(r *rand.Rand, maxRunes int) string {
	var b strings.Builder
	for range 1 + r.IntN(maxRunes) {
		b.WriteRune(reserveRunes[r.IntN(len(reserveRunes))])
	}
	return b.String()
}

// randomTime is a time present or zero.
func randomTime(r *rand.Rand, at time.Time) time.Time {
	if r.IntN(2) == 0 {
		return time.Time{}
	}
	return at.Add(time.Duration(r.Int64N(int64(400 * 24 * time.Hour))))
}

// randomID is an id present or zero.
func randomID(r *rand.Rand) ids.ID {
	if r.IntN(2) == 0 {
		return ids.ID{}
	}
	return ids.New()
}

// laterState returns a state the card comps of an agent can reach after it was accepted
// (D-M2-15-28): every member a later message.updated can change takes a random value within its
// configured bound (author renamed, content edited with up to message.max_mentions mentions,
// reply, edit time, pin, up to reaction.max_distinct_per_message reactions, every component in any
// state with expiry and use), the members fixed at creation keep their value. With worst every
// bound takes its longest form.
func laterState(r *rand.Rand, cfg Config, comps []messaging.Component, runID ids.ID, now time.Time, worst bool) messaging.Message {
	pick := func(n int) int {
		if worst {
			return n
		}
		return r.IntN(n + 1)
	}
	m := messaging.Message{
		ID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), ThreadSeq: 1 + r.Int64N(maxSafeInteger), ClientMsgID: ids.New(),
		Author: messaging.PrincipalRef{ID: ids.New(), Kind: "agent", Handle: strings.Repeat("h", 1+r.IntN(cfg.HandleMaxChars)),
			DisplayName: randomText(r, cfg.DisplayNameMaxChars), AvatarFileID: randomID(r)},
		Origin: string(domain.OriginAgent), Kind: string(domain.KindCard), ContentFormat: domain.ContentFormat,
		ContentMD: randomText(r, cfg.MaxContentBytes/4), ReplyToID: randomID(r), Pinned: r.IntN(2) == 0,
		Version: 1 + r.Int64N(maxSafeInteger), CreatedAt: now, EditedAt: randomTime(r, now), MyReactions: []string{},
	}
	if worst {
		m.Author.Handle, m.Author.DisplayName = strings.Repeat("h", cfg.HandleMaxChars), strings.Repeat(" ", cfg.DisplayNameMaxChars)
		m.Author.AvatarFileID, m.ReplyToID, m.Pinned, m.EditedAt = ids.New(), ids.New(), true, now
		m.ThreadSeq, m.Version = maxSafeInteger, maxSafeInteger
	}
	m.MentionIDs = make([]ids.ID, pick(cfg.MaxMentions))
	for i := range m.MentionIDs {
		m.MentionIDs[i] = ids.New()
	}
	m.Reactions = make([]messaging.ReactionCount, pick(cfg.MaxDistinctReactions))
	for i := range m.Reactions {
		emoji := strings.Repeat("😀", 1+r.IntN(cfg.EmojiMaxBytes/4))
		count := 1 + int(r.Int64N(maxSafeInteger))
		if worst {
			emoji, count = strings.Repeat("x", cfg.EmojiMaxBytes), maxSafeInteger
		}
		m.Reactions[i] = messaging.ReactionCount{Emoji: emoji, Count: count}
	}
	states := []string{"active", "used", "expired", "disabled"}
	m.Components = make([]messaging.Component, len(comps))
	for i, c := range comps {
		c.ID, c.RunID, c.Version = ids.New(), runID, 1+r.Int64N(maxSafeInteger)
		c.State, c.ExpiresAt, c.UsedBy, c.UsedAt = states[r.IntN(len(states))], randomTime(r, now), randomID(r), randomTime(r, now)
		if worst {
			c.State, c.ExpiresAt, c.UsedBy, c.UsedAt, c.Version = "disabled", now, ids.New(), now, maxSafeInteger
		}
		m.Components[i] = c
	}
	return m
}

// TestCardReserveCoversEveryLaterState is the model test of the size rule of a card (D-M2-15-26,
// D-M2-15-28): for the largest card cardFits accepts from an agent with a short name and no
// mention, every state the card can reach later, drawn at random from a fixed seed and the state
// with every bound at its longest, still has a message.updated payload below
// sync.event_payload_max_bytes. A reserve that counts a value of the card at its creation instead
// of its bound (the mentions of the content, the author's names) fails here.
func TestCardReserveCoversEveryLaterState(t *testing.T) {
	t.Parallel()
	base := Config{MaxContentBytes: 4096, MaxMentions: 50, EventPayloadMaxBytes: 65536, EmojiMaxBytes: 64, MaxDistinctReactions: 20,
		HandleMaxChars: 32, DisplayNameMaxChars: 64}
	small := base
	small.MaxMentions, small.EventPayloadMaxBytes, small.MaxDistinctReactions, small.EmojiMaxBytes = 4, 16384, 5, 16
	wide := base
	wide.HandleMaxChars, wide.DisplayNameMaxChars, wide.MaxMentions = 200, 500, 200
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	creator := messaging.PrincipalRef{ID: ids.New(), Kind: "agent", Handle: "a", DisplayName: "A"}
	for name, cfg := range map[string]Config{"defaults": base, "small": small, "wide": wide} {
		for _, runID := range []ids.ID{{}, ids.New()} {
			s := &Service{cfg: cfg}
			lo, hi := 0, cfg.EventPayloadMaxBytes
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if s.cardFits(creator.Kind, runID, reserveCard(mid), now) {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			comps := reserveCard(lo)
			r := rand.New(rand.NewPCG(1, uint64(lo))) //nolint:gosec // G404: reproducible states from a fixed seed, not security
			for i := range 400 {
				m := laterState(r, cfg, comps, runID, now, i == 0)
				if _, _, err := s.eventPayload(m); err != nil {
					t.Fatalf("%s, bound run %v, label %d, state %d (mentions %d, display name %d bytes, reactions %d): %v",
						name, !runID.IsZero(), lo, i, len(m.MentionIDs), len(m.Author.DisplayName), len(m.Reactions), err)
				}
			}
		}
	}
}
