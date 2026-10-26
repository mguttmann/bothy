// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// Compile-time check: the repository is the outgoing port of the participant app.PersonalData
// (M7-06).
var _ app.PersonalDataRows = (*PersonalRepository)(nil)

// ReadStatesOf implements app.PersonalDataRows: every read state of user, ascending by thread.
func (r *PersonalRepository) ReadStatesOf(ctx context.Context, user ids.ID) ([]sync.ReadState, error) {
	rows, err := r.read(ctx).ListReadStatesOfUser(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "listing the read states of a person")
	}
	out := make([]sync.ReadState, 0, len(rows))
	for _, row := range rows {
		out = append(out, readStateOf(row))
	}
	return out, nil
}

// DraftsOf implements app.PersonalDataRows: every draft of user, ascending by id.
func (r *PersonalRepository) DraftsOf(ctx context.Context, user ids.ID) ([]sync.Draft, error) {
	rows, err := r.read(ctx).ListDraftsOfUser(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "listing the drafts of a person")
	}
	return draftsOf(rows), nil
}

// DeleteReadStatesOf implements app.PersonalDataRows inside the unit of work of ctx.
func (r *PersonalRepository) DeleteReadStatesOf(ctx context.Context, user ids.ID) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteReadStatesOfUser(ctx, uuid(user))
	if err != nil {
		return 0, failed(err, "deleting the read states of a person")
	}
	return n, nil
}

// DeleteDraftsOf implements app.PersonalDataRows inside the unit of work of ctx and returns the
// deleted rows.
func (r *PersonalRepository) DeleteDraftsOf(ctx context.Context, user ids.ID) ([]sync.Draft, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.DeleteDraftsOfUser(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "deleting the drafts of a person")
	}
	return draftsOf(rows), nil
}

// DeleteSettingsOf implements app.PersonalDataRows inside the unit of work of ctx and returns the
// deleted rows, ascending by key so the events come in a stable order.
func (r *PersonalRepository) DeleteSettingsOf(ctx context.Context, user ids.ID) ([]sync.UserSetting, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.DeleteSettingsOfUser(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "deleting the settings of a person")
	}
	out := make([]sync.UserSetting, 0, len(rows))
	for _, row := range rows {
		out = append(out, settingOf(row))
	}
	sortSettings(out)
	return out, nil
}

// DeleteIdempotencyKeysOf implements app.PersonalDataRows inside the unit of work of ctx.
func (r *PersonalRepository) DeleteIdempotencyKeysOf(ctx context.Context, principal ids.ID) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteKeysOfPrincipal(ctx, uuid(principal))
	if err != nil {
		return 0, failed(err, "deleting the idempotency keys of a person")
	}
	return n, nil
}

// draftsOf converts rows.
func draftsOf(rows []sqlcgen.Draft) []sync.Draft {
	out := make([]sync.Draft, 0, len(rows))
	for _, row := range rows {
		out = append(out, draftOf(row))
	}
	return out
}

// sortSettings orders list by key; DELETE ... RETURNING has no ORDER BY.
func sortSettings(list []sync.UserSetting) {
	slices.SortFunc(list, func(a, b sync.UserSetting) int { return strings.Compare(a.Key, b.Key) })
}
