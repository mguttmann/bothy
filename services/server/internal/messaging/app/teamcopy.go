// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// CopyMessage writes the copy a team channel needs inside the caller's unit of work
// (docs/spec/09-agents.md 3.9.2 and 3.9.4, FEAT-TEAM-CHANNEL-SERVER): the content of the source
// message as a new text message of the author in the thread, with the author's own origin (human
// for a person, agent for an agent), silent, without reply, mentions and attachments, appended
// without reopening a closed thread. message.created goes without the client_msg_id echo, then
// MessagePosted with Relay set, which the acceptance of runs never decides as a trigger. It does not
// authorize: the team use case checked the readers of both channels and the trigger first.
//
// Errors: internal outside a unit of work, for a source without text (deleted, a system line, a
// card or an anchor), for a zero SourceVersion or for an author that writes no messages; 409
// conflict for a source whose version is not SourceVersion (edited after the caller's check, red
// team round 8); the errors of chat and the store.
//
// Example:
//
//	c, err := msgs.CopyMessage(ctx, messaging.CopyCmd{SourceMessageID: reply, ThreadID: teamMain, AuthorID: agent})
func (s *Service) CopyMessage(ctx context.Context, cmd messaging.CopyCmd) (messaging.Copied, error) {
	if !txn.InUnit(ctx) {
		return messaging.Copied{}, errs.New(errs.CodeInternalError, "messaging: CopyMessage runs only inside a unit of work")
	}
	src, err := s.copySource(ctx, cmd)
	if err != nil {
		return messaging.Copied{}, err
	}
	author, err := s.author(ctx, cmd.AuthorID)
	if err != nil {
		return messaging.Copied{}, err
	}
	origin, err := domain.OriginFor(author.Kind)
	if err != nil {
		return messaging.Copied{}, err
	}
	a := actor.Actor{ID: cmd.AuthorID, Kind: actor.Kind(author.Kind)}
	// As in AppendSystemNote: chat refuses a caller other than the principal of the request, and
	// without a reopen it uses the caller for nothing else.
	caller := a
	if p, ok := actor.FromContext(ctx); ok && !p.ID.IsZero() {
		caller = p
	}
	appended, err := s.d.Threads.AppendToThread(ctx, caller, chat.AppendCmd{ThreadID: cmd.ThreadID})
	if err != nil {
		return messaging.Copied{}, err
	}
	info := appended.Thread
	c := domain.TeamCopy(src.ContentMD, src.ID, info.ID, info.ChannelID, cmd.AuthorID, origin, appended.Seq)
	c.CreatedAt = s.now()
	if c.SearchLanguage, err = s.searchLanguage(ctx, a); err != nil {
		return messaging.Copied{}, err
	}
	row, err := s.d.Store.InsertMessage(ctx, c)
	if errors.Is(err, ErrClientMsgIDTaken) {
		return messaging.Copied{}, errs.New(errs.CodeInternalError, "messaging: the client_msg_id of a team copy is taken")
	}
	if err != nil {
		return messaging.Copied{}, err
	}
	s.countCreated(ctx, row)
	m := view(row, author, nil)
	if err := s.emitCreated(ctx, info, m, false); err != nil {
		return messaging.Copied{}, err
	}
	ev := s.posted(a, info, m, false)
	ev.Relay = true
	if err := txn.Publish(ctx, ev); err != nil {
		return messaging.Copied{}, err
	}
	return messaging.Copied{ID: m.ID, ThreadSeq: m.ThreadSeq, ContentMD: m.ContentMD, Version: row.Version}, nil
}

// copySource reads the source of a copy and checks it: a text message that exists, at the version
// the caller checked (CopyMessage).
func (s *Service) copySource(ctx context.Context, cmd messaging.CopyCmd) (domain.Message, error) {
	if cmd.SourceVersion <= 0 {
		return domain.Message{}, errs.New(errs.CodeInternalError, "messaging: a copy needs the version of its source")
	}
	src, found, err := s.d.Store.Message(ctx, cmd.SourceMessageID)
	if err != nil {
		return domain.Message{}, err
	}
	if !found || !src.DeletedAt.IsZero() || src.Kind != domain.KindText || src.ContentMD == "" {
		return domain.Message{}, errs.New(errs.CodeInternalError, "messaging: a copy needs a text message that exists")
	}
	if src.Version != cmd.SourceVersion {
		return domain.Message{}, errs.New(errs.CodeConflict, "messaging: the source of a copy changed after its check")
	}
	return src, nil
}
