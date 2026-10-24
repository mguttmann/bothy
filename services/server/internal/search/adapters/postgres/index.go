// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the search coordinator's port app.SearchIndex
// (docs/spec/14-search.md section 10, ADR 0032). The search module owns no tables: this adapter
// builds the tsquery text with the platform function search_tsquery and the prefix statement of
// 14 4.9 (no table is read) and delegates every search to the read operations of messaging, chat
// and files. Filters on thread facts (thread_state, tag_id, agent_id) for messages and files are
// resolved through chat.ThreadsMatching into a thread id list in the same unit (07 7.4 rule 6,
// docs/DECISIONS.md D-M3-14-3). An external search engine would replace only this adapter.
package postgres

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/search/app"
)

// buildQuerySQL is the query construction of 14 4.2: an OR over the configurations of
// search.languages plus simple; NULL means no term. The configurations come as text and are cast
// to regconfig in the statement.
const buildQuerySQL = `SELECT search_tsquery($1::text[]::regconfig[], $2)::text`

// buildPrefixSQL is the prefix construction of 14 4.9: every lexeme of the simple configuration
// with at least $2 (search.prefix_min_chars) characters as a prefix term, joined with AND; NULL
// when none is left.
const buildPrefixSQL = `SELECT string_agg(quote_literal(lexeme) || ':*', ' & ' ORDER BY lexeme)::tsquery::text
FROM unnest(to_tsvector('simple', $1)) AS l
WHERE length(lexeme) >= $2`

// Reads are the read operations of the owning modules the index delegates to.
type Reads struct {
	// Messages is messaging's read port.
	Messages messaging.SearchReads
	// Chat is chat's read port.
	Chat chat.SearchReads
	// Files is files' read port.
	Files files.SearchReads
}

// Index implements app.SearchIndex over PostgreSQL.
type Index struct {
	pool           *pg.Pool
	languages      []string
	prefixMinChars int
	reads          Reads
}

// Compile-time check: the index is the coordinator's port.
var _ app.SearchIndex = (*Index)(nil)

// New returns the index over pool with the configurations of search.languages (without simple,
// which search_tsquery adds), search.prefix_min_chars and the read operations of the owning
// modules. Errors: a missing pool or read operation, or prefixMinChars below one.
func New(pool *pg.Pool, languages []string, prefixMinChars int, r Reads) (*Index, error) {
	if pool == nil || r.Messages == nil || r.Chat == nil || r.Files == nil {
		return nil, errors.New("search: the index needs a pool and the read operations of messaging, chat and files")
	}
	if prefixMinChars < 1 {
		return nil, errors.New("search: search.prefix_min_chars must be at least 1")
	}
	return &Index{pool: pool, languages: slices.Clone(languages), prefixMinChars: prefixMinChars, reads: r}, nil
}

// BuildQuery implements app.SearchIndex with search_tsquery in the ambient unit of ctx.
func (x *Index) BuildQuery(ctx context.Context, q string) (string, bool, error) {
	var tsq pgtype.Text
	if err := pg.Querier(ctx, x.pool).QueryRow(ctx, buildQuerySQL, x.languages, q).Scan(&tsq); err != nil {
		return "", false, mapError(ctx, err, "building the search query")
	}
	return tsq.String, tsq.Valid, nil
}

// BuildPrefix implements app.SearchIndex with the prefix statement of 14 4.9.
func (x *Index) BuildPrefix(ctx context.Context, q string) (string, bool, error) {
	var tsq pgtype.Text
	if err := pg.Querier(ctx, x.pool).QueryRow(ctx, buildPrefixSQL, q, x.prefixMinChars).Scan(&tsq); err != nil {
		return "", false, mapError(ctx, err, "building the prefix query")
	}
	return tsq.String, tsq.Valid && strings.TrimSpace(tsq.String) != "", nil
}

// Messages implements app.SearchIndex: the thread facts become thread ids, then
// messaging.SearchMessages.
func (x *Index) Messages(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	q, ok, err := x.resolveThreads(ctx, q)
	if err != nil || !ok {
		return searchkit.Page{Hits: []searchkit.Hit{}}, err
	}
	return x.reads.Messages.SearchMessages(ctx, q)
}

// Threads implements app.SearchIndex: chat applies the thread facts itself.
func (x *Index) Threads(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	return x.reads.Chat.SearchThreadTitles(ctx, q)
}

// Files implements app.SearchIndex: the thread facts become thread ids, then files.SearchFiles.
func (x *Index) Files(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	q, ok, err := x.resolveThreads(ctx, q)
	if err != nil || !ok {
		return searchkit.Page{Hits: []searchkit.Hit{}}, err
	}
	return x.reads.Files.SearchFiles(ctx, q)
}

// TitlePrefix implements app.SearchIndex with chat.PrefixThreadTitles.
func (x *Index) TitlePrefix(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error) {
	return x.reads.Chat.PrefixThreadTitles(ctx, q)
}

// resolveThreads composes the thread facts of q (design note 5): with a non-zero q.Thread the
// threads of q.ChannelIDs that match it, intersected with q.ThreadIDs when given, become
// q.ThreadIDs and q.Thread is cleared. ok is false when no thread is left, so no further query
// runs.
func (x *Index) resolveThreads(ctx context.Context, q searchkit.Query) (searchkit.Query, bool, error) {
	if q.Thread.IsZero() {
		return q, q.ThreadIDs == nil || len(q.ThreadIDs) > 0, nil
	}
	matching, err := x.reads.Chat.ThreadsMatching(ctx, q.ChannelIDs, q.Thread)
	if err != nil {
		return q, false, err
	}
	if q.ThreadIDs != nil {
		matching = slices.DeleteFunc(matching, func(id ids.ID) bool { return !slices.Contains(q.ThreadIDs, id) })
	}
	q.ThreadIDs, q.Thread = matching, searchkit.ThreadFilter{}
	if q.ThreadIDs == nil {
		q.ThreadIDs = []ids.ID{}
	}
	return q, len(q.ThreadIDs) > 0, nil
}

// mapError maps a statement timeout to searchkit.ErrTimeout, a dead ctx to its error and every
// other error to an internal error.
func mapError(ctx context.Context, err error, what string) error {
	if pg.IsStatementTimeout(ctx, err) {
		return searchkit.ErrTimeout
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errs.Wrap(errs.CodeInternalError, err, "search: "+what+" failed")
}
