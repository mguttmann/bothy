// SPDX-License-Identifier: Apache-2.0

package txn

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// UnitOfWork runs a write use case in exactly one transaction (02 "Flow", R-AR-06).
// platform/pg implements it; app packages receive it through their constructor.
type UnitOfWork interface {
	// Do runs fn inside a unit of work. Outside a unit it begins a READ COMMITTED transaction,
	// runs the phases before the commit and commits; it retries the whole unit on deadlock or
	// lock timeout. Inside a unit it joins the ambient transaction, runs fn in the current phase
	// and never commits, rolls back or retries by itself.
	//
	// Errors: the error of fn, of a hook or of a subscriber unchanged (veto); after the last
	// retry an error of kind unavailable wrapping the database error; kind internal for misuse
	// (unsealed registry, nested RightsChange without cover).
	//
	// Example:
	//
	//	err := uow.Do(ctx, func(ctx context.Context) error {
	//		if err := repo.Insert(ctx, row); err != nil {
	//			return err
	//		}
	//		return txn.Publish(ctx, chat.ThreadCreated{ID: row.ID})
	//	}, txn.RightsChange(subject))
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error
}

// Option changes how a unit of work runs.
type Option func(*Options)

// Options are the collected options of one Do call. platform/pg reads them with ApplyOptions.
type Options struct {
	// RightsChange is true when the unit changes rights: then the rights begin and the rights
	// check hooks of authz run (02 "Flow" phase 1 and 3.1).
	RightsChange bool
	// Affected are the principals RightsChange named, in first-named order without duplicates.
	Affected []ids.ID
}

// RightsChange marks a unit that changes rights and names the affected principals (the subject
// of a grant, for example). It counts only at the outermost unit: a nested Do with
// RightsChange is an internal error unless the outer unit declared RightsChange with a
// superset of the principals, because the rights begin hook must lock permission_version before
// any domain row (03 "Lock order" item 1). Several RightsChange options add up.
func RightsChange(affected ...ids.ID) Option {
	return func(o *Options) {
		o.RightsChange = true
		for _, id := range affected {
			if !slices.Contains(o.Affected, id) {
				o.Affected = append(o.Affected, id)
			}
		}
	}
}

