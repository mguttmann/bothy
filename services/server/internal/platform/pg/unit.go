// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The SQLSTATEs that retry a whole unit of work (02 "Lock order", R-AR-18).
const (
	sqlstateDeadlock         = "40P01"
	sqlstateLockNotAvailable = "55P03"
)

// UnitConfig is the configuration of the unit of work, bound in internal/wire from
// database.tx_retry_max, database.statement_timeout, database.lock_timeout and
// database.idle_in_tx_timeout.
type UnitConfig struct {
	// RetryMax is how often a unit is retried after a deadlock or a lock timeout (attempts =
	// 1 + RetryMax); a negative value counts as 0.
	RetryMax int `config:"database.tx_retry_max"`
	// StatementTimeout is set with SET LOCAL in every transaction; it also bounds the rollback.
	StatementTimeout time.Duration `config:"database.statement_timeout"`
	// LockTimeout is set with SET LOCAL in every transaction.
	LockTimeout time.Duration `config:"database.lock_timeout"`
	// IdleInTxTimeout is set with SET LOCAL in every transaction.
	IdleInTxTimeout time.Duration `config:"database.idle_in_tx_timeout"`
}

// UnitOfWork is the implementation of txn.UnitOfWork over a Pool: the only place of the server
// that begins, commits and rolls back transactions (02 "Flow", R-AR-06). It is safe for
// concurrent use; every Do of an outermost unit gets its own transaction.
type UnitOfWork struct {
	pool *Pool
	cfg  UnitConfig
	reg  *txn.Registry
}

// Compile-time check: UnitOfWork is the port of kernel/txn.
var _ txn.UnitOfWork = (*UnitOfWork)(nil)

// NewUnitOfWork returns the unit of work over pool with the hooks and subscribers of reg.
// internal/wire seals reg first; Do refuses to run with an unsealed registry.
func NewUnitOfWork(pool *Pool, cfg UnitConfig, reg *txn.Registry) *UnitOfWork {
	cfg.RetryMax = max(cfg.RetryMax, 0)
	return &UnitOfWork{pool: pool, cfg: cfg, reg: reg}
}

// Do runs fn in a unit of work (txn.UnitOfWork).
//
// Outermost unit, per attempt: BEGIN with READ COMMITTED explicitly (never the database
// default, ADR 0009), SET LOCAL of the three deadlines, then the phases rights_begin (only with
// txn.RightsChange), work (fn), rights_check (only with RightsChange), sync, audit and the
// commit; a phase without a hook is skipped. Every attempt has a fresh per-unit state. Any error
// and any panic roll back; a panic is raised again after the rollback. An attempt that fails
// with SQLSTATE 40P01 or 55P03 while ctx is still live is retried, at most
// database.tx_retry_max times.
//
// Nested unit (ctx already inside one): fn runs in the ambient transaction in the current
// phase; the nested Do never commits, rolls back or retries, its error goes to the outer one.
//
// Errors: the error of fn, of a hook or a subscriber unchanged; the errors of Pool.Begin;
// after the last retry the kernel/errs error of code unavailable that wraps the database error;
// kind internal for a nil fn, a missing or unsealed registry, a ctx inside a unit platform/pg
// did not create, and a nested RightsChange the outer unit does not cover.
func (w *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error, opts ...txn.Option) error {
	if fn == nil {
		return errs.New(errs.CodeInternalError, "pg: Do without a function")
	}
	if w.reg == nil || !w.reg.Sealed() {
		return errs.New(errs.CodeInternalError, "pg: Do before the txn registry is sealed")
	}
	o := txn.ApplyOptions(opts...)
	if txn.InUnit(ctx) {
		return joinUnit(ctx, o, fn)
	}
	for attempt := 0; ; attempt++ {
		err := w.attempt(ctx, fn, o)
		if err == nil || !retryable(err) || ctx.Err() != nil {
			return err
		}
		if attempt >= w.cfg.RetryMax {
			return errs.Wrap(errs.CodeUnavailable, err,
				"the unit of work still hit a deadlock or lock timeout after database.tx_retry_max retries")
		}
	}
}

