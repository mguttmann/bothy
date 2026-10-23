// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository is the store of the read operations of the search (M3-14).
var _ app.SearchReadStore = (*Repository)(nil)

// searchError maps the error of a search read (14 7): a statement PostgreSQL cancelled at
// search.statement_timeout while ctx is alive is searchkit.ErrTimeout, an ended ctx is its own
// error (the caller gave up, also when pgx's cancel request produced the same SQLSTATE), anything
// else goes through mapError.
func searchError(ctx context.Context, err error, what string) error {
	switch {
	case err == nil:
		return nil
	case pg.IsStatementTimeout(ctx, err):
		return searchkit.ErrTimeout
	case ctx.Err() != nil:
		return ctx.Err()
	}
	return mapError(err, what)
}

// int32Param converts a configured count for a statement parameter; a value outside int32 is a
// programming error of the caller, never a silent wrap.
func int32Param(n int, what string) (int32, error) {
	if n < 0 || n > math.MaxInt32-1 {
		return 0, errs.New(errs.CodeInternalError, "chat: "+what+" outside its range")
	}
	return int32(n), nil
}

// optionalTimePtr converts an optional time: nil becomes NULL.
func optionalTimePtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// optionalUUIDPtr converts an optional id: nil becomes NULL.
func optionalUUIDPtr(id *ids.ID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return uuid(*id)
}

// SearchThreadTitles implements app.SearchReadStore with the one statement SearchThreadTitles
// (14 4.5 to 4.8).
func (r *Repository) SearchThreadTitles(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	params, err := searchParams(q)
	if err != nil {
		return searchkit.Page{}, err
	}
	rows, err := r.read(ctx).SearchThreadTitles(ctx, params)
	if err != nil {
		return searchkit.Page{}, searchError(ctx, err, "searching thread titles")
	}
	return titlePage(rows, q, int(params.PageSize)), nil
}

// searchParams converts q into the parameters of SearchThreadTitles. Errors: internal for a
// configured value outside its range.
func searchParams(q searchkit.Query) (sqlcgen.SearchThreadTitlesParams, error) {
	var counts [4]int32
	for i, c := range []struct {
		n    int
		what string
	}{
		{q.Limits.PageSize, "search page size"}, {q.Limits.MaxCandidates, "search.max_candidates"},
		{q.Limits.HeadlineInputMaxChars, "search.headline_input_max_chars"}, {q.Limits.RankNormalization, "search.rank_normalization"},
	} {
		v, err := int32Param(c.n, c.what)
		if err != nil {
			return sqlcgen.SearchThreadTitlesParams{}, err
		}
		counts[i] = v
	}
	if q.Limits.PageSize < 1 || q.Limits.RecencySeconds() <= 0 {
		return sqlcgen.SearchThreadTitlesParams{}, errs.New(errs.CodeInternalError, "chat: search page size and search.recency_scale must be positive")
	}
	p := sqlcgen.SearchThreadTitlesParams{
		Tsq: q.TSQuery, ChannelIds: uuids(q.ChannelIDs), RefTime: pgtype.Timestamptz{Time: q.RefTime, Valid: true},
		RestrictThreads: q.ThreadIDs != nil, ThreadIds: uuids(q.ThreadIDs), State: optionalText(q.Thread.State),
		TagIds: uuids(q.Thread.TagIDs), OperatorAgentID: optionalUUIDPtr(q.Thread.OperatorAgentID),
		After: optionalTimePtr(q.After), Before: optionalTimePtr(q.Before),
		PageSize: counts[0], MaxCandidates: counts[1], HeadlineMaxChars: counts[2], RankNormalization: counts[3],
		RecencySeconds: q.Limits.RecencySeconds(), Newest: q.Sort == searchkit.SortNewest, HeadlineOptions: q.Limits.HeadlineOptions(q.Markers),
	}
	if q.From != nil {
		p.HasFrom, p.FromScore, p.FromID = true, q.From.Score, uuid(q.From.ID)
		p.FromCreatedAt = pgtype.Timestamptz{Time: q.From.CreatedAt, Valid: true}
	}
	return p, nil
}

