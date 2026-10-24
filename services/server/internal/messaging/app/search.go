// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Compile-time check: the service offers the read operations of the search coordinator (M3-14).
var _ messaging.SearchReads = (*Service)(nil)

// SearchReadStore is the part of Store that answers the read operations of the search
// coordinator (M3-14, docs/spec/14-search.md section 10). Both reads carry the visible channels
// the coordinator passes (07 section 7.4 rule 1) and authorize nothing.
type SearchReadStore interface {
	// SearchMessages runs the one search statement of q (14 4.8) and returns its page.
	// Errors: searchkit.ErrTimeout at search.statement_timeout, the context error when ctx ended,
	// otherwise a kernel/errs error.
	SearchMessages(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// HitMessages returns the live messages of messageIDs whose channel is in channelIDs, in any
	// order.
	HitMessages(ctx context.Context, messageIDs, channelIDs []ids.ID) ([]domain.Message, error)
}

// SearchMessages implements messaging.SearchReads without authorization: the coordinator passed
// the visible channels in q.ChannelIDs and checks every delivered hit with message.read again
// (14 4.3).
func (s *Service) SearchMessages(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	return s.d.Store.SearchMessages(ctx, q)
}

// HitMessages implements messaging.SearchReads without authorization: the messages in the list
// representation of History (listThreadMessages), content cut at message.list_content_max_bytes,
// with the attachments, reactions, own reactions of the principal of ctx, pin and components that
// decorate adds there, in the order of messageIDs; an id that is missing, deleted or outside
// channelIDs is absent, and a repeated id appears once.
func (s *Service) HitMessages(ctx context.Context, messageIDs, channelIDs []ids.ID) ([]messaging.Message, error) {
	out := []messaging.Message{}
	if len(messageIDs) == 0 || len(channelIDs) == 0 {
		return out, nil
	}
	rows, err := s.d.Store.HitMessages(ctx, messageIDs, channelIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[ids.ID]domain.Message, len(rows))
	for _, m := range rows {
		byID[m.ID] = m
	}
	authors := map[ids.ID]messaging.PrincipalRef{}
	for _, id := range messageIDs {
		m, ok := byID[id]
		if !ok {
			continue
		}
		delete(byID, id)
		v, err := s.listed(ctx, m, authors)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := s.decorate(ctx, channelIDs, out); err != nil {
		return nil, err
	}
	return out, nil
}
