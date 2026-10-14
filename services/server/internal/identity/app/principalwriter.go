// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PrincipalStore is the outgoing port of PrincipalWriter over principals (implemented by
// adapters/postgres, M1-51). Writes run only inside a unit of work and fail with kind internal
// outside one; errors follow Store.
type PrincipalStore interface {
	// InsertPrincipal inserts p and sets p.CreatedAt from the database.
	InsertPrincipal(ctx context.Context, p *domain.Principal) error
	// LockPrincipal returns the principal id and locks its row until the end of the unit of work;
	// found is false for an unknown id.
	LockPrincipal(ctx context.Context, id ids.ID) (p domain.Principal, found bool, err error)
	// SetPrincipalStatus sets the status of id to active (disabled_at cleared) or disabled
	// (disabled_at set to the transaction time) and returns disabled_at.
	SetPrincipalStatus(ctx context.Context, id ids.ID, status domain.Status) (domain.Principal, error)
	// SetPrincipalDisplayName writes the display name of id.
	SetPrincipalDisplayName(ctx context.Context, id ids.ID, displayName domain.DisplayName) error
}

// PrincipalWriter implements identity.PrincipalWriter over a PrincipalStore with the handle and
// display name limits of the configuration. It is safe for concurrent use.
type PrincipalWriter struct {
	cfg   Config
	store PrincipalStore
}

// Compile-time check: the writer is the port.
var _ identity.PrincipalWriter = (*PrincipalWriter)(nil)

// NewPrincipalWriter returns the writer. Errors: a missing store, a limit below 1.
func NewPrincipalWriter(cfg Config, store PrincipalStore) (*PrincipalWriter, error) {
	if store == nil {
		return nil, errors.New("identity: NewPrincipalWriter needs a store")
	}
	if cfg.HandleMaxChars < 1 || cfg.DisplayNameMaxChars < 1 {
		return nil, errors.New("identity: principal limits must be at least 1")
	}
	return &PrincipalWriter{cfg: cfg, store: store}, nil
}

// errPeople is the refusal of a person: their lifecycle belongs to People (M1-31, M1-45).
func errPeople() error {
	return errs.New(errs.CodeInternalError, "identity: people change through the people use cases")
}

// CreatePrincipal implements identity.PrincipalWriter with the rules of M1-26: the handle and the
// display name are validated like those of a person, and the unique index on lower(handle)
// answers a taken handle with conflict on /handle.
func (w *PrincipalWriter) CreatePrincipal(ctx context.Context, cmd identity.CreatePrincipalCmd) (identity.PrincipalView, error) {
	var kind domain.Kind
	switch cmd.Kind {
	case actor.KindAgent:
		kind = domain.KindAgent
	case actor.KindApp:
		kind = domain.KindApp
	case actor.KindUser, actor.KindHost, actor.KindAnonymous:
		return identity.PrincipalView{}, errPeople()
	default:
		return identity.PrincipalView{}, errs.New(errs.CodeInternalError, "identity: unknown principal kind")
	}
	h, err := domain.NewHandle(cmd.Handle, w.cfg.HandleMaxChars)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	d, err := domain.NewDisplayName(cmd.DisplayName, w.cfg.DisplayNameMaxChars)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	id := cmd.ID
	if id.IsZero() {
		id = ids.New()
	}
	p, err := domain.NewPrincipal(id, kind, h, d)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	if err := w.store.InsertPrincipal(ctx, &p); err != nil {
		return identity.PrincipalView{}, err
	}
	return principalView(p), nil
}

// lockOther locks the principal id and refuses a person or an unknown id.
func (w *PrincipalWriter) lockOther(ctx context.Context, id ids.ID) (domain.Principal, error) {
	p, found, err := w.store.LockPrincipal(ctx, id)
	if err != nil {
		return domain.Principal{}, err
	}
	if !found {
		return domain.Principal{}, notFound("principal")
	}
	if p.Kind == domain.KindUser {
		return domain.Principal{}, errPeople()
	}
	return p, nil
}

// SetPrincipalStatus implements identity.PrincipalWriter: active and disabled are the targets; a
// deleted principal stays deleted (08 section 11).
func (w *PrincipalWriter) SetPrincipalStatus(ctx context.Context, id ids.ID, status identity.Status) (identity.PrincipalView, bool, error) {
	target := domain.Status(status)
	if target != domain.StatusActive && target != domain.StatusDisabled {
		return identity.PrincipalView{}, false, errs.New(errs.CodeInvalidStateTransition, "only active and disabled can be set")
	}
	p, err := w.lockOther(ctx, id)
	if err != nil {
		return identity.PrincipalView{}, false, err
	}
	switch {
	case p.Status == target:
		return principalView(p), false, nil
	case p.Status != domain.StatusActive && p.Status != domain.StatusDisabled:
		return identity.PrincipalView{}, false, errs.New(errs.CodeInvalidStateTransition, "the principal cannot change its status")
	}
	updated, err := w.store.SetPrincipalStatus(ctx, id, target)
	if err != nil {
		return identity.PrincipalView{}, false, err
	}
	return principalView(updated), true, nil
}

// SetDisplayName implements identity.PrincipalWriter.
func (w *PrincipalWriter) SetDisplayName(ctx context.Context, id ids.ID, displayName string) (identity.PrincipalView, bool, error) {
	d, err := domain.NewDisplayName(displayName, w.cfg.DisplayNameMaxChars)
	if err != nil {
		return identity.PrincipalView{}, false, err
	}
	p, err := w.lockOther(ctx, id)
	if err != nil {
		return identity.PrincipalView{}, false, err
	}
	if p.DisplayName == d {
		return principalView(p), false, nil
	}
	if err := w.store.SetPrincipalDisplayName(ctx, id, d); err != nil {
		return identity.PrincipalView{}, false, err
	}
	p.DisplayName = d
	return principalView(p), true, nil
}