// titlePage converts the rows of SearchThreadTitles into a page. The rows come in page order with
// one more than pageSize when a next page exists; that row is never returned, and Next is the
// position of the last returned row. An empty page arrives as one row without a thread that only
// carries the candidate flag.
func titlePage(rows []sqlcgen.SearchThreadTitlesRow, q searchkit.Query, pageSize int) searchkit.Page {
	page := searchkit.Page{Hits: []searchkit.Hit{}}
	for _, row := range rows {
		page.CandidatesCapped = page.CandidatesCapped || row.Capped
		if !row.ID.Valid {
			continue // the row of an empty page
		}
		if len(page.Hits) == pageSize {
			last := page.Hits[pageSize-1]
			page.Next = &searchkit.Position{ID: last.ID}
			if q.Sort == searchkit.SortNewest {
				page.Next.CreatedAt = last.CreatedAt
			} else {
				page.Next.Score = last.Score
			}
			break
		}
		id := idOf(row.ID)
		page.Hits = append(page.Hits, searchkit.Hit{
			ID: id, ThreadID: id, ChannelID: idOf(row.ChannelID), Score: row.Score, CreatedAt: timeOf(row.CreatedAt),
			Segments: searchkit.Split(row.Headline, q.Markers),
		})
	}
	return page
}

// PrefixThreadTitles implements app.SearchReadStore with the statement PrefixThreadTitles (14 4.9).
func (r *Repository) PrefixThreadTitles(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error) {
	limit, err := int32Param(q.Limit, "prefix search limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.read(ctx).PrefixThreadTitles(ctx, sqlcgen.PrefixThreadTitlesParams{ChannelIds: uuids(q.ChannelIDs), Tsq: q.TSQuery, MaxRows: limit})
	if err != nil {
		return nil, searchError(ctx, err, "searching thread titles by prefix")
	}
	out := make([]searchkit.TitleHit, 0, len(rows))
	for _, row := range rows {
		out = append(out, searchkit.TitleHit{ThreadID: idOf(row.ID), ChannelID: idOf(row.ChannelID), Title: row.Title, LastActivityAt: timeOf(row.LastActivityAt)})
	}
	return out, nil
}

// ThreadsMatching implements app.SearchReadStore with the statement ThreadsMatching.
func (r *Repository) ThreadsMatching(ctx context.Context, channels []ids.ID, f searchkit.ThreadFilter) ([]ids.ID, error) {
	rows, err := r.read(ctx).ThreadsMatching(ctx, sqlcgen.ThreadsMatchingParams{
		ChannelIds: uuids(channels), State: optionalText(f.State), TagIds: uuids(f.TagIDs), OperatorAgentID: optionalUUIDPtr(f.OperatorAgentID),
	})
	if err != nil {
		return nil, searchError(ctx, err, "reading the threads of a filter")
	}
	return idList(rows), nil
}

// ChannelsInSpaces implements app.SearchReadStore with the statement ChannelsInSpaces.
func (r *Repository) ChannelsInSpaces(ctx context.Context, spaces []ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).ChannelsInSpaces(ctx, uuids(spaces))
	if err != nil {
		return nil, mapError(err, "reading the channels of spaces")
	}
	return idList(rows), nil
}

// Labels implements app.SearchReadStore with the statements ChannelLabels and ThreadTitleLabels.
func (r *Repository) Labels(ctx context.Context, visibleSpaces, channels, threads []ids.ID) (chat.Labels, error) {
	q := r.read(ctx)
	out := chat.Labels{Channels: map[ids.ID]chat.ChannelLabel{}, ThreadTitles: map[ids.ID]string{}}
	crows, err := q.ChannelLabels(ctx, sqlcgen.ChannelLabelsParams{VisibleSpaceIds: uuids(visibleSpaces), ChannelIds: uuids(channels)})
	if err != nil {
		return chat.Labels{}, mapError(err, "reading the channel labels")
	}
	for _, row := range crows {
		out.Channels[idOf(row.ID)] = chat.ChannelLabel{Name: row.Name, SpaceName: row.SpaceName}
	}
	trows, err := q.ThreadTitleLabels(ctx, sqlcgen.ThreadTitleLabelsParams{ChannelIds: uuids(channels), ThreadIds: uuids(threads)})
	if err != nil {
		return chat.Labels{}, mapError(err, "reading the thread labels")
	}
	for _, row := range trows {
		out.ThreadTitles[idOf(row.ID)] = row.Title
	}
	return out, nil
}

// ThreadsIn implements app.SearchReadStore with the statement ThreadsIn.
func (r *Repository) ThreadsIn(ctx context.Context, threads, channels []ids.ID) ([]domain.Thread, error) {
	rows, err := r.read(ctx).ThreadsIn(ctx, sqlcgen.ThreadsInParams{ChannelIds: uuids(channels), ThreadIds: uuids(threads)})
	if err != nil {
		return nil, mapError(err, "reading the threads of hits")
	}
	out := make([]domain.Thread, 0, len(rows))
	for _, row := range rows {
		out = append(out, toThread(threadRow(row)))
	}
	return out, nil
}

// idList converts a column of ids.
func idList(rows []pgtype.UUID) []ids.ID {
	out := make([]ids.ID, 0, len(rows))
	for _, row := range rows {
		out = append(out, idOf(row))
	}
	return out
}
