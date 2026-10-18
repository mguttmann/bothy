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
	//	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "grant.create", Principals: []ids.ID{subject}}))
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
	// Action is the catalogue action that causes the rights change (DeclareRightsChange); the
	// authz hooks write it into access.changed and the deny audit entry. Empty when only
	// RightsChange was used.
	Action string
	// Resource is the resource the deny audit entry names; the zero Ref for none.
	Resource Ref
	// Targets are the spaces or channels whose rights change (archive); authz derives the
	// affected principals from them (02 section 8.1). First-named order without duplicates.
	Targets []Ref
	// LateRightsChange lets an outermost unit without RightsChange become one through its first
	// nested Do that declares a rights change (AllowLateRightsChange).
	LateRightsChange bool
}

// Ref names a resource or a target of a rights change by its catalogue resource kind and id. The
// kind is a plain string because kernel ports never import each other (02 section 7.2); authz
// maps it to its own types.
type Ref struct {
	// Kind is the resource kind of the catalogue, for example "space", "channel" or "grant".
	Kind string
	// ID is the resource.
	ID ids.ID
}

// RightsChangeSpec declares a rights change to the unit of work (02 section 8.1, brief M1-29):
// the action, the resource of the deny audit entry, the principals whose visible targets may
// change and the targets whose rights change. The authz hooks read it with RightsChangeOf.
type RightsChangeSpec struct {
	// Action is the catalogue action that causes the change, for example "grant.create".
	Action string
	// Resource is written into the deny audit entry (kind and id).
	Resource Ref
	// Principals are affected directly (the subject of a grant, a new owner).
	Principals []ids.ID
	// Targets are spaces or channels whose rights change; authz derives the affected principals.
	Targets []Ref
}

// DeclareRightsChange marks a unit as a rights change with the full declaration of spec. It
// adds up with RightsChange and further DeclareRightsChange options: principals and targets are
// joined without duplicates; the first non-empty action and resource win.
//
// Example:
//
//	err := uow.Do(ctx, fn, txn.DeclareRightsChange(txn.RightsChangeSpec{
//		Action: "channel.archive", Resource: txn.Ref{Kind: "channel", ID: ch},
//		Targets: []txn.Ref{{Kind: "channel", ID: ch}},
//	}))
func DeclareRightsChange(spec RightsChangeSpec) Option {
	return func(o *Options) {
		RightsChange(spec.Principals...)(o)
		if o.Action == "" {
			o.Action = spec.Action
		}
		if o.Resource == (Ref{}) {
			o.Resource = spec.Resource
		}
		for _, t := range spec.Targets {
			if !slices.Contains(o.Targets, t) {
				o.Targets = append(o.Targets, t)
			}
		}
	}
}

// Spec returns the declaration of o as a RightsChangeSpec with copied slices.
func (o Options) Spec() RightsChangeSpec {
	return RightsChangeSpec{
		Action: o.Action, Resource: o.Resource,
		Principals: slices.Clone(o.Affected), Targets: slices.Clone(o.Targets),
	}
}

// RightsChange marks a unit that changes rights and names the affected principals (the subject
// of a grant, for example). It adds principals to a DeclareRightsChange; alone it names no
// catalogue action, which the authz rights hooks refuse (M1-29), so production code declares with
// DeclareRightsChange. It counts only at the outermost unit: a nested Do with
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

// AllowLateRightsChange lets an outermost unit that does not declare a rights change become one
// later, once: the first nested Do that declares a rights change runs the rights begin hook
// before its function and the rights check hook right after it, inside the ambient transaction;
// sync and audit still run once at the end of the outer unit. Only the Idempotency-Key stage of
// platform/httpserver opens its unit with it (docs/DECISIONS.md M1-20/4 and M1-31): when the
// nested unit begins, the stage has written nothing but its claim row of idempotency_keys, so the
// begin hook still locks permission_version before any domain row (03 "Lock order"). The
// caller of this option guarantees that; platform/pg cannot see it. A failed late rights change
// fails the whole unit, also when its caller swallows the error, so a use case behind the stage
// must not rely on swallowing an error of its rights unit (authz twoUnits does for errRaced;
// M1-48, which keys revokeGrant, must hand that case on, docs/DECISIONS.md M1-31).
func AllowLateRightsChange() Option {
	return func(o *Options) { o.LateRightsChange = true }
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
// has no RightsChange, or o has one that names every principal and target inner names and the
// same action (or inner names none).
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
	for _, t := range inner.Targets {
		if !slices.Contains(o.Targets, t) {
			return false
		}
	}
	return inner.Action == "" || inner.Action == o.Action
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
	// after are the functions AfterCommit registered, in order.
	after []func()
}

