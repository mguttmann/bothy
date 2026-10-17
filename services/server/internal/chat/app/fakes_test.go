// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"cmp"
	"context"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
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
	threadStore
	// policies are the agent policies storePolicies answers.
	policies map[ids.ID]authz.AgentPolicy
	fail     error
}

func newStore() *store {
	return &store{spaces: map[ids.ID]domain.Space{}, sidebar: map[[2]ids.ID]domain.SidebarState{},
		channels: map[ids.ID]domain.Channel{}, tags: map[ids.ID]domain.ForumTag{}, policies: map[ids.ID]authz.AgentPolicy{}}
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
	s.ensureThreads()
	s.threads[th.ID] = th
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

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (*authorizer) Observe(context.Context, authorize.LogEvent) {}

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

// affected records the principals the operator use cases name (M1-54).
type affected struct {
	mu   sync.Mutex
	list []ids.ID
}

func (a *affected) AddAffected(_ context.Context, ps ...ids.ID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = append(a.list, ps...)
	return nil
}

// directory answers the principals of agents; an unknown id is not_found.
type directory struct {
	identity.Directory
	principals map[ids.ID]identity.PrincipalView
}

// SearchLanguage answers the language of a thread's creator: german for every principal (the
// rule itself is tested in identity/app and test/integration).
func (d directory) SearchLanguage(context.Context, ids.ID) (string, error) { return "german", nil }

// InstanceSearchLanguage answers the instance language: english.
func (d directory) InstanceSearchLanguage(context.Context) (string, error) { return "english", nil }

func (d directory) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	p, ok := d.principals[id]
	if !ok {
		return identity.PrincipalView{}, errs.New(errs.CodeNotFound, "")
	}
	return p, nil
}

func (s *store) SaveSpaceOperator(ctx context.Context, sp domain.Space) error {
	return s.MoveSpace(ctx, sp)
}

func (s *store) SaveChannelOperator(ctx context.Context, c domain.Channel) error {
	return s.SaveChannel(ctx, c)
}

func (s *store) TouchChannel(ctx context.Context, c domain.Channel) error {
	return s.SaveChannel(ctx, c)
}

func (s *store) LockInheritingChannels(_ context.Context, space ids.ID) ([]app.ChannelRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.ChannelRow
	for _, c := range s.liveChannels(space) {
		if c.OperatorInherit {
			out = append(out, s.row(c))
		}
	}
	return out, s.fail
}

// threadStore is the in-memory thread part of store (M1-55): threads with their tags and the
// unique parent message.
type threadStore struct {
	threads    map[ids.ID]domain.Thread
	threadTags map[ids.ID][]ids.ID
}

func (s *store) ensureThreads() {
	if s.threads == nil {
		s.threads, s.threadTags = map[ids.ID]domain.Thread{}, map[ids.ID][]ids.ID{}
	}
}

func (s *store) DeleteTagAssignments(_ context.Context, tag ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	for th, tags := range s.threadTags {
		s.threadTags[th] = slices.DeleteFunc(tags, func(t ids.ID) bool { return t == tag })
	}
	return s.fail
}

func (s *store) Thread(_ context.Context, id ids.ID) (domain.Thread, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	th, ok := s.threads[id]
	return th, ok, s.fail
}

func (s *store) LockThread(ctx context.Context, id ids.ID) (domain.Thread, bool, error) {
	return s.Thread(ctx, id)
}

func (s *store) ShareThread(ctx context.Context, id ids.ID) (domain.Thread, bool, error) {
	return s.Thread(ctx, id)
}

func (s *store) ThreadCounters(_ context.Context, channels []ids.ID) ([]chat.ThreadCounter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	var out []chat.ThreadCounter
	for _, th := range s.threads {
		if slices.Contains(channels, th.ChannelID) && th.LastMessageSeq > th.UnreadFloorSeq {
			out = append(out, chat.ThreadCounter{ThreadID: th.ID, ChannelID: th.ChannelID, LastMessageSeq: th.LastMessageSeq,
				UnreadFloorSeq: th.UnreadFloorSeq})
		}
	}
	return out, s.fail
}

func (s *store) InsertThread(_ context.Context, th domain.Thread) (domain.Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	for _, o := range s.threads {
		if !th.ParentMessageID.IsZero() && o.ParentMessageID == th.ParentMessageID {
			return domain.Thread{}, errs.New(errs.CodeConflict, "taken")
		}
	}
	s.threads[th.ID] = th
	return th, s.fail
}

func (s *store) SaveThread(_ context.Context, th domain.Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	s.threads[th.ID] = th
	return s.fail
}

