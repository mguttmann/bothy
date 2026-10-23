// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// searchStore is the fake app.SearchReadStore: the search reads answer canned values and count
// their calls; ThreadsIn reads the threads of the in-memory store in reverse id order, so the
// tests see that HitThreads restores the order of its input.
type searchStore struct {
	s      *store
	calls  map[string]int
	page   searchkit.Page
	titles []searchkit.TitleHit
	idList []ids.ID
	labels chat.Labels
	fail   error
}

func newSearchStore(s *store) *searchStore { return &searchStore{s: s, calls: map[string]int{}} }

func (f *searchStore) SearchThreadTitles(context.Context, searchkit.Query) (searchkit.Page, error) {
	f.calls["SearchThreadTitles"]++
	return f.page, f.fail
}

func (f *searchStore) PrefixThreadTitles(context.Context, searchkit.PrefixQuery) ([]searchkit.TitleHit, error) {
	f.calls["PrefixThreadTitles"]++
	return f.titles, f.fail
}

func (f *searchStore) ThreadsMatching(context.Context, []ids.ID, searchkit.ThreadFilter) ([]ids.ID, error) {
	f.calls["ThreadsMatching"]++
	return f.idList, f.fail
}

func (f *searchStore) ChannelsInSpaces(context.Context, []ids.ID) ([]ids.ID, error) {
	f.calls["ChannelsInSpaces"]++
	return f.idList, f.fail
}

func (f *searchStore) Labels(context.Context, []ids.ID, []ids.ID, []ids.ID) (chat.Labels, error) {
	f.calls["Labels"]++
	return f.labels, f.fail
}

func (f *searchStore) ThreadsIn(_ context.Context, threads, channels []ids.ID) ([]domain.Thread, error) {
	f.calls["ThreadsIn"]++
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var out []domain.Thread
	for _, th := range f.s.threads {
		if slices.Contains(threads, th.ID) && slices.Contains(channels, th.ChannelID) {
			out = append(out, th)
		}
	}
	slices.SortFunc(out, func(a, b domain.Thread) int { return -compareIDs(a.ID, b.ID) })
	return out, f.fail
}

