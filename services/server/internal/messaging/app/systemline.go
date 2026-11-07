// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// AppendSystemLine implements messaging.Messages (M2-12 design note 12): the line of M1-56 D6
// with the given code and author, written like the reopen line of nextSeq but through
// AppendToThread without ReopenIfClosed, because a system line never reopens a closed thread
// (R-DM-42). message.created goes without the client_msg_id echo, then MessagePosted with Kind
// system; the acceptance of runs ignores that kind (SR4).
func (s *Service) AppendSystemLine(ctx context.Context, l messaging.SystemLine) (messaging.Message, error) {
	if !txn.InUnit(ctx) {
		return messaging.Message{}, errs.New(errs.CodeInternalError, "messaging: AppendSystemLine runs only inside a unit of work")
	}
	if !domain.IsSystemCode(l.Code) {
		return messaging.Message{}, errs.New(errs.CodeInternalError, "messaging: a system line with a code that is not a system_code")
	}
	author, err := s.author(ctx, l.AuthorID)
	if err != nil {
		return messaging.Message{}, err
	}
	a := actor.Actor{ID: l.AuthorID, Kind: actor.Kind(author.Kind)}
	// As in CreateRunAnchor: chat refuses a caller other than the principal of the request, and
	// without a reopen it uses the caller for nothing else, so the principal of the unit takes the
	// number, and the author only outside a request.
	caller := a
	if p, ok := actor.FromContext(ctx); ok && !p.ID.IsZero() {
		caller = p
	}
	appended, err := s.d.Threads.AppendToThread(ctx, caller, chat.AppendCmd{ThreadID: l.ThreadID})
	if err != nil {
		return messaging.Message{}, err
	}
	info := appended.Thread
	line := domain.SystemLine(l.Code, info.ID, info.ChannelID, l.AuthorID, appended.Seq)
	line.CreatedAt = s.now()
	// Origin system takes the instance language, whoever caused the line (14 3.1, D-M3-13-1).
	if line.SearchLanguage, err = s.d.Directory.InstanceSearchLanguage(ctx); err != nil {
		return messaging.Message{}, err
	}
	row, err := s.d.Store.InsertMessage(ctx, line)
	if errors.Is(err, ErrClientMsgIDTaken) {
		// A fresh server UUIDv7 never repeats; a new error without the cause keeps it off the
		// replay path of a createMessage in the same unit, which matches ErrClientMsgIDTaken.
		return messaging.Message{}, errs.New(errs.CodeInternalError, "messaging: the client_msg_id of a system line is taken")
	}
	if err != nil {
		return messaging.Message{}, err
	}
	s.countCreated(ctx, row)
	m := view(row, author, nil)
	if err := s.emitCreated(ctx, info, m, false); err != nil {
		return messaging.Message{}, err
	}
	if err := txn.Publish(ctx, s.posted(a, info, m, false)); err != nil {
		return messaging.Message{}, err
	}
	return m, nil
}

// AppendSystemNote writes the note of a wake-up end or an ended chain of autonomous inputs
// (docs/spec/09-agents.md 5.8.4, 5.8.6, AR-04) inside the caller's unit of work: a message with
// origin system and kind text whose content names the label and the reason, silent, authored by
// the agent, appended like a system line without reopening a closed thread. It never triggers
// (origin system, 09 3.2) and creates no hint (13 section 2: origin system); message.created and
// MessagePosted follow as for every message. The text is the server's own English line (ruling
// D-AR-04-6: 03 messages.system_code is a closed list of the core contract that has no code with
// parameters). It returns the id of the note.
//
// Errors: internal outside a unit of work or for an empty or oversized text; the errors of chat and
// the store.
//
// Example:
//
//	id, err := msgs.AppendSystemNote(ctx, thread, agent, "Wake-up 'CI of #123' expired: its deadline passed before it could fire")
func (s *Service) AppendSystemNote(ctx context.Context, threadID, authorID ids.ID, text string) (ids.ID, error) {
	if !txn.InUnit(ctx) {
		return ids.ID{}, errs.New(errs.CodeInternalError, "messaging: AppendSystemNote runs only inside a unit of work")
	}
	if text == "" || len(text) > s.cfg.MaxContentBytes {
		return ids.ID{}, errs.New(errs.CodeInternalError, "messaging: a system note is empty or too long")
	}
	author, err := s.author(ctx, authorID)
	if err != nil {
		return ids.ID{}, err
	}
	a := actor.Actor{ID: authorID, Kind: actor.Kind(author.Kind)}
	caller := a
	if p, ok := actor.FromContext(ctx); ok && !p.ID.IsZero() {
		caller = p
	}
	appended, err := s.d.Threads.AppendToThread(ctx, caller, chat.AppendCmd{ThreadID: threadID})
	if err != nil {
		return ids.ID{}, err
	}
	info := appended.Thread
	note := domain.SystemNote(text, info.ID, info.ChannelID, authorID, appended.Seq)
	note.CreatedAt = s.now()
	if note.SearchLanguage, err = s.d.Directory.InstanceSearchLanguage(ctx); err != nil {
		return ids.ID{}, err
	}
	row, err := s.d.Store.InsertMessage(ctx, note)
	if errors.Is(err, ErrClientMsgIDTaken) {
		return ids.ID{}, errs.New(errs.CodeInternalError, "messaging: the client_msg_id of a system note is taken")
	}
	if err != nil {
		return ids.ID{}, err
	}
	s.countCreated(ctx, row)
	m := view(row, author, nil)
	if err := s.emitCreated(ctx, info, m, false); err != nil {
		return ids.ID{}, err
	}
	if err := txn.Publish(ctx, s.posted(a, info, m, false)); err != nil {
		return ids.ID{}, err
	}
	return m.ID, nil
}
