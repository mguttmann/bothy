// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// The metric names the use cases derive from the source aggregates (the operation descriptions of
// docs/api/openapi.yaml are the list).
const (
	metricMessages           = "messages"
	metricRunFailureRate     = "run_failure_rate"
	metricWebhookFailureRate = "webhook_failure_rate"
	metricPushFailurePrefix  = "push_failure_rate_"
	// pushRegistrationsPrefix and pushDisabledPrefix carry the push counts per provider through
	// the merge; they never leave.
	pushRegistrationsPrefix = "push_registrations_"
	pushDisabledPrefix      = "push_disabled_"
	// reasonBillingModeMismatch is the state_reason of runs that adds to invocation_denials.
	reasonBillingModeMismatch = "billing_mode_mismatch"
	// defaultCurrency is the currency of a cost series without any entry: the default of
	// cost_entries.currency (03 section 4.9).
	defaultCurrency = "USD"
)

// Config are the configuration keys of the statistics queries (17 section 11, 06, 20 section 9).
type Config struct {
	// CacheTTL is stats.cache_ttl: how long a computed result stays valid.
	CacheTTL time.Duration `config:"stats.cache_ttl"`
	// CacheMaxEntries is stats.cache_max_entries: the bound of the result cache per process.
	CacheMaxEntries int `config:"stats.cache_max_entries"`
	// MaxRange is stats.max_range: the largest queryable range.
	MaxRange time.Duration `config:"stats.max_range"`
	// TimeZone is stats.time_zone: the IANA zone of the day, week and month boundaries.
	TimeZone string `config:"stats.time_zone"`
	// MaxBuckets is api.stats_max_buckets: the largest number of buckets per query.
	MaxBuckets int `config:"api.stats_max_buckets"`
	// MinGroupSize is privacy.stats_min_group_size: the smallest number of people a counter about
	// people may show.
	MinGroupSize int `config:"privacy.stats_min_group_size"`
	// PerPersonStatsEnabled is privacy.per_person_stats_enabled. It changes nothing here: no
	// operation groups by person, whatever its value (25 section 1 rule 2).
	PerPersonStatsEnabled bool `config:"privacy.per_person_stats_enabled"`
}

// Deps are the collaborators of Statistics: the Authorizer and the statistics sources of the
// modules that own the tables (admin-statistics.md section 2).
type Deps struct {
	Authorizer authorize.Authorizer
	Chat       chat.StatsSource
	Messaging  messaging.StatsSource
	Runs       runs.StatsSource
	Billing    billing.StatsSource
	Files      files.StatsSource
	Apps       apps.StatsSource
	Notify     notify.StatsSource
	// Clock expires the cache; nil means clock.System().
	Clock clock.Clock
}

// Statistics implements stats.Statistics (M6-05): every use case authorizes, builds the channel
// set of the caller, calls the source ports, merges and caches. It is safe for concurrent use.
type Statistics struct {
	cfg   Config
	d     Deps
	loc   *time.Location
	cache *Cache
}

// Compile-time check: Statistics is the statistics port.
var _ stats.Statistics = (*Statistics)(nil)

// NewStatistics returns the use cases over cfg and d. Errors: a missing collaborator, a time zone
// that is no IANA name, a non-positive range, bucket or group bound.
func NewStatistics(cfg Config, d Deps) (*Statistics, error) {
	if d.Authorizer == nil || d.Chat == nil || d.Messaging == nil || d.Runs == nil || d.Billing == nil || d.Files == nil ||
		d.Apps == nil || d.Notify == nil {
		return nil, errors.New("stats: the statistics need the Authorizer and every source")
	}
	if cfg.TimeZone == "" || cfg.TimeZone == "Local" {
		return nil, errors.New("stats: stats.time_zone must name an IANA time zone")
	}
	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, errors.New("stats: stats.time_zone is no known IANA time zone")
	}
	if cfg.MaxRange <= 0 || cfg.MaxBuckets <= 0 || cfg.MinGroupSize <= 0 {
		return nil, errors.New("stats: stats.max_range, api.stats_max_buckets and privacy.stats_min_group_size must be positive")
	}
	return &Statistics{cfg: cfg, d: d, loc: loc, cache: NewCache(cfg.CacheTTL, cfg.CacheMaxEntries, d.Clock)}, nil
}

