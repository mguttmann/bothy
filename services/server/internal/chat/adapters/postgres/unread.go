// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the badge read (M3-19).
var _ app.UnreadStore = (*Repository)(nil)

// UnreadCandidates implements app.UnreadStore with the one statement UnreadCandidates; outside a
// unit of work it reads on the pool.
func (r *Repository) UnreadCandidates(ctx context.Context, channels []ids.ID) ([]chat.ThreadPosition, error) {
	rows, err := r.read(ctx).UnreadCandidates(ctx, uuids(channels))
	if err != nil {
		return nil, mapError(err, "reading unread candidates")
	}
	out := make([]chat.ThreadPosition, len(rows))
	for i, row := range rows {
		out[i] = chat.ThreadPosition{ThreadID: idOf(row.ID), ChannelID: idOf(row.ChannelID), ChannelKind: row.ChannelKind,
			LastMessageSeq: row.LastMessageSeq, UnreadFloorSeq: row.UnreadFloorSeq}
	}
	return out, nil
}
