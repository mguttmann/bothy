// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// state implements instance.State.
type state struct {
	repo Repository
	uow  txn.UnitOfWork
}

// Compile-time check: state is the port.
var _ instance.State = (*state)(nil)

// NewState returns the implementation of instance.State over repo; RecordMigration writes in a
// unit of work of uow (joining the ambient one when ctx is inside a unit).
func NewState(repo Repository, uow txn.UnitOfWork) instance.State {
	return &state{repo: repo, uow: uow}
}

// Load returns the instance row (instance.State.Load); a missing row is kind not_found with the
// hint to run bothy migrate up.
func (s *state) Load(ctx context.Context) (instance.Record, error) {
	rec, found, err := s.repo.LoadInstance(ctx)
	if err != nil {
		return instance.Record{}, err
	}
	if !found {
		return instance.Record{}, errs.New(errs.CodeNotFound, `the instance row is missing; run "bothy migrate up"`)
	}
	return rec, nil
}

// RecordMigration creates the row if missing and records the versions in one unit of work
// (instance.State.RecordMigration). The IDs are new UUIDv7 values of the application (R-DM-01);
// when the row exists they are discarded by ON CONFLICT DO NOTHING.
func (s *state) RecordMigration(ctx context.Context, schemaVersion int64, minServerVersion string) (instance.Record, error) {
	if schemaVersion < 1 {
		return instance.Record{}, errs.New(errs.CodeInternalError, fmt.Sprintf("instance: schema version %d; want at least 1", schemaVersion))
	}
	if err := domain.ValidateSemVer(minServerVersion); err != nil {
		return instance.Record{}, errs.Wrap(errs.CodeInternalError, err, "instance: the minimum server version is not SemVer 2.0.0")
	}
	var rec instance.Record
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.EnsureInstance(ctx, ids.New(), ids.New()); err != nil {
			return err
		}
		if err := s.repo.SetVersions(ctx, schemaVersion, minServerVersion); err != nil {
			return err
		}
		var err error
		rec, err = s.Load(ctx)
		return err
	})
	return rec, err
}

// LockForSetup implements instance.State: the row locked FOR UPDATE in the ambient unit of work.
func (s *state) LockForSetup(ctx context.Context) (instance.Record, error) {
	if !txn.InUnit(ctx) {
		return instance.Record{}, errs.New(errs.CodeInternalError, "instance: LockForSetup needs a unit of work")
	}
	rec, found, err := s.repo.LockInstance(ctx)
	if err != nil {
		return instance.Record{}, err
	}
	if !found {
		return instance.Record{}, errs.New(errs.CodeNotFound, `the instance row is missing; run "bothy migrate up"`)
	}
	return rec, nil
}

// CompleteSetup implements instance.State.
func (s *state) CompleteSetup(ctx context.Context, at time.Time) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "instance: CompleteSetup needs a unit of work")
	}
	changed, err := s.repo.CompleteSetup(ctx, at)
	if err != nil {
		return err
	}
	if !changed {
		return errs.New(errs.CodeConflict, "instance: setup is already complete")
	}
	return nil
}