// target is the authorized scope of one call: the aligned range, the visible channels with their
// space (DM channels have none and are not counted), and whether the caller sees the values
// without a channel (stats.read on the instance by an owner, design note 1).
type target struct {
	q        stats.Query
	rng      domain.Range
	channels []ids.ID
	spaceOf  map[ids.ID]ids.ID
	instance bool
	scope    string
}

// prepare authorizes action on the instance (no space) or the space, aligns the range and builds
// the channel set: Scope(message.read) of the caller, kept to the channels with a space and, with
// a space, to the channels of that space (design note 1).
func (s *Statistics) prepare(ctx context.Context, action authorize.Action, q stats.Query) (target, error) {
	a := actor.Anonymous()
	if c, ok := actor.FromContext(ctx); ok {
		a = c
	}
	res := authorize.Resource{Kind: authorize.ResourceInstance}
	if !q.SpaceID.IsZero() {
		res = authorize.Resource{Kind: authorize.ResourceSpace, ID: q.SpaceID}
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, action, res)
	if err != nil {
		return target{}, err
	}
	if err := d.Err(); err != nil {
		return target{}, err
	}
	g, err := domain.ParseGranularity(q.Granularity)
	if err != nil {
		return target{}, err
	}
	rng, err := domain.Align(q.From, q.To, g, s.loc, domain.Limits{MaxRange: s.cfg.MaxRange, MaxBuckets: s.cfg.MaxBuckets})
	if err != nil {
		return target{}, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead)
	if err != nil {
		return target{}, err
	}
	visible, err := s.d.Chat.StatsChannels(ctx, sc.ChannelIDs)
	if err != nil {
		return target{}, err
	}
	allowed := make(map[ids.ID]bool, len(sc.ChannelIDs))
	for _, c := range sc.ChannelIDs {
		allowed[c] = true
	}
	t := target{q: q, rng: rng, spaceOf: map[ids.ID]ids.ID{}, instance: q.SpaceID.IsZero() && sc.InstanceWide}
	q.Granularity = string(g)
	t.q = q
	for _, c := range visible {
		// The structure source never widens the set; the check keeps that true by construction.
		if !allowed[c.ChannelID] || (!q.SpaceID.IsZero() && c.SpaceID != q.SpaceID) {
			continue
		}
		t.channels = append(t.channels, c.ChannelID)
		t.spaceOf[c.ChannelID] = c.SpaceID
	}
	slices.SortFunc(t.channels, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	t.scope = scopeKey(sc.PermissionVersion, t.instance, t.channels)
	return t, nil
}

// key is the cache key of op for t with the extra normalized parameters.
func (t target) key(op string, extra ...string) string {
	params := append([]string{strconv.FormatInt(t.rng.From.UnixNano(), 10), strconv.FormatInt(t.rng.To.UnixNano(), 10),
		string(t.rng.Granularity), t.rng.Zone(), t.q.SpaceID.String(), t.q.AgentID.String()}, extra...)
	return cacheKey(op, params, t.scope)
}

// groups returns the channels and, parallel to them, their spaces.
func (t target) groups() ([]ids.ID, []ids.ID) {
	spaces := make([]ids.ID, len(t.channels))
	for i, c := range t.channels {
		spaces[i] = t.spaceOf[c]
	}
	return t.channels, spaces
}

// spaceKeys returns the identity group map over the visible spaces.
func (t target) spaceKeys() map[string]string {
	out := map[string]string{}
	for _, sp := range t.spaceOf {
		out[sp.String()] = sp.String()
	}
	return out
}

// threads resolves the visible channels to their threads, with the space of each thread, because
// runs has no channel_id (D-M2-05-9).
func (s *Statistics) threads(ctx context.Context, t target) ([]ids.ID, []ids.ID, error) {
	ts, err := s.d.Chat.StatsThreads(ctx, t.channels, t.rng.To)
	if err != nil {
		return nil, nil, err
	}
	threads, spaces := make([]ids.ID, 0, len(ts)), make([]ids.ID, 0, len(ts))
	for _, th := range ts {
		sp, ok := t.spaceOf[th.ChannelID]
		if !ok {
			continue
		}
		threads, spaces = append(threads, th.ThreadID), append(spaces, sp)
	}
	return threads, spaces, nil
}

// cached returns the cached series of key or computes, stores and returns it.
func (s *Statistics) cached(key string, compute func() (stats.Series, error)) (stats.Series, error) {
	if v, ok := s.cache.Get(key); ok {
		return v.(stats.Series), nil
	}
	out, err := compute()
	if err != nil {
		return stats.Series{}, err
	}
	s.cache.Put(key, out)
	return out, nil
}

// series converts the merged buckets.
func (s *Statistics) series(t target, m *domain.Merger) stats.Series {
	bs := m.Buckets(s.cfg.MinGroupSize)
	out := stats.Series{Granularity: string(t.rng.Granularity), Buckets: make([]stats.Bucket, len(bs))}
	for i, b := range bs {
		out.Buckets[i] = stats.Bucket{Start: b.Start, GroupKey: b.Group, Metrics: b.Metrics, BelowMinGroup: b.BelowMinGroup}
	}
	return out
}

// Usage implements stats.Statistics (getUsageStats, design note 2): per space the messages by
// origin and their sum, the threads, the ended runs, the active people (week and month only) and
// the current storage in the last bucket.
func (s *Statistics) Usage(ctx context.Context, q stats.Query) (stats.Series, error) {
	q.AgentID = ids.ID{}
	t, err := s.prepare(ctx, authorize.ActionStatsRead, q)
	if err != nil {
		return stats.Series{}, err
	}
	return s.cached(t.key("usage"), func() (stats.Series, error) {
		m := domain.NewMerger(t.rng, t.spaceKeys())
		for _, read := range []func(context.Context, target, *domain.Merger) error{s.usageMessages, s.usageThreads, s.usageRuns, s.usageStorage} {
			if err := read(ctx, t, m); err != nil {
				return stats.Series{}, err
			}
		}
		m.Each(func(_ time.Time, _ string, ms map[string]float64) { sumMessages(ms) })
		return s.series(t, m), nil
	})
}

// usageMessages adds the message counts by origin and, for weeks and months, the active people.
func (s *Statistics) usageMessages(ctx context.Context, t target, m *domain.Merger) error {
	chans, spaces := t.groups()
	people := t.rng.Granularity == domain.Week || t.rng.Granularity == domain.Month
	vs, err := s.d.Messaging.MessageStats(ctx, messaging.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
		TimeZone: t.rng.Zone(), ChannelIDs: chans, GroupIDs: spaces, CountPeople: people})
	for _, v := range vs {
		m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
	}
	return err
}

