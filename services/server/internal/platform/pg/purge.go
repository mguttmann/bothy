// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// classIntegrity is the SQLSTATE class 23, integrity constraint violation: a delete of one row by
// its key fails with it only through a rule of another row, a foreign key (23001 restrict, 23503
// no action) or a check of a referencing row that an ON DELETE SET NULL would break (23514).
const classIntegrity = "23"

// PurgeEach deletes the candidate rows of a retention participant (M7-05) in one short
// transaction on p: each candidate with del in its own savepoint, so a row that a foreign key with
// ON DELETE RESTRICT of a row in another table holds through a rule of another row (SQLSTATE class 23) is skipped and stays
// until that row is gone, while the other candidates are deleted. Why per row: a participant
// deletes only its own tables and never reads another module's table to find out which rows are
// still referenced (03 principle 12); the database knows it. Any integrity violation counts:
// besides the foreign keys, an ON DELETE SET NULL can break a check of the referencing row (a
// completed device pairing keeps its device, M7-05 review).
//
// It returns how many rows del removed. Errors: the transaction, and every error of del other
// than a foreign key violation; then nothing of the batch is committed.
func PurgeEach[T any](ctx context.Context, p *Pool, candidates []T, del func(ctx context.Context, q DBTX, c T) (int64, error)) (int, error) {
	if len(candidates) == 0 {
		return 0, nil
	}
	tx, err := p.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	deleted := 0
	for _, c := range candidates {
		n, err := deleteOne(ctx, tx, c, del)
		if err != nil {
			return 0, err
		}
		deleted += int(n)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return deleted, nil
}

// PurgeStep is one batch of one table of a retention participant (M7-05): it reads at most batch
// candidates with candidates on p (outside a transaction), deletes them with PurgeEach and
// reports more when the batch was full and removed at least one row, so a batch that only held
// referenced rows does not make the job loop. The candidates query and del must both check the
// end time against the cutoff, so a row that changed in between is not deleted.
//
// Errors: those of candidates and of PurgeEach.
func PurgeStep[T any](ctx context.Context, p *Pool, batch int, candidates func(ctx context.Context, q DBTX) ([]T, error), del func(ctx context.Context, q DBTX, c T) (int64, error)) (deleted int, more bool, err error) {
	list, err := candidates(ctx, p)
	if err != nil {
		return 0, false, err
	}
	n, err := PurgeEach(ctx, p, list, del)
	if err != nil {
		return 0, false, err
	}
	return n, len(list) >= batch && n > 0, nil
}

// deleteOne deletes candidate c with del in its own savepoint of tx; a delete another row's rule
// refuses rolls the savepoint back and counts nothing.
func deleteOne[T any](ctx context.Context, tx pgx.Tx, c T, del func(ctx context.Context, q DBTX, c T) (int64, error)) (int64, error) {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return 0, err
	}
	n, err := del(ctx, sp, c)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, classIntegrity) {
		return 0, sp.Rollback(ctx)
	}
	if err != nil {
		return 0, err
	}
	return n, sp.Commit(ctx)
}

// Limit returns n as the int32 of a LIMIT parameter, at least 0 and at most math.MaxInt32.
func Limit(n int) int32 {
	return int32(min(max(n, 0), math.MaxInt32)) // #nosec G115 -- bounded to the int32 range above.
}
