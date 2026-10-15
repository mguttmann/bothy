// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Values of the filter-mismatch record (signals.md "Logs"; the keys equal the constants of
// platform/otel/attr, which the application layer may not import).
// reasonFilterMismatch is shared with the mention list (mentions.go, INT-W22/2).
const (
	msgFilterMismatch = "messaging: a message of a visible channel failed message.read (resource kind message)"
	keyAction         = "bothy.action"
	keyReason         = "bothy.reason"
)

// requestPrincipal returns the principal of ctx; anonymous when the chain set none.
func requestPrincipal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// historyRequest returns the anchors of q as the query parameters they came from, so the port
// validates exactly like the REST handler (domain.ValidateHistoryRequest).
func historyRequest(q messaging.HistoryQuery) (before, after, around *int64) {
	v := q.Anchor
	switch q.Mode {
	case messaging.HistoryBefore:
		return &v, nil, nil
	case messaging.HistoryAfter:
		return nil, &v, nil
	case messaging.HistoryAround:
		return nil, nil, &v
	case messaging.HistoryNewest:
	}
	return nil, nil, nil
}

// History implements messaging.Messages (listThreadMessages, brief M1-57 note 2): the anchors
// without any lookup; the thread through chat and thread.read on it (a missing and an invisible
// thread take the same path); the visible channels of message.read; the one statement of the
// mode; message.read per item in filter mode, a refused item dropped, logged and counted as
// filter_mismatch; each item in the representation of the port, content cut at
// message.list_content_max_bytes.
func (s *Service) History(ctx context.Context, q messaging.HistoryQuery) (messaging.HistoryPage, error) {
	a := requestPrincipal(ctx)
	mode, anchor, err := domain.ValidateHistoryRequest(historyRequest(q))
	if err != nil {
		return messaging.HistoryPage{}, err
	}
	info, err := s.readThread(ctx, a, q.ThreadID)
	if err != nil {
		return messaging.HistoryPage{}, err
	}
	scope, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead)
	if err != nil {
		return messaging.HistoryPage{}, err
	}
	limit := q.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	rows, err := s.d.Store.History(ctx, HistoryRead{ThreadID: info.ID, Visible: scope.ChannelIDs, Mode: mode, Anchor: anchor, RowLimit: limit + 1})
	if err != nil {
		return messaging.HistoryPage{}, err
	}
	w := domain.PageFlags(mode, len(rows.Below), len(rows.Above), limit, rows.Opposite)
	page := slices.Clone(rows.Below[:w.Below])
	slices.Reverse(page)
	page = append(page, rows.Above[:w.Above]...)
	out := messaging.HistoryPage{Items: make([]messaging.Message, 0, len(page)), HasMoreBefore: w.HasMoreBefore, HasMoreAfter: w.HasMoreAfter}
	authors := map[ids.ID]messaging.PrincipalRef{}
	fctx := authorize.Filter(ctx)
	for _, m := range page {
		ok, err := s.allowed(fctx, a, authorize.ActionMessageRead, historyResource(m))
		if err != nil {
			return messaging.HistoryPage{}, err
		}
		if !ok {
			s.historyFilterMismatch(ctx)
			continue
		}
		v, err := s.listed(ctx, m, authors)
		if err != nil {
			return messaging.HistoryPage{}, err
		}
		out.Items = append(out.Items, v)
	}
	if err := s.decorate(ctx, scope.ChannelIDs, out.Items); err != nil {
		return messaging.HistoryPage{}, err
	}
	return out, nil
}

// Get implements messaging.Messages (getMessage, brief note 7): the message read with the visible
// channels of message.read, then message.read on it; a missing and an invisible message give the
// same not_found. When the visible read misses, an existing message is still decided by
// message.read, so an app with a grant on the channel but without messages:read gets 403
// scope_missing as from listThreadMessages (06 section 4 item 4, 07 section 2.10), and a person
// without access the same not_found as for an unknown id. The content is always whole, a
// tombstone included.
func (s *Service) Get(ctx context.Context, messageID ids.ID) (messaging.Message, error) {
	m, visible, err := s.readMessage(ctx, messageID)
	if err != nil {
		return messaging.Message{}, err
	}
	return s.stored(ctx, m, visible)
}

