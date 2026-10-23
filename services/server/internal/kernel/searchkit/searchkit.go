// SPDX-License-Identifier: Apache-2.0

package searchkit

import (
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Sort is the order of the hits of one search (14 4.5).
type Sort string

// The two orders of 14 4.5.
const (
	// SortRelevance orders by score descending, then id descending.
	SortRelevance Sort = "relevance"
	// SortNewest orders by created_at descending, then id descending.
	SortNewest Sort = "newest"
)

// Position is the keyset of the last row of a page (14 4.7): Score for SortRelevance, CreatedAt
// for SortNewest, and ID for both. The next page starts strictly after it.
type Position struct {
	// Score is the score of the row; only for SortRelevance.
	Score float64
	// CreatedAt is created_at of the row; only for SortNewest.
	CreatedAt time.Time
	// ID is the id of the row.
	ID ids.ID
}

// Limits carries the configured values of one search; the coordinator fills it from the search.*
// keys (docs/operations/configuration.md), so the read operations never name a number.
type Limits struct {
	// MaxCandidates is search.max_candidates: the newest rows after filters and rights that are
	// ranked (14 4.5 item 1).
	MaxCandidates int
	// RankNormalization is search.rank_normalization, the normalization of ts_rank.
	RankNormalization int
	// RecencyScale is search.recency_scale: the age at which the score falls to half.
	RecencyScale time.Duration
	// HeadlineInputMaxChars is search.headline_input_max_chars: the text ts_headline reads.
	HeadlineInputMaxChars int
	// HeadlineMaxFragments is search.headline_max_fragments (MaxFragments of ts_headline).
	HeadlineMaxFragments int
	// HeadlineMaxWords is search.headline_max_words (MaxWords of ts_headline).
	HeadlineMaxWords int
	// HeadlineMinWords is search.headline_min_words (MinWords of ts_headline).
	HeadlineMinWords int
	// PageSize is the page size of the request, already capped by the coordinator at
	// search.page_size_max.
	PageSize int
}

// headlineShortWord is the ShortWord option of ts_headline, which docs/spec/14-search.md 4.6 fixes
// in the text of the chapter rather than in a configuration key.
const headlineShortWord = 3

// HeadlineOptions returns the options text of ts_headline for this search (14 4.6): StartSel and
// StopSel from m, MaxFragments, MaxWords and MinWords from the configured values and ShortWord.
// The markers are hex and the values numbers, so the text needs no quoting.
//
// Example:
//
//	opts := q.Limits.HeadlineOptions(q.Markers) // "StartSel=...,StopSel=...,MaxFragments=2,..."
func (l Limits) HeadlineOptions(m Markers) string {
	return fmt.Sprintf("StartSel=%s,StopSel=%s,MaxFragments=%d,MaxWords=%d,MinWords=%d,ShortWord=%d",
		m.Start, m.Stop, l.HeadlineMaxFragments, l.HeadlineMaxWords, l.HeadlineMinWords, headlineShortWord)
}

// RecencySeconds returns search.recency_scale in seconds, the divisor of the age in the score
// (14 4.5 item 2).
func (l Limits) RecencySeconds() float64 { return l.RecencyScale.Seconds() }

// ThreadFilter holds the filters of 14 4.4 that need chat's tables: agent_id, thread_state and
// tag_id. The zero value restricts nothing.
type ThreadFilter struct {
	// State is "", "open" or "closed".
	State string
	// TagIDs match a thread that carries at least one of them; empty restricts nothing.
	TagIDs []ids.ID
	// OperatorAgentID matches threads.operator_agent_id; nil restricts nothing. Main threads
	// never match.
	OperatorAgentID *ids.ID
}

// IsZero reports whether f restricts nothing.
func (f ThreadFilter) IsZero() bool {
	return f.State == "" && len(f.TagIDs) == 0 && f.OperatorAgentID == nil
}

// Query is one search as a read operation receives it. The coordinator fills every field; the
// read operation adds no rights logic and trusts ChannelIDs (07 7.4 rule 1).
type Query struct {
	// TSQuery is the tsquery text the search module built (07 7.4 example: "search expression,
	// built by the search module"); the statement casts it with ::tsquery.
	TSQuery string
	// ChannelIDs are the visible channels after every channel filter, ascending; never empty
	// (the coordinator answers an empty set without a query).
	ChannelIDs []ids.ID
	// ThreadIDs is nil for no restriction; otherwise only rows of these threads match.
	ThreadIDs []ids.ID
	// Thread is applied by chat for the type threads; for messages and files the index adapter
	// resolves it into ThreadIDs through chat.ThreadsMatching first.
	Thread ThreadFilter
	// AuthorID is messages.author_principal_id or files.uploaded_by; nil restricts nothing.
	AuthorID *ids.ID
	// After keeps rows with created_at >= After.
	After *time.Time
	// Before keeps rows with created_at < Before.
	Before *time.Time
	// HasFile keeps messages with (true) or without (false) attachments; messages only.
	HasFile *bool
	// Sort is the order.
	Sort Sort
	// RefTime is the reference time of the first page: candidates have created_at <= RefTime and
	// the age of the score is measured from it, so later pages rank the same rows the same way.
	RefTime time.Time
	// From is the position of the last row of the previous page; nil on the first page.
	From *Position
	// Limits are the configured values.
	Limits Limits
	// Markers are StartSel and StopSel of ts_headline.
	Markers Markers
}

// Hit is one row of a page.
type Hit struct {
	// ID is the message, thread or file.
	ID ids.ID
	// ThreadID is the thread of the hit (the thread itself for type threads).
	ThreadID ids.ID
	// ChannelID is the channel of the hit at query time.
	ChannelID ids.ID
	// MessageID is, for files only, the message the file is attached to.
	MessageID ids.ID
	// Score is the score of SortRelevance; 0 for SortNewest.
	Score float64
	// CreatedAt is created_at of the row.
	CreatedAt time.Time
	// Segments is the headline split at the markers; never nil.
	Segments []Segment
}

// Page is the answer of one read operation.
type Page struct {
	// Hits are at most Limits.PageSize rows in page order.
	Hits []Hit
	// Next is the position of the last row the statement returned when a further row exists,
	// nil on the last page.
	Next *Position
	// CandidatesCapped reports that more than Limits.MaxCandidates rows matched.
	CandidatesCapped bool
}

// PrefixQuery is the prefix search over thread titles of the quick switcher (14 4.9).
type PrefixQuery struct {
	// TSQuery is the prefix tsquery text the search module built.
	TSQuery string
	// ChannelIDs are the visible channels of thread.read, ascending; never empty.
	ChannelIDs []ids.ID
	// Limit is the number of titles, already capped.
	Limit int
}

// TitleHit is one thread of the prefix search.
type TitleHit struct {
	// ThreadID is the thread.
	ThreadID ids.ID
	// ChannelID is its channel.
	ChannelID ids.ID
	// Title is threads.title.
	Title string
	// LastActivityAt is threads.last_activity_at.
	LastActivityAt time.Time
}

// ErrTimeout reports that a read operation hit search.statement_timeout; the coordinator answers
// 503 unavailable with reason search_timeout (14 section 7).
var ErrTimeout = errors.New("searchkit: statement timeout")
