// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// NotifySignals implements app.Signals for cluster.mode multi (M2-47 Design note 4): Stage
// executes NotifyEphemeral on the transaction of the unit of work, so PostgreSQL delivers the
// notification only after the commit and never after a rollback; Committed does nothing, because
// every node, the sender included, receives the notification through its EphemeralListener; Send
// executes NotifyEphemeral on the pool in its own statement. It is safe for concurrent use.
type NotifySignals struct {
	pool *pg.Pool
}

// Compile-time check.
var _ app.Signals = (*NotifySignals)(nil)

// NewNotifySignals returns the signals over pool.
func NewNotifySignals(pool *pg.Pool) *NotifySignals { return &NotifySignals{pool: pool} }

// Stage implements app.Signals. Errors: kind internal outside a unit of work or for a signal that
// does not encode; the errors of the statement.
func (n *NotifySignals) Stage(ctx context.Context, s domain.Signal) error {
	payload, err := domain.EncodeSignal(s)
	if err != nil {
		return mapError(err, "encoding a terminal signal")
	}
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	return mapError(sqlcgen.New(tx).NotifyEphemeral(ctx, string(payload)), "staging a terminal signal")
}

// Committed implements app.Signals: the listener of every node delivers the notification.
func (*NotifySignals) Committed(context.Context, domain.Signal) {}

// Send implements app.Signals. Errors: kind internal for a signal that does not encode; the errors
// of the statement.
func (n *NotifySignals) Send(ctx context.Context, s domain.Signal) error {
	payload, err := domain.EncodeSignal(s)
	if err != nil {
		return mapError(err, "encoding a terminal signal")
	}
	return mapError(sqlcgen.New(n.pool).NotifyEphemeral(ctx, string(payload)), "sending a terminal signal")
}

// HostStatesByIDs implements app.Store.
func (r *Repository) HostStatesByIDs(ctx context.Context, list []ids.ID) ([]app.HostState, error) {
	rows, err := r.read(ctx).HostStatesByIDs(ctx, uuids(list))
	if err != nil {
		return nil, mapError(err, "reading host states")
	}
	out := make([]app.HostState, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.HostState{ID: idOf(row.ID), State: domain.State(row.State), DisabledReason: row.DisabledReason.String})
	}
	return out, nil
}