// joinUnit runs fn inside the ambient unit of ctx.
func joinUnit(ctx context.Context, o txn.Options, fn func(context.Context) error) error {
	u, ok := ambient(ctx)
	if !ok {
		return errs.New(errs.CodeInternalError, "pg: the context carries a unit of work platform/pg did not create")
	}
	cur := u.unit.Options()
	if o.RightsChange && u.lateDone {
		// A late rights change already ran its check; a rights change after it would write
		// without one.
		return errs.New(errs.CodeInternalError, "pg: a second rights change after a late rights change")
	}
	if cur.Covers(o) {
		return fn(ctx)
	}
	if cur.LateRightsChange && !cur.RightsChange && !u.lateDone && u.unit.Phase() == txn.PhaseWork {
		return u.lateRightsChange(ctx, cur, o, fn)
	}
	// The rights begin hook locks permission_version before any domain row (03
	// "Lock order" item 1); a nested unit cannot add RightsChange afterwards.
	return errs.New(errs.CodeInternalError, "pg: nested RightsChange that the outermost unit of work does not declare")
}

// lateRightsChange turns the unit into the rights change o declares (txn.AllowLateRightsChange):
// the rights begin hook, fn and the rights check hook run in this order inside the ambient
// transaction, then the unit is back in the work phase. Sync and audit run at the end of the
// outer unit as always. Any error goes to the caller; the outer unit rolls back with it.
func (u *unit) lateRightsChange(ctx context.Context, cur, o txn.Options, fn func(context.Context) error) error {
	declared := cur
	declared.RightsChange = true
	declared.LateRightsChange = false
	txn.DeclareRightsChange(o.Spec())(&declared)
	u.ctrl.Declare(declared)
	defer func() {
		u.setPhase(txn.PhaseWork)
		u.lateDone = true
		if r := recover(); r != nil {
			// A recovered panic must not commit an unchecked rights change either.
			u.lateErr = errs.New(errs.CodeInternalError, "pg: panic in a late rights change")
			panic(r)
		}
	}()
	err := u.lateRun(ctx, fn)
	// The error fails the whole attempt even if the caller swallows it: nothing of a rights
	// change whose check did not pass may commit (red team M1-31).
	u.lateErr = err
	return err
}

// lateRun runs the phases of a late rights change.
func (u *unit) lateRun(ctx context.Context, fn func(context.Context) error) error {
	if err := u.hook(ctx, txn.PhaseRightsBegin); err != nil {
		return err
	}
	u.setPhase(txn.PhaseWork)
	if err := fn(ctx); err != nil {
		return err
	}
	return u.hook(ctx, txn.PhaseRightsCheck)
}

// retryable reports whether err carries SQLSTATE 40P01 or 55P03.
func retryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == sqlstateDeadlock || pgErr.Code == sqlstateLockNotAvailable)
}

// attempt runs one attempt of an outermost unit: one transaction from BEGIN to COMMIT or
// rollback.
func (w *UnitOfWork) attempt(ctx context.Context, fn func(context.Context) error, o txn.Options) (err error) {
	tx, err := w.pool.Begin(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite})
	if err != nil {
		return err
	}
	done := false
	defer func() {
		if done {
			return
		}
		r := recover()
		w.rollback(ctx, tx)
		if r != nil {
			panic(r)
		}
	}()
	if err := SetLocalTimeouts(ctx, tx, Timeouts{
		Statement: w.cfg.StatementTimeout, Lock: w.cfg.LockTimeout, IdleInTx: w.cfg.IdleInTxTimeout,
	}); err != nil {
		return err
	}
	tu, ctrl := txn.NewControlledUnit(w.reg, o)
	u := &unit{tx: tx, unit: tu, ctrl: ctrl, setPhase: ctrl.SetPhase, reg: w.reg, opts: o}
	uctx := context.WithValue(txn.WithUnit(ctx, tu), unitKey{}, u)
	if err := u.run(uctx, fn); err != nil {
		return err
	}
	u.setPhase(txn.PhaseNone)
	err = tx.Commit(ctx)
	// After a failed COMMIT the transaction is over as well; a rollback has nothing to do.
	done = true
	return err
}

