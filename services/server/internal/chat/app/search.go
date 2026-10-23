// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// Compile-time check: the read operations of the search coordinator (M3-14).
var _ chat.SearchReads = (*Service)(nil)

// SearchReadStore is the outgoing port of the read operations of the search coordinator (M3-14,
// docs/spec/14-search.md section 10), implemented by adapters/postgres. Like UnreadStore it is a
// port of its own beside Store, so the in-memory stores of the tests that do not need it stay
// unchanged; Deps.Search falls back to Store when Store implements it. Every read carries the
// visible set it is given (07 7.4 rule 1) and authorizes nothing. Errors: searchkit.ErrTimeout
// where chat.SearchReads names it, the context error when ctx ended, otherwise kernel/errs errors
// without SQL text or values.
type SearchReadStore interface {
	// SearchThreadTitles runs the one statement of chat.SearchReads.SearchThreadTitles and
	// returns the page with the headlines already split.
	SearchThreadTitles(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// PrefixThreadTitles returns up to q.Limit threads of q.ChannelIDs whose title matches the
	// prefix tsquery, newest last_activity_at first, then id descending.
	PrefixThreadTitles(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error)
	// ThreadsMatching returns the ids of the threads of channels that match f, ascending.
	ThreadsMatching(ctx context.Context, channels []ids.ID, f searchkit.ThreadFilter) ([]ids.ID, error)
	// ChannelsInSpaces returns the ids of the channels of spaces, ascending.
	ChannelsInSpaces(ctx context.Context, spaces []ids.ID) ([]ids.ID, error)
	// Labels returns the names of the channels among channels whose space is in visibleSpaces
	// and the non-empty titles of the threads among threads that lie in channels; both maps are
	// never nil.
	Labels(ctx context.Context, visibleSpaces, channels, threads []ids.ID) (chat.Labels, error)
	// ThreadsIn returns the threads among threads that lie in channels, in any order.
	ThreadsIn(ctx context.Context, threads, channels []ids.ID) ([]domain.Thread, error)
}

// searchStore returns the bound search store or the internal error of a service without one: an
// empty answer would silently hide every hit.
func (s *Service) searchStore() (SearchReadStore, error) {
	if s.d.Search == nil {
		return nil, errs.New(errs.CodeInternalError, "chat: no search store bound")
	}
	return s.d.Search, nil
}

// emptyPage is the page of a search that runs no statement: no hits, no next page, not capped.
func emptyPage() searchkit.Page { return searchkit.Page{Hits: []searchkit.Hit{}} }

// SearchThreadTitles implements chat.SearchReads. No channel and an empty (non-nil) thread id list
// answer an empty page without a statement: nothing can match.
func (s *Service) SearchThreadTitles(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	if len(q.ChannelIDs) == 0 || (q.ThreadIDs != nil && len(q.ThreadIDs) == 0) {
		return emptyPage(), nil
	}
	st, err := s.searchStore()
	if err != nil {
		return searchkit.Page{}, err
	}
	return st.SearchThreadTitles(ctx, q)
}

// PrefixThreadTitles implements chat.SearchReads; no channel or a limit below one answers an empty
// list without a statement.
func (s *Service) PrefixThreadTitles(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error) {
	if len(q.ChannelIDs) == 0 || q.Limit < 1 {
		return []searchkit.TitleHit{}, nil
	}
	st, err := s.searchStore()
	if err != nil {
		return nil, err
	}
	return st.PrefixThreadTitles(ctx, q)
}

// ThreadsMatching implements chat.SearchReads; no channel answers an empty list without a
// statement.
func (s *Service) ThreadsMatching(ctx context.Context, channelIDs []ids.ID, f searchkit.ThreadFilter) ([]ids.ID, error) {
	if len(channelIDs) == 0 {
		return []ids.ID{}, nil
	}
	st, err := s.searchStore()
	if err != nil {
		return nil, err
	}
	return st.ThreadsMatching(ctx, channelIDs, f)
}

// ChannelsInSpaces implements chat.SearchReads; no space answers an empty list without a
// statement.
func (s *Service) ChannelsInSpaces(ctx context.Context, spaceIDs []ids.ID) ([]ids.ID, error) {
	if len(spaceIDs) == 0 {
		return []ids.ID{}, nil
	}
	st, err := s.searchStore()
	if err != nil {
		return nil, err
	}
	return st.ChannelsInSpaces(ctx, spaceIDs)
}

// Labels implements chat.SearchReads; no channel answers empty maps without a statement (a thread
// title is only given for a thread in channelIDs).
func (s *Service) Labels(ctx context.Context, visibleSpaceIDs, channelIDs, threadIDs []ids.ID) (chat.Labels, error) {
	if len(channelIDs) == 0 {
		return chat.Labels{Channels: map[ids.ID]chat.ChannelLabel{}, ThreadTitles: map[ids.ID]string{}}, nil
	}
	st, err := s.searchStore()
	if err != nil {
		return chat.Labels{}, err
	}
	return st.Labels(ctx, visibleSpaceIDs, channelIDs, threadIDs)
}

// HitThreads implements chat.SearchReads: the threads through views, the code path of getThread
// (live tags, billing mode of the fixed operator), in the order of threadIDs; a repeated id is
// given once. No thread or no channel answers an empty list without a statement.
func (s *Service) HitThreads(ctx context.Context, threadIDs, channelIDs []ids.ID) ([]chat.Thread, error) {
	if len(threadIDs) == 0 || len(channelIDs) == 0 {
		return []chat.Thread{}, nil
	}
	st, err := s.searchStore()
	if err != nil {
		return nil, err
	}
	rows, err := st.ThreadsIn(ctx, threadIDs, channelIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[ids.ID]domain.Thread, len(rows))
	for _, th := range rows {
		byID[th.ID] = th
	}
	ordered := make([]domain.Thread, 0, len(rows))
	for _, id := range threadIDs {
		if th, ok := byID[id]; ok {
			ordered = append(ordered, th)
			delete(byID, id)
		}
	}
	return s.views(ctx, ordered)
}
