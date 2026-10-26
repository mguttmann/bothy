// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// Retention is the participant of messaging in the job retention.content (M7-05,
// docs/spec/20-privacy.md 4.1 and 4.2): every message created before the cutoff (retention.content
// before now; the job skips the class while it is none) is deleted through the deletion path of
// message.delete, one unit of work per message: tombstone, revisions, mentions, reactions and the
// pin removed, the attachments through files (their purge job queued), the event payloads
// redacted, MessageDeleted published (runs removes the components) and message.deleted appended.
// Never a bare DELETE. The deletion has no actor, so it is not audited and deleted_by stays empty.
type Retention struct {
	s *Service
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// Retention returns the participant of the job retention.content over the service.
func (s *Service) Retention() *Retention { return &Retention{s: s} }

// Class returns retention.ClassContent.
func (*Retention) Class() retention.Class { return retention.ClassContent }

// Tables returns messages: the rows the job tombstones. The detail rows that go with a message
// (03 section 6.5) are listed with this parent in the coverage test of internal/wire.
func (*Retention) Tables() []string { return []string{"messages"} }

// Purge implements retention.Participant: at most batch messages, each in its own unit of work. A
// message deleted or moved to another channel since it was read is skipped. Errors: those of the
// storage and of the deletion path.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	list, err := r.s.d.Store.ExpiredMessages(ctx, cutoff, batch)
	if err != nil {
		return 0, false, err
	}
	deleted := 0
	for _, m := range list {
		done, err := r.remove(ctx, m)
		if err != nil {
			return deleted, false, err
		}
		if done {
			deleted++
		}
	}
	return deleted, len(list) >= batch && deleted > 0, nil
}

// remove deletes the candidate m in its own unit of work and reports whether it did.
func (r *Retention) remove(ctx context.Context, m ExpiredMessage) (bool, error) {
	done := false
	err := r.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		done = false
		row, found, err := r.s.d.Store.VisibleMessage(ctx, m.ID, []ids.ID{m.ChannelID})
		if err != nil || !found || !row.DeletedAt.IsZero() {
			return err
		}
		if err := r.s.remove(ctx, actor.Anonymous(), row, []ids.ID{m.ChannelID}); err != nil {
			return err
		}
		done = true
		return nil
	})
	if e, ok := errs.As(err); ok && e.Code == errs.CodeNotFound {
		// Moved to another channel since the read: the next run reads it with its channel.
		return false, nil
	}
	return done, err
}

// Journal implements messaging.JournalMessages over the service (brief M7-07): the deletion
// path of Retention for one message the deletion journal names.
type Journal struct {
	s *Service
}

// Compile-time check: Journal is the port of bothy restore finalize.
var _ messaging.JournalMessages = (*Journal)(nil)

// Journal returns the port of the deletion journal over the service.
func (s *Service) Journal() *Journal { return &Journal{s: s} }

// DeleteForJournal implements messaging.JournalMessages: it reads the channel of the message and
// deletes it in its own unit of work through the path of Retention.remove.
func (j *Journal) DeleteForJournal(ctx context.Context, id ids.ID) (bool, error) {
	m, found, err := j.s.d.Store.Message(ctx, id)
	if err != nil || !found || !m.DeletedAt.IsZero() {
		return false, err
	}
	return (&Retention{s: j.s}).remove(ctx, ExpiredMessage{ID: m.ID, ChannelID: m.ChannelID})
}
