// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/search/app"
)

// fakeUoW runs fn directly and records the options of each Do.
type fakeUoW struct {
	mu   sync.Mutex
	opts []txn.Options
}

func (u *fakeUoW) Do(ctx context.Context, fn func(context.Context) error, opts ...txn.Option) error {
	u.mu.Lock()
	u.opts = append(u.opts, txn.ApplyOptions(opts...))
	u.mu.Unlock()
	return fn(ctx)
}

// fakeAuthorizer answers Scope with the scopes in order (the last one repeats) and Authorize with
// allow unless deny lists the resource id.
type fakeAuthorizer struct {
	mu        sync.Mutex
	scopes    []authorize.Scope
	scopeErr  error
	deny      map[ids.ID]bool
	denyQuery bool
	calls     []string
	memos     []*authorize.RequestMemo
}

func (a *fakeAuthorizer) Authorize(ctx context.Context, _ authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, "authorize "+string(act))
	if act == authorize.ActionSearchQuery {
		if a.denyQuery {
			return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "scope_missing"}, nil
		}
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	if !authorize.IsFilter(ctx) {
		return authorize.Decision{}, errors.New("a page check outside filter mode")
	}
	if a.deny[r.ID] {
		return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_read"}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (a *fakeAuthorizer) Scope(ctx context.Context, _ authorize.Principal, act authorize.Action) (authorize.Scope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, "scope "+string(act))
	m, _ := authorize.RequestMemoFrom(ctx)
	a.memos = append(a.memos, m)
	if a.scopeErr != nil {
		return authorize.Scope{}, a.scopeErr
	}
	s := a.scopes[0]
	if len(a.scopes) > 1 {
		a.scopes = a.scopes[1:]
	}
	return s, nil
}

func (a *fakeAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// fakeIndex returns page for every read operation and records the queries.
type fakeIndex struct {
	mu        sync.Mutex
	page      searchkit.Page
	err       error
	noTerm    bool
	queries   []searchkit.Query
	prefix    []searchkit.PrefixQuery
	titleHits []searchkit.TitleHit
}

func (f *fakeIndex) BuildQuery(_ context.Context, q string) (string, bool, error) {
	if f.noTerm {
		return "", false, nil
	}
	return "'" + q + "'", true, nil
}

func (f *fakeIndex) BuildPrefix(_ context.Context, q string) (string, bool, error) {
	if f.noTerm {
		return "", false, nil
	}
	return "'" + q + "':*", true, nil
}

func (f *fakeIndex) run(q searchkit.Query) (searchkit.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	return f.page, f.err
}

func (f *fakeIndex) Messages(_ context.Context, q searchkit.Query) (searchkit.Page, error) {
	return f.run(q)
}
func (f *fakeIndex) Threads(_ context.Context, q searchkit.Query) (searchkit.Page, error) {
	return f.run(q)
}
func (f *fakeIndex) Files(_ context.Context, q searchkit.Query) (searchkit.Page, error) {
	return f.run(q)
}

func (f *fakeIndex) TitlePrefix(_ context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prefix = append(f.prefix, q)
	return f.titleHits, f.err
}

// fakeStructure maps spaces to channels and names every channel.
type fakeStructure struct {
	spaces map[ids.ID][]ids.ID
	asked  [][]ids.ID
}

func (s *fakeStructure) ChannelsInSpaces(_ context.Context, spaceIDs []ids.ID) ([]ids.ID, error) {
	s.asked = append(s.asked, slices.Clone(spaceIDs))
	var out []ids.ID
	for _, sp := range spaceIDs {
		out = append(out, s.spaces[sp]...)
	}
	return out, nil
}

func (s *fakeStructure) Labels(_ context.Context, _, channelIDs, threadIDs []ids.ID) (chat.Labels, error) {
	l := chat.Labels{Channels: map[ids.ID]chat.ChannelLabel{}, ThreadTitles: map[ids.ID]string{}}
	for _, c := range channelIDs {
		l.Channels[c] = chat.ChannelLabel{Name: "Allgemein", SpaceName: "Support"}
	}
	for _, t := range threadIDs {
		l.ThreadTitles[t] = "Invoice run"
	}
	return l, nil
}

// fakeCursors encodes the key as JSON with the binding in front, which is enough to check that
// the coordinator binds and decodes.
type fakeCursors struct{}

type fakeCursor struct {
	List    string          `json:"l"`
	Binding []byte          `json:"b"`
	Key     json.RawMessage `json:"k"`
}

func (fakeCursors) Encode(list string, binding []byte, key any) (string, error) {
	k, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(fakeCursor{List: list, Binding: binding, Key: k})
	return string(c), err
}

func (fakeCursors) Decode(list string, binding []byte, s string, key any) error {
	var c fakeCursor
	if err := json.Unmarshal([]byte(s), &c); err != nil || c.List != list || !bytes.Equal(c.Binding, binding) {
		return errs.New(errs.CodeCursorInvalid, "")
	}
	if err := json.Unmarshal(c.Key, key); err != nil {
		return errs.New(errs.CodeCursorInvalid, "")
	}
	return nil
}

// fakeMetrics records the outcomes of bothy.search.duration.
type fakeMetrics struct {
	mu       sync.Mutex
	outcomes []string
}

func (m *fakeMetrics) SearchCompleted(_ context.Context, outcome string, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outcomes = append(m.outcomes, outcome)
}

// fakeErrors counts bothy.authz.errors per reason.
type fakeErrors struct {
	mu     sync.Mutex
	counts map[string]int
}

func (e *fakeErrors) CountError(_ context.Context, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.counts == nil {
		e.counts = map[string]int{}
	}
	e.counts[reason]++
}

// logSink collects log records.
type logSink struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *logSink) Enabled(context.Context, slog.Level) bool { return true }
func (l *logSink) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r.Clone())
	return nil
}
func (l *logSink) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logSink) WithGroup(string) slog.Handler      { return l }

