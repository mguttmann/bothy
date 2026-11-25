// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
)

// TitleRetention is the participant of chat in the job retention.content (docs/spec/20-privacy.md
// 4.1, AUD-PKG-R2-owner-server): a live thread keeps its row (03 principle 7) but loses its title
// once its last activity lies before the cutoff, with the period of the messages. The title
// source stays; each thread is emptied in its own unit of work with thread.updated, so clients
// drop the title too. The change has no actor: no audit entry. A deleted thread is left to the
// purge of messaging.purge_deleted, which empties its title itself.
type TitleRetention struct {
	s *Service
}

// Compile-time check: TitleRetention is a participant of kernel/retention.
var _ retention.Participant = (*TitleRetention)(nil)

// TitleRetention returns the participant of the job retention.content over the service.
func (s *Service) TitleRetention() *TitleRetention { return &TitleRetention{s: s} }

// Class returns retention.ClassContent.
func (*TitleRetention) Class() retention.Class { return retention.ClassContent }

// Tables returns threads: the rows whose title Purge empties.
func (*TitleRetention) Tables() []string { return []string{"threads"} }

// Purge implements retention.Participant: at most batch threads, each in its own unit of work. A
// thread that got a message, lost its title or was deleted since it was read is skipped.
//
// Errors: those of the storage and of the event log.
func (r *TitleRetention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	list, err := r.s.d.Store.ExpiredThreadTitles(ctx, cutoff, batch)
	if err != nil {
		return 0, false, err
	}
	emptied := 0
	for _, id := range list {
		done, err := r.clear(ctx, id, cutoff)
		if err != nil {
			return emptied, false, err
		}
		if done {
			emptied++
		}
	}
	return emptied, len(list) >= batch && emptied > 0, nil
}

// clear empties the title of the thread id in its own unit of work and reports whether it did.
func (r *TitleRetention) clear(ctx context.Context, id ids.ID, cutoff time.Time) (bool, error) {
	done := false
	err := r.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		done = false
		th, found, err := r.s.d.Store.LockThread(ctx, id)
		if err != nil || !found || th.Title == "" || th.Deletion.Deleted() || !th.LastActivityAt.Before(cutoff) {
			return err
		}
		th.SetTitle("", th.TitleSource)
		if err := r.s.d.Store.SaveThread(ctx, th); err != nil {
			return err
		}
		done = true
		return r.s.emitThreadChange(ctx, th)
	})
	return done, err
}
