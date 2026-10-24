// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// store keeps sources and items in memory.
type store struct {
	mu      sync.Mutex
	sources map[ids.ID]importer.Source
	items   []app.ItemRow
}

func newStore() *store { return &store{sources: map[ids.ID]importer.Source{}} }

func (s *store) InsertSource(_ context.Context, src importer.Source) (importer.Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src.State, src.Stats, src.Cursor = importer.StateDraft, json.RawMessage(`{}`), json.RawMessage(`{}`)
	s.sources[src.ID] = src
	return src, nil
}

func (s *store) Source(_ context.Context, id ids.ID) (importer.Source, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.sources[id]
	return src, ok, nil
}

func (s *store) LockSource(ctx context.Context, id ids.ID) (importer.Source, bool, error) {
	return s.Source(ctx, id)
}

func (s *store) SaveSource(_ context.Context, src importer.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[src.ID] = src
	return nil
}

func (s *store) Sources(_ context.Context, after *app.SourceKey, limit int) ([]importer.Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []importer.Source
	for _, src := range s.sources {
		all = append(all, src)
	}
	slices.SortFunc(all, func(a, b importer.Source) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	var out []importer.Source
	for _, src := range all {
		if after != nil && (src.CreatedAt.After(after.At) || (src.CreatedAt.Equal(after.At) && bytes.Compare(src.ID[:], after.ID[:]) >= 0)) {
			continue
		}
		if len(out) < limit {
			out = append(out, src)
		}
	}
	return out, nil
}

func (s *store) find(source ids.ID, key string) int {
	return slices.IndexFunc(s.items, func(r app.ItemRow) bool { return r.SourceID == source && r.OriginKey == key })
}

func (s *store) InsertPendingItem(_ context.Context, n app.NewItem) (app.ItemRow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.find(n.SourceID, n.OriginKey) >= 0 {
		return app.ItemRow{}, false, nil
	}
	r := app.ItemRow{ID: n.ID, SourceID: n.SourceID, OriginKey: n.OriginKey, ItemKind: n.ItemKind, State: importer.ItemPending,
		ContentSHA256: n.ContentSHA256, CreatedAt: n.At}
	s.items = append(s.items, r)
	return r, true, nil
}

func (s *store) LockItem(ctx context.Context, source ids.ID, key string) (app.ItemRow, bool, error) {
	return s.Item(ctx, source, key)
}

func (s *store) Item(_ context.Context, source ids.ID, key string) (app.ItemRow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.find(source, key); i >= 0 {
		return s.items[i], true, nil
	}
	return app.ItemRow{}, false, nil
}

func (s *store) FinishItem(_ context.Context, id ids.ID, state, kind string, target ids.ID, _ time.Time) (app.ItemRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.items, func(r app.ItemRow) bool { return r.ID == id })
	s.items[i].State, s.items[i].TargetKind, s.items[i].TargetID = state, kind, target
	return s.items[i], nil
}

