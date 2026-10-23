// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// The field codes of the card rules (M2-15 design note 6 a).
const (
	fieldRequired = "required"
	fieldTooLong  = "too_long"
)

// cardRules applies the body rules of a card after the decoder (M2-15 design note 6 a) to the kind
// of the request and returns whether it is a card: components require kind card, a card requires
// a component, and only agents and apps send cards (people: invalid; M2-25).
// run_id follows the answer rules of M2-19 for every kind; on a card it also binds the components.
func cardRules(a actor.Actor, kind string, cmd messaging.CreateMessageCmd) (bool, error) {
	card := kind == string(domain.KindCard)
	switch {
	case len(cmd.Components) > 0 && !card:
		return false, cardField("/kind", domain.FieldInvalid, "components need kind card")
	case card && len(cmd.Components) == 0:
		return false, cardField("/components", fieldRequired, "a card needs at least one component")
	}
	if !card {
		return false, nil
	}
	switch a.Kind {
	case actor.KindAgent, actor.KindApp:
		// Apps send cards like agents (M2-25 design note 6); their components never trigger an
		// agent (R-AZ-18).
		return true, nil
	case actor.KindUser, actor.KindHost, actor.KindAnonymous:
	}
	return false, cardField("/components", domain.FieldInvalid, "only agents and apps send cards")
}

