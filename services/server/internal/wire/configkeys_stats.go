// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// The keys of the own measurement that the start and the config check read by name.
const (
	keyStatsFlushInterval   = "stats.flush_interval"
	keyStatsFlushQueueMax   = "stats.flush_queue_max"
	keyRetentionStatsMinute = "retention.stats_minutes"
)

// The keys of the live stream (M6-15).
const (
	keyStatsLiveEnabled        = "stats.live_enabled"
	keyStatsLiveInterval       = "stats.live_interval"
	keyStatsLiveMaxRuns        = "stats.live_max_runs"
	keyStatsLiveMaxSubscribers = "stats.live_max_subscribers"
	keyStatsLiveSLIWindow      = "stats.live_sli_window"
	keyStatsLiveMaxTasks       = "stats.live_max_tasks"
	keyStatsLiveTaskMaxDepth   = "stats.live_task_max_depth"
	keyStatsLiveTaskKeep       = "stats.live_task_keep"
)

// The keys of the views (M6-14).
const (
	keyStatsTimelineMaxRange   = "stats.timeline_max_range"
	keyStatsTimelineMaxRuns    = "stats.timeline_max_runs"
	keyStatsSLOWarnBudgetRatio = "stats.slo_warn_budget_ratio"
	keyStatsSLOMinEvents       = "stats.slo_min_events"
)

// statsConfigKeys returns the keys of the stats module: the own measurement (M6-13), the flush of
// docs/spec/25-visualization.md 15 and the retention class stats of docs/spec/20-privacy.md; and
// the statistics queries (M6-05), docs/spec/17-observability.md section 11 (stats.cache_ttl,
// stats.cache_max_entries, stats.max_range, stats.time_zone), docs/spec/06-api.md
// (api.stats_max_buckets) and docs/spec/20-privacy.md section 9 (privacy.stats_min_group_size,
// privacy.per_person_stats_enabled); the keys of the live stream stats.live and getStatsLive
// (M6-15, 25 15); and the keys of the timeline and the traffic light (M6-14, 25 15).
func statsConfigKeys() []config.Key {
	return []config.Key{
		{Name: keyStatsFlushInterval, Kind: config.KindDuration, Default: defaultTenSeconds, Min: "1s", Chapter: chapter25,
			Doc: "Interval at which a process writes its counters to stats_minutes."},
		{Name: keyStatsFlushQueueMax, Kind: config.KindInt, Default: "20000", Min: "1", Max: "10000000", Chapter: chapter25,
			Doc: "Counter keys in memory per process; above that, the oldest are discarded."},
		{Name: keyRetentionStatsMinute, Kind: config.KindDuration, Default: defaultNinetyDays, Min: "1d", Chapter: chapter20,
			Doc: "How long the minute aggregates of the own measurement stay; at least the longest SLO period."},
		{Name: "stats.cache_ttl", Kind: config.KindDuration, Default: "5m", Min: "1s", Max: "1h", Chapter: chapter17,
			Doc: "How long a computed statistics result stays valid."},
		{Name: "stats.cache_max_entries", Kind: config.KindInt, Default: "1000", Min: "1", Max: "1000000", Chapter: chapter17,
			Doc: "Statistics results cached per process; above it the least recently used one is evicted."},
		{Name: "stats.max_range", Kind: config.KindDuration, Default: "400d", Min: "1d", Chapter: chapter17,
			Doc: "Largest queryable time range of the statistics."},
		{Name: "stats.time_zone", Kind: config.KindString, Default: "UTC", Chapter: chapter17,
			Doc: "IANA time zone of the day, week and month boundaries of the statistics."},
		{Name: "api.stats_max_buckets", Kind: config.KindInt, Default: "1000", Min: "1", Max: "100000", Chapter: chapter06,
			Doc: "Time buckets per statistics query; more gives 400."},
		{Name: "privacy.stats_min_group_size", Kind: config.KindInt, Default: "5", Min: "2", Max: "1000", Chapter: chapter20,
			Doc: "Smallest group a statistics counter about people may show."},
		{Name: "privacy.per_person_stats_enabled", Kind: config.KindBool, Default: defaultFalse, Chapter: chapter20,
			Doc: "Statistics per person; no statistics operation groups by person whatever its value."},
		{Name: keyStatsLiveEnabled, Kind: config.KindBool, Default: defaultTrue, Chapter: chapter25,
			Doc: "Live stream and getStatsLive on; off: 403 feature_disabled."},
		{Name: keyStatsLiveInterval, Kind: config.KindDuration, Default: "1s", Min: "100ms", Chapter: chapter25,
			Doc: "Interval between live frames; also the lifetime of the base frame for getStatsLive."},
		{Name: keyStatsLiveMaxRuns, Kind: config.KindInt, Default: "200", Min: "1", Max: "10000", Chapter: chapter25,
			Doc: "At most this many active runs per frame."},
		{Name: keyStatsLiveMaxSubscribers, Kind: config.KindInt, Default: "200", Min: "1", Max: "100000", Chapter: chapter25,
			Doc: "Subscriptions to stats.live per api node."},
		{Name: keyStatsLiveSLIWindow, Kind: config.KindDuration, Default: "5m", Min: "1m", Chapter: chapter25,
			Doc: "Period of the SLI values in the live frame."},
		{Name: keyStatsLiveMaxTasks, Kind: config.KindInt, Default: "25", Min: "0", Max: "500", Chapter: chapter25,
			Doc: "At most this many nodes of the agent tree per run in the live frame; 0 sends no tree."},
		{Name: keyStatsLiveTaskMaxDepth, Kind: config.KindInt, Default: "4", Min: "1", Max: "16", Chapter: chapter25,
			Doc: "Deepest level of the agent tree in the live frame; the children of a run are level 1."},
		{Name: keyStatsLiveTaskKeep, Kind: config.KindDuration, Default: "2m", Min: "1s", Max: "1h", Chapter: chapter25,
			Doc: "How long a finished node of the agent tree stays in the live frame after its end."},
		{Name: keyStatsTimelineMaxRange, Kind: config.KindDuration, Default: defaultDay, Min: "1m", Max: "31d", Chapter: chapter25,
			Doc: "Longest period of a timeline query."},
		{Name: keyStatsTimelineMaxRuns, Kind: config.KindInt, Default: "5000", Min: "1", Max: "100000", Chapter: chapter25,
			Doc: "At most this many runs in a timeline response."},
		{Name: keyStatsSLOWarnBudgetRatio, Kind: config.KindFloat, Default: "0.25", Min: "0", Max: "1", Chapter: chapter25,
			Doc: "Below this remaining budget, the traffic light turns yellow."},
		{Name: keyStatsSLOMinEvents, Kind: config.KindInt, Default: "100", Min: "1", Max: "1000000000", Chapter: chapter25,
			Doc: "Below this many events in the SLO period, the traffic light is gray."},
	}
}
