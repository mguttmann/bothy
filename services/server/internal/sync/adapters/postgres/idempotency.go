// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// sqlstateLockNotAvailable is lock_not_available, the SQLSTATE of a lock timeout.
const sqlstateLockNotAvailable = "55P03"

// stateCompleted is the state of a row with a final response.
const stateCompleted = "completed"

// IdempotencyRepository implements app.KeyRows over the sqlc queries of idempotency_keys. Inside
// a unit of work every statement uses the ambient transaction (pg.Querier). It is safe for
// concurrent use.
type IdempotencyRepository struct {
	pool *pg.Pool
}

// Compile-time check: the repository is the outgoing port of app.
var _ app.KeyRows = (*IdempotencyRepository)(nil)

// NewIdempotencyRepository returns the repository over pool.
func NewIdempotencyRepository(pool *pg.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

func (r *IdempotencyRepository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// DeleteExpired implements app.KeyRows. A lock timeout (a concurrent request removes the same
// expired row) is idempotency.ErrInProgress.
func (r *IdempotencyRepository) DeleteExpired(ctx context.Context, principal, key ids.ID, now time.Time) error {
	_, err := r.queries(ctx).DeleteExpiredKey(ctx, sqlcgen.DeleteExpiredKeyParams{
		PrincipalID: uuid(principal), Key: uuid(key), Now: timestamptz(now),
	})
	return inProgress(err)
}

// Insert implements app.KeyRows. A lock timeout at the primary key (the first request has not
// committed yet) is idempotency.ErrInProgress, never the database error, so the unit of work
// does not retry the wait (02 "Lock order").
func (r *IdempotencyRepository) Insert(ctx context.Context, req idempotency.Request) (bool, error) {
	n, err := r.queries(ctx).ClaimKey(ctx, sqlcgen.ClaimKeyParams{
		PrincipalID: uuid(req.PrincipalID), Key: uuid(req.Key), Action: req.Action,
		RequestSha256: req.SHA256[:], CreatedAt: timestamptz(req.Now), ExpiresAt: timestamptz(req.Now.Add(req.TTL)),
	})
	if err != nil {
		return false, inProgress(err)
	}
	return n == 1, nil
}

// Get implements app.KeyRows.
func (r *IdempotencyRepository) Get(ctx context.Context, principal, key ids.ID) (idempotency.Entry, bool, error) {
	row, err := r.queries(ctx).GetKey(ctx, sqlcgen.GetKeyParams{PrincipalID: uuid(principal), Key: uuid(key)})
	if errors.Is(err, pgx.ErrNoRows) {
		return idempotency.Entry{}, false, nil
	}
	if err != nil {
		return idempotency.Entry{}, false, err
	}
	var e idempotency.Entry
	if len(row.RequestSha256) != len(e.SHA256) {
		return idempotency.Entry{}, false, errs.New(errs.CodeInternalError, "sync: a stored fingerprint is not 32 bytes")
	}
	copy(e.SHA256[:], row.RequestSha256)
	e.Completed = row.State == stateCompleted
	e.Status = int(row.ResponseStatus.Int16)
	e.ResourceID = idOf(row.ResourceID)
	if len(row.ResponseBody) > 0 {
		// The text form of jsonb has a space after every colon and comma.
		var buf bytes.Buffer
		if err := json.Compact(&buf, row.ResponseBody); err != nil {
			return idempotency.Entry{}, false, errs.Wrap(errs.CodeInternalError, err, "sync: a stored response is not JSON")
		}
		e.Body = buf.Bytes()
	}
	return e, true, nil
}

// Complete implements app.KeyRows.
func (r *IdempotencyRepository) Complete(ctx context.Context, req idempotency.Request, resp idempotency.Response) (bool, error) {
	status, err := smallint(resp.Status)
	if err != nil {
		return false, err
	}
	n, err := r.queries(ctx).CompleteKey(ctx, sqlcgen.CompleteKeyParams{
		ResponseStatus: status, ResponseBody: body(resp.Body), ResourceID: uuid(resp.ResourceID),
		PrincipalID: uuid(req.PrincipalID), Key: uuid(req.Key),
	})
	return n == 1, err
}

// SaveFinal implements app.KeyRows.
func (r *IdempotencyRepository) SaveFinal(ctx context.Context, req idempotency.Request, resp idempotency.Response) error {
	status, err := smallint(resp.Status)
	if err != nil {
		return err
	}
	_, err = r.queries(ctx).SaveFinalKey(ctx, sqlcgen.SaveFinalKeyParams{
		PrincipalID: uuid(req.PrincipalID), Key: uuid(req.Key), Action: req.Action, RequestSha256: req.SHA256[:],
		ResponseStatus: status, ResponseBody: body(resp.Body), ResourceID: uuid(resp.ResourceID),
		CreatedAt: timestamptz(req.Now), ExpiresAt: timestamptz(req.Now.Add(req.TTL)),
	})
	return err
}

// DeleteExpiredBatch implements app.KeyRows.
func (r *IdempotencyRepository) DeleteExpiredBatch(ctx context.Context, now time.Time, n int) (int64, error) {
	if n < 1 || n > math.MaxInt32 {
		return 0, errs.New(errs.CodeInternalError, "sync: cleanup batch size outside integer")
	}
	return r.queries(ctx).DeleteExpiredKeys(ctx, sqlcgen.DeleteExpiredKeysParams{Now: timestamptz(now), BatchSize: int32(n)}) // #nosec G115 -- checked above
}

// inProgress converts a lock timeout to idempotency.ErrInProgress, without wrapping the database
// error, which the unit of work would retry.
func inProgress(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateLockNotAvailable {
		return idempotency.ErrInProgress
	}
	return err
}

// smallint converts an HTTP status to the column type.
func smallint(status int) (pgtype.Int2, error) {
	if status < 0 || status > math.MaxInt16 {
		return pgtype.Int2{}, errs.New(errs.CodeInternalError, "sync: status outside smallint")
	}
	return pgtype.Int2{Int16: int16(status), Valid: true}, nil // #nosec G115 -- checked above
}

// body returns the stored body: nil (NULL) for a response without body.
func body(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

// timestamptz converts a time to the pgx type.
func timestamptz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
