// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// SearchReads are the read operations of messaging for the search coordinator
// (docs/spec/14-search.md section 10, R-SU-09): search owns no tables and reads messages only
// through them. Neither operation authorizes: the coordinator passes the visible channels of
// Authorizer.Scope and checks every delivered item with authorize again (14 4.3).
type SearchReads interface {
	// SearchMessages runs the full-text search over messages.search_vector in one statement
	// (14 4.5 to 4.8): rows of q.ChannelIDs that are not deleted, with the filters of q
	// (ThreadIDs, AuthorID, After, Before, HasFile on messages.has_attachments) and created_at at
	// or before q.RefTime; the newest q.Limits.MaxCandidates of them are the candidates
	// (CandidatesCapped when more exist), ranked by q.Sort, the keyset after q.From, at most
	// q.Limits.PageSize hits with a headline over left(content_md,
	// search.headline_input_max_chars) in messages.search_language, split at q.Markers. q.Thread
	// is ignored: the coordinator resolves it into q.ThreadIDs. Errors: searchkit.ErrTimeout when
	// the statement hit search.statement_timeout, the context error when ctx ended, otherwise the
	// database error.
	//
	// Example:
	//
	//	page, err := reads.SearchMessages(ctx, searchkit.Query{TSQuery: tsq, ChannelIDs: visible, Sort: searchkit.SortNewest, RefTime: now, Limits: lim, Markers: m})
	SearchMessages(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// HitMessages returns the messages messageIDs in the list representation of
	// listThreadMessages (content cut at message.list_content_max_bytes with ContentTruncated),
	// in the order of messageIDs; an id that is missing, deleted or outside channelIDs is absent.
	// It does not authorize. Errors: the database error.
	HitMessages(ctx context.Context, messageIDs, channelIDs []ids.ID) ([]Message, error)
}