// compareIDs orders ids by their bytes, the order of PostgreSQL.
func compareIDs(a, b ids.ID) int {
	for i := range a {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

// searchService returns the use cases of e with f as the search store.
func searchService(t *testing.T, e *env, f app.SearchReadStore) *app.Service {
	t.Helper()
	svc, err := app.New(testConfig(12), app.Deps{
		Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: rightsPort{e.rights}, Audit: e.rec, Events: e.events,
		Policies: storePolicies{e.store}, Cursors: cursors{}, Clock: clock.NewFake(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)),
		RightsChanges: e.affected, Directory: e.dir, Search: f,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// TestSearchReads_ImplementsPort: the service is the chat.SearchReads of the search coordinator.
func TestSearchReads_ImplementsPort(t *testing.T) {
	t.Parallel()
	var _ chat.SearchReads = newEnv(t).svc
}

// TestSearchReads_EmptyInputRunsNoQuery: no channel, no space, an empty thread id list or a
// limit below one answer empty, non-nil results without asking the store; otherwise the store's
// answer comes back unchanged, and its error too.
func TestSearchReads_EmptyInputRunsNoQuery(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := newSearchStore(e.store)
	svc := searchService(t, e, f)
	ctx := context.Background()
	ch := []ids.ID{ids.New()}

	if p, err := svc.SearchThreadTitles(ctx, searchkit.Query{}); err != nil || p.Hits == nil || len(p.Hits) != 0 || p.Next != nil || p.CandidatesCapped {
		t.Fatalf("no channels: %+v %v", p, err)
	}
	if p, err := svc.SearchThreadTitles(ctx, searchkit.Query{ChannelIDs: ch, ThreadIDs: []ids.ID{}}); err != nil || p.Hits == nil || len(p.Hits) != 0 {
		t.Fatalf("empty thread ids: %+v %v", p, err)
	}
	if got, err := svc.PrefixThreadTitles(ctx, searchkit.PrefixQuery{Limit: 5}); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("prefix without channels: %v %v", got, err)
	}
	if got, err := svc.PrefixThreadTitles(ctx, searchkit.PrefixQuery{ChannelIDs: ch}); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("prefix with limit 0: %v %v", got, err)
	}
	if got, err := svc.ThreadsMatching(ctx, nil, searchkit.ThreadFilter{}); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("matching without channels: %v %v", got, err)
	}
	if got, err := svc.ChannelsInSpaces(ctx, nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no spaces: %v %v", got, err)
	}
	if got, err := svc.Labels(ctx, nil, nil, nil); err != nil || got.Channels == nil || got.ThreadTitles == nil {
		t.Fatalf("no labels: %+v %v", got, err)
	}
	if got, err := svc.HitThreads(ctx, nil, ch); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no threads: %v %v", got, err)
	}
	if got, err := svc.HitThreads(ctx, []ids.ID{ids.New()}, nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("hit threads without channels: %v %v", got, err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("store calls %v, want none", f.calls)
	}

	f.page = searchkit.Page{Hits: []searchkit.Hit{{ID: ids.New()}}, CandidatesCapped: true}
	f.titles = []searchkit.TitleHit{{ThreadID: ids.New()}}
	f.idList = []ids.ID{ids.New()}
	f.labels = chat.Labels{Channels: map[ids.ID]chat.ChannelLabel{ch[0]: {Name: "questions"}}, ThreadTitles: map[ids.ID]string{}}
	if p, err := svc.SearchThreadTitles(ctx, searchkit.Query{ChannelIDs: ch}); err != nil || !reflect.DeepEqual(p, f.page) {
		t.Fatalf("page %+v %v", p, err)
	}
	if got, err := svc.PrefixThreadTitles(ctx, searchkit.PrefixQuery{ChannelIDs: ch, Limit: 1}); err != nil || !slices.Equal(got, f.titles) {
		t.Fatalf("titles %v %v", got, err)
	}
	if got, err := svc.ThreadsMatching(ctx, ch, searchkit.ThreadFilter{State: "open"}); err != nil || !slices.Equal(got, f.idList) {
		t.Fatalf("matching %v %v", got, err)
	}
	if got, err := svc.ChannelsInSpaces(ctx, []ids.ID{ids.New()}); err != nil || !slices.Equal(got, f.idList) {
		t.Fatalf("channels %v %v", got, err)
	}
	if got, err := svc.Labels(ctx, []ids.ID{ids.New()}, ch, nil); err != nil || !reflect.DeepEqual(got, f.labels) {
		t.Fatalf("labels %+v %v", got, err)
	}
	f.fail = searchkit.ErrTimeout
	if _, err := svc.SearchThreadTitles(ctx, searchkit.Query{ChannelIDs: ch}); !errors.Is(err, searchkit.ErrTimeout) {
		t.Fatalf("store error swallowed: %v", err)
	}
	if _, err := svc.HitThreads(ctx, []ids.ID{ids.New()}, ch); !errors.Is(err, searchkit.ErrTimeout) {
		t.Fatalf("store error swallowed: %v", err)
	}
}

// TestSearchReads_WithoutStoreIsInternal: a service without a search store answers internal,
// never an empty page that would hide every hit.
func TestSearchReads_WithoutStoreIsInternal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	ch := []ids.ID{ids.New()}
	if _, err := e.svc.SearchThreadTitles(ctx, searchkit.Query{ChannelIDs: ch}); !internalErr(err) {
		t.Fatalf("SearchThreadTitles: %v", err)
	}
	if _, err := e.svc.PrefixThreadTitles(ctx, searchkit.PrefixQuery{ChannelIDs: ch, Limit: 1}); !internalErr(err) {
		t.Fatalf("PrefixThreadTitles: %v", err)
	}
	if _, err := e.svc.ThreadsMatching(ctx, ch, searchkit.ThreadFilter{}); !internalErr(err) {
		t.Fatalf("ThreadsMatching: %v", err)
	}
	if _, err := e.svc.ChannelsInSpaces(ctx, ch); !internalErr(err) {
		t.Fatalf("ChannelsInSpaces: %v", err)
	}
	if _, err := e.svc.Labels(ctx, ch, ch, nil); !internalErr(err) {
		t.Fatalf("Labels: %v", err)
	}
	if _, err := e.svc.HitThreads(ctx, ch, ch); !internalErr(err) {
		t.Fatalf("HitThreads: %v", err)
	}
}

// TestHitThreads_GetRepresentationInInputOrder (M3-14 design note 9): HitThreads renders exactly
// what getThread renders (live tags, billing mode of the fixed operator), in the order of its
// input; an unknown id and a thread outside the channels are absent.
func TestHitThreads_GetRepresentationInInputOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	forum, other := e.channel(t, sp.ID, "forum", "forum"), e.channel(t, sp.ID, "forum", "other")
	bug, err := e.svc.CreateForumTag(ctx, e.owner, forum.ID, chat.CreateForumTagCmd{Name: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	var threads []ids.ID
	for _, title := range []string{"Invoice export", "Invoice totals", "Invoice run"} {
		th, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, Title: ptr(title), TagIDs: []ids.ID{bug.ID}})
		if err != nil {
			t.Fatal(err)
		}
		threads = append(threads, th.ID)
	}
	agent := ids.New()
	e.store.policies[agent] = authz.AgentPolicy{BillingMode: "personal"}
	e.setThreadOperator(threads[1], agent)
	outside, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: other.ID, Title: ptr("Elsewhere")})
	if err != nil {
		t.Fatal(err)
	}
	svc := searchService(t, e, newSearchStore(e.store))
	input := []ids.ID{threads[1], ids.New(), threads[0], outside.ID, threads[2]}
	got, err := svc.HitThreads(ctx, input, []ids.ID{forum.ID})
	if err != nil {
		t.Fatal(err)
	}
	var want []chat.Thread
	for _, id := range []ids.ID{threads[1], threads[0], threads[2]} {
		v, err := svc.Get(ctx, e.owner, id)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, v)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HitThreads = %+v\nwant the getThread representation %+v", got, want)
	}
	if got[0].BillingMode != "personal" || !slices.Equal(got[1].TagIDs, []ids.ID{bug.ID}) {
		t.Fatalf("representation without billing mode or tags: %+v", got)
	}
}
