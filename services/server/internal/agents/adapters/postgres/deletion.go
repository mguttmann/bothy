// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the source port of chat that counts the agent sessions of a deletion.
var _ chat.SessionCounter = (*Repository)(nil)

// SessionsOfThreads implements chat.SessionCounter (FEAT-delete-spaces-chats): the agent sessions
// of threads, read without authorization.
func (r *Repository) SessionsOfThreads(ctx context.Context, threadIDs []ids.ID) (int64, error) {
	list := make([]pgtype.UUID, 0, len(threadIDs))
	for _, id := range threadIDs {
		list = append(list, uuid(id))
	}
	n, err := r.read(ctx).CountSessionsOfThreads(ctx, list)
	if err != nil {
		return 0, mapError(err, "counting the agent sessions of threads")
	}
	return n, nil
}