// readMessage reads the message id for message.read the way Get does and returns it with the
// visible channels of the principal of ctx (also listReactions, M1-59).
func (s *Service) readMessage(ctx context.Context, messageID ids.ID) (domain.Message, []ids.ID, error) {
	a := requestPrincipal(ctx)
	scope, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead)
	if err != nil {
		return domain.Message{}, nil, err
	}
	m, visible, err := s.d.Store.VisibleMessage(ctx, messageID, scope.ChannelIDs)
	if err != nil {
		return domain.Message{}, nil, err
	}
	if !visible {
		return domain.Message{}, nil, s.notVisible(ctx, a, messageID)
	}
	if err := s.check(ctx, a, authorize.ActionMessageRead, historyResource(m)); err != nil {
		return domain.Message{}, nil, err
	}
	return m, scope.ChannelIDs, nil
}

// notVisible is the error of a message outside the visible channels: the refusal of message.read
// for an existing message (not_found without access, scope_missing for an app with a grant), the
// same not_found for an unknown one, and not_found when message.read allows what the visible read
// did not return, so the answer stays closed.
func (s *Service) notVisible(ctx context.Context, a actor.Actor, id ids.ID) error {
	m, found, err := s.d.Store.Message(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	if err := s.check(ctx, a, authorize.ActionMessageRead, historyResource(m)); err != nil {
		return err
	}
	return invisible()
}

// readThread loads the thread id and authorizes thread.read on it; an unknown thread is checked
// by its id alone and then answers the same not_found as an invisible one (02 section 9.2).
func (s *Service) readThread(ctx context.Context, a actor.Actor, id ids.ID) (chat.ThreadInfo, error) {
	info, err := s.d.Threads.Info(ctx, id)
	found := err == nil
	if err != nil && errs.KindOf(err) != errs.KindNotFound {
		return chat.ThreadInfo{}, err
	}
	r := authorize.Resource{Kind: authorize.ResourceThread, ID: id}
	if found {
		r = threadResource(info)
	}
	if err := s.check(ctx, a, authorize.ActionThreadRead, r); err != nil {
		return chat.ThreadInfo{}, err
	}
	if !found {
		return chat.ThreadInfo{}, invisible()
	}
	return info, nil
}

// historyResource describes m for message.read in History and Get: its thread as path hint and
// its author as owner (named apart from messageResource of the mention list, INT-W22/2).
func historyResource(m domain.Message) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceMessage, ID: m.ID, ThreadID: m.ThreadID, OwnerID: m.AuthorID}
}

// historyFilterMismatch records a listed message that the visible predicate let through and message.read
// refused: a programming error between Scope and Authorize (07 section 7.4 rule 3). The record has
// only the action and the reason, never an id or content.
func (s *Service) historyFilterMismatch(ctx context.Context) {
	s.d.Log.LogAttrs(ctx, slog.LevelError, msgFilterMismatch,
		slog.String(keyAction, string(authorize.ActionMessageRead)), slog.String(keyReason, reasonFilterMismatch))
	if s.d.Errors != nil {
		s.d.Errors.CountError(ctx, reasonFilterMismatch)
	}
}

// listed returns the list representation of m: the representation of Get with content_md cut at
// message.list_content_max_bytes. authors caches the authors of one page.
func (s *Service) listed(ctx context.Context, m domain.Message, authors map[ids.ID]messaging.PrincipalRef) (messaging.Message, error) {
	author, ok := authors[m.AuthorID]
	if !ok {
		var err error
		if author, err = s.author(ctx, m.AuthorID); err != nil {
			return messaging.Message{}, err
		}
		authors[m.AuthorID] = author
	}
	var mentions []ids.ID
	if m.DeletedAt.IsZero() {
		var err error
		if mentions, err = s.d.Store.Mentions(ctx, m.ID); err != nil {
			return messaging.Message{}, err
		}
	}
	v := view(m, author, mentions)
	v.ContentMD, v.ContentTruncated = domain.CutForList(v.ContentMD, s.cfg.ListContentMaxBytes)
	return v, nil
}
