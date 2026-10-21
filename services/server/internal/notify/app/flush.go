// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// Flush is the use case of the job notify.coalesce_flush of (user, thread) at the end of its
// bundling window end (13 4.5, brief M3-19 design note 9 d, D-M3-19-8), in one unit of work: it
// gathers the facts of the person for the thread at the flush time, computes the unread count by
// the formula of 03 read_states (threads.last_message_seq minus the greater of last_read_seq and
// threads.unread_floor_seq) and lets domain.Flush decide. A send carries the unread count as
// Count, points at the newest message of the thread (no message ID if none), has t0 = end and
// opens the next window. A person who is no longer an active human, and a DM thread (whose hints
// are never collected), give nothing.
//
// Errors: those of the reads, the decision and the inserts; River retries the job.
func (p *Push) Flush(ctx context.Context, user, thread ids.ID, end time.Time) error {
	return p.d.UoW.Do(ctx, func(ctx context.Context) error {
		users, err := p.activeHumans(ctx, []ids.ID{user})
		if err != nil || len(users) == 0 {
			return err
		}
		info, err := p.d.Threads.Info(ctx, thread)
		if err != nil {
			return err
		}
		if info.ChannelKind == channelKindDM {
			return nil
		}
		now := p.d.Clock.Now()
		g := group{kind: domain.KindMessage, threadID: thread, channelID: info.ChannelID, spaceID: info.SpaceID}
		facts, err := p.gather(ctx, g, users, now)
		if err != nil {
			return err
		}
		pos, err := p.d.Positions.PositionsFor(ctx, thread, users)
		if err != nil {
			return err
		}
		unread := info.LastMessageSeq - max(pos[user], info.UnreadFloorSeq)
		d, err := domain.Flush(facts[user], domain.FlushState{Collected: 1, Unread: unread})
		if err != nil {
			return err
		}
		h := Hint{RecipientID: user, ThreadID: thread, Kind: domain.KindMessage, DueAt: end}
		if d.Outcome == domain.OutcomeSend {
			newest, found, err := p.d.Messages.Newest(ctx, thread)
			if err != nil {
				return err
			}
			if found {
				h.MessageID = newest.ID
			}
		}
		return p.apply(ctx, h, facts[user], d)
	})
}
