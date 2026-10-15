// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the PrincipalWriter (M1-51).
var _ app.PrincipalStore = (*Repository)(nil)

// LockPrincipal implements app.PrincipalStore.
func (r *Repository) LockPrincipal(ctx context.Context, id ids.ID) (domain.Principal, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Principal{}, false, err
	}
	row, err := q.LockPrincipal(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Principal{}, false, nil
	}
	if err != nil {
		return domain.Principal{}, false, mapError(err, "locking a principal")
	}
	return toPrincipal(row.Principal), true, nil
}

// SetPrincipalStatus implements app.PrincipalStore.
func (r *Repository) SetPrincipalStatus(ctx context.Context, id ids.ID, status domain.Status) (domain.Principal, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Principal{}, err
	}
	row, err := q.SetPrincipalStatus(ctx, sqlcgen.SetPrincipalStatusParams{ID: uuid(id), Status: string(status)})
	if err != nil {
		return domain.Principal{}, mapError(err, "setting the status of a principal")
	}
	return toPrincipal(row), nil
}

// SetPrincipalDisplayName implements app.PrincipalStore.
func (r *Repository) SetPrincipalDisplayName(ctx context.Context, id ids.ID, displayName domain.DisplayName) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if err := q.UpdatePrincipalDisplayName(ctx, sqlcgen.UpdatePrincipalDisplayNameParams{ID: uuid(id), DisplayName: displayName.String()}); err != nil {
		return mapError(err, "updating a principal")
	}
	return nil
}
