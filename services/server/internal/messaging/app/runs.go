// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// The two ports of runs (M2-05) write inside the unit of work of their caller and never authorize:
// runs decided the acceptance or the transition. Neither takes the thread lock for a change of a
// message: runs holds its runs row and then updates the message, and both orders of the README
// take runs before messages.

// SetTriggerState implements messaging.Messages (M2-05, 09 section 6, brief note 7): one UPDATE
// that writes only a different state or reason, then message.updated with the stored
// representation (a tombstone keeps its form). No row is read back as the reason: an unknown
// message or one whose origin is not human is internal, the same state and reason is a no-op
// without an event (D-M2-05-17).
func (s *Service) SetTriggerState(ctx context.Context, cmd messaging.SetTriggerStateCmd) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "messaging: SetTriggerState runs only inside a unit of work")
	}
	if err := domain.CheckTriggerState(cmd.State, cmd.Reason); err != nil {
		return err
	}
	row, changed, err := s.d.Store.SetTriggerState(ctx, cmd.MessageID, cmd.State, cmd.Reason, s.now())
	if err != nil {
		return err
	}
	if !changed {
		m, found, err := s.d.Store.Message(ctx, cmd.MessageID)
		if err != nil {
			return err
		}
		if !found || m.Origin != domain.OriginHuman {
			return errs.New(errs.CodeInternalError, "messaging: a trigger state for a missing message or one not written by a person")
		}
		return nil
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return err
	}
	v, err := s.base(ctx, row)
	if err != nil {
		return err
	}
	// The reactions and the pin as the edit path shows them; the event never carries
	// my_reactions, so the channel of the message is the only scope the decoration needs.
	list := []messaging.Message{v}
	if err := s.decorate(ctx, []ids.ID{row.ChannelID}, list); err != nil {
		return err
	}
	return s.emitMessage(ctx, authorize.EventMessageUpdated, info, list[0], false)
}

// CreateRunAnchor implements messaging.Messages (M2-05, 09 section 5.4, brief note 7): the next
// thread_seq through chat (no reopen), the anchor row of domain.RunAnchor in the
// instance language (origin agent, D-M3-13-1), message.created without the client_msg_id echo,
// then MessagePosted with Kind run.
func (s *Service) CreateRunAnchor(ctx context.Context, cmd messaging.RunAnchorCmd) (messaging.AnchorRef, error) {
	if !txn.InUnit(ctx) {
		return messaging.AnchorRef{}, errs.New(errs.CodeInternalError, "messaging: CreateRunAnchor runs only inside a unit of work")
	}
	a := actor.Actor{ID: cmd.AgentID, Kind: actor.KindAgent}
	// chat refuses a caller other than the principal of the request, and the acceptance runs in
	// the unit of the person's message; without a reopen chat uses the caller for nothing else,
	// so the principal of the unit takes the number, and the agent only outside a request.
	caller := a
	if p, ok := actor.FromContext(ctx); ok && !p.ID.IsZero() {
		caller = p
	}
	appended, err := s.d.Threads.AppendToThread(ctx, caller, chat.AppendCmd{ThreadID: cmd.ThreadID})
	if err != nil {
		return messaging.AnchorRef{}, err
	}
	info := appended.Thread
	lang, err := s.d.Directory.InstanceSearchLanguage(ctx)
	if err != nil {
		return messaging.AnchorRef{}, err
	}
	anchor := domain.RunAnchor(cmd.RunID, info.ID, info.ChannelID, cmd.AgentID, appended.Seq)
	anchor.CreatedAt, anchor.SearchLanguage = s.now(), lang
	row, err := s.d.Store.InsertMessage(ctx, anchor)
	if errors.Is(err, ErrClientMsgIDTaken) {
		// A fresh server UUIDv7 never repeats; a new error without the cause keeps it off the
		// replay path of the caller's createMessage, which matches ErrClientMsgIDTaken.
		return messaging.AnchorRef{}, errs.New(errs.CodeInternalError, "messaging: the client_msg_id of a run anchor is taken")
	}
	if err != nil {
		return messaging.AnchorRef{}, err
	}
	s.countCreated(ctx, row)
	author, err := s.author(ctx, cmd.AgentID)
	if err != nil {
		return messaging.AnchorRef{}, err
	}
	m := view(row, author, nil)
	if err := s.emitCreated(ctx, info, m, false); err != nil {
		return messaging.AnchorRef{}, err
	}
	if err := txn.Publish(ctx, s.posted(a, info, m, false)); err != nil {
		return messaging.AnchorRef{}, err
	}
	return messaging.AnchorRef{MessageID: row.ID, ThreadSeq: row.ThreadSeq}, nil
}

// PrincipalRefs implements messaging.Messages (M2-16): the public views of principals from the
// directory of identity, without an authorization.
func (s *Service) PrincipalRefs(ctx context.Context, principalIDs []ids.ID) (map[ids.ID]messaging.PrincipalRef, error) {
	ps, err := s.d.Directory.Principals(ctx, principalIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]messaging.PrincipalRef, len(ps))
	for id, p := range ps {
		out[id] = messaging.PrincipalRef{ID: p.ID, Kind: string(p.Kind), Handle: p.Handle, DisplayName: p.DisplayName, AvatarFileID: p.AvatarFileID}
	}
	return out, nil
}

// TriggerMessageOf implements messaging.Messages (M2-08): the stored row, its mentions and its
// author, without content and without an authorization; runs decides with agent.invoke.
func (s *Service) TriggerMessageOf(ctx context.Context, messageID ids.ID) (messaging.TriggerMessage, error) {
	m, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil {
		return messaging.TriggerMessage{}, err
	}
	if !found {
		return messaging.TriggerMessage{}, errs.New(errs.CodeNotFound, "")
	}
	mentions, err := s.d.Store.Mentions(ctx, m.ID)
	if err != nil {
		return messaging.TriggerMessage{}, err
	}
	author, err := s.author(ctx, m.AuthorID)
	if err != nil {
		return messaging.TriggerMessage{}, err
	}
	return messaging.TriggerMessage{
		ID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, Author: author, Origin: string(m.Origin), Kind: string(m.Kind),
		MentionedIDs: mentions, TriggerState: m.TriggerState, TriggerStateReason: m.TriggerStateReason,
		ThreadSeq: m.ThreadSeq, Deleted: !m.DeletedAt.IsZero(),
	}, nil
}

// PrincipalOf implements messaging.Messages (M2-15): the PrincipalRef of a principal as the author
// of a message shows it, without an authorization; runs puts it into agent.invocation.
func (s *Service) PrincipalOf(ctx context.Context, principalID ids.ID) (messaging.PrincipalRef, error) {
	return s.author(ctx, principalID)
}
