// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// MarkFilesDeleted implements app.Repository: deleted_at = at on the files of d that are not
// deleted yet, of d.Purpose and d.UploadedBy where they are set (M3-07, M3-10); it returns how
// many it marked.
func (r *Repository) MarkFilesDeleted(ctx context.Context, d app.Deletion, at time.Time) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.MarkFilesDeleted(ctx, sqlcgen.MarkFilesDeletedParams{DeletedAt: ts(at), Ids: uuids(d.FileIDs), Purpose: text(d.Purpose),
		UploadedBy: optionalUUID(d.UploadedBy)})
	return int(n), mapError(err, "marking files deleted")
}
