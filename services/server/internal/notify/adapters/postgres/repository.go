// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the notify module (M3-18): the Repository
// implements the outgoing port app.Store over the sqlc queries of notification_preferences and
// quiet_hours in queries/. Only this module reads and writes the two tables (03 principle 12).
package postgres

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store. Reads use the ambient transaction of ctx when there is one and
// the pool otherwise; writes and locks need a unit of work. Errors are kernel/errs errors without
// SQL text or values. It is safe for concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: the repository is the outgoing port of app.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool; it opens no connection.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries on the ambient transaction or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries on the transaction of the unit of work; internal outside one.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// failed wraps a database error of what as internal; errs errors pass unchanged.
func failed(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "notify: "+what+" failed")
}

// UpsertPreference implements app.Store with one statement; no returned row is no change.
func (r *Repository) UpsertPreference(ctx context.Context, user ids.ID, p notify.Preference, at time.Time) (notify.Preference, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return notify.Preference{}, false, err
	}
	row, err := q.UpsertPreference(ctx, sqlcgen.UpsertPreferenceParams{
		UserID: uuid(user), TargetKind: p.Target.Kind, TargetID: uuid(p.Target.ID), Level: p.Level, MutedUntil: stamp(p.MutedUntil),
		UpdatedAt: stamp(at),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return notify.Preference{}, false, nil
	}
	if err != nil {
		return notify.Preference{}, false, failed(err, "writing a notification preference")
	}
	return preference(row), true, nil
}

// Preference implements app.Store.
func (r *Repository) Preference(ctx context.Context, user ids.ID, t notify.Target) (notify.Preference, bool, error) {
	row, err := r.read(ctx).GetPreference(ctx, sqlcgen.GetPreferenceParams{UserID: uuid(user), TargetKind: t.Kind, TargetID: uuid(t.ID)})
	return onePreference(row, err, "reading a notification preference")
}

// DeletePreference implements app.Store.
func (r *Repository) DeletePreference(ctx context.Context, user ids.ID, t notify.Target) (notify.Preference, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return notify.Preference{}, false, err
	}
	row, err := q.DeletePreference(ctx, sqlcgen.DeletePreferenceParams{UserID: uuid(user), TargetKind: t.Kind, TargetID: uuid(t.ID)})
	return onePreference(row, err, "deleting a notification preference")
}

// Preferences implements app.Store.
func (r *Repository) Preferences(ctx context.Context, user ids.ID) ([]notify.Preference, error) {
	rows, err := r.read(ctx).ListPreferences(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "listing notification preferences")
	}
	out := make([]notify.Preference, 0, len(rows))
	for _, row := range rows {
		out = append(out, preference(row))
	}
	return out, nil
}

// DeletePreferencesOfTargets implements app.Store; the rows come back in ascending (user,
// target kind, target id), the order of the cleanup events.
func (r *Repository) DeletePreferencesOfTargets(ctx context.Context, space ids.ID, channels, threads []ids.ID) ([]app.PreferenceRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.DeletePreferencesOfTargets(ctx, sqlcgen.DeletePreferencesOfTargetsParams{
		SpaceID: uuid(space), ChannelIds: uuids(channels), ThreadIds: uuids(threads),
	})
	if err != nil {
		return nil, failed(err, "deleting the notification preferences of archived targets")
	}
	out := preferenceRows(rows)
	slices.SortFunc(out, func(a, b app.PreferenceRow) int {
		return cmp.Or(bytes.Compare(a.UserID[:], b.UserID[:]), cmp.Compare(a.Target.Kind, b.Target.Kind),
			bytes.Compare(a.Target.ID[:], b.Target.ID[:]))
	})
	return out, nil
}

// LevelRows implements app.Store with one query.
func (r *Repository) LevelRows(ctx context.Context, users []ids.ID, thread, channel, space ids.ID) ([]app.PreferenceRow, error) {
	if len(users) == 0 {
		return nil, nil
	}
	rows, err := r.read(ctx).LevelRows(ctx, sqlcgen.LevelRowsParams{UserIds: uuids(users), ThreadID: uuid(thread), ChannelID: uuid(channel), SpaceID: uuid(space)})
	if err != nil {
		return nil, failed(err, "reading notification levels")
	}
	return preferenceRows(rows), nil
}

// LockRules implements app.Store with a transaction-scoped advisory lock.
func (r *Repository) LockRules(ctx context.Context, user ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if err := q.LockRulesOfUser(ctx, user.String()); err != nil {
		return failed(err, "locking the quiet hours of a person")
	}
	return nil
}

// Rules implements app.Store.
func (r *Repository) Rules(ctx context.Context, user ids.ID) ([]notify.QuietHoursRule, error) {
	rows, err := r.read(ctx).ListRules(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "listing quiet hours")
	}
	out := make([]notify.QuietHoursRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, rule(row))
	}
	return out, nil
}

