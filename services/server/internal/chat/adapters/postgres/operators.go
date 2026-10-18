// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"bytes"
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SaveSpaceOperator implements app.Store.
func (r *Repository) SaveSpaceOperator(ctx context.Context, s domain.Space) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveSpaceOperator(ctx, sqlcgen.SaveSpaceOperatorParams{
		ID: uuid(s.ID), DefaultOperatorAgentID: optionalUUID(s.DefaultOperatorAgentID), Version: s.Version,
	})
	return mapError(err, "setting the default operator")
}

// SaveChannelOperator implements app.Store.
func (r *Repository) SaveChannelOperator(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveChannelOperator(ctx, sqlcgen.SaveChannelOperatorParams{
		ID: uuid(c.ID), OperatorInherit: c.OperatorInherit, OperatorAgentID: optionalUUID(c.OperatorAgentID), Version: c.Version,
	})
	return mapError(err, "setting the channel operator")
}

// TouchChannel implements app.Store.
func (r *Repository) TouchChannel(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.TouchChannel(ctx, sqlcgen.TouchChannelParams{ID: uuid(c.ID), Version: c.Version}), "updating a channel version")
}

// LockInheritingChannels implements app.Store.
func (r *Repository) LockInheritingChannels(ctx context.Context, space ids.ID) ([]app.ChannelRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockInheritingChannels(ctx, uuid(space))
	if err != nil {
		return nil, mapError(err, "locking the inheriting channels")
	}
	out := make([]app.ChannelRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChannel(sqlcgen.GetChannelRow(row)))
	}
	return out, nil
}

// ServedTargets implements app.Store: the served channels and the spaces of their default
// operator, merged, without duplicates, ascending by (SpaceID, ChannelID); the zero channel of a
// space entry sorts first.
func (r *Repository) ServedTargets(ctx context.Context, agent ids.ID) ([]app.ServedTarget, error) {
	q := r.read(ctx)
	chans, err := q.ServedChannels(ctx, uuid(agent))
	if err != nil {
		return nil, mapError(err, "reading the served channels")
	}
	spaces, err := q.DefaultOperatedSpaces(ctx, uuid(agent))
	if err != nil {
		return nil, mapError(err, "reading the served spaces")
	}
	out := make([]app.ServedTarget, 0, len(chans)+len(spaces))
	for _, c := range chans {
		out = append(out, app.ServedTarget{ChannelID: idOf(c.ID), SpaceID: idOf(c.SpaceID)})
	}
	for _, s := range spaces {
		out = append(out, app.ServedTarget{SpaceID: idOf(s)})
	}
	slices.SortFunc(out, func(a, b app.ServedTarget) int {
		if c := bytes.Compare(a.SpaceID[:], b.SpaceID[:]); c != 0 {
			return c
		}
		return bytes.Compare(a.ChannelID[:], b.ChannelID[:])
	})
	return slices.Compact(out), nil
}

// Thread implements app.Store.
func (r *Repository) Thread(ctx context.Context, id ids.ID) (app.ThreadRow, bool, error) {
	row, err := r.read(ctx).GetThreadFacts(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ThreadRow{}, false, nil
	}
	if err != nil {
		return app.ThreadRow{}, false, mapError(err, "reading a thread")
	}
	return app.ThreadRow{
		ID: idOf(row.ID), ChannelID: idOf(row.ChannelID), Main: row.Kind == domain.ThreadKindMain,
		OperatorAgentID: idOf(row.OperatorAgentID), CreatedBy: idOf(row.CreatedBy),
	}, true, nil
}
