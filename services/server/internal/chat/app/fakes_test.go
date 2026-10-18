// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// store is an in-memory app.Store with the rules of the tables: a name unique case-insensitively
// among spaces that are not archived.
type store struct {
	mu      sync.Mutex
	spaces  map[ids.ID]domain.Space
	sidebar map[[2]ids.ID]domain.SidebarState
	fail    error
}

func newStore() *store {
	return &store{spaces: map[ids.ID]domain.Space{}, sidebar: map[[2]ids.ID]domain.SidebarState{}}
}

func (s *store) Space(_ context.Context, id ids.ID) (domain.Space, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp, ok := s.spaces[id]
	return sp, ok, s.fail
}

func (s *store) LockSpace(ctx context.Context, id ids.ID) (domain.Space, bool, error) {
	return s.Space(ctx, id)
}

func (s *store) LockActiveSpaces(context.Context) error { return s.fail }

func (s *store) ActiveOrder(context.Context) ([]app.SpaceKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.SpaceKey
	for _, sp := range s.sorted() {
		out = append(out, app.SpaceKey{ID: sp.ID, SortKey: sp.SortKey})
	}
	return out, s.fail
}

// sorted returns the spaces that are not archived in the global order.
func (s *store) sorted() []domain.Space {
	var out []domain.Space
	for _, sp := range s.spaces {
		if !sp.Archived() {
			out = append(out, sp)
		}
	}
	slices.SortFunc(out, func(a, b domain.Space) int {
		return cmp.Or(strings.Compare(a.SortKey, b.SortKey), strings.Compare(a.ID.String(), b.ID.String()))
	})
	return out
}

func (s *store) VisibleSpaces(_ context.Context, all bool, visible []ids.ID) ([]domain.Space, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Space
	for _, sp := range s.sorted() {
		if all || slices.Contains(visible, sp.ID) {
			out = append(out, sp)
		}
	}
	return out, s.fail
}

func (s *store) SpaceIDs(context.Context) ([]ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for id := range s.spaces {
		out = append(out, id)
	}
	return out, s.fail
}

func (s *store) taken(sp domain.Space) error {
	for _, o := range s.spaces {
		if o.ID != sp.ID && !o.Archived() && strings.EqualFold(o.Name, sp.Name) {
			return errs.New(errs.CodeConflict, "taken")
		}
	}
	return nil
}

func (s *store) InsertSpace(_ context.Context, sp domain.Space) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.taken(sp); err != nil {
		return err
	}
	s.spaces[sp.ID] = sp
	return s.fail
}

func (s *store) UpdateSpace(_ context.Context, sp domain.Space) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.taken(sp); err != nil {
		return err
	}
	s.spaces[sp.ID] = sp
	return s.fail
}

func (s *store) MoveSpace(_ context.Context, sp domain.Space) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spaces[sp.ID] = sp
	return s.fail
}

func (s *store) ArchiveSpace(ctx context.Context, sp domain.Space) error { return s.MoveSpace(ctx, sp) }

func (s *store) PutSidebarState(_ context.Context, st domain.SidebarState, keep bool) (domain.SidebarState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := [2]ids.ID{st.UserID, st.SpaceID}
	old, ok := s.sidebar[k]
	if keep {
		st.SortKey = old.SortKey
	}
	st.Version = 1
	if ok {
		st.Version = old.Version + 1
	}
	s.sidebar[k] = st
	return st, s.fail
}

func (s *store) SidebarStates(_ context.Context, user ids.ID) ([]domain.SidebarState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.SidebarState
	for k, st := range s.sidebar {
		if k[0] == user && !s.spaces[k[1]].Archived() {
			out = append(out, st)
		}
	}
	slices.SortFunc(out, func(a, b domain.SidebarState) int { return strings.Compare(a.SpaceID.String(), b.SpaceID.String()) })
	return out, s.fail
}

// uow runs fn in a unit in the work phase; the options are recorded, the registry is sealed and
// carries the test subscribers.
type uow struct {
	reg  *txn.Registry
	mu   sync.Mutex
	opts []txn.Options
}

func (u *uow) Do(ctx context.Context, fn func(ctx context.Context) error, opts ...txn.Option) error {
	if _, ok := txn.UnitFrom(ctx); ok {
		return fn(ctx)
	}
	var o txn.Options
	for _, f := range opts {
		f(&o)
	}
	u.mu.Lock()
	u.opts = append(u.opts, o)
	u.mu.Unlock()
	unit, set := txn.NewUnit(u.reg, o)
	set(txn.PhaseWork)
	return fn(txn.WithUnit(ctx, unit))
}

// authorizer decides by a rule per action; nil rules allow. Scope returns scope.
type authorizer struct {
	rules map[authorize.Action]func(p authorize.Principal, r authorize.Resource) authorize.Decision
	scope authorize.Scope
	err   error
}

var allow = authorize.Decision{Outcome: authorize.OutcomeAllow}

func (a *authorizer) Authorize(_ context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if a.err != nil {
		return authorize.Decision{}, a.err
	}
	if f := a.rules[act]; f != nil {
		return f(p, r), nil
	}
	return allow, nil
}

func (a *authorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return a.scope, a.err
}

// rights answers a fixed level and capabilities.
type rights struct {
	r   authz.EffectiveRights
	err error
}

func (r rights) EffectiveRights(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authz.EffectiveRights, error) {
	return r.r, r.err
}

// recorder keeps the allow entries.
type recorder struct {
	mu      sync.Mutex
	entries []auditing.Entry
}

func (r *recorder) Record(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}
func (r *recorder) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (r *recorder) RecordDetached(context.Context, auditing.Entry) error { return nil }

// appender keeps the events.
type appender struct {
	mu  sync.Mutex
	evs []eventlog.Event
}

func (a *appender) Append(_ context.Context, evs ...eventlog.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.evs = append(a.evs, evs...)
	return nil
}
