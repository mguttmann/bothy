// SPDX-License-Identifier: Apache-2.0

// Package postgres implements the outgoing ports of the gateway across several api nodes over
// PostgreSQL (M7-17, R-DM-51): the presence writer's store and the presence reader over the table
// gateway_presence, which only the gateway reads and writes (03 principle 12, ADR 0050), and the
// sender of the gateway signals on the channel ephemeral.
package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// PresenceStore implements app.PresenceStore over gateway_presence. It is safe for concurrent use.
type PresenceStore struct {
	pool *pg.Pool
}

// Compile-time check.
var _ app.PresenceStore = (*PresenceStore)(nil)

// NewPresenceStore returns the store over pool.
func NewPresenceStore(pool *pg.Pool) *PresenceStore { return &PresenceStore{pool: pool} }

// Flush implements app.PresenceStore in one transaction: the upsert of rows, the delete of the
// node's gone rows and the delete of the stale rows of every node. Errors: those of the database,
// after a rollback.
func (s *PresenceStore) Flush(ctx context.Context, node ids.ID, rows []app.PresenceRow, at, staleBefore time.Time) (err error) {
	tx, err := s.pool.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	q := sqlcgen.New(tx)
	sessions := make([]pgtype.UUID, 0, len(rows))
	if len(rows) > 0 {
		p := sqlcgen.UpsertPresenceParams{NodeID: pg.UUID(node), At: pgtype.Timestamptz{Time: at, Valid: true}}
		for _, r := range rows {
			sessions = append(sessions, pg.UUID(r.SessionID))
			p.PrincipalIds = append(p.PrincipalIds, pg.UUID(r.PrincipalID))
			p.DeviceIds = append(p.DeviceIds, pg.UUID(r.DeviceID))
			p.Focus = append(p.Focus, uuidArray(r.Focus))
			p.Idle = append(p.Idle, r.Idle)
		}
		p.SessionIds = sessions
		if err = q.UpsertPresence(ctx, p); err != nil {
			return err
		}
	}
	if err = q.DeleteGonePresence(ctx, sqlcgen.DeleteGonePresenceParams{NodeID: pg.UUID(node), SessionIds: sessions}); err != nil {
		return err
	}
	if err = q.DeleteStalePresence(ctx, pgtype.Timestamptz{Time: staleBefore, Valid: true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteNode implements app.PresenceStore.
func (s *PresenceStore) DeleteNode(ctx context.Context, node ids.ID) error {
	return sqlcgen.New(s.pool).DeleteNodePresence(ctx, pg.UUID(node))
}

// uuidArray returns the PostgreSQL array literal of list, for example {a,b}; {} when empty.
func uuidArray(list []ids.ID) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, id := range list {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(id.String())
	}
	b.WriteByte('}')
	return b.String()
}

// PresenceView implements the kernel port presence.View over gateway_presence (13 12 point 4,
// R-DM-51): the presence of every api node for the decision of the consumer push in a process
// without the hub of those connections (role worker, and role all with cluster.mode multi). It
// answers exactly what the hub's view answers for the same connections: per person one entry per
// device, the newest row of the device first; rows not refreshed within cluster.presence_ttl are
// ignored. It is safe for concurrent use.
type PresenceView struct {
	pool *pg.Pool
	ttl  time.Duration
	clk  clock.Clock
}

// Compile-time check.
var _ presence.View = (*PresenceView)(nil)

// NewPresenceView returns the view over pool that ignores rows older than ttl
// (cluster.presence_ttl) at the time of clk (nil means clock.System()). Errors: a ttl that is not
// positive.
func NewPresenceView(pool *pg.Pool, ttl time.Duration, clk clock.Clock) (*PresenceView, error) {
	if pool == nil || ttl <= 0 {
		return nil, errors.New("postgres: the presence view needs the pool and a positive cluster.presence_ttl")
	}
	if clk == nil {
		clk = clock.System()
	}
	return &PresenceView{pool: pool, ttl: ttl, clk: clk}, nil
}

// Connections implements presence.View. People without a fresh row are absent; every returned
// slice is new. Errors: those of the database (the caller rolls its round back).
func (v *PresenceView) Connections(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]presence.Connection, error) {
	out := map[ids.ID][]presence.Connection{}
	if len(userIDs) == 0 {
		return out, nil
	}
	people := make([]pgtype.UUID, 0, len(userIDs))
	for _, u := range userIDs {
		people = append(people, pg.UUID(u))
	}
	rows, err := sqlcgen.New(pg.Querier(ctx, v.pool)).PresenceOf(ctx, sqlcgen.PresenceOfParams{
		PrincipalIds: people, FreshAfter: pgtype.Timestamptz{Time: v.clk.Now().Add(-v.ttl), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	seen := map[[2]ids.ID]bool{}
	for _, r := range rows {
		person, device := pg.IDOf(r.PrincipalID), pg.IDOf(r.DeviceID)
		if seen[[2]ids.ID{person, device}] {
			continue
		}
		seen[[2]ids.ID{person, device}] = true
		focus := make([]ids.ID, 0, len(r.FocusThreadIds))
		for _, f := range r.FocusThreadIds {
			focus = append(focus, pg.IDOf(f))
		}
		out[person] = append(out[person], presence.Connection{DeviceID: device, Focus: focus, Idle: r.Idle})
	}
	return out, nil
}

// SignalSender implements app.SignalSender with pg_notify on the pool, in its own statement. It is
// safe for concurrent use.
type SignalSender struct {
	pool *pg.Pool
}

// Compile-time check.
var _ app.SignalSender = (*SignalSender)(nil)

// NewSignalSender returns the sender over pool.
func NewSignalSender(pool *pg.Pool) *SignalSender { return &SignalSender{pool: pool} }

// Send implements app.SignalSender.
func (s *SignalSender) Send(ctx context.Context, payload []byte) error {
	return sqlcgen.New(s.pool).NotifyEphemeral(ctx, string(payload))
}
