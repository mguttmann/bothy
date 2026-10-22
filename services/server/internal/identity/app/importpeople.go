// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ImportPeopleStore is the outgoing port of ImportPeople over principals and users (implemented
// by adapters/postgres). Writes run only inside a unit of work.
type ImportPeopleStore interface {
	// PrincipalByHandle returns the principal whose handle equals handle without regard to case.
	PrincipalByHandle(ctx context.Context, handle string) (p domain.Principal, found bool, err error)
	// InsertPrincipalIfAbsent inserts p and sets p.CreatedAt; a taken handle inserts nothing and
	// reports false without aborting the transaction.
	InsertPrincipalIfAbsent(ctx context.Context, p *domain.Principal) (bool, error)
	// InsertUser inserts the users row of u.
	InsertUser(ctx context.Context, u domain.User) error
	// SetPrincipalStatus sets status and disabled_at of id.
	SetPrincipalStatus(ctx context.Context, id ids.ID, status domain.Status) (domain.Principal, error)
}

// ImportPeople implements identity.ImportPeople (M8-03 design notes 7 and 8) with the handle and
// display name rules of every principal. It is safe for concurrent use.
type ImportPeople struct {
	cfg      Config
	store    ImportPeopleStore
	settings instance.Settings
	events   eventlog.Appender
}

// Compile-time check: the port.
var _ identity.ImportPeople = (*ImportPeople)(nil)

// NewImportPeople returns the port over store, the instance settings (locale.default of a new
// person) and the appender of user.updated. Errors: a missing collaborator or a limit below 1.
func NewImportPeople(cfg Config, store ImportPeopleStore, settings instance.Settings, events eventlog.Appender) (*ImportPeople, error) {
	if store == nil || settings == nil || events == nil || cfg.HandleMaxChars < 1 || cfg.DisplayNameMaxChars < 1 {
		return nil, errors.New("identity: NewImportPeople needs the store, the settings, the appender and the principal limits")
	}
	return &ImportPeople{cfg: cfg, store: store, settings: settings, events: events}, nil
}

// Resolve implements identity.ImportPeople: the principal of the handle must have the kind of
// ref (a person for user, an agent for agent) and be active.
func (p *ImportPeople) Resolve(ctx context.Context, ref identity.ImportRef) (ids.ID, error) {
	pr, found, err := p.store.PrincipalByHandle(ctx, ref.Handle)
	if err != nil {
		return ids.ID{}, err
	}
	if !found || actor.Kind(pr.Kind) != ref.Kind {
		return ids.ID{}, errs.New(errs.CodeNotFound, "")
	}
	if pr.Status != domain.StatusActive {
		return ids.ID{}, errs.New(errs.CodeConflict, "the principal is not active").WithReason(identity.ReasonImportPrincipalInactive)
	}
	return pr.ID, nil
}

// HandleFree implements identity.ImportPeople.
func (p *ImportPeople) HandleFree(ctx context.Context, handle string) (bool, error) {
	_, found, err := p.store.PrincipalByHandle(ctx, handle)
	return !found, err
}

// Placeholder implements identity.ImportPeople: a person with the users row of the instance
// defaults (locale.default), then status disabled with disabled_at in the same unit, and
// user.updated. Nothing else: no identity, no credential, no grant, no owner row (R-IM-06).
func (p *ImportPeople) Placeholder(ctx context.Context, spec identity.PlaceholderSpec) (ids.ID, error) {
	if !txn.InUnit(ctx) {
		return ids.ID{}, errOutsideUnit()
	}
	h, err := domain.NewHandle(spec.Handle, p.cfg.HandleMaxChars)
	if err != nil {
		return ids.ID{}, err
	}
	d, err := domain.NewDisplayName(spec.DisplayName, p.cfg.DisplayNameMaxChars)
	if err != nil {
		return ids.ID{}, err
	}
	pr, err := domain.NewPrincipal(ids.New(), domain.KindUser, h, d)
	if err != nil {
		return ids.ID{}, err
	}
	locale, err := p.settings.LocaleDefault(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	inserted, err := p.store.InsertPrincipalIfAbsent(ctx, &pr)
	if err != nil {
		return ids.ID{}, err
	}
	if !inserted {
		// Taken since HandleFree, also by a concurrent unit: the caller takes the next counter.
		return ids.ID{}, errs.New(errs.CodeConflict, "the value of handle is already taken").
			WithFields(errs.FieldError{Pointer: pointerTakenHandle, Code: "taken", Detail: "the handle is taken"})
	}
	u, err := domain.NewUser(pr, "", locale, "")
	if err != nil {
		return ids.ID{}, err
	}
	if err := p.store.InsertUser(ctx, u); err != nil {
		return ids.ID{}, err
	}
	if u.Principal, err = p.store.SetPrincipalStatus(ctx, pr.ID, domain.StatusDisabled); err != nil {
		return ids.ID{}, err
	}
	return pr.ID, appendUserUpdated(ctx, p.events, userView(UserRecord{User: u}))
}

// pointerTakenHandle is the pointer of the conflict of a taken placeholder handle.
const pointerTakenHandle = "/handle"