// rollback ends tx. It runs also when ctx is already canceled (the case that made the attempt
// fail), bounded by database.statement_timeout; its error is dropped, because the attempt
// already fails with a better one and pgx closes a connection whose rollback failed.
func (w *UnitOfWork) rollback(ctx context.Context, tx pgx.Tx) {
	rctx := context.WithoutCancel(ctx)
	if w.cfg.StatementTimeout > 0 {
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(rctx, w.cfg.StatementTimeout)
		defer cancel()
	}
	_ = tx.Rollback(rctx)
}

// unit is one attempt: the transaction and the txn.Unit the kernel helpers read.
type unit struct {
	tx       pgx.Tx
	unit     *txn.Unit
	setPhase func(txn.Phase)
	ctrl     txn.UnitControl
	reg      *txn.Registry
	// opts are the options of the outermost Do; a late rights change does not change them, so
	// run never runs its rights hooks a second time.
	opts txn.Options
	// lateDone reports that a late rights change ran (txn.AllowLateRightsChange); lateErr is its
	// error, which fails the attempt before sync, audit and commit.
	lateDone bool
	lateErr  error
}

// unitKey is the context key of the attempt.
type unitKey struct{}

// run runs the phases before the commit in the order of 02 "Flow".
func (u *unit) run(ctx context.Context, fn func(context.Context) error) error {
	if u.opts.RightsChange {
		if err := u.hook(ctx, txn.PhaseRightsBegin); err != nil {
			return err
		}
	}
	u.setPhase(txn.PhaseWork)
	if err := fn(ctx); err != nil {
		return err
	}
	if u.lateErr != nil {
		return u.lateErr
	}
	if u.opts.RightsChange {
		if err := u.hook(ctx, txn.PhaseRightsCheck); err != nil {
			return err
		}
	}
	if err := u.hook(ctx, txn.PhaseSync); err != nil {
		return err
	}
	return u.hook(ctx, txn.PhaseAudit)
}

// hook enters phase p and runs its hook, if one is registered.
func (u *unit) hook(ctx context.Context, p txn.Phase) error {
	u.setPhase(p)
	if h := u.reg.Hook(p); h != nil {
		return h(ctx)
	}
	return nil
}

// Tx returns the ambient transaction of ctx: repositories write through it, and River's
// InsertTx (M1-14) joins it. The error (kind internal) reports a ctx outside a unit of work of
// platform/pg, because a write outside a unit would bypass the phases.
func Tx(ctx context.Context) (pgx.Tx, error) {
	if u, ok := ambient(ctx); ok {
		return u.tx, nil
	}
	return nil, errs.New(errs.CodeInternalError, "pg: Tx outside a unit of work")
}

// Querier returns what a repository runs its statements on: the ambient transaction inside a
// unit of work, otherwise pool (reads outside a unit). pool may be nil only where ctx is known
// to be inside a unit.
func Querier(ctx context.Context, pool *Pool) DBTX {
	if u, ok := ambient(ctx); ok {
		return u.tx
	}
	return pool
}

// ambient returns the attempt of ctx if platform/pg created the unit of ctx: the attempt
// stored by this package must belong to the txn.Unit the kernel helpers see.
func ambient(ctx context.Context) (*unit, bool) {
	u, ok := ctx.Value(unitKey{}).(*unit)
	if !ok || u == nil {
		return nil, false
	}
	tu, ok := txn.UnitFrom(ctx)
	return u, ok && tu == u.unit
}
