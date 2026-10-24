// SPDX-License-Identifier: Apache-2.0

package search

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// Type is the result type of one search (docs/spec/14-search.md section 11).
type Type string

// The three result types; one type per query (R-SU-05).
const (
	// TypeMessages searches messages (the default).
	TypeMessages Type = "messages"
	// TypeThreads searches thread titles.
	TypeThreads Type = "threads"
	// TypeFiles searches attached files by name and transcript.
	TypeFiles Type = "files"
)

// Request is one full-text search (operation search, 14 section 4). Zero values mean "not given":
// Type defaults to TypeMessages, Sort to searchkit.SortRelevance, Limit to
// search.page_size_default.
type Request struct {
	// Type is the result type.
	Type Type
	// Q is the raw query text in web search syntax; the coordinator validates and cleans it.
	Q string
	// Sort is the order.
	Sort searchkit.Sort
	// SpaceIDs keeps channels of these spaces (intersected with the visible ones).
	SpaceIDs []ids.ID
	// ChannelIDs keeps these channels (intersected with the visible ones).
	ChannelIDs []ids.ID
	// ThreadID keeps one thread; nil for none.
	ThreadID *ids.ID
	// AuthorID is the author of a message or the uploader of a file.
	AuthorID *ids.ID
	// AgentID keeps threads whose operator is this agent.
	AgentID *ids.ID
	// After keeps rows created at or after it; Before rows created before it.
	After, Before *time.Time
	// HasFile keeps messages with (true) or without (false) attachments.
	HasFile *bool
	// ThreadState is "", "open" or "closed".
	ThreadState string
	// TagIDs keeps threads with at least one of these forum tags.
	TagIDs []ids.ID
	// Limit is the page size; 0 means search.page_size_default, more than search.page_size_max
	// is capped.
	Limit int
	// Cursor is next_cursor of the previous page; "" for the first page.
	Cursor string
}

// ResultHit is one delivered hit. Empty strings mean absent.
type ResultHit struct {
	// Kind is the type of the object: message, thread or file.
	Kind string
	// ID is the message, thread or file; ThreadID and ChannelID its place; MessageID the message
	// a file is attached to (files only).
	ID, ThreadID, ChannelID, MessageID ids.ID
	// Score is the score of searchkit.SortRelevance, nil for SortNewest.
	Score *float64
	// Segments is the excerpt; never nil.
	Segments []searchkit.Segment
	// ThreadTitle is the title of the hit's thread; "" for a main thread.
	ThreadTitle string
	// ChannelName and SpaceName name the channel and its space; "" for a channel without a
	// visible space (a DM channel).
	ChannelName, SpaceName string
}

// Result is one page of hits. There is no total and no count per channel (14 4.3 item 4).
type Result struct {
	// Hits are the delivered hits in page order; never nil.
	Hits []ResultHit
	// Next is the keyset position of the next page, nil on the last page; NextCursor is its
	// opaque cursor, bound to the request and the principal.
	Next       *searchkit.Position
	NextCursor string
	// RefTime is the reference time of the first page of this search (14 4.7).
	RefTime time.Time
	// CandidatesCapped reports that search.max_candidates was reached.
	CandidatesCapped bool
}

// PrefixRequest is the prefix search of the quick switcher (operation searchThreadTitles, 14 4.9).
type PrefixRequest struct {
	// Q is the typed text.
	Q string
	// Limit is the number of titles; 0 means search.page_size_default, more than
	// search.page_size_max is capped.
	Limit int
}

// Search is the inbound port of the search module: the coordinator of 14 section 10, which owns
// no tables, takes the visible channels from Authorizer.Scope and reads through the read
// operations of messaging, chat and files.
type Search interface {
	// Query runs one full-text search for p (operation search). Errors: 400 validation_failed on
	// q (too_long, empty) or on a filter outside its types (not_applicable), 400 cursor_invalid,
	// the refusal of search.query (401, 403), 503 unavailable with reason search_timeout, and
	// internal errors.
	//
	// Example:
	//
	//	res, err := s.Query(ctx, a, search.Request{Type: search.TypeMessages, Q: "invoice"})
	Query(ctx context.Context, p actor.Actor, r Request) (Result, error)
	// ThreadTitles runs the prefix search over thread titles for p (operation searchThreadTitles).
	// A query without a word of search.prefix_min_chars gives an empty list. Errors: 400
	// validation_failed on q (too_long), the refusal of search.query, 503 search_timeout,
	// internal errors.
	ThreadTitles(ctx context.Context, p actor.Actor, r PrefixRequest) ([]searchkit.TitleHit, error)
}
