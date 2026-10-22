// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
)

// SaveChannelReadOnly implements app.Store: read_only and the version of c (M8-03).
func (r *Repository) SaveChannelReadOnly(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveChannelReadOnly(ctx, sqlcgen.SaveChannelReadOnlyParams{ID: uuid(c.ID), ReadOnly: c.ReadOnly, Version: c.Version})
	return mapError(err, "setting the read-only flag of a channel")
}