// cardField is a validation_failed with one field error.
func cardField(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// checkCard checks the components of a card through the source port of runs and the size rule
// (M2-15 design note 6 a, D-M2-15-26): the largest message.updated payload of the card must fit
// sync.event_payload_max_bytes even after the content cut of M1-56 D7, because components cannot
// be cut (04 2.2 rule 2). handles are the mention handles of the content, each of which may become
// a mention. Errors: 400 validation_failed (not_enabled without the port, the findings of Check,
// too_long on /components), the errors of the author lookup.
func (s *Service) checkCard(ctx context.Context, a actor.Actor, cmd messaging.CreateMessageCmd, thread ids.ID) (*messaging.CheckedCard, error) {
	if s.d.Cards == nil {
		return nil, domain.NotEnabled("/components")
	}
	in := messaging.CardInput{AuthorID: a.ID, AuthorKind: string(a.Kind), ThreadID: thread, Components: cmd.Components, Now: s.now()}
	if cmd.RunID != nil {
		in.RunID = *cmd.RunID
	}
	checked, err := s.d.Cards.Check(ctx, in)
	if err != nil {
		return nil, err
	}
	if !s.cardFits(string(a.Kind), in.RunID, checked.Components, in.Now) {
		return nil, cardField("/components", fieldTooLong, "the card does not fit sync.event_payload_max_bytes")
	}
	return &checked, nil
}

// cardFits reports whether the largest message.updated payload a card with the components comps
// can reach fits sync.event_payload_max_bytes with an empty content and content_truncated (the
// content cut of M1-56 D7 can shorten everything else; components cannot be cut, 04 2.2 rule 2). The reserve is built
// from configured bounds only, never from a value the card has now (D-M2-15-28): the author of
// kind authorKind with the longest handle, display name (every character in its longest JSON
// form) and an avatar, message.max_mentions mentions, reply_to_message_id, edited_at, pinned,
// reaction.max_distinct_per_message reactions of reaction.emoji_max_bytes each, every ID, number
// and time in its longest form, and on every component run_id when bound, state disabled,
// expires_at, used_by and used_at (D-M2-15-26). Only comps and runID, which no later change of
// the card's message can grow, come from the card. A card that fits here keeps fitting through
// clicks, reactions, pins, edits, renames of its author, expiry and the end of its run.
func (s *Service) cardFits(authorKind string, runID ids.ID, comps []messaging.Component, now time.Time) bool {
	placeholder := ids.New()
	author := messaging.PrincipalRef{
		ID: placeholder, Kind: authorKind, Handle: strings.Repeat("x", s.cfg.HandleMaxChars),
		DisplayName: strings.Repeat(longestJSONRune, s.cfg.DisplayNameMaxChars), AvatarFileID: placeholder,
	}
	m := messaging.Message{
		ID: placeholder, ThreadID: placeholder, ChannelID: placeholder, ThreadSeq: maxSafeInteger, Author: author, ClientMsgID: placeholder,
		Origin: string(domain.OriginAgent), Kind: string(domain.KindCard), ContentFormat: domain.ContentFormat, Version: maxSafeInteger,
		CreatedAt: now, EditedAt: now, ReplyToID: placeholder, Pinned: true, MentionIDs: make([]ids.ID, s.cfg.MaxMentions),
		Reactions: make([]messaging.ReactionCount, s.cfg.MaxDistinctReactions), MyReactions: []string{},
	}
	for i := range m.MentionIDs {
		m.MentionIDs[i] = placeholder
	}
	for i := range m.Reactions {
		m.Reactions[i] = messaging.ReactionCount{Emoji: strings.Repeat("x", s.cfg.EmojiMaxBytes), Count: maxSafeInteger}
	}
	m.Components = make([]messaging.Component, len(comps))
	for i, c := range comps {
		c.ID, c.Version, c.State = placeholder, maxSafeInteger, longestComponentState
		c.ExpiresAt, c.UsedAt, c.UsedBy = now, now, placeholder
		if !runID.IsZero() {
			c.RunID = runID
		}
		m.Components[i] = c
	}
	// The cut payload of eventPayload carries content_truncated, which messageBody never sets.
	body := messageBody(m)
	body.ContentTruncated = true
	p, err := payload(body)
	return err == nil && len(p) < s.cfg.EventPayloadMaxBytes
}

// longestJSONRune is a character of the longest JSON form a display name can carry: U+2028 is
// written as the six bytes \u2028, more than any character in UTF-8 (at most four bytes).
const longestJSONRune = "\u2028"

// longestComponentState is the longest value of schema ComponentState (active, used, expired,
// disabled).
const longestComponentState = "disabled"

// CardFits implements messaging.Messages (D-M2-15-26): whether the stored card messageID, with its
// components as they are now in the ambient unit of work, still fits the reserve of cardFits;
// updateComponent calls it after a new definition is written. Errors: 400 validation_failed
// too_long on /definition when it does not fit; internal for a missing or deleted card.
func (s *Service) CardFits(ctx context.Context, messageID ids.ID) error {
	cur, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil {
		return err
	}
	if !found || !cur.DeletedAt.IsZero() {
		return errs.New(errs.CodeInternalError, "messaging: a size check of a missing or deleted card")
	}
	v, err := s.base(ctx, cur)
	if err != nil {
		return err
	}
	list := []messaging.Message{v}
	if err := s.attachComponents(ctx, list); err != nil {
		return err
	}
	if !s.cardFits(v.Author.Kind, ids.ID{}, list[0].Components, s.now()) {
		return cardField("/definition", fieldTooLong, "the card does not fit sync.event_payload_max_bytes")
	}
	return nil
}

// maxSafeInteger is the largest integer the API carries (2^53 - 1, 06 section 2), the longest
// form of a sequence or version number in a payload.
const maxSafeInteger = 1<<53 - 1

// attachComponents fills the components of the live cards of list with one ComponentReader call
// for the whole list (M2-15 design note 7); every other message, tombstones included, keeps none.
func (s *Service) attachComponents(ctx context.Context, list []messaging.Message) error {
	var cards []ids.ID
	for i := range list {
		list[i].Components = []messaging.Component{}
		if list[i].Kind == string(domain.KindCard) && list[i].DeletedAt.IsZero() {
			cards = append(cards, list[i].ID)
		}
	}
	if len(cards) == 0 || s.d.Components == nil {
		return nil
	}
	got, err := s.d.Components.ForMessages(ctx, cards)
	if err != nil {
		return err
	}
	for i := range list {
		if c, ok := got[list[i].ID]; ok && list[i].DeletedAt.IsZero() {
			list[i].Components = c
		}
	}
	return nil
}

// LockCard implements messaging.Messages (M2-15 design note 2 b): the row of the card locked in the
// unit of work of runs, without a visibility predicate (runs authorized on the card's path) and
// without an authorization. Errors: internal outside a unit of work; not_found for an unknown
// message.
func (s *Service) LockCard(ctx context.Context, messageID ids.ID) (messaging.CardFacts, error) {
	if !txn.InUnit(ctx) {
		return messaging.CardFacts{}, errs.New(errs.CodeInternalError, "messaging: LockCard runs only inside a unit of work")
	}
	m, found, err := s.d.Store.LockCard(ctx, messageID)
	if err != nil {
		return messaging.CardFacts{}, err
	}
	if !found {
		return messaging.CardFacts{}, errs.New(errs.CodeNotFound, "")
	}
	info, err := s.d.Threads.Info(ctx, m.ThreadID)
	if err != nil {
		return messaging.CardFacts{}, err
	}
	author, err := s.author(ctx, m.AuthorID)
	if err != nil {
		return messaging.CardFacts{}, err
	}
	return messaging.CardFacts{
		MessageID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, SpaceID: info.SpaceID, AuthorID: m.AuthorID,
		AuthorKind: author.Kind, Kind: string(m.Kind), Deleted: !m.DeletedAt.IsZero(), Version: m.Version,
	}, nil
}

// Authors implements messaging.Messages (M2-25): the authors of messageIDs from the store, without
// an authorization.
func (s *Service) Authors(ctx context.Context, messageIDs []ids.ID) (map[ids.ID]ids.ID, error) {
	return s.d.Store.Authors(ctx, messageIDs)
}

// ComponentsChanged implements messaging.Messages (M2-15 design note 2 b): the version of the card
// locked by LockCard rises by one and message.updated is appended with the stored representation,
// its components read now (05 section 6 "component state"; one producer of message.updated, 02
// 4.7). Errors: internal outside a unit of work, for a missing or deleted card.
func (s *Service) ComponentsChanged(ctx context.Context, messageID ids.ID) (int64, error) {
	if !txn.InUnit(ctx) {
		return 0, errs.New(errs.CodeInternalError, "messaging: ComponentsChanged runs only inside a unit of work")
	}
	cur, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil {
		return 0, err
	}
	if !found || !cur.DeletedAt.IsZero() {
		return 0, errs.New(errs.CodeInternalError, "messaging: a component change of a missing or deleted card")
	}
	row, err := s.d.Store.RaiseVersion(ctx, messageID)
	if err != nil {
		return 0, err
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return 0, err
	}
	v, err := s.base(ctx, row)
	if err != nil {
		return 0, err
	}
	list := []messaging.Message{v}
	if err := s.decorate(ctx, []ids.ID{row.ChannelID}, list); err != nil {
		return 0, err
	}
	if err := s.emitMessage(ctx, authorize.EventMessageUpdated, info, list[0], false); err != nil {
		return 0, err
	}
	return row.Version, nil
}

// componentJSON is the schema Component in an event and in a version_conflict body: optional
// members left out, never null (ADR 0053).
type componentJSON struct {
	ID                 string          `json:"id"`
	CustomID           string          `json:"custom_id"`
	Type               string          `json:"type"`
	Definition         json.RawMessage `json:"definition"`
	Position           int             `json:"position"`
	SingleUse          bool            `json:"single_use"`
	RequiredCapability string          `json:"required_capability,omitempty"`
	RestrictedTo       string          `json:"restricted_to"`
	RunID              string          `json:"run_id,omitempty"`
	State              string          `json:"state"`
	ExpiresAt          string          `json:"expires_at,omitempty"`
	UsedBy             string          `json:"used_by,omitempty"`
	UsedAt             string          `json:"used_at,omitempty"`
	Version            int64           `json:"version"`
}

// componentsBody converts the components of a message; never nil.
func componentsBody(list []messaging.Component) []componentJSON {
	out := make([]componentJSON, 0, len(list))
	for _, c := range list {
		def := c.Definition
		if len(def) == 0 {
			def = json.RawMessage("{}")
		}
		out = append(out, componentJSON{
			ID: c.ID.String(), CustomID: c.CustomID, Type: c.Type, Definition: def, Position: c.Position, SingleUse: c.SingleUse,
			RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo, RunID: optionalID(c.RunID), State: c.State,
			ExpiresAt: stamp(c.ExpiresAt), UsedBy: optionalID(c.UsedBy), UsedAt: stamp(c.UsedAt), Version: c.Version,
		})
	}
	return out
}

// cardPrepared is the card part of a prepared request.
type cardPrepared struct {
	checked *messaging.CheckedCard
}

// attach stores the components of the card row through the source port inside the unit of work
// and returns them (M2-15 design note 6 b and d).
func (s *Service) attach(ctx context.Context, row domain.Message, c cardPrepared) ([]messaging.Component, error) {
	if c.checked == nil {
		return []messaging.Component{}, nil
	}
	return s.d.Cards.Attach(ctx, row.ID, *c.checked)
}
