// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Reactions implements messaging.Reactions over the use cases (Service.Reactions).
type Reactions struct{ *Service }

// Reactions returns the incoming port of reactions (M1-59).
func (s *Service) Reactions() Reactions { return Reactions{s} }

// reactionsList is the list name of the listReactions cursors.
const reactionsList = "reactions"

// symbol normalises raw with the symbol rule; a refusal is 400 invalid_emoji at pointer. It runs
// before any lookup, so it reveals nothing (brief design note 2).
func (s *Service) symbol(raw, pointer string) (string, error) {
	sym, err := s.d.Symbols.Normalize(raw, s.cfg.EmojiMaxBytes)
	if err != nil {
		return "", domain.InvalidEmoji(pointer)
	}
	return sym, nil
}

// reactionResource describes the caller's reaction on m for reaction.remove: the message's path
// and the caller as owner (brief design note 1).
func reactionResource(m domain.Message, owner ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceReaction, ID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, OwnerID: owner}
}

// Add implements messaging.Reactions (brief design notes 2, 4 and 6): the symbol, the message
// with reaction.add, then in one unit of work the row lock, 410, the bound of distinct symbols and
// the idempotent insert with reaction.added. messages.version and trigger_state stay unchanged.
func (s Reactions) Add(ctx context.Context, messageID ids.ID, emoji string) error {
	sym, err := s.symbol(emoji, domain.PointerPathEmoji)
	if err != nil {
		return err
	}
	a := caller(ctx)
	m, visible, err := s.loadMessage(ctx, a, authorize.ActionReactionAdd, messageID)
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := s.lockVisible(ctx, m.ID, visible)
		if err != nil {
			return err
		}
		if !row.DeletedAt.IsZero() {
			return errs.New(errs.CodeMessageDeleted, "the message was deleted")
		}
		distinct, ofEmoji, err := s.d.Store.ReactionStats(ctx, row.ID, sym, visible)
		if err != nil {
			return err
		}
		if ofEmoji == 0 && distinct >= s.cfg.MaxDistinctReactions {
			return domain.ReactionLimitReached()
		}
		inserted, err := s.d.Store.InsertReaction(ctx, row.ID, a.ID, sym, s.now(), visible)
		if err != nil || !inserted {
			return err
		}
		return s.emitReaction(ctx, authorize.EventReactionAdded, row, a, sym, ofEmoji+1)
	})
}

// Remove implements messaging.Reactions (brief design note 5): only the caller's own row; a
// deleted message or no row is nil without an event.
func (s Reactions) Remove(ctx context.Context, messageID ids.ID, emoji string) error {
	sym, err := s.symbol(emoji, domain.PointerPathEmoji)
	if err != nil {
		return err
	}
	a := caller(ctx)
	m, visible, err := s.loadAs(ctx, a, authorize.ActionReactionRemove, messageID, func(m domain.Message) authorize.Resource {
		return reactionResource(m, a.ID)
	})
	if err != nil || !m.DeletedAt.IsZero() {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := s.lockVisible(ctx, m.ID, visible)
		if err != nil || !row.DeletedAt.IsZero() {
			return err
		}
		_, ofEmoji, err := s.d.Store.ReactionStats(ctx, row.ID, sym, visible)
		if err != nil {
			return err
		}
		deleted, err := s.d.Store.DeleteReaction(ctx, row.ID, a.ID, sym, visible)
		if err != nil || !deleted {
			return err
		}
		return s.emitReaction(ctx, authorize.EventReactionRemoved, row, a, sym, ofEmoji-1)
	})
}

// reactionJSON is the payload ReactionPayload.
type reactionJSON struct {
	MessageID   string `json:"message_id"`
	ThreadID    string `json:"thread_id"`
	Emoji       string `json:"emoji"`
	PrincipalID string `json:"principal_id"`
	Count       int    `json:"count"`
}