// ApplyOptions collects opts in order; nil entries are ignored.
func ApplyOptions(opts ...Option) Options {
	var o Options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// Covers reports whether a nested unit with options inner may join a unit with options o: inner
// has no RightsChange, or o has one that names every principal inner names.
func (o Options) Covers(inner Options) bool {
	if !inner.RightsChange {
		return true
	}
	if !o.RightsChange {
		return false
	}
	for _, id := range inner.Affected {
		if !slices.Contains(o.Affected, id) {
			return false
		}
	}
	return true
}

// Unit is one attempt of a unit of work as the kernel helpers see it: its phase, its registry,
// its options and its per-attempt state behind Load and Store. Only platform/pg creates units
// (one per attempt, NewUnit) and moves them through the phases; other packages only read them
// through CurrentPhase, InUnit, AffectedPrincipals, Publish, Load and Store. It is safe for
// concurrent use.
//
// Unit is a kernel type rather than an interface platform/pg implements: the architecture gate
// (go-arch-lint, deep scan) forbids handing an implementation of platform/pg to kernel, which
// depends on nothing (docs/DECISIONS.md M1-13/11).
type Unit struct {
	phase  atomic.Uint32
	reg    *Registry
	opts   Options
	mu     sync.Mutex
	values map[any]any
}

// NewUnit returns a unit in PhaseNone with an empty state, and the function that moves it to
// another phase. Only the caller of NewUnit holds that function, so code that reads the unit
// from a context cannot change its phase (instance.Counters trusts the phase, M1-15).
func NewUnit(reg *Registry, opts Options) (*Unit, func(Phase)) {
	u := &Unit{reg: reg, opts: opts, values: map[any]any{}}
	return u, func(p Phase) { u.phase.Store(uint32(p)) }
}

// Phase returns the current phase.
func (u *Unit) Phase() Phase {
	return Phase(u.phase.Load()) // #nosec G115 -- only the setter of NewUnit stores, from a Phase.
}

// Registry returns the registry of hooks and subscribers.
func (u *Unit) Registry() *Registry { return u.reg }

// Options returns a copy of the options of the outermost Do.
func (u *Unit) Options() Options {
	o := u.opts
	o.Affected = slices.Clone(o.Affected)
	return o
}

// unitKey is the context key of the unit.
type unitKey struct{}

// WithUnit returns a context that carries u. Only platform/pg calls it.
func WithUnit(ctx context.Context, u *Unit) context.Context {
	return context.WithValue(ctx, unitKey{}, u)
}

// UnitFrom returns the unit of ctx and whether there is one.
func UnitFrom(ctx context.Context) (*Unit, bool) {
	u, ok := ctx.Value(unitKey{}).(*Unit)
	return u, ok && u != nil
}

// Detach returns a copy of ctx that is outside any unit of work but keeps every other value
// (actor, correlation ID, client IP) and the cancellation of ctx. A UnitOfWork.Do on the result
// begins its own transaction instead of joining the ambient one; the audit recorder writes a
// denial after a rollback and an error entry this way (03 "I1 check", M1-24).
//
// Example:
//
//	err := uow.Do(txn.Detach(ctx), func(ctx context.Context) error { ... })
func Detach(ctx context.Context) context.Context {
	return context.WithValue(ctx, unitKey{}, (*Unit)(nil))
}

// InUnit reports whether ctx is inside a unit of work.
func InUnit(ctx context.Context) bool {
	_, ok := UnitFrom(ctx)
	return ok
}

// CurrentPhase returns the phase of the unit of ctx, or PhaseNone outside a unit.
func CurrentPhase(ctx context.Context) Phase {
	if u, ok := UnitFrom(ctx); ok {
		return u.Phase()
	}
	return PhaseNone
}

// AffectedPrincipals returns a copy of the principals the outermost unit named with
// RightsChange and true, or nil and false outside a unit or without RightsChange. The authz
// hooks (M1-29) read it.
func AffectedPrincipals(ctx context.Context) ([]ids.ID, bool) {
	u, ok := UnitFrom(ctx)
	if !ok || !u.opts.RightsChange {
		return nil, false
	}
	return slices.Clone(u.opts.Affected), true
}

// Key names one value of the per-unit state (durable events of sync, audit records of
// audit). platform/pg creates a fresh Unit for every attempt and drops it with the rollback, so
// a retry or a veto never sees values of an earlier attempt. Keys compare by identity, not by name: two
// NewKey calls with the same name are two keys. Declare a key once per package.
type Key[T any] struct {
	name string
}

// NewKey returns a new key; name only labels it in errors.
func NewKey[T any](name string) *Key[T] { return &Key[T]{name: name} }

// Name returns the label of the key.
func (k *Key[T]) Name() string { return k.name }

// Load returns the value stored under k in the unit of ctx and true, or the zero value and
// false when nothing is stored or ctx is outside a unit.
func Load[T any](ctx context.Context, k *Key[T]) (T, bool) {
	var zero T
	u, ok := UnitFrom(ctx)
	if !ok {
		return zero, false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	v, ok := u.values[k].(T)
	if !ok {
		return zero, false
	}
	return v, true
}

// Store stores v under k in the unit of ctx. The error (kind internal) reports a ctx outside a
// unit: there is no state to keep the value, and dropping it silently would lose an event.
func Store[T any](ctx context.Context, k *Key[T], v T) error {
	u, ok := UnitFrom(ctx)
	if !ok {
		return errs.New(errs.CodeInternalError, "txn: Store of "+k.name+" outside a unit of work")
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.values[k] = v
	return nil
}
