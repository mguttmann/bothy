// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository is the flag port of app.
var _ app.FlagRepository = (*Repository)(nil)

// rolloutJSON is the stored shape of feature_flag_rollout.v1: empty lists are omitted, so an empty
// rollout is {}.
type rolloutJSON struct {
	UserIDs  []ids.ID `json:"user_ids,omitempty"`
	GroupIDs []ids.ID `json:"group_ids,omitempty"`
}

// GetFlag reads one flag by its primary key (app.FlagRepository).
func (r *Repository) GetFlag(ctx context.Context, key string) (app.FlagRow, bool, error) {
	rows, err := r.queries(ctx).GetFlag(ctx, key)
	return firstFlag(rows, err)
}

// ListFlags returns every stored flag in key order (app.FlagRepository).
func (r *Repository) ListFlags(ctx context.Context) ([]app.FlagRow, error) {
	rows, err := r.queries(ctx).ListFlags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]app.FlagRow, 0, len(rows))
	for _, row := range rows {
		f, err := flagRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// LockFlag reads one flag under its row lock (app.FlagRepository). It needs a unit of work,
// because a lock outside one would end with its autocommit.
func (r *Repository) LockFlag(ctx context.Context, key string) (app.FlagRow, bool, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return app.FlagRow{}, false, err
	}
	rows, err := sqlcgen.New(tx).LockFlag(ctx, key)
	return firstFlag(rows, err)
}

// InsertFlag inserts the first row of a flag (app.FlagRepository); inserted is false when a
// concurrent writer inserted it first.
func (r *Repository) InsertFlag(ctx context.Context, row app.FlagRow) (app.FlagRow, bool, error) {
	raw, err := marshalRollout(row.Rollout)
	if err != nil {
		return app.FlagRow{}, false, err
	}
	rows, err := r.queries(ctx).InsertFlag(ctx, sqlcgen.InsertFlagParams{
		Key: row.Key, Enabled: row.Enabled, Rollout: raw, UpdatedBy: nullUUID(row.UpdatedBy), UpdatedAt: timestamptz(row.UpdatedAt),
	})
	return firstFlag(rows, err)
}

// UpdateFlag writes a later value of a locked row with version + 1 (app.FlagRepository).
func (r *Repository) UpdateFlag(ctx context.Context, row app.FlagRow) (app.FlagRow, error) {
	raw, err := marshalRollout(row.Rollout)
	if err != nil {
		return app.FlagRow{}, err
	}
	out, err := r.queries(ctx).UpdateFlag(ctx, sqlcgen.UpdateFlagParams{
		Key: row.Key, Enabled: row.Enabled, Rollout: raw, UpdatedBy: nullUUID(row.UpdatedBy), UpdatedAt: timestamptz(row.UpdatedAt),
	})
	if err != nil {
		return app.FlagRow{}, err
	}
	return flagRow(out)
}

// firstFlag converts the one row of a :many query; found is false without one.
func firstFlag(rows []sqlcgen.FeatureFlag, err error) (app.FlagRow, bool, error) {
	if err != nil || len(rows) == 0 {
		return app.FlagRow{}, false, err
	}
	f, err := flagRow(rows[0])
	if err != nil {
		return app.FlagRow{}, false, err
	}
	return f, true, nil
}

// marshalRollout writes the normalized rollout in its stored shape.
func marshalRollout(r domain.Rollout) ([]byte, error) {
	n := domain.NormalizeRollout(r)
	raw, err := json.Marshal(rolloutJSON{UserIDs: n.UserIDs, GroupIDs: n.GroupIDs})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "instance: the rollout of a flag")
	}
	return raw, nil
}

// flagRow converts one sqlc row. Errors: a rollout that is not feature_flag_rollout.v1 (internal).
func flagRow(r sqlcgen.FeatureFlag) (app.FlagRow, error) {
	var ro rolloutJSON
	if err := json.Unmarshal(r.Rollout, &ro); err != nil {
		return app.FlagRow{}, errs.Wrap(errs.CodeInternalError, err, "instance: the stored rollout of a flag is not feature_flag_rollout.v1")
	}
	out := app.FlagRow{
		Key: r.Key, Enabled: r.Enabled, Version: r.Version, UpdatedAt: r.UpdatedAt.Time.UTC(),
		Rollout: domain.NormalizeRollout(domain.Rollout{UserIDs: ro.UserIDs, GroupIDs: ro.GroupIDs}),
	}
	if r.UpdatedBy.Valid {
		out.UpdatedBy = ids.ID(r.UpdatedBy.Bytes)
	}
	return out, nil
}
