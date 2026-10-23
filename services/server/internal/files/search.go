// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// SearchReads are the read operations of files for the search coordinator
// (docs/spec/14-search.md section 10, R-SU-09): search owns no tables and reads files only
// through them. Neither operation authorizes: the coordinator passes the visible channels of
// Authorizer.Scope and checks every delivered item with authorize again (14 4.3).
type SearchReads interface {
	// SearchFiles runs the full-text search over files.search_vector in one statement (14 4.5 to
	// 4.8): deliverable attachments (upload_state complete, scan_status clean or skipped, not
	// deleted, purpose attachment; 12 section 2) that hang on a message, filtered by the channel
	// of that message (message_attachments.channel_id = ANY(q.ChannelIDs), never
	// files.scope_channel_id) and by q.ThreadIDs against message_attachments.thread_id, with
	// AuthorID on files.uploaded_by, After and Before on files.created_at and created_at at or
	// before q.RefTime; candidates, ranking, keyset and page as for
	// messaging.SearchReads.SearchMessages, the headline in simple over the name followed by the
	// transcript, cut at search.headline_input_max_chars. Each hit carries the message, thread
	// and channel of its attachment. q.Thread and q.HasFile are ignored. Errors:
	// searchkit.ErrTimeout at search.statement_timeout, the context error when ctx ended,
	// otherwise the database error.
	SearchFiles(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// HitFiles returns the files fileIDs in the representation of getFile, in the order of
	// fileIDs; an id that is missing, not deliverable or not attached in channelIDs is absent. It
	// does not authorize. Errors: the database error.
	HitFiles(ctx context.Context, fileIDs, channelIDs []ids.ID) ([]File, error)
}
