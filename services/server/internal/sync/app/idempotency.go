// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// KeyRows reads and writes idempotency_keys (adapters/postgres). Every method runs in the
// ambient transaction of ctx, or on the pool outside a unit of work.
type KeyRows interface {
	// DeleteExpired removes the row of (principal, key) when it expired at now.
	DeleteExpired(ctx context.Context, principal, key ids.ID, now time.Time) error
	// Insert inserts the row of req in state in_progress with created_at req.Now and expires_at
	// req.Now + req.TTL. claimed is false when a committed row of the key exists. A wait at the
	// primary key that ends with the lock timeout is idempotency.ErrInProgress.
	Insert(ctx context.Context, req idempotency.Request) (claimed bool, err error)
	// Get returns the row of (principal, key) with the body in its compact form; found is false
	// when there is none.
	Get(ctx context.Context, principal, key ids.ID) (entry idempotency.Entry, found bool, err error)
	// Complete sets the claimed row of req to completed with resp; ok is false when no claimed
	// row exists.
	Complete(ctx context.Context, req idempotency.Request, resp idempotency.Response) (ok bool, err error)
	// SaveFinal inserts the completed row of req with resp unless a row of the key exists.
	SaveFinal(ctx context.Context, req idempotency.Request, resp idempotency.Response) error
	// DeleteExpiredBatch deletes at most n rows that expired at now, oldest first, and returns
	// how many it deleted.
	DeleteExpiredBatch(ctx context.Context, now time.Time, n int) (int64, error)
}

// IdempotencyStore implements kernel idempotency.Store over KeyRows (02 "Idempotency in the Unit of
// Work"). It is safe for concurrent use.
type IdempotencyStore struct {
	rows KeyRows
	uow  txn.UnitOfWork
}

// Compile-time check: the store is the kernel port.
var _ idempotency.Store = (*IdempotencyStore)(nil)

// NewIdempotencyStore returns the store over rows; uow opens the own transaction of SaveFinal.
func NewIdempotencyStore(rows KeyRows, uow txn.UnitOfWork) *IdempotencyStore {
	return &IdempotencyStore{rows: rows, uow: uow}
}

// claimAttempts bounds Claim when the row it found taken is gone again before it is read (the
// cleanup job deleted it as expired in between): one more claim decides.
const claimAttempts = 2

// Claim implements idempotency.Store: it needs the ambient transaction of a unit of work. A row
// that expired at req.Now is removed first, then the key is claimed; a taken key returns its
// committed row. Errors: kind internal outside a unit of work or for a request without
// principal, key or positive TTL; idempotency.ErrInProgress after a lock timeout at the key;
// the errors of KeyRows.
func (s *IdempotencyStore) Claim(ctx context.Context, req idempotency.Request) (idempotency.Entry, bool, error) {
	if err := checkRequest(ctx, req); err != nil {
		return idempotency.Entry{}, false, err
	}
	for range claimAttempts {
		if err := s.rows.DeleteExpired(ctx, req.PrincipalID, req.Key, req.Now); err != nil {
			return idempotency.Entry{}, false, err
		}
		claimed, err := s.rows.Insert(ctx, req)
		if err != nil || claimed {
			return idempotency.Entry{}, false, err
		}
		entry, found, err := s.rows.Get(ctx, req.PrincipalID, req.Key)
		if err != nil || found {
			return entry, found, err
		}
	}
	return idempotency.Entry{}, false, errs.New(errs.CodeInternalError, "sync: an idempotency key was neither claimed nor found")
}

// Complete implements idempotency.Store in the ambient transaction. Errors: kind internal
// outside a unit of work, for a status outside smallint or when the key was not claimed in this
// unit; the errors of KeyRows.
func (s *IdempotencyStore) Complete(ctx context.Context, req idempotency.Request, resp idempotency.Response) error {
	if err := checkRequest(ctx, req); err != nil {
		return err
	}
	if err := checkResponse(resp); err != nil {
		return err
	}
	ok, err := s.rows.Complete(ctx, req, resp)
	if err != nil {
		return err
	}
	if !ok {
		return errs.New(errs.CodeInternalError, "sync: Complete for an idempotency key that was not claimed")
	}
	return nil
}