// usageThreads adds the threads created and done.
func (s *Statistics) usageThreads(ctx context.Context, t target, m *domain.Merger) error {
	chans, spaces := t.groups()
	vs, err := s.d.Chat.ThreadStats(ctx, chat.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
		TimeZone: t.rng.Zone(), ChannelIDs: chans, GroupIDs: spaces})
	for _, v := range vs {
		m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
	}
	return err
}

// usageRuns adds the ended runs per space.
func (s *Statistics) usageRuns(ctx context.Context, t target, m *domain.Merger) error {
	threadIDs, threadSpaces, err := s.threads(ctx, t)
	if err != nil {
		return err
	}
	vs, err := s.d.Runs.RunStats(ctx, runs.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
		TimeZone: t.rng.Zone(), ThreadIDs: threadIDs, GroupIDs: threadSpaces})
	for _, v := range vs {
		if v.Metric == runs.MetricRuns {
			m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
		}
	}
	return err
}

// usageStorage adds the current storage per space to the last bucket.
func (s *Statistics) usageStorage(ctx context.Context, t target, m *domain.Merger) error {
	chans, spaces := t.groups()
	vs, err := s.d.Files.StorageBytes(ctx, chans, spaces)
	for _, v := range vs {
		m.Add(domain.Value{Bucket: t.rng.Last(), Key: v.Group, Metric: v.Metric, Value: v.Value})
	}
	return err
}

