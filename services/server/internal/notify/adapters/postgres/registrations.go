// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Registrations is the adapter of push_registrations (M3-19): it implements the read port
// notify.PushRegistrations, which never returns the token or the preview hash, and the outgoing
// port app.RegistrationStore of the job notify.push_send, which reads the token and writes only
// the delivery columns. None of its reads authorizes: the callers select the people or the
// registration (07 7.4 rule 1). Reads use the ambient transaction of ctx when there is one and
// the pool otherwise; Lock and Update need a unit of work. Errors are kernel/errs errors without
// SQL text or values; no error or log names a token. It is safe for concurrent use.
type Registrations struct {
	pool *pg.Pool
}

// Compile-time checks: Registrations is the read port of notify and the outgoing port of app.
var (
	_ notify.PushRegistrations = (*Registrations)(nil)
	_ app.RegistrationStore    = (*Registrations)(nil)
)

// NewRegistrations returns the adapter over pool; it opens no connection.
//
// Example:
//
//	regs := postgres.NewRegistrations(pool)
//	active, err := regs.Active(ctx, []ids.ID{alex})
func NewRegistrations(pool *pg.Pool) *Registrations { return &Registrations{pool: pool} }

// read returns the queries on the ambient transaction or the pool.
func (r *Registrations) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries on the transaction of the unit of work; internal outside one.
func (r *Registrations) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// Active implements notify.PushRegistrations with one query on the index (user_id) WHERE
// disabled_at IS NULL; the rows come grouped by person in ascending id. People without an active
// registration are absent; no people give an empty map.
func (r *Registrations) Active(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]notify.PushRegistration, error) {
	out := map[ids.ID][]notify.PushRegistration{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).ActiveRegistrations(ctx, uuids(userIDs))
	if err != nil {
		return nil, failed(err, "reading the active push registrations")
	}
	for _, row := range rows {
		u := idOf(row.UserID)
		out[u] = append(out[u], notify.PushRegistration{
			ID: idOf(row.ID), UserID: u, DeviceID: idOf(row.DeviceID),
			Provider: row.Provider, Environment: row.Environment, AppID: row.AppID,
			CreatedAt: timeOf(row.CreatedAt), LastSuccessAt: timeOf(row.LastSuccessAt), FailureCount: int(row.FailureCount),
		})
	}
	return out, nil
}

// Load implements app.RegistrationStore: the registration with its token and its delivery state;
// found is false when id does not exist.
func (r *Registrations) Load(ctx context.Context, id ids.ID) (app.Registration, domain.RegistrationState, bool, error) {
	row, err := r.read(ctx).LoadRegistration(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Registration{}, domain.RegistrationState{}, false, nil
	}
	if err != nil {
		return app.Registration{}, domain.RegistrationState{}, false, failed(err, "reading a push registration")
	}
	reg := app.Registration{
		ID: idOf(row.ID), UserID: idOf(row.UserID), DeviceID: idOf(row.DeviceID),
		Provider: row.Provider, Environment: row.Environment, AppID: row.AppID, Token: row.Token,
		CreatedAt: timeOf(row.CreatedAt),
	}
	return reg, deliveryState(row.CreatedAt, row.LastSuccessAt, row.FailureCount, row.DisabledAt, row.DisabledReason), true, nil
}

// Lock implements app.RegistrationStore with SELECT ... FOR UPDATE in the unit of work of ctx;
// found is false when id does not exist. Outside a unit of work it returns an internal error.
func (r *Registrations) Lock(ctx context.Context, id ids.ID) (domain.RegistrationState, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.RegistrationState{}, false, err
	}
	row, err := q.LockRegistration(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistrationState{}, false, nil
	}
	if err != nil {
		return domain.RegistrationState{}, false, failed(err, "locking a push registration")
	}
	return deliveryState(row.CreatedAt, row.LastSuccessAt, row.FailureCount, row.DisabledAt, row.DisabledReason), true, nil
}

// Update implements app.RegistrationStore: it writes last_success_at, failure_count, disabled_at
// and disabled_reason of id (zero times and "" as NULL) and never created_at. Outside a unit of
// work, and when id does not exist (the caller locked it first, so that is a bug), it returns an
// internal error; a state the checks of 03 refuse fails with an internal error too.
func (r *Registrations) Update(ctx context.Context, id ids.ID, state domain.RegistrationState) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.UpdateRegistrationDelivery(ctx, sqlcgen.UpdateRegistrationDeliveryParams{
		LastSuccessAt: stamp(state.LastSuccessAt), FailureCount: int32(state.FailureCount), //nolint:gosec // G115: bounded by push.max_consecutive_failures
		DisabledAt: stamp(state.DisabledAt), DisabledReason: text(state.DisabledReason), ID: uuid(id),
	})
	if err != nil {
		return failed(err, "writing the delivery state of a push registration")
	}
	if n == 0 {
		return errs.New(errs.CodeInternalError, "notify: the push registration to update does not exist")
	}
	return nil
}

// ActiveCounts implements app.RegistrationStore: the active rows per provider, the data points of
// bothy.push.registrations.active. Providers without an active row are absent.
func (r *Registrations) ActiveCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := r.read(ctx).ActiveRegistrationCounts(ctx)
	if err != nil {
		return nil, failed(err, "counting the active push registrations")
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Provider] = row.Active
	}
	return out, nil
}

// deliveryState converts the delivery columns of a row.
func deliveryState(created, success pgtype.Timestamptz, failures int32, disabled pgtype.Timestamptz, reason pgtype.Text) domain.RegistrationState {
	s := domain.RegistrationState{CreatedAt: timeOf(created), LastSuccessAt: timeOf(success), FailureCount: int(failures), DisabledAt: timeOf(disabled)}
	if reason.Valid {
		s.DisabledReason = reason.String
	}
	return s
}

// timeOf converts a timestamp in UTC; NULL is the zero time.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