func (s *store) AppendToThread(_ context.Context, id ids.ID, reopen bool, at time.Time) (domain.Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	th := s.threads[id]
	th.LastMessageSeq++
	th.MessageCount++
	th.Version++
	th.LastActivityAt = at
	if reopen {
		th.State, th.ClosedAt, th.ClosedBy = domain.ThreadOpen, time.Time{}, ids.ID{}
	}
	s.threads[id] = th
	return th, s.fail
}

func (s *store) DecrementMessageCount(_ context.Context, id ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	th, ok := s.threads[id]
	if !ok || th.MessageCount < 1 {
		return errs.New(errs.CodeInternalError, "no thread with a message")
	}
	th.MessageCount--
	th.Version++
	s.threads[id] = th
	return s.fail
}

func (s *store) ListThreads(_ context.Context, q app.ThreadQuery) ([]domain.Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	at := func(th domain.Thread) time.Time {
		if q.ByCreated {
			return th.CreatedAt
		}
		return th.LastActivityAt
	}
	var out []domain.Thread
	for _, th := range s.threads {
		switch {
		case th.ChannelID != q.ChannelID || th.Kind != domain.ThreadKindThread:
		case q.State != "" && string(th.State) != q.State:
		case !q.TagID.IsZero() && !slices.Contains(s.threadTags[th.ID], q.TagID):
		case q.HasAfter && (at(th).After(q.AfterAt) || (at(th).Equal(q.AfterAt) && th.ID.String() >= q.AfterID.String())):
		default:
			out = append(out, th)
		}
	}
	slices.SortFunc(out, func(a, b domain.Thread) int {
		return cmp.Or(at(b).Compare(at(a)), strings.Compare(b.ID.String(), a.ID.String()))
	})
	return out[:min(len(out), q.Limit)], s.fail
}

func (s *store) ThreadTags(_ context.Context, threads []ids.ID) (map[ids.ID][]ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	out := map[ids.ID][]ids.ID{}
	for _, th := range threads {
		if tags := s.threadTags[th]; len(tags) > 0 {
			out[th] = slices.Clone(tags)
		}
	}
	return out, s.fail
}

func (s *store) ServedTargets(context.Context, ids.ID) ([]app.ServedTarget, error) {
	return nil, s.fail
}

func (s *store) ReplaceThreadTags(_ context.Context, thread ids.ID, tags []ids.ID, _ ids.ID, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	s.threadTags[thread] = slices.Clone(tags)
	return s.fail
}

// policies is an agent policy source that knows no agent.
type policies struct{}

func (policies) PersonalAgents(context.Context) ([]authz.PersonalAgent, error) { return nil, nil }
func (policies) AgentPolicy(context.Context, ids.ID) (authz.AgentPolicy, bool, error) {
	return authz.AgentPolicy{}, false, nil
}
func (policies) DevEnvironmentReady(context.Context, ids.ID, ids.ID) (bool, error) { return false, nil }

// storePolicies answers the agent policies of the store.
type storePolicies struct{ s *store }

func (storePolicies) PersonalAgents(context.Context) ([]authz.PersonalAgent, error) { return nil, nil }
func (p storePolicies) AgentPolicy(_ context.Context, id ids.ID) (authz.AgentPolicy, bool, error) {
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	pol, ok := p.s.policies[id]
	return pol, ok, nil
}
func (storePolicies) DevEnvironmentReady(context.Context, ids.ID, ids.ID) (bool, error) {
	return false, nil
}

// cursors is a plain codec: the list and binding in hex, then the key as JSON.
type cursors struct{}

func (cursors) Encode(list string, binding []byte, key any) (string, error) {
	k, err := json.Marshal(key)
	return hex.EncodeToString([]byte(list)) + "." + hex.EncodeToString(binding) + "." + string(k), err
}

func (cursors) Decode(list string, binding []byte, s string, key any) error {
	prefix := hex.EncodeToString([]byte(list)) + "." + hex.EncodeToString(binding) + "."
	if !strings.HasPrefix(s, prefix) || json.Unmarshal([]byte(strings.TrimPrefix(s, prefix)), key) != nil {
		return errs.New(errs.CodeCursorInvalid, "")
	}
	return nil
}

func (s *store) MoveThread(_ context.Context, id, channel ids.ID, at time.Time) (domain.Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	th := s.threads[id]
	th.ChannelID, th.MovedAt = channel, at
	th.Version++
	s.threads[id] = th
	return th, s.fail
}

func (s *store) DeleteThreadTags(_ context.Context, thread ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	delete(s.threadTags, thread)
	return s.fail
}