// sumMessages sets the metric messages of a bucket to the sum of its origin counts, when it has
// one.
func sumMessages(ms map[string]float64) {
	sum, seen := 0.0, false
	for _, name := range []string{messaging.MetricMessagesHuman, messaging.MetricMessagesAgent, messaging.MetricMessagesApp, messaging.MetricMessagesImport} {
		if v, ok := ms[name]; ok {
			sum, seen = sum+v, true
		}
	}
	if seen {
		ms[metricMessages] = sum
	}
}

// Runs implements stats.Statistics (getRunStats): per agent the ended runs by end state, the
// failure reasons that occur and the duration percentiles.
func (s *Statistics) Runs(ctx context.Context, q stats.Query) (stats.Series, error) {
	t, err := s.prepare(ctx, authorize.ActionStatsRead, q)
	if err != nil {
		return stats.Series{}, err
	}
	return s.cached(t.key("runs"), func() (stats.Series, error) {
		threadIDs, _, err := s.threads(ctx, t)
		if err != nil {
			return stats.Series{}, err
		}
		rs, err := s.d.Runs.RunStats(ctx, runs.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
			TimeZone: t.rng.Zone(), ThreadIDs: threadIDs, AgentID: q.AgentID, ByAgent: true})
		if err != nil {
			return stats.Series{}, err
		}
		m := domain.NewMerger(t.rng, agentKeys(q.AgentID))
		for _, v := range rs {
			m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
		}
		return s.series(t, m), nil
	})
}

// ToolCalls implements stats.Statistics (getToolCallStats): per tool name the calls, the failed
// ones and the median duration of the finished ones.
func (s *Statistics) ToolCalls(ctx context.Context, q stats.Query) (stats.Series, error) {
	t, err := s.prepare(ctx, authorize.ActionStatsRead, q)
	if err != nil {
		return stats.Series{}, err
	}
	return s.cached(t.key("tool_calls"), func() (stats.Series, error) {
		threadIDs, _, err := s.threads(ctx, t)
		if err != nil {
			return stats.Series{}, err
		}
		vs, err := s.d.Runs.ToolStats(ctx, runs.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
			TimeZone: t.rng.Zone(), ThreadIDs: threadIDs, AgentID: q.AgentID})
		if err != nil {
			return stats.Series{}, err
		}
		m := domain.NewMerger(t.rng, nil)
		for _, v := range vs {
			m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
		}
		return s.series(t, m), nil
	})
}

// Errors implements stats.Statistics (getErrorStats): the run failure rate and the invocation
// denials of the visible channels, grouped by nothing on the instance and by the space with one;
// for an owner on the instance also the webhook and push failure rates and the scan results, which
// have no channel (design note 1).
func (s *Statistics) Errors(ctx context.Context, q stats.Query) (stats.Series, error) {
	q.AgentID = ids.ID{}
	t, err := s.prepare(ctx, authorize.ActionStatsRead, q)
	if err != nil {
		return stats.Series{}, err
	}
	return s.cached(t.key("errors"), func() (stats.Series, error) {
		group := ""
		if !q.SpaceID.IsZero() {
			group = q.SpaceID.String()
		}
		keys := map[string]string{"": group}
		for sp := range t.spaceKeys() {
			keys[sp] = group
		}
		m := domain.NewMerger(t.rng, keys)
		reads := []func(context.Context, target, *domain.Merger) error{s.errorRuns, s.errorDenials}
		if t.instance {
			reads = append(reads, s.instanceErrors)
		}
		for _, read := range reads {
			if err := read(ctx, t, m); err != nil {
				return stats.Series{}, err
			}
		}
		m.Each(func(_ time.Time, _ string, ms map[string]float64) { deriveRates(ms) })
		return s.series(t, m), nil
	})
}

// errorRuns adds the ended and failed runs and the runs failed with billing_mode_mismatch, which
// count as invocation denials.
func (s *Statistics) errorRuns(ctx context.Context, t target, m *domain.Merger) error {
	threadIDs, threadSpaces, err := s.threads(ctx, t)
	if err != nil {
		return err
	}
	vs, err := s.d.Runs.RunStats(ctx, runs.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
		TimeZone: t.rng.Zone(), ThreadIDs: threadIDs, GroupIDs: threadSpaces})
	for _, v := range vs {
		switch v.Metric {
		case runs.MetricRuns, runs.MetricFailed:
			m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
		case runs.MetricFailedReasonPrefix + reasonBillingModeMismatch:
			m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: messaging.MetricInvocationDenials, Value: v.Value})
		}
	}
	return err
}

