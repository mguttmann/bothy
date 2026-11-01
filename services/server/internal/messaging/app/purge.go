// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Purge is the use case of the job messaging.purge_deleted (FEAT-delete-spaces-chats, D-DEL-1,
// 20 4.2a): for every space, channel and thread whose undo window passed and that is not purged
// yet, oldest deletion first, it deletes the live messages through the deletion path of 20 4.2
// (tombstone, revisions, mentions, reactions, pin, attachments to files.purge_objects, event
// redaction, message.deleted without deleted_by, no audit entry), each message in its own unit of
// work, in batches; then chat clears the titles and topics and sets purged_at. It is idempotent
// and resumable: a run that stops anywhere leaves the item due, and the next run goes on with
// the messages that are left.
type Purge struct {
	r     *Retention
	chat  chat.Purges
	batch int
}

// Purge returns the use case over the deletion path of the service, the purge port of chat and
// the batch size (retention.batch_size); a batch below 1 is 1.
func (s *Service) Purge(p chat.Purges, batch int) *Purge {
	return &Purge{r: s.Retention(), chat: p, batch: max(batch, 1)}
}

// Run purges every due item and returns the number of deleted messages. Errors: those of the
// database, and internal when a round over the messages of an item deletes none of them, so a
// persistent refusal cannot loop.
func (p *Purge) Run(ctx context.Context) (int, error) {
	total := 0
	for {
		items, err := p.chat.DuePurges(ctx, p.batch)
		if err != nil {
			return total, err
		}
		for _, t := range items {
			n, err := p.item(ctx, t)
			total += n
			if err != nil {
				return total, err
			}
		}
		if len(items) < p.batch {
			return total, nil
		}
	}
}

// item deletes the live messages of t in batches and then finishes it in chat.
func (p *Purge) item(ctx context.Context, t chat.PurgeTarget) (int, error) {
	total := 0
	for {
		list, err := p.r.s.d.Store.LiveMessagesOfDeleted(ctx, t.ChannelIDs, t.ThreadIDs, p.batch)
		if err != nil {
			return total, err
		}
		if len(list) == 0 {
			break
		}
		deleted := 0
		for _, m := range list {
			done, err := p.r.remove(ctx, m)
			if err != nil {
				return total + deleted, err
			}
			if done {
				deleted++
			}
		}
		total += deleted
		if deleted == 0 {
			return total, errs.New(errs.CodeInternalError, "messaging: a purge round deleted no message of a deleted item")
		}
	}
	return total, p.chat.FinishPurge(ctx, t)
}
