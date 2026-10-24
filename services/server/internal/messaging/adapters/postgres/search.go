// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// maxSearchRows bounds the row counts a search passes to LIMIT, so the int32 parameters cannot
// overflow; the configured values stay far below it.
const maxSearchRows = 1 << 20

// SearchMessages implements app.SearchReadStore with the one statement SearchMessages
// (queries/search.sql, docs/spec/14-search.md 4.8). An empty channel set answers an empty page
// without a statement. The statement fetches the page size plus one row, so Next is the position
// of the last hit of the page exactly when a further row exists.
func (r *Repository) SearchMessages(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	l := q.Limits
	if l.MaxCandidates < 1 || l.MaxCandidates >= maxSearchRows || l.PageSize < 1 || l.PageSize >= maxSearchRows ||
		l.HeadlineInputMaxChars < 1 || l.HeadlineInputMaxChars > maxSearchRows ||
		l.RankNormalization < 0 || l.RankNormalization > maxSearchRows || l.RecencySeconds() <= 0 {
		return searchkit.Page{}, errs.New(errs.CodeInternalError, "messaging: search limits out of range")
	}
	if len(q.ChannelIDs) == 0 {
		return searchkit.Page{Hits: []searchkit.Hit{}}, nil
	}
	p := sqlcgen.SearchMessagesParams{
		Tsq: q.TSQuery, RefTime: timestamp(&q.RefTime), HeadlineInputMaxChars: int32(l.HeadlineInputMaxChars),
		ChannelIds: uuids(q.ChannelIDs), ThreadIds: threadFilter(q.ThreadIDs), AuthorID: nullableID(q.AuthorID),
		After: timestamp(q.After), Before: timestamp(q.Before), HasFile: pgtype.Bool{Bool: q.HasFile != nil && *q.HasFile, Valid: q.HasFile != nil},
		CandidateRows: int32(l.MaxCandidates + 1), MaxCandidates: int32(l.MaxCandidates), Newest: q.Sort == searchkit.SortNewest,
		RankNormalization: int32(l.RankNormalization), RecencySeconds: l.RecencySeconds(),
		PageRows: int32(l.PageSize + 1), PageSize: int32(l.PageSize), HeadlineOptions: l.HeadlineOptions(q.Markers),
		// The keyset parameters are valid values even on the first page; HasFrom decides.
		FromCreatedAt: pgtype.Timestamptz{Valid: true}, FromID: uuid(ids.ID{}),
	}
	if q.From != nil {
		p.HasFrom, p.FromScore, p.FromCreatedAt, p.FromID = true, q.From.Score, timestamp(&q.From.CreatedAt), uuid(q.From.ID)
	}
	rows, err := r.read(ctx).SearchMessages(ctx, p)
	if err != nil {
		return searchkit.Page{}, searchError(ctx, err, "searching messages")
	}
	page := searchkit.Page{Hits: []searchkit.Hit{}}
	for i, row := range rows {
		page.CandidatesCapped = row.Capped
		if !row.ID.Valid {
			continue
		}
		if i == l.PageSize {
			// The extra row: a next page exists; it starts after the last hit of this one.
			last := page.Hits[len(page.Hits)-1]
			page.Next = &searchkit.Position{ID: last.ID}
			if q.Sort == searchkit.SortNewest {
				page.Next.CreatedAt = last.CreatedAt
			} else {
				page.Next.Score = last.Score
			}
			break
		}
		page.Hits = append(page.Hits, searchkit.Hit{
			ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), Score: row.Score,
			CreatedAt: timeOf(row.CreatedAt), Segments: searchkit.Split(row.Headline, q.Markers),
		})
	}
	return page, nil
}

// HitMessages implements app.SearchReadStore with the statement HitMessages.
func (r *Repository) HitMessages(ctx context.Context, messageIDs, channelIDs []ids.ID) ([]domain.Message, error) {
	rows, err := r.read(ctx).HitMessages(ctx, sqlcgen.HitMessagesParams{MessageIds: uuids(messageIDs), ChannelIds: uuids(channelIDs)})
	if err != nil {
		return nil, mapError(err, "reading the messages of search hits")
	}
	out := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMessage(messageRow(row)))
	}
	return out, nil
}

// searchError maps the error of a search statement (brief M3-14 design note 6): the statement
// timeout of the search unit (search.statement_timeout) is searchkit.ErrTimeout, which the
// coordinator answers with 503 search_timeout; a statement the caller cancelled returns the
// context error; anything else is the internal error of mapError.
func searchError(ctx context.Context, err error, what string) error {
	if pg.IsStatementTimeout(ctx, err) {
		return searchkit.ErrTimeout
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return mapError(err, what)
}

// threadFilter converts Query.ThreadIDs: nil is NULL (no restriction), any other list, also an
// empty one, restricts to its threads.
func threadFilter(list []ids.ID) []pgtype.UUID {
	if list == nil {
		return nil
	}
	return uuids(list)
}

// nullableID converts an optional id filter: nil is NULL.
func nullableID(id *ids.ID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return uuid(*id)
}

// timestamp converts an optional time filter: nil is NULL.
func timestamp(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
