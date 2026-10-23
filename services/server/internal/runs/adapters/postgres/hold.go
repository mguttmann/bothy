// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LockHeldRunsOfMessage implements app.Repository: the held queued runs of the message, locked in
// ascending id (queries/hold.sql). Errors: internal outside a unit of work.
func (r *Repository) LockHeldRunsOfMessage(ctx context.Context, messageID ids.ID) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockHeldRunsOfMessage(ctx, uuid(messageID))
	if err != nil {
		return nil, mapError(err, "locking the held runs of a message")
	}
	out := make([]ids.ID, len(rows))
	for i, id := range rows {
		out[i] = idOf(id)
	}
	return out, nil
}

// ReleaseHeldInvocation implements app.Repository: invocation_held_until = NULL and version + 1
// for a queued held run; released is false when no row matched. Errors: internal outside a unit
// of work.
func (r *Repository) ReleaseHeldInvocation(ctx context.Context, id ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.ReleaseHeldInvocation(ctx, uuid(id))
	if err != nil {
		return false, mapError(err, "releasing a held invocation")
	}
	return n == 1, nil
}
