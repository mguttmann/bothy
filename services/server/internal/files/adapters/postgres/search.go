// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the read operations for the search coordinator (M3-14).
var _ app.SearchReadStore = (*Repository)(nil)

// SearchFiles implements app.SearchReadStore with the one statement SearchFiles in the ambient
// unit of work of the coordinator (its statement_timeout is search.statement_timeout). It fetches
// the page size plus one row to know whether a next page exists; the extra row is neither returned
// nor highlighted. Errors as searchError.
func (r *Repository) SearchFiles(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	newest := q.Sort == searchkit.SortNewest
	arg := sqlcgen.SearchFilesParams{
		Tsq: q.TSQuery, ChannelIds: uuids(q.ChannelIDs), FilterThreads: q.ThreadIDs != nil, ThreadIds: uuids(q.ThreadIDs),
		RefTime: ts(q.RefTime), MaxCandidates: int32(q.Limits.MaxCandidates), //nolint:gosec // G115: search.max_candidates
		RankNormalization: int32(q.Limits.RankNormalization), //nolint:gosec // G115: search.rank_normalization
		RecencySeconds:    q.Limits.RecencySeconds(), Newest: newest,
		PageSize:         int32(q.Limits.PageSize),              //nolint:gosec // G115: capped at search.page_size_max
		HeadlineMaxChars: int32(q.Limits.HeadlineInputMaxChars), //nolint:gosec // G115: search.headline_input_max_chars
		HeadlineOptions:  q.Limits.HeadlineOptions(q.Markers),
	}
	if q.AuthorID != nil {
		arg.AuthorID = uuid(*q.AuthorID)
	}
	if q.After != nil {
		arg.After = ts(*q.After)
	}
	if q.Before != nil {
		arg.Before = ts(*q.Before)
	}
	if q.From != nil {
		arg.HasFrom, arg.FromID, arg.FromScore, arg.FromCreatedAt = true, uuid(q.From.ID), q.From.Score, ts(q.From.CreatedAt)
	}
	rows, err := r.read(ctx).SearchFiles(ctx, arg)
	if err != nil {
		return searchkit.Page{}, searchError(ctx, err, "searching files")
	}
	page := searchkit.Page{Hits: []searchkit.Hit{}}
	for _, row := range rows {
		page.CandidatesCapped = row.Capped
		if !row.FileID.Valid {
			// The one row of an empty page: only the flag.
			continue
		}
		if len(page.Hits) == q.Limits.PageSize {
			last := page.Hits[len(page.Hits)-1]
			page.Next = &searchkit.Position{ID: last.ID}
			if newest {
				page.Next.CreatedAt = last.CreatedAt
			} else {
				page.Next.Score = last.Score
			}
			break
		}
		page.Hits = append(page.Hits, searchkit.Hit{
			ID: idOf(row.FileID), MessageID: idOf(row.MessageID), ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID),
			Score: row.Score, CreatedAt: timeOf(row.CreatedAt), Segments: searchkit.Split(row.Headline, q.Markers),
		})
	}
	return page, nil
}

// HitFiles implements app.SearchReadStore: the files in one statement, their thumbnails in one
// more, both with the visibility predicate over channelIDs.
func (r *Repository) HitFiles(ctx context.Context, fileIDs, channelIDs []ids.ID) ([]app.FileRecord, error) {
	q := r.read(ctx)
	rows, err := q.HitFiles(ctx, sqlcgen.HitFilesParams{FileIds: uuids(fileIDs), ChannelIds: uuids(channelIDs)})
	if err != nil {
		return nil, mapError(err, "reading the files of search hits")
	}
	out := make([]app.FileRecord, 0, len(rows))
	found := make([]ids.ID, 0, len(rows))
	for _, row := range rows {
		out = append(out, fileRecord(sqlcgen.GetFileRow(row)))
		found = append(found, idOf(row.ID))
	}
	if len(found) == 0 {
		return out, nil
	}
	thumbs, err := q.HitFileThumbnails(ctx, sqlcgen.HitFileThumbnailsParams{FileIds: uuids(found), ChannelIds: uuids(channelIDs)})
	if err != nil {
		return nil, mapError(err, "listing the thumbnails of search hits")
	}
	byFile := make(map[ids.ID][]app.ThumbnailRecord, len(found))
	for _, t := range thumbs {
		id := idOf(t.FileID)
		byFile[id] = append(byFile[id], app.ThumbnailRecord{Variant: domain.Variant(t.Variant), ObjectKey: t.ObjectKey, MIMEType: t.MimeType,
			Width: int(t.Width), Height: int(t.Height), SizeBytes: t.SizeBytes})
	}
	for i := range out {
		out[i].Thumbnails = byFile[out[i].ID]
	}
	return out, nil
}

// searchError maps the error of a search statement (files.SearchReads.SearchFiles): the context
// error when ctx ended, searchkit.ErrTimeout when the statement hit statement_timeout
// (pg.IsStatementTimeout: SQLSTATE 57014 while ctx is alive), otherwise mapError.
func searchError(ctx context.Context, err error, what string) error {
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case pg.IsStatementTimeout(ctx, err):
		return searchkit.ErrTimeout
	}
	return mapError(err, what)
}
