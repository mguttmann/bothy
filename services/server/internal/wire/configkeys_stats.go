// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// The keys of the own measurement that the start and the config check read by name.
const (
	keyStatsFlushInterval   = "stats.flush_interval"
	keyStatsFlushQueueMax   = "stats.flush_queue_max"
	keyRetentionStatsMinute = "retention.stats_minutes"
)

// statsConfigKeys returns the keys of the own measurement (M6-13): the flush of
// docs/spec/25-visualization.md 15 and the retention class stats of docs/spec/20-privacy.md.
func statsConfigKeys() []config.Key {
	return []config.Key{
		{Name: keyStatsFlushInterval, Kind: config.KindDuration, Default: defaultTenSeconds, Min: "1s", Chapter: chapter25,
			Doc: "Interval at which a process writes its counters to stats_minutes."},
		{Name: keyStatsFlushQueueMax, Kind: config.KindInt, Default: "20000", Min: "1", Max: "10000000", Chapter: chapter25,
			Doc: "Counter keys in memory per process; above that, the oldest are discarded."},
		{Name: keyRetentionStatsMinute, Kind: config.KindDuration, Default: "90d", Min: "1d", Chapter: chapter20,
			Doc: "How long the minute aggregates of the own measurement stay; at least the longest SLO period."},
	}
}
