// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SetAvatar implements app.Store.
func (r *Repository) SetAvatar(ctx context.Context, id, fileID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.SetPrincipalAvatar(ctx, sqlcgen.SetPrincipalAvatarParams{ID: uuid(id), AvatarFileID: optionalUUID(fileID)})
	if err != nil {
		return mapError(err, "setting an avatar")
	}
	if n != 1 {
		// The use case locked the person first, so a missing row is a programming error.
		return errs.New(errs.CodeInternalError, "identity: the avatar of an unknown person")
	}
	return nil
}
