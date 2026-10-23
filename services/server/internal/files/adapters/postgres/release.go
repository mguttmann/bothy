// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReleaseFile implements app.Repository (M3-10): the deleted mark of the file fileID of purpose,
// of uploadedBy unless it is zero, at at; released reports whether this call set it.
func (r *Repository) ReleaseFile(ctx context.Context, fileID ids.ID, purpose string, uploadedBy ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.ReleaseFile(ctx, sqlcgen.ReleaseFileParams{DeletedAt: ts(at), ID: uuid(fileID), Purpose: purpose, UploadedBy: optionalUUID(uploadedBy)})
	return n == 1, mapError(err, "releasing a file")
}