// NewUnit returns a unit in PhaseNone with an empty state, and the function that moves it to
// another phase. Only the caller of NewUnit holds that function, so code that reads the unit
// from a context cannot change its phase (instance.Counters trusts the phase, M1-15).
func NewUnit(reg *Registry, opts Options) (*Unit, func(Phase)) {
	u, c := NewControlledUnit(reg, opts)
	return u, c.SetPhase
}

// UnitControl changes a unit: its phase and, once, its declaration of a late rights change. Only
// the caller of NewControlledUnit holds it.
type UnitControl struct {
	u *Unit
}

// NewControlledUnit returns a unit in PhaseNone with an empty state and its control. platform/pg
// uses it to declare a late rights change (AllowLateRightsChange).
func NewControlledUnit(reg *Registry, opts Options) (*Unit, UnitControl) {
	u := &Unit{reg: reg, opts: opts, values: map[any]any{}}
	return u, UnitControl{u: u}
}

// SetPhase moves the unit to p.
func (c UnitControl) SetPhase(p Phase) { c.u.phase.Store(uint32(p)) }

// Declare replaces the options of the unit with o; platform/pg calls it once, when a unit opened
// with AllowLateRightsChange becomes a rights change.
func (c UnitControl) Declare(o Options) {
	c.u.mu.Lock()
	defer c.u.mu.Unlock()
	c.u.opts = o
}

// Committed runs the functions AfterCommit registered on the unit, in registration order, once;
// platform/pg calls it right after a successful COMMIT of the attempt and never for an attempt
// that rolled back.
func (c UnitControl) Committed() {
	c.u.mu.Lock()
	after := c.u.after
	c.u.after = nil
	c.u.mu.Unlock()
	for _, fn := range after {
		fn()
	}
}

// AfterCommit registers fn to run after the outermost unit of ctx commits, for facts that may
// only be told once they are durable (bothy.message.created and bothy.event.appended count after
// the commit, signals.md 4.2). A nested unit registers on the outermost one. A rolled-back attempt
// drops its registrations, and a retried attempt starts without them. fn runs in the goroutine of
// Do, after COMMIT and before Do returns; it must be quick and must not fail (counting a metric).
// AfterCommit reports false, and fn is not kept, outside a unit of work.
func AfterCommit(ctx context.Context, fn func()) bool {
	u, ok := UnitFrom(ctx)
	if !ok || fn == nil {
		return false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.after = append(u.after, fn)
	return true
}

// Phase returns the current phase.
func (u *Unit) Phase() Phase {
	return Phase(u.phase.Load()) // #nosec G115 -- only the setter of NewUnit stores, from a Phase.
}

// Registry returns the registry of hooks and subscribers.
func (u *Unit) Registry() *Registry { return u.reg }

// Options returns a copy of the options of the outermost Do.
func (u *Unit) Options() Options {
	u.mu.Lock()
	o := u.opts
	u.mu.Unlock()
	o.Affected = slices.Clone(o.Affected)
	o.Targets = slices.Clone(o.Targets)
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
	if !ok {
		return nil, false
	}
	o := u.Options()
	if !o.RightsChange {
		return nil, false
	}
	return o.Affected, true
}

// RightsChangeOf returns the declaration of the outermost unit of ctx and true, or the zero
// value and false outside a unit or without RightsChange. The authz hooks (M1-29) read it.
func RightsChangeOf(ctx context.Context) (RightsChangeSpec, bool) {
	u, ok := UnitFrom(ctx)
	if !ok {
		return RightsChangeSpec{}, false
	}
	o := u.Options()
	if !o.RightsChange {
		return RightsChangeSpec{}, false
	}
	return o.Spec(), true
}

// InRightsChange reports whether ctx is inside a unit of work declared as a rights change.
// Stores whose writes change rights refuse to write otherwise (authz, M1-29), and the
// Authorizer bypasses its cache there, because the unit sees an uncommitted rights version.
func InRightsChange(ctx context.Context) bool {
	_, ok := RightsChangeOf(ctx)
	return ok
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
