// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// ChannelLabel names a channel in a search hit (docs/spec/14-search.md section 11).
type ChannelLabel struct {
	// Name is channels.name.
	Name string
	// SpaceName is the name of the channel's space.
	SpaceName string
}

// Labels are the names a page of search hits shows: per channel its name and the name of its
// space, per thread its title. A channel without a visible space (a DM channel has none) and a
// main thread (empty title) are absent.
type Labels struct {
	// Channels are keyed by channel id.
	Channels map[ids.ID]ChannelLabel
	// ThreadTitles are keyed by thread id; only threads with a non-empty title.
	ThreadTitles map[ids.ID]string
}

// SearchReads are the read operations of chat for the search coordinator (docs/spec/14-search.md
// section 10, R-SU-09): search owns no tables and reads thread titles, thread facts and names
// only through them. None of them authorizes: every statement carries the visible set the
// coordinator passes (07 7.4 rule 1), and the coordinator checks every delivered item with
// authorize again (14 4.3).
type SearchReads interface {
	// SearchThreadTitles runs the full-text search over threads.title_search in one statement
	// (14 4.5 to 4.8): threads of q.ChannelIDs with a title (main threads never match), with
	// q.Thread (state, any of the tags, operator agent), q.ThreadIDs, After and Before on
	// threads.created_at and created_at at or before q.RefTime; candidates, ranking, keyset and
	// page as for messaging.SearchReads.SearchMessages, the headline over the title in
	// threads.search_language. q.AuthorID and q.HasFile do not apply to threads and are ignored.
	// Errors: searchkit.ErrTimeout at search.statement_timeout, the context error when ctx ended,
	// otherwise the database error.
	SearchThreadTitles(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// PrefixThreadTitles returns up to q.Limit threads of q.ChannelIDs whose title_search matches
	// the prefix tsquery q.TSQuery (14 4.9), newest last_activity_at first, then id descending.
	// Errors as SearchThreadTitles.
	PrefixThreadTitles(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error)
	// ThreadsMatching returns the ids of the threads of channelIDs that match f (07 7.4 rule 6:
	// the coordinator composes thread facts into a thread id list), ascending. A zero f matches
	// every thread. Errors as SearchThreadTitles.
	ThreadsMatching(ctx context.Context, channelIDs []ids.ID, f searchkit.ThreadFilter) ([]ids.ID, error)
	// ChannelsInSpaces returns the ids of the channels of spaceIDs, ascending; the caller passes
	// only visible spaces. Errors: searchkit.ErrTimeout at search.statement_timeout (it runs in
	// the unit of the search), the context error when ctx ended, otherwise the database error.
	ChannelsInSpaces(ctx context.Context, spaceIDs []ids.ID) ([]ids.ID, error)
	// Labels returns the names of channelIDs whose space is in visibleSpaceIDs and the titles of
	// threadIDs that lie in channelIDs. Errors as ChannelsInSpaces.
	Labels(ctx context.Context, visibleSpaceIDs, channelIDs, threadIDs []ids.ID) (Labels, error)
	// HitThreads returns the threads threadIDs in the representation of getThread, in the order
	// of threadIDs; an id that is missing or outside channelIDs is absent. It does not authorize.
	// Errors: the database error.
	HitThreads(ctx context.Context, threadIDs, channelIDs []ids.ID) ([]Thread, error)
}