// Rule implements app.Store.
func (r *Repository) Rule(ctx context.Context, user, id ids.ID) (notify.QuietHoursRule, bool, error) {
	row, err := r.read(ctx).GetRule(ctx, sqlcgen.GetRuleParams{ID: uuid(id), UserID: uuid(user)})
	return oneRule(row, err, "reading a quiet-hour rule")
}

// InsertRule implements app.Store.
func (r *Repository) InsertRule(ctx context.Context, user ids.ID, in notify.QuietHoursRule) (notify.QuietHoursRule, error) {
	q, err := r.write(ctx)
	if err != nil {
		return notify.QuietHoursRule{}, err
	}
	row, err := q.InsertRule(ctx, sqlcgen.InsertRuleParams{
		ID: uuid(in.ID), UserID: uuid(user), Weekdays: int16(in.Weekdays), StartLocal: in.StartLocal, EndLocal: in.EndLocal, //nolint:gosec // G115: 1 to 127, validated
		TimeZone: text(in.TimeZone), Enabled: in.Enabled,
	})
	if err != nil {
		return notify.QuietHoursRule{}, failed(err, "inserting a quiet-hour rule")
	}
	return rule(row), nil
}

// UpdateRule implements app.Store.
func (r *Repository) UpdateRule(ctx context.Context, user ids.ID, in notify.QuietHoursRule) (notify.QuietHoursRule, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return notify.QuietHoursRule{}, false, err
	}
	row, err := q.UpdateRule(ctx, sqlcgen.UpdateRuleParams{
		ID: uuid(in.ID), UserID: uuid(user), Weekdays: int16(in.Weekdays), StartLocal: in.StartLocal, EndLocal: in.EndLocal, //nolint:gosec // G115: 1 to 127, validated
		TimeZone: text(in.TimeZone), Enabled: in.Enabled,
	})
	return oneRule(row, err, "updating a quiet-hour rule")
}

// DeleteRule implements app.Store.
func (r *Repository) DeleteRule(ctx context.Context, user, id ids.ID) (notify.QuietHoursRule, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return notify.QuietHoursRule{}, false, err
	}
	row, err := q.DeleteRule(ctx, sqlcgen.DeleteRuleParams{ID: uuid(id), UserID: uuid(user)})
	return oneRule(row, err, "deleting a quiet-hour rule")
}

// RulesOfUsers implements app.Store with one query.
func (r *Repository) RulesOfUsers(ctx context.Context, users []ids.ID) (map[ids.ID][]notify.QuietHoursRule, error) {
	out := map[ids.ID][]notify.QuietHoursRule{}
	if len(users) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).RulesOfUsers(ctx, uuids(users))
	if err != nil {
		return nil, failed(err, "reading quiet hours")
	}
	for _, row := range rows {
		u := idOf(row.UserID)
		out[u] = append(out[u], rule(row))
	}
	return out, nil
}

// onePreference converts the result of a single-row query; no row is found false.
func onePreference(row sqlcgen.NotificationPreference, err error, what string) (notify.Preference, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return notify.Preference{}, false, nil
	}
	if err != nil {
		return notify.Preference{}, false, failed(err, what)
	}
	return preference(row), true, nil
}

// oneRule converts the result of a single-row query; no row is found false.
func oneRule(row sqlcgen.QuietHour, err error, what string) (notify.QuietHoursRule, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return notify.QuietHoursRule{}, false, nil
	}
	if err != nil {
		return notify.QuietHoursRule{}, false, failed(err, what)
	}
	return rule(row), true, nil
}

// preference converts a row.
func preference(row sqlcgen.NotificationPreference) notify.Preference {
	p := notify.Preference{Target: notify.Target{Kind: row.TargetKind, ID: idOf(row.TargetID)}, Level: row.Level, Version: row.Version}
	if row.MutedUntil.Valid {
		p.MutedUntil = row.MutedUntil.Time.UTC()
	}
	return p
}

// preferenceRows converts rows with their persons.
func preferenceRows(rows []sqlcgen.NotificationPreference) []app.PreferenceRow {
	out := make([]app.PreferenceRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.PreferenceRow{UserID: idOf(row.UserID), Preference: preference(row)})
	}
	return out
}

// rule converts a row; the times become HH:MM.
func rule(row sqlcgen.QuietHour) notify.QuietHoursRule {
	r := notify.QuietHoursRule{ID: idOf(row.ID), Weekdays: int(row.Weekdays), StartLocal: clockTime(row.StartLocal), EndLocal: clockTime(row.EndLocal),
		Enabled: row.Enabled, Version: row.Version}
	if row.TimeZone.Valid {
		r.TimeZone = row.TimeZone.String
	}
	return r
}

// clockTime formats a time column as HH:MM.
func clockTime(t pgtype.Time) string {
	minutes := t.Microseconds / int64(time.Minute/time.Microsecond)
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// uuid converts an ID; the zero ID is NULL.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// uuids converts a list; nil becomes an empty array, never NULL.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}

// idOf converts a pgx UUID; NULL is the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// stamp converts a time; the zero time is NULL.
func stamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// text converts a string; "" is NULL.
func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
