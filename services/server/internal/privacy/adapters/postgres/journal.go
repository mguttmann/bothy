// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// JournalState implements app.JournalState over deletion_journal_state (03 4.12, R-DM-52). Every
// method runs in the ambient unit of work of ctx when there is one. It is safe for concurrent use.
type JournalState struct {
	pool *pg.Pool
}

// Compile-time check.
var _ app.JournalState = (*JournalState)(nil)

// NewJournalState returns the store over pool.
func NewJournalState(pool *pg.Pool) *JournalState { return &JournalState{pool: pool} }

// q returns the queries on the ambient transaction of ctx or on the pool.
func (s *JournalState) q(ctx context.Context) *sqlcgen.Queries { return sqlcgen.New(pg.Querier(ctx, s.pool)) }

// errNoRow is the error of a database without the one row the migration inserts.
var errNoRow = errs.New(errs.CodeInternalError, "privacy: deletion_journal_state has no row; run bothy migrate up")

// Take implements app.JournalState.
func (s *JournalState) Take(ctx context.Context) (int64, error) {
	n, err := s.q(ctx).TakeJournalNumber(ctx)
	if err != nil {
		return 0, noRow(err)
	}
	return n, nil
}

// Numbers implements app.JournalState.
func (s *JournalState) Numbers(ctx context.Context) (int64, int64, error) {
	r, err := s.q(ctx).JournalNumbers(ctx)
	if err != nil {
		return 0, 0, noRow(err)
	}
	return r.NextNumber, r.LowNumber, nil
}

// SetNext implements app.JournalState.
func (s *JournalState) SetNext(ctx context.Context, next int64) error {
	n, err := s.q(ctx).SetJournalNext(ctx, next)
	if err != nil {
		return err
	}
	if n != 1 {
		return errNoRow
	}
	return nil
}

// SetLow implements app.JournalState: a value at or below the stored one or above next_number
// changes nothing.
func (s *JournalState) SetLow(ctx context.Context, low int64) error {
	_, err := s.q(ctx).SetJournalLow(ctx, low)
	return err
}

// LockDeletionOf implements app.JournalRequests.
func (s *Requests) LockDeletionOf(ctx context.Context, subject ids.ID) (domain.Request, bool, error) {
	return one(s.q(ctx).LockDeletionOf(ctx, uuid(subject)))
}

// Compile-time check.
var _ app.JournalRequests = (*Requests)(nil)

// noRow names the missing row instead of pgx.ErrNoRows.
func noRow(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errNoRow
	}
	return fmt.Errorf("privacy: deletion_journal_state: %w", err)
}
