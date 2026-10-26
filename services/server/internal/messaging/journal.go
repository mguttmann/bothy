// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// JournalMessages is the inbound port through which bothy restore finalize deletes a message again
// that the deletion journal of docs/spec/20-privacy.md section 4.6 names and that the restored
// database holds (SEC-BAK-3, brief M7-07 design note 3).
type JournalMessages interface {
	// DeleteForJournal deletes the message id through the deletion of 20 section 4.2 without an
	// actor, like the job retention.content: tombstone, revisions, mentions, reactions, pin and
	// attachments removed (their purge queued), the event payloads redacted and message.deleted
	// without deleted_by appended, no audit entry. It reports whether it deleted; an unknown or
	// deleted message gives false. It runs its own unit of work.
	DeleteForJournal(ctx context.Context, id ids.ID) (bool, error)
}
