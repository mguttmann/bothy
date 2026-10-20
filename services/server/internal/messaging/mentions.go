// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// MentionListQuery is the request of listMyMentions: the opaque cursor ("" for the first page)
// and the page size (0 for api.page_size_default; larger than api.page_size_max is capped).
type MentionListQuery struct {
	Cursor string
	Limit  int
}

// MentionItem is one entry of the own mention list (schema MentionItem): the message in its REST
// representation and the time of the mention.
type MentionItem struct {
	Message   Message
	CreatedAt time.Time
}

// MentionPage is one page of the own mention list (schema MentionItemPage); NextCursor is ""
// on the last page.
type MentionPage struct {
	Items      []MentionItem
	NextCursor string
}

// UnreadMentionQuery asks for the unread mentions of one principal (brief M1-60 note 4).
type UnreadMentionQuery struct {
	// PrincipalID is the mentioned principal.
	PrincipalID ids.ID
	// VisibleChannelIDs are the channels the principal may read (Authorizer.Scope of
	// message.read); empty returns nothing.
	VisibleChannelIDs []ids.ID
	// LastReadSeq is the read sequence per thread; a thread outside the map counts from 0.
	LastReadSeq map[ids.ID]int64
}

// ThreadMentionCount is the number of unread mentions of one thread; Count is at least 1.
type ThreadMentionCount struct {
	ThreadID, ChannelID ids.ID
	Count               int
}

// Mentions is the incoming port of the mention list and the unread mention counters (M1-60).
// The caller of ListMine is the principal in ctx (kernel/actor); errors are kernel/errs errors.
type Mentions interface {
	// ListMine returns one page of the mentions of the caller in channels it may read, newest
	// first (mention.list, filtered by message.read, 07 section 7.4). A mention in a channel the
	// caller cannot read stays stored but is never listed (AZ-M48). Errors: 403 for an agent or
	// app (subject_kind_not_allowed), 400 cursor_invalid for a cursor of another list, another
	// principal or a changed one.
	//
	// Example:
	//
	//	page, err := mentions.ListMine(ctx, messaging.MentionListQuery{Limit: 50})
	ListMine(ctx context.Context, q MentionListQuery) (MentionPage, error)
	// CountUnread returns the unread mentions of q.PrincipalID per thread, ascending by thread,
	// above LastReadSeq, without tombstones and only in q.VisibleChannelIDs, in one statement. It
	// does not authorize: sync calls it with the scope it built for the principal (M1-61, M1-63:
	// ReadState.mention_count and the snapshot). No counter is stored (R-DM-32).
	//
	// Example:
	//
	//	counts, err := mentions.CountUnread(ctx, messaging.UnreadMentionQuery{PrincipalID: p, VisibleChannelIDs: sc.ChannelIDs})
	CountUnread(ctx context.Context, q UnreadMentionQuery) ([]ThreadMentionCount, error)
}