// emitReaction appends typ for the reaction of a with sym on m and count reactions with sym
// after the change: resource the message without a version (04 section 9 item 3: the event
// changes no version of the message), scope space, channel and thread, receive action
// message.read.
func (s *Service) emitReaction(ctx context.Context, typ authorize.EventType, m domain.Message, a actor.Actor, sym string, count int) error {
	p, err := encode(reactionJSON{MessageID: m.ID.String(), ThreadID: m.ThreadID.String(), Emoji: sym, PrincipalID: a.ID.String(), Count: count})
	if err != nil {
		return err
	}
	return s.emitOnMessage(ctx, typ, m, receiveMessageRead, p)
}

// emitOnMessage appends typ with payload p on the message m without a resource version, scope
// space, channel and thread, receive action receive.
func (s *Service) emitOnMessage(ctx context.Context, typ authorize.EventType, m domain.Message, receive string, p []byte) error {
	info, err := s.d.Threads.Info(ctx, m.ThreadID)
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(typ), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: m.ChannelID, ThreadID: m.ThreadID, ResourceKind: resourceMessage, ResourceID: m.ID,
		ReceiveAction: receive, Payload: p,
	})
}

// List implements messaging.Reactions (brief design note 8): the filter symbol before any lookup,
// the message with message.read like Get, the cursor bound to the list, the message and the
// filter, then one keyset page with the principals in one directory read. The reactions belong
// to the one message that passed message.read, so no item needs a check of its own.
func (s Reactions) List(ctx context.Context, q messaging.ReactionListQuery) (messaging.ReactionPage, error) {
	filter := ""
	if q.Emoji != nil {
		var err error
		if filter, err = s.symbol(*q.Emoji, domain.PointerQueryEmoji); err != nil {
			return messaging.ReactionPage{}, err
		}
	}
	m, visible, err := s.readMessage(ctx, q.MessageID)
	if err != nil {
		return messaging.ReactionPage{}, err
	}
	binding := []byte(m.ID.String() + "\x00" + filter)
	read := ReactionRead{MessageID: m.ID, Visible: visible, Emoji: filter, Limit: s.pageSize(q.Limit) + 1}
	if q.Cursor != "" {
		var k domain.ReactionKey
		if err := s.d.Cursors.Decode(reactionsList, binding, q.Cursor, &k); err != nil {
			return messaging.ReactionPage{}, err
		}
		if !k.Valid() {
			return messaging.ReactionPage{}, domain.CursorInvalid()
		}
		read.HasAfter, read.After = true, k
	}
	out := messaging.ReactionPage{Items: []messaging.ReactionView{}}
	if !m.DeletedAt.IsZero() {
		return out, nil
	}
	rows, err := s.d.Store.ListReactions(ctx, read)
	if err != nil {
		return messaging.ReactionPage{}, err
	}
	limit := read.Limit - 1
	more := len(rows) > limit
	rows = rows[:min(len(rows), limit)]
	if out.Items, err = s.reactionViews(ctx, rows); err != nil {
		return messaging.ReactionPage{}, err
	}
	if !more {
		return out, nil
	}
	last := rows[len(rows)-1]
	key := domain.ReactionKey{At: last.CreatedAt, PrincipalID: last.PrincipalID, Emoji: last.Emoji}
	if out.NextCursor, err = s.d.Cursors.Encode(reactionsList, binding, key); err != nil {
		return messaging.ReactionPage{}, errs.Wrap(errs.CodeInternalError, err, "messaging: reaction list cursor")
	}
	return out, nil
}

// reactionViews converts rows with their principals from one directory read.
func (s *Service) reactionViews(ctx context.Context, rows []ReactionRow) ([]messaging.ReactionView, error) {
	out := make([]messaging.ReactionView, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	want := make([]ids.ID, 0, len(rows))
	for _, r := range rows {
		want = append(want, r.PrincipalID)
	}
	found, err := s.d.Directory.Principals(ctx, want)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		p, ok := found[r.PrincipalID]
		if !ok {
			return nil, errs.New(errs.CodeInternalError, "messaging: a reaction without its principal")
		}
		ref := messaging.PrincipalRef{ID: p.ID, Kind: string(p.Kind), Handle: p.Handle, DisplayName: p.DisplayName, AvatarFileID: p.AvatarFileID}
		out = append(out, messaging.ReactionView{Emoji: r.Emoji, Principal: ref, CreatedAt: r.CreatedAt})
	}
	return out, nil
}
