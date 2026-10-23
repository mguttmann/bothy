// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// MarkFilesDeleted implements app.Repository: deleted_at = at on the files of d that are not
// deleted yet, of d.Purpose and d.UploadedBy where they are set (M3-07, M3-10); it returns the
// ids it marked (M3-08).
func (r *Repository) MarkFilesDeleted(ctx context.Context, d app.Deletion, at time.Time) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.MarkFilesDeleted(ctx, sqlcgen.MarkFilesDeletedParams{DeletedAt: ts(at), Ids: uuids(d.FileIDs), Purpose: text(d.Purpose),
		UploadedBy: optionalUUID(d.UploadedBy)})
	if err != nil {
		return nil, mapError(err, "marking files deleted")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out, nil
}
