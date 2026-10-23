// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the repository of the import files (M8-11).
var _ app.ImportRepository = (*Repository)(nil)

// ReopenSession implements app.ImportRepository: the session opens again, its parts go and its
// file is pending again, in the caller's unit of work.
func (r *Repository) ReopenSession(ctx context.Context, fileID ids.ID, createdAt, expiresAt time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.ReopenImportSession(ctx, sqlcgen.ReopenImportSessionParams{FileID: uuid(fileID), CreatedAt: ts(createdAt), ExpiresAt: ts(expiresAt)})
	if err != nil {
		return mapError(err, "reopening an import session")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "files: reopening a session that is completed or missing")
	}
	if err := q.DeleteParts(ctx, uuid(fileID)); err != nil {
		return mapError(err, "deleting parts")
	}
	return mapError(q.PendImportFile(ctx, uuid(fileID)), "setting an import file pending")
}
