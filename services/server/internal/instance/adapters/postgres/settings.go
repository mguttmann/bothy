// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository is the settings port of app.
var _ app.SettingsRepository = (*Repository)(nil)

// ListSettings returns every stored setting in key order (app.SettingsRepository).
func (r *Repository) ListSettings(ctx context.Context) ([]app.SettingRow, error) {
	rows, err := r.queries(ctx).ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	return settingRows(rows), nil
}

// LockSettings takes the advisory lock of the settings writers and then locks the stored rows of
// keys until the commit (app.SettingsRepository). It needs a unit of work, because a lock outside
// one would end with its autocommit. The rows are read in a statement after the advisory lock, so
// in READ COMMITTED they include every row a writer before committed.
func (r *Repository) LockSettings(ctx context.Context, keys []string) ([]app.SettingRow, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	q := sqlcgen.New(tx)
	if err := q.LockSettingsWriters(ctx); err != nil {
		return nil, err
	}
	sorted := slices.Clone(keys)
	slices.Sort(sorted)
	rows, err := q.LockSettings(ctx, sorted)
	if err != nil {
		return nil, err
	}
	return settingRows(rows), nil
}

// InsertSetting inserts the first row of a key (app.SettingsRepository); inserted is false when a
// concurrent writer inserted it first.
func (r *Repository) InsertSetting(ctx context.Context, row app.SettingRow) (app.SettingRow, bool, error) {
	rows, err := r.queries(ctx).InsertSetting(ctx, sqlcgen.InsertSettingParams{
		Key: row.Key, Value: row.Value, UpdatedBy: nullUUID(row.UpdatedBy), UpdatedAt: timestamptz(row.UpdatedAt),
	})
	if err != nil || len(rows) == 0 {
		return app.SettingRow{}, false, err
	}
	return settingRow(rows[0]), true, nil
}

// UpdateSetting writes a later value of a locked row with version + 1 (app.SettingsRepository).
func (r *Repository) UpdateSetting(ctx context.Context, row app.SettingRow) (app.SettingRow, error) {
	out, err := r.queries(ctx).UpdateSetting(ctx, sqlcgen.UpdateSettingParams{
		Key: row.Key, Value: row.Value, UpdatedBy: nullUUID(row.UpdatedBy), UpdatedAt: timestamptz(row.UpdatedAt),
	})
	if err != nil {
		return app.SettingRow{}, err
	}
	return settingRow(out), nil
}

// nullUUID returns NULL for the zero id.
func nullUUID(id ids.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return uuid(id)
}

// settingRows converts sqlc rows.
func settingRows(rows []sqlcgen.InstanceSetting) []app.SettingRow {
	out := make([]app.SettingRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, settingRow(r))
	}
	return out
}

// settingRow converts one sqlc row; jsonb comes back in the canonical form of PostgreSQL.
func settingRow(r sqlcgen.InstanceSetting) app.SettingRow {
	out := app.SettingRow{Key: r.Key, Value: slices.Clone(r.Value), Version: r.Version, UpdatedAt: r.UpdatedAt.Time.UTC()}
	if r.UpdatedBy.Valid {
		out.UpdatedBy = ids.ID(r.UpdatedBy.Bytes)
	}
	return out
}