// errorDenials adds the messages whose trigger was refused with billing_mode_personal_not_holder.
func (s *Statistics) errorDenials(ctx context.Context, t target, m *domain.Merger) error {
	chans, spaces := t.groups()
	vs, err := s.d.Messaging.InvocationDenials(ctx, messaging.StatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity),
		TimeZone: t.rng.Zone(), ChannelIDs: chans, GroupIDs: spaces})
	for _, v := range vs {
		m.Add(domain.Value{Bucket: v.Bucket, Key: v.Group, Metric: v.Metric, Value: v.Value})
	}
	return err
}

// instanceErrors adds the values without a channel of getErrorStats to m: webhook deliveries, push
// registrations per provider and scan results. Only an owner on the instance reaches it.
func (s *Statistics) instanceErrors(ctx context.Context, t target, m *domain.Merger) error {
	g, tz := string(t.rng.Granularity), t.rng.Zone()
	hooks, err := s.d.Apps.WebhookStats(ctx, t.rng.From, t.rng.To, g, tz)
	if err != nil {
		return err
	}
	for _, v := range hooks {
		m.Add(domain.Value{Bucket: v.Bucket, Metric: v.Metric, Value: v.Value})
	}
	push, err := s.d.Notify.PushStats(ctx, t.rng.Starts(), t.rng.Ends())
	if err != nil {
		return err
	}
	for _, v := range push {
		prefix := pushRegistrationsPrefix
		if v.Metric == notify.MetricPushDisabled {
			prefix = pushDisabledPrefix
		}
		m.Add(domain.Value{Bucket: v.Bucket, Metric: prefix + v.Group, Value: v.Value})
	}
	scans, err := s.d.Files.ScanStats(ctx, t.rng.From, t.rng.To, g, tz)
	if err != nil {
		return err
	}
	for _, v := range scans {
		m.Add(domain.Value{Bucket: v.Bucket, Metric: v.Metric, Value: v.Value})
	}
	return nil
}

// deriveRates turns the counts of one error bucket into its rates and removes the counts: a rate
// without a denominator is left out.
func deriveRates(ms map[string]float64) {
	if n := ms[runs.MetricRuns]; n > 0 {
		ms[metricRunFailureRate] = ms[runs.MetricFailed] / n
	}
	delete(ms, runs.MetricRuns)
	delete(ms, runs.MetricFailed)
	if n := ms[apps.MetricWebhookFinished]; n > 0 {
		ms[metricWebhookFailureRate] = ms[apps.MetricWebhookFailed] / n
	}
	delete(ms, apps.MetricWebhookFinished)
	delete(ms, apps.MetricWebhookFailed)
	for name, n := range ms {
		provider, ok := strings.CutPrefix(name, pushRegistrationsPrefix)
		if !ok {
			continue
		}
		if n > 0 {
			ms[metricPushFailurePrefix+provider] = ms[pushDisabledPrefix+provider] / n
		}
		delete(ms, name)
		delete(ms, pushDisabledPrefix+provider)
	}
}

// agentKeys restricts the merge to one agent; nil allows every agent (the thread set bounds them).
func agentKeys(agent ids.ID) map[string]string {
	if agent.IsZero() {
		return nil
	}
	return map[string]string{agent.String(): agent.String()}
}

// costCell is one cost bucket of one group and billing mode.
type costCell struct {
	start       int64
	group, mode string
}

