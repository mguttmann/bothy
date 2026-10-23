// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// SearchIndex is the coordinator's index port (docs/spec/14-search.md section 10); the PostgreSQL
// adapter delegates to the read operations of messaging, chat and files, and an external engine
// would replace only it (14 section 9). Every method runs in the ambient unit of work of ctx, so
// the statements share the coordinator's search.statement_timeout.
type SearchIndex interface {
	// BuildQuery returns the tsquery text of q over search.languages plus simple (14 4.2,
	// search_tsquery); ok is false when q has no term. Errors: searchkit.ErrTimeout, the database
	// error.
	BuildQuery(ctx context.Context, q string) (tsq string, ok bool, err error)
	// BuildPrefix returns the prefix tsquery text of q (14 4.9, search.prefix_min_chars); ok is
	// false when no word is long enough. Errors as BuildQuery.
	BuildPrefix(ctx context.Context, q string) (tsq string, ok bool, err error)
	// Messages, Threads and Files run the search of one type (messaging.SearchReads,
	// chat.SearchReads, files.SearchReads); Messages and Files resolve q.Thread into q.ThreadIDs
	// through chat first. Errors: searchkit.ErrTimeout, the context error, the database error.
	Messages(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	Threads(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	Files(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// TitlePrefix runs the prefix search over thread titles (chat.SearchReads.PrefixThreadTitles).
	TitlePrefix(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error)
}

// Structure is the part of chat.SearchReads the coordinator itself reads: the channels of the
// space filter and the names of a page.
type Structure interface {
	// ChannelsInSpaces returns the channels of the given (visible) spaces.
	ChannelsInSpaces(ctx context.Context, spaceIDs []ids.ID) ([]ids.ID, error)
	// Labels returns channel names, space names and thread titles.
	Labels(ctx context.Context, visibleSpaceIDs, channelIDs, threadIDs []ids.ID) (chat.Labels, error)
}

// Cursors is the cursor codec of M1-18 (platform/httpserver), an outgoing port so the order of
// checks of design note 2 stays in one use case: Decode answers 400 cursor_invalid on any
// mismatch.
type Cursors interface {
	// Encode returns the opaque cursor of key for list, bound to binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key.
	Decode(list string, binding []byte, s string, key any) error
}

// Metrics records bothy.search.duration (docs/observability/signals.md 4.4); NoMetrics records
// nothing.
type Metrics interface {
	// SearchCompleted records one search with its outcome (ok, client_error, timeout,
	// server_error) and duration.
	SearchCompleted(ctx context.Context, outcome string, d time.Duration)
}

// ErrorCounter counts bothy.authz.errors (signals.md 4.1); the page check counts
// filter_mismatch with it.
type ErrorCounter interface {
	// CountError adds one error with reason.
	CountError(ctx context.Context, reason string)
}

// NoMetrics is the Metrics that records nothing.
type NoMetrics struct{}

// SearchCompleted implements Metrics.
func (NoMetrics) SearchCompleted(context.Context, string, time.Duration) {}
