// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the source port of chat that counts the files of a deletion.
var _ chat.FileCounter = (*Repository)(nil)

// FilesOfThreads implements chat.FileCounter (FEAT-delete-spaces-chats): the files attached to the
// messages of threads, read without authorization.
func (r *Repository) FilesOfThreads(ctx context.Context, threadIDs []ids.ID) (int64, error) {
	n, err := r.read(ctx).CountFilesOfThreads(ctx, uuids(threadIDs))
	if err != nil {
		return 0, mapError(err, "counting the files of threads")
	}
	return n, nil
}
