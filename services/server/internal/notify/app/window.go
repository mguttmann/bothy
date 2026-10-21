// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The bundling window of 13 4.5 lives only in two unique River jobs per (recipient, thread)
// (brief M3-19 design note 9, D-M3-19-7): notify.coalesce_window marks an open window and is
// scheduled at its end, notify.coalesce_flush exists exactly when a message hint was collected in
// that window and is scheduled at the same end. A send that opens a window inserts the marker
// (Scheduler.OpenWindow); when a late marker of the previous window still stands, the hint goes
// without a new window. A collect inserts the flush (Scheduler.Collect); a duplicate means the
// window already collected.

// window returns the open bundling window of (user, thread): the ScheduledAt of the pending
// notify.coalesce_window when it is after now, else none (design note 9 c).
func (p *Push) window(ctx context.Context, user, thread ids.ID, now time.Time) (domain.Window, error) {
	end, ok, err := p.d.Scheduler.WindowEnd(ctx, user, thread)
	if err != nil || !ok || !end.After(now) {
		return domain.Window{}, err
	}
	return domain.Window{End: end}, nil
}
