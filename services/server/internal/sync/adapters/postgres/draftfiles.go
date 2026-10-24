// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
)

// DraftFiles implements the source port files.DraftFileSource over drafts (M3-08,
// docs/spec/12-files.md section 12 row "Never attached"; 04 section 14 row "Drafts"): a file stays
// while a draft of its own uploader lists it in file_ids; thread and channel drafts count alike,
// and a draft of a channel the author lost still counts, because it stays stored. It locks no
// draft. It is safe for concurrent use.
//
// Example:
//
//	src := postgres.NewDraftFiles(pool)
//	held, err := src.HeldByUploader(ctx, []files.UploaderFile{{FileID: f, UploaderID: alex}})
type DraftFiles struct {
	pool *pg.Pool
}

// Compile-time check: the source port of files.
var _ files.DraftFileSource = (*DraftFiles)(nil)

// NewDraftFiles returns the source over pool; it opens no connection.
func NewDraftFiles(pool *pg.Pool) *DraftFiles { return &DraftFiles{pool: pool} }

// HeldByUploader implements files.DraftFileSource: the files of fs that a draft of their own
// uploader holds, in the ambient unit of work of ctx or on the pool. Errors: internal for a
// database error.
func (d *DraftFiles) HeldByUploader(ctx context.Context, fs []files.UploaderFile) (map[ids.ID]bool, error) {
	out := map[ids.ID]bool{}
	if len(fs) == 0 {
		return out, nil
	}
	fileIDs := make([]pgtype.UUID, len(fs))
	uploaders := make([]pgtype.UUID, len(fs))
	for i, f := range fs {
		fileIDs[i], uploaders[i] = uuid(f.FileID), uuid(f.UploaderID)
	}
	rows, err := sqlcgen.New(pg.Querier(ctx, d.pool)).HeldByUploaderDrafts(ctx, sqlcgen.HeldByUploaderDraftsParams{FileIds: fileIDs,
		UploaderIds: uploaders})
	if err != nil {
		return nil, failed(err, "reading the drafts of uploaders")
	}
	for _, r := range rows {
		if r.Valid {
			out[ids.ID(r.Bytes)] = true
		}
	}
	return out, nil
}
