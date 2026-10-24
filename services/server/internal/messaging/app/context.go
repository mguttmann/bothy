// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// ContextWindow implements messaging.Messages (M2-13 design note 3): the one statement of the
// window, then message.read per item in filter mode, a refused item dropped, logged and counted
// as filter_mismatch, each item in the representation of the port with its full content. A limit
// of zero or below reads nothing but the flag.
func (s *Service) ContextWindow(ctx context.Context, q messaging.ContextQuery) (messaging.ContextPage, error) {
	a := requestPrincipal(ctx)
	rows, err := s.d.Store.ContextWindow(ctx, ContextRead{ThreadID: q.ThreadID, AfterSeq: q.AfterSeq, BeforeSeq: q.BeforeSeq,
		Limit: max(q.Limit, 0), ExcludeAuthorID: q.ExcludeAuthorID})
	if err != nil {
		return messaging.ContextPage{}, err
	}
	out := messaging.ContextPage{Items: make([]messaging.Message, 0, len(rows.Items)), HasOlder: rows.HasOlder}
	authors := map[ids.ID]messaging.PrincipalRef{}
	fctx := authorize.Filter(ctx)
	for _, m := range rows.Items {
		ok, err := s.allowed(fctx, a, authorize.ActionMessageRead, historyResource(m))
		if err != nil {
			return messaging.ContextPage{}, err
		}
		if !ok {
			s.historyFilterMismatch(ctx)
			continue
		}
		v, err := s.whole(ctx, m, authors)
		if err != nil {
			return messaging.ContextPage{}, err
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// ContextMessage implements messaging.Messages (M2-13 design note 3): the stored row without the
// visible predicate, then message.read in filter mode; an unknown or refused message is ok false,
// never an error, so the caller answers one 404 for both (ruling D-M2-13-12).
func (s *Service) ContextMessage(ctx context.Context, messageID ids.ID) (messaging.Message, bool, error) {
	m, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil || !found {
		return messaging.Message{}, false, err
	}
	ok, err := s.allowed(authorize.Filter(ctx), requestPrincipal(ctx), authorize.ActionMessageRead, historyResource(m))
	if err != nil || !ok {
		return messaging.Message{}, false, err
	}
	v, err := s.whole(ctx, m, map[ids.ID]messaging.PrincipalRef{})
	if err != nil {
		return messaging.Message{}, false, err
	}
	return v, true, nil
}

// whole returns the representation of m with its full content (a tombstone without content);
// authors caches the authors of one request.
func (s *Service) whole(ctx context.Context, m domain.Message, authors map[ids.ID]messaging.PrincipalRef) (messaging.Message, error) {
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
	return view(m, author, mentions), nil
}
