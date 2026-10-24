// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// statsConfigKeys returns the keys of the statistics queries of the stats module (M6-05):
// docs/spec/17-observability.md section 11 (stats.cache_ttl, stats.cache_max_entries,
// stats.max_range, stats.time_zone), docs/spec/06-api.md (api.stats_max_buckets) and
// docs/spec/20-privacy.md section 9 (privacy.stats_min_group_size,
// privacy.per_person_stats_enabled).
func statsConfigKeys() []config.Key {
	return []config.Key{
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
	}
}