// SaveFinal implements idempotency.Store: in its own unit of work, outside the ambient one of
// ctx (txn.Detach), it removes an expired row of the key and inserts the completed row unless
// one exists. The unit appends no event and no audit entry. Errors: kind internal for an invalid
// request or status; those of the unit of work and of KeyRows.
func (s *IdempotencyStore) SaveFinal(ctx context.Context, req idempotency.Request, resp idempotency.Response) error {
	if err := checkResponse(resp); err != nil {
		return err
	}
	return s.uow.Do(txn.Detach(ctx), func(ctx context.Context) error {
		if err := checkRequest(ctx, req); err != nil {
			return err
		}
		if err := s.rows.DeleteExpired(ctx, req.PrincipalID, req.Key, req.Now); err != nil {
			return err
		}
		return s.rows.SaveFinal(ctx, req, resp)
	})
}

// checkRequest refuses a call outside a unit of work and a request the table cannot hold.
func checkRequest(ctx context.Context, req idempotency.Request) error {
	switch {
	case !txn.InUnit(ctx):
		return errs.New(errs.CodeInternalError, "sync: the idempotency store needs a unit of work")
	case req.PrincipalID.IsZero() || req.Key.IsZero():
		return errs.New(errs.CodeInternalError, "sync: an idempotency request without principal or key")
	case req.TTL <= 0:
		return errs.New(errs.CodeInternalError, "sync: sync.idempotency_key_ttl must be positive")
	}
	return nil
}

// checkResponse refuses a status the smallint column cannot hold.
func checkResponse(resp idempotency.Response) error {
	if resp.Status < 100 || resp.Status > math.MaxInt16 {
		return errs.New(errs.CodeInternalError, "sync: an idempotency response without a valid status")
	}
	return nil
}

// IdempotencyCleanupConfig is the configuration of the job sync.idempotency_cleanup (02
// "Job catalogue", "Configuration values of this chapter").
type IdempotencyCleanupConfig struct {
	// Interval is sync.idempotency_cleanup_interval, the period of the job.
	Interval time.Duration `config:"sync.idempotency_cleanup_interval"`
	// BatchSize is sync.idempotency_cleanup_batch_size, the rows per delete.
	BatchSize int `config:"sync.idempotency_cleanup_batch_size"`
}

// CleanupResult is the outcome of one run of the cleanup.
type CleanupResult struct {
	// Deleted is the number of rows deleted.
	Deleted int64
	// Batches is the number of delete statements, the last one short.
	Batches int
}

// IdempotencyCleanup deletes expired idempotency keys (job sync.idempotency_cleanup). It is
// safe for concurrent use; two runs at once skip each other's rows.
type IdempotencyCleanup struct {
	cfg   IdempotencyCleanupConfig
	uow   txn.UnitOfWork
	rows  KeyRows
	clock clock.Clock
}

// NewIdempotencyCleanup returns the use case. The error reports an interval that is not
// positive or a batch size below 1.
func NewIdempotencyCleanup(cfg IdempotencyCleanupConfig, uow txn.UnitOfWork, rows KeyRows, c clock.Clock) (*IdempotencyCleanup, error) {
	switch {
	case cfg.Interval <= 0:
		return nil, errors.New("sync: sync.idempotency_cleanup_interval must be positive")
	case cfg.BatchSize < 1:
		return nil, errors.New("sync: sync.idempotency_cleanup_batch_size must be at least 1")
	}
	return &IdempotencyCleanup{cfg: cfg, uow: uow, rows: rows, clock: c}, nil
}

// Interval returns sync.idempotency_cleanup_interval, the period of the job.
func (c *IdempotencyCleanup) Interval() time.Duration { return c.cfg.Interval }

// Run deletes the rows that expired at the time of the clock in batches of BatchSize, ordered by
// expires_at, each batch in its own unit of work (02 "Rules for jobs" rule 6), until a batch
// is smaller than the batch size. A repeated run is harmless. Errors: ctx.Err() when ctx ends
// before or between batches, with the result so far; those of the unit of work.
func (c *IdempotencyCleanup) Run(ctx context.Context) (CleanupResult, error) {
	var res CleanupResult
	now := c.clock.Now()
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		var n int64
		err := c.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			n, err = c.rows.DeleteExpiredBatch(ctx, now, c.cfg.BatchSize)
			return err
		})
		if err != nil {
			return res, err
		}
		res.Deleted += n
		res.Batches++
		if n < int64(c.cfg.BatchSize) {
			return res, nil
		}
	}
}
