// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// statsAuthorizer decides each action by the outcome of its map (allow when absent) and answers
// Scope with the scope it holds; it counts the source reads through the fakes.
type statsAuthorizer struct {
	outcomes map[authorize.Action]authorize.Outcome
	scope    authorize.Scope
}

func (a *statsAuthorizer) Authorize(_ context.Context, _ authorize.Principal, act authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	if o, ok := a.outcomes[act]; ok {
		return authorize.Decision{Outcome: o, Reason: "fixture"}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (a *statsAuthorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return a.scope, nil
}

func (*statsAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// fakeSources implements every StatsSource: chat maps every channel to space; billing answers the
// currencies and sums it holds; every other source answers nothing. reads counts the calls.
type fakeSources struct {
	space      ids.ID
	currencies []string
	costs      []billing.CostStatsValue
	reads      int
}

func (f *fakeSources) StatsChannels(_ context.Context, cs []ids.ID) ([]chat.StatsChannel, error) {
	f.reads++
	out := make([]chat.StatsChannel, len(cs))
	for i, c := range cs {
		out[i] = chat.StatsChannel{ChannelID: c, SpaceID: f.space}
	}
	return out, nil
}

func (f *fakeSources) StatsThreads(context.Context, []ids.ID, time.Time) ([]chat.StatsThread, error) {
	return nil, nil
}

func (f *fakeSources) ThreadStats(context.Context, chat.StatsQuery) ([]chat.StatsValue, error) {
	return nil, nil
}

func (f *fakeSources) MessageStats(context.Context, messaging.StatsQuery) ([]messaging.StatsValue, error) {
	f.reads++
	return nil, nil
}

func (f *fakeSources) InvocationDenials(context.Context, messaging.StatsQuery) ([]messaging.StatsValue, error) {
	return nil, nil
}

func (f *fakeSources) RunStats(context.Context, runs.StatsQuery) ([]runs.StatsValue, error) {
	return nil, nil
}

func (f *fakeSources) ToolStats(context.Context, runs.StatsQuery) ([]runs.StatsValue, error) {
	return nil, nil
}

func (f *fakeSources) CostStats(context.Context, billing.CostStatsQuery) ([]billing.CostStatsValue, error) {
	f.reads++
	return f.costs, nil
}

func (f *fakeSources) CostCurrencies(context.Context, billing.CostStatsQuery) ([]string, error) {
	return f.currencies, nil
}

func (f *fakeSources) StorageBytes(context.Context, []ids.ID, []ids.ID) ([]files.StatsValue, error) {
	return nil, nil
}

func (f *fakeSources) ScanStats(context.Context, time.Time, time.Time, string, string) ([]files.StatsValue, error) {
	return nil, nil
}

func (f *fakeSources) WebhookStats(context.Context, time.Time, time.Time, string, string) ([]apps.StatsValue, error) {
	return nil, nil
}

// pushSources wraps fakeSources with the push source, whose method name differs per module.
type pushSources struct{ *fakeSources }

func (pushSources) PushStats(context.Context, []time.Time, []time.Time) ([]notify.StatsValue, error) {
	return nil, nil
}

// statsConfig is the configuration of the tests: the defaults of the keys.
var statsConfig = app.Config{CacheTTL: 5 * time.Minute, CacheMaxEntries: 10, MaxRange: 400 * 24 * time.Hour, TimeZone: "UTC",
	MaxBuckets: 1000, MinGroupSize: 5}

// newStatistics builds the use cases over the fakes and a fake clock.
func newStatistics(t *testing.T, az *statsAuthorizer, src *fakeSources, clk clock.Clock) *app.Statistics {
	t.Helper()
	st, err := app.NewStatistics(statsConfig, app.Deps{Authorizer: az, Chat: src, Messaging: src, Runs: src, Billing: src, Files: src,
		Apps: src, Notify: pushSources{src}, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// statsCtx is a request of a person.
func statsCtx() context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
}

var (
	day0 = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	day1 = day0.AddDate(0, 0, 1)
)

// TestCostStats_WithoutCostReadAggregate (brief M6-05 "Tests", TestStats_Costs): the path "no
// cost.read.aggregate" cannot be reached by a person with stats.read today (05 gives both to
// owners and managers), so a stubbed decision proves it: forbidden for cost.read.aggregate is 403
// even when stats.read is allowed, and no source is read.
func TestCostStats_WithoutCostReadAggregate(t *testing.T) {
	t.Parallel()
	az := &statsAuthorizer{outcomes: map[authorize.Action]authorize.Outcome{authorize.ActionCostReadAggregate: authorize.OutcomeForbidden}}
	src := &fakeSources{space: ids.New()}
	st := newStatistics(t, az, src, nil)
	_, err := st.Costs(statsCtx(), stats.CostQuery{Query: stats.Query{From: day0, To: day1, SpaceID: src.space}})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeForbidden {
		t.Fatalf("Costs without cost.read.aggregate: %v", err)
	}
	if src.reads != 0 {
		t.Fatalf("%d source reads after a denial", src.reads)
	}
	if _, err := st.Usage(statsCtx(), stats.Query{From: day0, To: day1, SpaceID: src.space}); err != nil {
		t.Fatalf("Usage with stats.read: %v", err)
	}
}

// TestCostStats_Currency: several currencies without the parameter are 400 validation_failed with
// errors[].code currency_required naming them; one currency answers in it; none answers USD; values
// of another currency or a space outside the scope are dropped (defence in depth).
func TestCostStats_Currency(t *testing.T) {
	t.Parallel()
	space := ids.New()
	az := &statsAuthorizer{scope: authorize.Scope{ChannelIDs: []ids.ID{ids.New()}, PermissionVersion: 1}}
	src := &fakeSources{space: space, currencies: []string{"EUR", "USD"}}
	st := newStatistics(t, az, src, nil)
	q := stats.CostQuery{Query: stats.Query{From: day0, To: day1, SpaceID: space}, GroupBy: billing.GroupBySpace}
	_, err := st.Costs(statsCtx(), q)
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Code != "currency_required" ||
		e.Fields[0].Detail != "The range has costs in several currencies: EUR, USD." {
		t.Fatalf("several currencies: %v", err)
	}

	src.currencies = []string{"EUR"}
	src.costs = []billing.CostStatsValue{
		{Bucket: day0, Group: space.String(), BillingMode: billing.ModeOrganization, Currency: "EUR", AmountMicros: 5, IsEstimate: true},
		{Bucket: day0, Group: space.String(), BillingMode: billing.ModePersonal, Currency: "EUR", AmountMicros: 7},
		{Bucket: day0, Group: ids.New().String(), BillingMode: billing.ModeOrganization, Currency: "EUR", AmountMicros: 1000},
		{Bucket: day0, Group: space.String(), BillingMode: billing.ModeOrganization, Currency: "USD", AmountMicros: 1000},
	}
	got, err := st.Costs(statsCtx(), q)
	if err != nil {
		t.Fatal(err)
	}
	if got.Currency != "EUR" || len(got.Buckets) != 2 || got.Buckets[0].BillingMode != billing.ModeOrganization ||
		got.Buckets[0].AmountMicros != 5 || !got.Buckets[0].IsEstimate || got.Buckets[1].BillingMode != billing.ModePersonal ||
		got.Buckets[1].AmountMicros != 7 || got.Buckets[1].IsEstimate {
		t.Fatalf("one currency: %+v", got)
	}

	src.currencies, src.costs = nil, nil
	q.Granularity = stats.GranularityHour
	if got, err := st.Costs(statsCtx(), q); err != nil || got.Currency != "USD" || len(got.Buckets) != 0 {
		t.Fatalf("no costs: %+v %v", got, err)
	}
}

// TestStatistics_CacheKeysOnScope (design note 6): a repeated query within stats.cache_ttl is
// served from the cache; a new permission version or another channel set misses; after the TTL
// by the clock it is read again. The cache never outlives stats.cache_max_entries.
func TestStatistics_CacheKeysOnScope(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(day1)
	chans := []ids.ID{ids.New()}
	az := &statsAuthorizer{scope: authorize.Scope{ChannelIDs: chans, PermissionVersion: 1}}
	src := &fakeSources{space: ids.New()}
	st := newStatistics(t, az, src, clk)
	q := stats.Query{From: day0, To: day1, SpaceID: src.space}
	read := func() int {
		t.Helper()
		before := src.reads
		if _, err := st.Usage(statsCtx(), q); err != nil {
			t.Fatal(err)
		}
		// One read is the channel set of every call; a computation adds the message read.
		return src.reads - before - 1
	}
	if n := read(); n != 1 {
		t.Fatalf("first call read %d sources", n)
	}
	if n := read(); n != 0 {
		t.Fatalf("cached call read %d sources", n)
	}
	az.scope.PermissionVersion = 2
	if n := read(); n != 1 {
		t.Fatalf("new permission version read %d sources", n)
	}
	az.scope.ChannelIDs = []ids.ID{chans[0], ids.New()}
	if n := read(); n != 1 {
		t.Fatalf("new channel set read %d sources", n)
	}
	clk.Advance(statsConfig.CacheTTL - time.Second)
	if n := read(); n != 0 {
		t.Fatalf("within the TTL read %d sources", n)
	}
	clk.Advance(time.Second)
	if n := read(); n != 1 {
		t.Fatalf("after the TTL read %d sources", n)
	}
}

// TestCache_BoundAndTTL: the least recently used entry goes when stats.cache_max_entries is
// reached; an entry expires at stats.cache_ttl by the clock.
func TestCache_BoundAndTTL(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(day0)
	c := app.NewCache(time.Minute, 2, clk)
	c.Put("a", 1)
	c.Put("b", 2)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a missing")
	}
	c.Put("c", 3)
	if _, ok := c.Get("b"); ok || c.Len() != 2 {
		t.Fatalf("b not evicted, len %d", c.Len())
	}
	clk.Advance(time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a outlived the TTL")
	}
}