// env is one coordinator with its fakes.
type env struct {
	coord   *app.Coordinator
	az      *fakeAuthorizer
	index   *fakeIndex
	str     *fakeStructure
	uow     *fakeUoW
	metrics *fakeMetrics
	errors  *fakeErrors
	log     *logSink
	clock   *clock.Fake
	alex    actor.Actor
}

// testConfig is a configuration with the defaults of 14 section 15 and small limits.
func testConfig() app.Config {
	return app.Config{
		QueryMaxChars: 256, MaxCandidates: 2000, RankNormalization: 1, RecencyScale: 30 * 24 * time.Hour,
		PageSizeDefault: 20, PageSizeMax: 50, HeadlineInputMaxChars: 20000, HeadlineMaxFragments: 2,
		HeadlineMaxWords: 30, HeadlineMinWords: 10, PrefixMinChars: 2, StatementTimeout: 3 * time.Second,
	}
}

func newEnv(t *testing.T, scopes ...authorize.Scope) *env {
	t.Helper()
	e := &env{
		az: &fakeAuthorizer{scopes: scopes, deny: map[ids.ID]bool{}}, index: &fakeIndex{}, str: &fakeStructure{spaces: map[ids.ID][]ids.ID{}},
		uow: &fakeUoW{}, metrics: &fakeMetrics{}, errors: &fakeErrors{}, log: &logSink{},
		clock: clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)), alex: actor.Actor{ID: ids.New(), Kind: actor.KindUser},
	}
	c, err := app.NewCoordinator(testConfig(), app.Deps{
		Authorizer: e.az, UoW: e.uow, Index: e.index, Structure: e.str, Cursors: fakeCursors{}, Clock: e.clock,
		Metrics: e.metrics, Errors: e.errors, Log: slog.New(e.log),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.coord = c
	return e
}