// Costs implements stats.Statistics (getCostStats): the valid sums per bucket, group and billing
// mode in one currency (design note 2).
func (s *Statistics) Costs(ctx context.Context, q stats.CostQuery) (stats.CostSeries, error) {
	t, err := s.prepare(ctx, authorize.ActionCostReadAggregate, q.Query)
	if err != nil {
		return stats.CostSeries{}, err
	}
	groupBy := q.GroupBy
	if groupBy == "" {
		groupBy = billing.GroupByNone
	}
	if groupBy != billing.GroupBySpace && groupBy != billing.GroupByAgent && groupBy != billing.GroupByNone {
		return stats.CostSeries{}, errs.New(errs.CodeValidationFailed, "Unknown grouping.").
			WithFields(errs.FieldError{Pointer: "/group_by", Code: "invalid", Detail: "Unknown grouping."})
	}
	key := t.key("costs", groupBy, q.Currency)
	if v, ok := s.cache.Get(key); ok {
		return v.(stats.CostSeries), nil
	}
	bq := billing.CostStatsQuery{From: t.rng.From, To: t.rng.To, Granularity: string(t.rng.Granularity), TimeZone: t.rng.Zone(),
		ChannelIDs: t.channels, GroupBy: groupBy, AgentID: q.AgentID, Currency: q.Currency}
	if bq.Currency, err = s.currency(ctx, bq); err != nil {
		return stats.CostSeries{}, err
	}
	vs, err := s.d.Billing.CostStats(ctx, bq)
	if err != nil {
		return stats.CostSeries{}, err
	}
	out := stats.CostSeries{Granularity: string(t.rng.Granularity), Currency: bq.Currency,
		Buckets: sumCosts(t.rng, vs, bq.Currency, t.costGroups(groupBy, q.AgentID))}
	s.cache.Put(key, out)
	return out, nil
}

// currency returns the currency of a cost series: the requested one, else the one currency of the
// range, defaultCurrency without any entry. Errors: validation_failed with errors[].code
// currency_required naming the currencies when the range has several.
func (s *Statistics) currency(ctx context.Context, q billing.CostStatsQuery) (string, error) {
	if q.Currency != "" {
		return q.Currency, nil
	}
	found, err := s.d.Billing.CostCurrencies(ctx, q)
	if err != nil {
		return "", err
	}
	switch len(found) {
	case 0:
		return defaultCurrency, nil
	case 1:
		return found[0], nil
	}
	detail := fmt.Sprintf("The range has costs in several currencies: %s.", strings.Join(found, ", "))
	return "", errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: "/currency", Code: "currency_required", Detail: detail})
}

// sumCosts sums the values of currency into buckets of rng per group and billing mode, ordered by
// start, group and mode; a value outside the range, of another currency or of a group outside
// allowed (nil allows every group) is dropped (defence in depth).
func sumCosts(rng domain.Range, vs []billing.CostStatsValue, currency string, allowed map[string]bool) []stats.CostBucket {
	sums := map[costCell]*stats.CostBucket{}
	var order []costCell
	for _, v := range vs {
		start, ok := rng.BucketOf(v.Bucket)
		if !ok || v.Currency != currency || (allowed != nil && !allowed[v.Group]) {
			continue
		}
		c := costCell{start.UnixNano(), v.Group, v.BillingMode}
		b, ok := sums[c]
		if !ok {
			b = &stats.CostBucket{Start: start, GroupKey: v.Group, BillingMode: v.BillingMode}
			sums[c] = b
			order = append(order, c)
		}
		b.AmountMicros += v.AmountMicros
		b.IsEstimate = b.IsEstimate || v.IsEstimate
	}
	slices.SortFunc(order, func(a, b costCell) int {
		if c := cmp.Compare(a.start, b.start); c != 0 {
			return c
		}
		if c := strings.Compare(a.group, b.group); c != 0 {
			return c
		}
		return strings.Compare(a.mode, b.mode)
	})
	out := make([]stats.CostBucket, len(order))
	for i, c := range order {
		out[i] = *sums[c]
	}
	return out
}

// costGroups returns the group keys a cost value may carry: the visible spaces, the one agent
// asked for, or ""; nil allows every agent (the channel set bounds them).
func (t target) costGroups(groupBy string, agent ids.ID) map[string]bool {
	switch groupBy {
	case billing.GroupBySpace:
		out := map[string]bool{}
		for sp := range t.spaceKeys() {
			out[sp] = true
		}
		return out
	case billing.GroupByAgent:
		if agent.IsZero() {
			return nil
		}
		return map[string]bool{agent.String(): true}
	default:
		return map[string]bool{"": true}
	}
}
