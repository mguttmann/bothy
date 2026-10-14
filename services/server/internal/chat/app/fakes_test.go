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
	mu       sync.Mutex
	spaces   map[ids.ID]domain.Space
	sidebar  map[[2]ids.ID]domain.SidebarState
	channels map[ids.ID]domain.Channel
	tags     map[ids.ID]domain.ForumTag
	fail     error
}

func newStore() *store {
	return &store{spaces: map[ids.ID]domain.Space{}, sidebar: map[[2]ids.ID]domain.SidebarState{},
		channels: map[ids.ID]domain.Channel{}, tags: map[ids.ID]domain.ForumTag{}}
}

// row returns the channel c with the facts of its space.
func (s *store) row(c domain.Channel) app.ChannelRow {
	sp := s.spaces[c.SpaceID]
	return app.ChannelRow{Channel: c, SpaceDefaultOperatorID: sp.DefaultOperatorAgentID, SpaceArchived: sp.Archived()}
}

func (s *store) Channel(_ context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.channels[id]
	return s.row(c), ok, s.fail
}

func (s *store) LockChannel(ctx context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	return s.Channel(ctx, id)
}

// liveChannels returns the channels of space that are not archived in their order.
func (s *store) liveChannels(space ids.ID) []domain.Channel {
	var out []domain.Channel
	for _, c := range s.channels {
		if c.SpaceID == space && !c.Archived() {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b domain.Channel) int {
		return cmp.Or(strings.Compare(a.SortKey, b.SortKey), strings.Compare(a.ID.String(), b.ID.String()))
	})
	return out
}

func (s *store) ChannelOrder(_ context.Context, space ids.ID) ([]domain.Keyed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Keyed
	for _, c := range s.liveChannels(space) {
		out = append(out, domain.Keyed{ID: c.ID, SortKey: c.SortKey})
	}
	return out, s.fail
}

func (s *store) VisibleChannels(_ context.Context, space ids.ID, all bool, visible, fully []ids.ID) ([]app.ChannelRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.ChannelRow
	for _, c := range s.liveChannels(space) {
		if all || slices.Contains(visible, c.ID) || slices.Contains(fully, c.SpaceID) {
			out = append(out, s.row(c))
		}
	}
	return out, s.fail
}

func (s *store) AllChannels(context.Context) ([]app.ChannelRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.ChannelRow
	for _, c := range s.channels {
		out = append(out, s.row(c))
	}
	return out, s.fail
}

// channelTaken reports a name taken in the space.
func (s *store) channelTaken(c domain.Channel) error {
	for _, o := range s.channels {
		if o.ID != c.ID && o.SpaceID == c.SpaceID && !o.Archived() && strings.EqualFold(o.Name, c.Name) {
			return errs.New(errs.CodeConflict, "taken")
		}
	}
	return nil
}

func (s *store) InsertChannel(_ context.Context, c domain.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return s.fail
	}
	if err := s.channelTaken(c); err != nil {
		return err
	}
	s.channels[c.ID] = c
	return nil
}

func (s *store) SaveChannel(_ context.Context, c domain.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.channelTaken(c); err != nil {
		return err
	}
	c.MainThreadID = s.channels[c.ID].MainThreadID
	s.channels[c.ID] = c
	return s.fail
}

func (s *store) InsertMainThread(_ context.Context, th domain.Thread) (domain.Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.channels[th.ChannelID]
	c.MainThreadID = th.ID
	s.channels[th.ChannelID] = c
	return th, s.fail
}

func (s *store) ForumTags(_ context.Context, channel ids.ID) ([]domain.ForumTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.ForumTag
	for _, t := range s.tags {
		if t.ChannelID == channel && !t.Deleted() {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b domain.ForumTag) int { return strings.Compare(a.SortKey, b.SortKey) })
	return out, s.fail
}

func (s *store) ForumTag(_ context.Context, id ids.ID) (domain.ForumTag, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tags[id]
	return t, ok, s.fail
}

func (s *store) InsertForumTag(ctx context.Context, t domain.ForumTag) error {
	return s.SaveForumTag(ctx, t)
}

func (s *store) SaveForumTag(_ context.Context, t domain.ForumTag) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.tags {
		if o.ID != t.ID && o.ChannelID == t.ChannelID && !o.Deleted() && strings.EqualFold(o.Name, t.Name) {
			return errs.New(errs.CodeConflict, "taken")
		}
	}
	s.tags[t.ID] = t
	return s.fail
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
