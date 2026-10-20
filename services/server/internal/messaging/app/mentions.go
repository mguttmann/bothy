// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Mentions implements messaging.Mentions over the use cases (Service.Mentions).
type Mentions struct{ *Service }

// Mentions returns the incoming port of the mention list and counters.
func (s *Service) Mentions() Mentions { return Mentions{s} }

// mentionsList is the list name of the mention cursors.
const mentionsList = "mentions"

// The log attributes of a filter mismatch (signals catalogue; the log handlers drop other keys).
const (
	attrAction = "bothy.action"
	attrReason = "bothy.reason"
)

// reasonFilterMismatch is the bothy.reason of a list item the SQL filter kept and authorize
// refused (07 section 7.4 rule 3, alarm BothyAuthzError).
const reasonFilterMismatch = "filter_mismatch"

// caller returns the principal of the request in ctx; anonymous when there is none, which
// authorize refuses with 401.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// reasonKindNotUser is the reason of 07 section 4.1 for an agent or app on an action of people.
const reasonKindNotUser = "subject_kind_not_allowed"

// checkPerson authorizes act on the caller as resource user and then refuses an agent or app
// with 403 subject_kind_not_allowed: the self requirement holds for an agent naming itself, but
// own mentions and personal templates belong to people (users), docs/DECISIONS.md M1-60/1 and
// the precedent M1-31/6.
func (s *Service) checkPerson(ctx context.Context, a actor.Actor, act authorize.Action) error {
	if err := s.check(ctx, a, act, userResource(a.ID)); err != nil {
		return err
	}
	if a.Kind != actor.KindUser {
		return errs.New(errs.CodeForbidden, "").WithReason(reasonKindNotUser)
	}
	return nil
}

// userResource describes the person id for the self actions: kind user, owned by itself.
func userResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceUser, ID: id, OwnerID: id}
}

// ListMine implements messaging.Mentions (brief notes 2 and 3): mention.list on the caller, the
// channels of message.read from Scope, then the mentions in keyset order in batches until one
// more than the page size is visible, each item checked with message.read in filter mode. The
// cursor is bound to the list and the caller.
func (s Mentions) ListMine(ctx context.Context, q messaging.MentionListQuery) (messaging.MentionPage, error) {
	a := caller(ctx)
	if err := s.checkPerson(ctx, a, authorize.ActionMentionList); err != nil {
		return messaging.MentionPage{}, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead)
	if err != nil {
		return messaging.MentionPage{}, err
	}
	binding := []byte(a.ID.String())
	mq, err := s.mentionQuery(a, sc.ChannelIDs, binding, q.Cursor)
	if err != nil {
		return messaging.MentionPage{}, err
	}
	limit := s.pageSize(q.Limit)
	out := messaging.MentionPage{Items: []messaging.MentionItem{}}
	if len(sc.ChannelIDs) == 0 {
		return out, nil
	}
	rows, err := s.visibleMentions(authorize.Filter(ctx), a, mq, limit+1)
	if err != nil {
		return messaging.MentionPage{}, err
	}
	more := len(rows) > limit
	rows = rows[:min(len(rows), limit)]
	for _, r := range rows {
		m, err := s.stored(ctx, r.Message)
		if err != nil {
			return messaging.MentionPage{}, err
		}
		out.Items = append(out.Items, messaging.MentionItem{Message: m, CreatedAt: r.MentionedAt})
	}
	if !more {
		return out, nil
	}
	last := rows[len(rows)-1]
	if out.NextCursor, err = s.d.Cursors.Encode(mentionsList, binding, domain.MentionKey{At: last.MentionedAt, MessageID: last.Message.ID}); err != nil {
		return messaging.MentionPage{}, errs.Wrap(errs.CodeInternalError, err, "messaging: mention list cursor")
	}
	return out, nil
}

// mentionQuery returns the store query of the caller a over visible, positioned after the
// cursor when one is given. Errors: 400 cursor_invalid.
func (s *Service) mentionQuery(a actor.Actor, visible []ids.ID, binding []byte, cursor string) (MentionQuery, error) {
	mq := MentionQuery{PrincipalID: a.ID, Visible: visible}
	if cursor == "" {
		return mq, nil
	}
	var k domain.MentionKey
	if err := s.d.Cursors.Decode(mentionsList, binding, cursor, &k); err != nil {
		return MentionQuery{}, err
	}
	if k.At.IsZero() || k.MessageID.IsZero() {
		return MentionQuery{}, errs.New(errs.CodeCursorInvalid, "")
	}
	mq.HasAfter, mq.AfterAt, mq.AfterID = true, k.At, k.MessageID
	return mq, nil
}

// pageSize returns the page size of a request: api.page_size_default for none, capped at
// api.page_size_max.
func (s *Service) pageSize(limit int) int {
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	return min(limit, s.cfg.PageSizeMax)
}

// visibleMentions reads the mentions of mq in batches of want and keeps those authorize allows,
// until want are visible or the list ends.
func (s *Service) visibleMentions(ctx context.Context, a actor.Actor, mq MentionQuery, want int) ([]MentionRow, error) {
	var out []MentionRow
	mq.Limit = want
	for len(out) < want {
		batch, err := s.d.Store.MyMentions(ctx, mq)
		if err != nil {
			return nil, err
		}
		if out, err = s.readableMentions(ctx, a, batch, out, want); err != nil {
			return nil, err
		}
		if len(batch) < want {
			break
		}
		last := batch[len(batch)-1]
		mq.HasAfter, mq.AfterAt, mq.AfterID = true, last.MentionedAt, last.Message.ID
	}
	return out, nil
}

// readableMentions appends to out the rows of batch that a may read, up to want in total. A row
// authorize refuses is a filter mismatch.
func (s *Service) readableMentions(ctx context.Context, a actor.Actor, batch, out []MentionRow, want int) ([]MentionRow, error) {
	for _, r := range batch {
		if len(out) == want {
			break
		}
		ok, err := s.allowed(ctx, a, authorize.ActionMessageRead, messageResource(r.Message))
		if err != nil {
			return nil, err
		}
		if !ok {
			s.filterMismatch(ctx, authorize.ActionMessageRead)
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// filterMismatch logs and counts an item the SQL filter kept and authorize refused: the line
// names only the action and the resource kind, never an id or content (07 section 7.4 rule 3).
func (s *Service) filterMismatch(ctx context.Context, act authorize.Action) {
	s.d.Log.LogAttrs(ctx, slog.LevelError, "authz filter mismatch on a message",
		slog.String(attrAction, string(act)), slog.String(attrReason, reasonFilterMismatch))
	if s.d.Errors != nil {
		s.d.Errors.CountError(ctx, reasonFilterMismatch)
	}
}

// messageResource describes the message m for authorize: its thread and channel as path hints
// and its author as owner.
func messageResource(m domain.Message) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceMessage, ID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, OwnerID: m.AuthorID}
}

// CountUnread implements messaging.Mentions (brief note 4) with one statement; an empty
// VisibleChannelIDs returns nothing without a query.
func (s Mentions) CountUnread(ctx context.Context, q messaging.UnreadMentionQuery) ([]messaging.ThreadMentionCount, error) {
	if len(q.VisibleChannelIDs) == 0 {
		return []messaging.ThreadMentionCount{}, nil
	}
	return s.d.Store.CountUnreadMentions(ctx, q.PrincipalID, q.VisibleChannelIDs, q.LastReadSeq)
}