func (s *store) Items(_ context.Context, source ids.ID, state string, after ids.ID, limit int) ([]app.ItemRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.ItemRow
	for _, r := range s.items {
		if r.SourceID == source && (state == "" || r.State == state) && bytes.Compare(r.ID[:], after[:]) > 0 && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *store) ItemsOfKind(_ context.Context, source ids.ID, kind, state string) ([]app.ItemRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.ItemRow
	for _, r := range s.items {
		if r.SourceID == source && r.ItemKind == kind && r.State == state {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *store) ControlledChannel(_ context.Context, channel ids.ID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.items {
		if r.TargetID == channel && r.ItemKind == "channel" && r.State == importer.ItemPosted {
			if st := s.sources[r.SourceID].State; st != importer.StateCompleted && st != importer.StateFailed {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *store) OriginOfTarget(_ context.Context, kind string, id ids.ID) (app.ItemRow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.items {
		if r.TargetKind == kind && r.TargetID == id && r.State == importer.ItemPosted {
			return r, true, nil
		}
	}
	return app.ItemRow{}, false, nil
}

func (s *store) ItemOfTarget(_ context.Context, source ids.ID, kind string, id ids.ID) (app.ItemRow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.items {
		if r.SourceID == source && r.TargetKind == kind && r.TargetID == id {
			return r, true, nil
		}
	}
	return app.ItemRow{}, false, nil
}

// uow runs fn inside a unit of a sealed registry and rolls the store back on an error, like the
// transaction of the real unit of work.
type uow struct {
	reg   *txn.Registry
	store *store
}

func (u uow) Do(ctx context.Context, fn func(ctx context.Context) error, opts ...txn.Option) error {
	if _, ok := txn.UnitFrom(ctx); ok {
		return fn(ctx)
	}
	unit, set := txn.NewUnit(u.reg, txn.ApplyOptions(opts...))
	set(txn.PhaseWork)
	var items []app.ItemRow
	var sources map[ids.ID]importer.Source
	if u.store != nil {
		u.store.mu.Lock()
		items, sources = slices.Clone(u.store.items), maps.Clone(u.store.sources)
		u.store.mu.Unlock()
	}
	err := fn(txn.WithUnit(ctx, unit))
	if err != nil && u.store != nil {
		u.store.mu.Lock()
		u.store.items, u.store.sources = items, sources
		u.store.mu.Unlock()
	}
	return err
}

// authorizer allows unless deny names the action.
type authorizer struct {
	deny map[authorize.Action]authorize.Decision
}

func (a *authorizer) Authorize(_ context.Context, _ authorize.Principal, act authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	if d, ok := a.deny[act]; ok {
		return d, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (*authorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

func (*authorizer) Observe(context.Context, authorize.LogEvent) {}

// recorder keeps entries and denials.
type recorder struct {
	mu              sync.Mutex
	entries, denied []auditing.Entry
}

func (r *recorder) Record(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}

func (r *recorder) RecordDenial(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.denied = append(r.denied, e)
	return nil
}

func (r *recorder) RecordDetached(ctx context.Context, e auditing.Entry) error {
	return r.RecordDenial(ctx, e)
}

// appender keeps events.
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

// structure fakes chat.ImportStructure: names are unique per space, tags adopted by name.
type structure struct {
	mu       sync.Mutex
	spaces   map[ids.ID]string
	channels map[ids.ID]chat.ImportedChannel
	tags     map[ids.ID]map[string]ids.ID
	readOnly map[ids.ID]bool
}

func newStructure() *structure {
	return &structure{spaces: map[ids.ID]string{}, channels: map[ids.ID]chat.ImportedChannel{}, tags: map[ids.ID]map[string]ids.ID{},
		readOnly: map[ids.ID]bool{}}
}

func (s *structure) Space(_ context.Context, in chat.ImportedSpace) (ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.ExistingID != nil {
		if _, ok := s.spaces[*in.ExistingID]; !ok {
			return ids.ID{}, errs.New(errs.CodeNotFound, "")
		}
		return *in.ExistingID, nil
	}
	for _, n := range s.spaces {
		if strings.EqualFold(n, in.Name) {
			return ids.ID{}, errs.New(errs.CodeConflict, "").WithReason(chat.ReasonNameCollision)
		}
	}
	if in.SortKey == "!" {
		return ids.ID{}, errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: "/sort_key", Code: "invalid"})
	}
	id := ids.New()
	s.spaces[id] = in.Name
	return id, nil
}

func (s *structure) Channel(ctx context.Context, in chat.ImportedChannel) (ids.ID, error) {
	if in.ExistingID != nil {
		s.mu.Lock()
		_, ok := s.channels[*in.ExistingID]
		s.mu.Unlock()
		if !ok {
			return ids.ID{}, errs.New(errs.CodeNotFound, "")
		}
		return *in.ExistingID, s.SetReadOnly(ctx, *in.ExistingID, in.ReadOnly, "")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.channels {
		if c.SpaceID == in.SpaceID && strings.EqualFold(c.Name, in.Name) {
			return ids.ID{}, errs.New(errs.CodeConflict, "").WithReason(chat.ReasonNameCollision)
		}
	}
	id := ids.New()
	s.channels[id], s.readOnly[id] = in, in.ReadOnly
	return id, nil
}

func (s *structure) ForumTag(_ context.Context, channel ids.ID, t chat.ImportedTag) (ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.channels[channel].Kind != "forum" {
		return ids.ID{}, errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Code: "not_a_forum"})
	}
	if s.tags[channel] == nil {
		s.tags[channel] = map[string]ids.ID{}
	}
	if id, ok := s.tags[channel][strings.ToLower(t.Name)]; ok {
		return id, nil
	}
	id := ids.New()
	s.tags[channel][strings.ToLower(t.Name)] = id
	return id, nil
}

func (s *structure) SetReadOnly(_ context.Context, channel ids.ID, ro bool, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readOnly[channel] = ro
	return nil
}

func (s *structure) LiveSpace(_ context.Context, id ids.ID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.spaces[id]
	return ok, nil
}

func (s *structure) LiveChannel(_ context.Context, id ids.ID) (ids.ID, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.channels[id]
	return c.SpaceID, ok, nil
}

// people fakes identity.ImportPeople.
type people struct {
	mu      sync.Mutex
	handles map[string]ids.ID
	active  map[string]bool
	kinds   map[string]actor.Kind
	created []identity.PlaceholderSpec
	// race holds handles a concurrent batch takes between HandleFree and Placeholder.
	race map[string]bool
}

func newPeople() *people {
	return &people{handles: map[string]ids.ID{}, active: map[string]bool{}, kinds: map[string]actor.Kind{}}
}

func (p *people) add(kind actor.Kind, handle string, active bool) ids.ID {
	id := ids.New()
	p.handles[handle], p.active[handle], p.kinds[handle] = id, active, kind
	return id
}

func (p *people) Resolve(_ context.Context, ref identity.ImportRef) (ids.ID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id, ok := p.handles[strings.ToLower(ref.Handle)]
	if !ok || p.kinds[strings.ToLower(ref.Handle)] != ref.Kind {
		return ids.ID{}, errs.New(errs.CodeNotFound, "")
	}
	if !p.active[strings.ToLower(ref.Handle)] {
		return ids.ID{}, errs.New(errs.CodeConflict, "").WithReason(identity.ReasonImportPrincipalInactive)
	}
	return id, nil
}

func (p *people) HandleFree(_ context.Context, h string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, taken := p.handles[h]
	return !taken, nil
}

func (p *people) Placeholder(_ context.Context, spec identity.PlaceholderSpec) (ids.ID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.race[spec.Handle] {
		return ids.ID{}, errs.New(errs.CodeConflict, "").WithFields(errs.FieldError{Pointer: "/handle", Code: "taken"})
	}
	id := ids.New()
	p.handles[spec.Handle], p.kinds[spec.Handle] = id, actor.KindUser
	p.created = append(p.created, spec)
	return id, nil
}

// cursors encodes keys as JSON with the list name.
type cursors struct{}

func (cursors) Encode(list string, binding []byte, key any) (string, error) {
	b, err := json.Marshal(key)
	return list + "|" + string(binding) + "|" + string(b), err
}

func (cursors) Decode(list string, binding []byte, s string, key any) error {
	prefix := list + "|" + string(binding) + "|"
	if !strings.HasPrefix(s, prefix) {
		return errs.New(errs.CodeCursorInvalid, "")
	}
	return json.Unmarshal([]byte(strings.TrimPrefix(s, prefix)), key)
}

// env is the use cases over the fakes.
type env struct {
	svc       *app.Service
	store     *store
	az        *authorizer
	rec       *recorder
	events    *appender
	structure *structure
	people    *people
	owner     actor.Actor
	clk       *clock.Fake
	space     ids.ID
	channel   ids.ID
	alex      ids.ID
	lotse     ids.ID
	published *[]importer.SourceChanged
}

var start = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWith(t, nil)
}

// newEnvWith is newEnv with the deps changed by with before the service is built (nil changes
// nothing).
func newEnvWith(t *testing.T, with func(*app.Deps)) *env {
	t.Helper()
	e := &env{store: newStore(), az: &authorizer{deny: map[authorize.Action]authorize.Decision{}}, rec: &recorder{}, events: &appender{},
		structure: newStructure(), people: newPeople(), owner: actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New()},
		clk: clock.NewFake(start), published: &[]importer.SourceChanged{}}
	reg := txn.NewRegistry()
	if err := txn.Subscribe(reg, func(_ context.Context, ev importer.SourceChanged) error {
		*e.published = append(*e.published, ev)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	e.alex = e.people.add(actor.KindUser, "alex", true)
	e.lotse = e.people.add(actor.KindAgent, "lotse", true)
	e.people.add(actor.KindUser, "sam", false)
	e.people.add(actor.KindUser, "imported-sam", true)
	e.space = ids.New()
	e.structure.spaces[e.space] = "Existing"
	e.channel = ids.New()
	e.structure.channels[e.channel] = chat.ImportedChannel{SpaceID: e.space, Name: "old", Kind: "text"}
	d := app.Deps{Store: e.store, UoW: uow{reg: reg, store: e.store}, Authorizer: e.az, Audit: e.rec, Events: e.events,
		Structure: e.structure, People: e.people, Cursors: cursors{}, Clock: e.clk}
	if with != nil {
		with(&d)
	}
	svc, err := app.New(testConfig(), d)
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

func testConfig() app.Config {
	return app.Config{SpaceNameMaxChars: 12, ChannelNameMaxChars: 12, HandleMaxChars: 20, DisplayNameMaxChars: 20, PageSizeDefault: 2, PageSizeMax: 3,
		ThreadTitleMaxChars: 20}
}

// wantErr fails unless err has code and (when given) reason.
func wantErr(t *testing.T, err error, code errs.Code, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != code || (reason != "" && e.Reason != reason) {
		t.Fatalf("error %v, want %s %s", err, code, reason)
	}
}

// fieldsOf fails unless err is validation_failed and returns its error.
func fieldsOf(t *testing.T, err error) *errs.Error {
	t.Helper()
	wantErr(t, err, errs.CodeValidationFailed, "")
	e, _ := errs.As(err)
	return e
}

// hasField reports a field error with pointer and code.
func hasField(e *errs.Error, pointer, code string) bool {
	return slices.ContainsFunc(e.Fields, func(f errs.FieldError) bool { return f.Pointer == pointer && f.Code == code })
}
