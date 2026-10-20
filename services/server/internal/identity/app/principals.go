// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PrincipalWrites implements identity.PrincipalWrites over a Store with the handle and display
// name rules of M1-26 (principal.handle_max_chars, principal.display_name_max_chars). It is safe
// for concurrent use.
type PrincipalWrites struct {
	cfg   Config
	store Store
	clk   clock.Clock
}

// Compile-time check: PrincipalWrites is the port.
var _ identity.PrincipalWrites = (*PrincipalWrites)(nil)

// NewPrincipalWrites returns the port over store with the limits of cfg and the clock clk (nil
// means clock.System()) for disabled_at. Errors: a nil store or a limit below 1.
func NewPrincipalWrites(cfg Config, store Store, clk clock.Clock) (*PrincipalWrites, error) {
	if store == nil || cfg.HandleMaxChars < 1 || cfg.DisplayNameMaxChars < 1 {
		return nil, errors.New("identity: NewPrincipalWrites needs the store and the handle and display name limits")
	}
	if clk == nil {
		clk = clock.System()
	}
	return &PrincipalWrites{cfg: cfg, store: store, clk: clk}, nil
}

// errOutsideUnit is the misuse of a write outside the caller's unit of work.
func errOutsideUnit() error {
	return errs.New(errs.CodeInternalError, "identity: principal writes run inside the caller's unit of work")
}

// CreatePrincipal implements identity.PrincipalWrites.
func (w *PrincipalWrites) CreatePrincipal(ctx context.Context, cmd identity.NewPrincipalCmd) (identity.PrincipalView, error) {
	if !txn.InUnit(ctx) {
		return identity.PrincipalView{}, errOutsideUnit()
	}
	kind := domain.Kind(cmd.Kind)
	if cmd.Kind != actor.KindAgent && cmd.Kind != actor.KindApp {
		return identity.PrincipalView{}, errs.New(errs.CodeInternalError, "identity: CreatePrincipal takes agents and apps only")
	}
	handle, err := domain.NewHandle(cmd.Handle, w.cfg.HandleMaxChars)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	name, err := domain.NewDisplayName(cmd.DisplayName, w.cfg.DisplayNameMaxChars)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	p, err := domain.NewPrincipal(cmd.ID, kind, handle, name)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	if err := w.store.InsertPrincipal(ctx, &p); err != nil {
		return identity.PrincipalView{}, err
	}
	return principalView(p), nil
}

// SetPrincipalStatus implements identity.PrincipalWrites.
func (w *PrincipalWrites) SetPrincipalStatus(ctx context.Context, id ids.ID, status identity.Status) error {
	if !txn.InUnit(ctx) {
		return errOutsideUnit()
	}
	var at time.Time
	switch status {
	case identity.StatusActive:
	case identity.StatusDisabled:
		at = w.clk.Now()
	case identity.StatusPending, identity.StatusDeleted:
		return errs.New(errs.CodeValidationFailed, "only active and disabled").
			WithFields(errs.FieldError{Pointer: "/status", Code: domain.FieldPattern, Detail: "only active and disabled"})
	default:
		return errs.New(errs.CodeValidationFailed, "only active and disabled").
			WithFields(errs.FieldError{Pointer: "/status", Code: domain.FieldPattern, Detail: "only active and disabled"})
	}
	found, err := w.store.SetBotPrincipalStatus(ctx, id, domain.Status(status), at)
	if err != nil {
		return err
	}
	if !found {
		return notFound("principal")
	}
	return nil
}

// UpdateDisplayName implements identity.PrincipalWrites.
func (w *PrincipalWrites) UpdateDisplayName(ctx context.Context, id ids.ID, name string) error {
	if !txn.InUnit(ctx) {
		return errOutsideUnit()
	}
	n, err := domain.NewDisplayName(name, w.cfg.DisplayNameMaxChars)
	if err != nil {
		return err
	}
	found, err := w.store.UpdateBotDisplayName(ctx, id, n)
	if err != nil {
		return err
	}
	if !found {
		return notFound("principal")
	}
	return nil
}
