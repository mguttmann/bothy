// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// rateLimitDefaults are the defaults of docs/spec/05-actions.md "Configuration values of this
// chapter": per_minute and burst of every class of 05 "Rate-limit classes". The schema test
// holds them against the chapter table, and rateLimitConfigKeys refuses a class of the catalogue
// without a row here.
var rateLimitDefaults = map[ratelimit.Class]ratelimit.Limits{
	"rl.public_auth":     {PerMinute: 30, Burst: 10},
	"rl.gateway_connect": {PerMinute: 12, Burst: 4},
	"rl.read":            {PerMinute: 600, Burst: 100},
	"rl.write":           {PerMinute: 120, Burst: 30},
	"rl.message":         {PerMinute: 60, Burst: 20},
	"rl.trigger":         {PerMinute: 30, Burst: 10},
	"rl.interaction":     {PerMinute: 60, Burst: 20},
	"rl.upload":          {PerMinute: 30, Burst: 10},
	"rl.search":          {PerMinute: 60, Burst: 20},
	"rl.typing":          {PerMinute: 30, Burst: 5},
	"rl.admin":           {PerMinute: 60, Burst: 20},
	"rl.export":          {PerMinute: 2, Burst: 2},
	"rl.agent_runtime":   {PerMinute: 1200, Burst: 200},
	"rl.webhook_in":      {PerMinute: 60, Burst: 20},
	"rl.import":          {PerMinute: 120, Burst: 30},
	"rl.terminal":        {PerMinute: 20, Burst: 5},
	"rl.host":            {PerMinute: 120, Burst: 30},
}

// Key names of the rate limiter.
const (
	keyMaxTrackedKeys = "ratelimit.max_tracked_keys"
	keySweepInterval  = "ratelimit.sweep_interval"
)

// perMinuteKey and burstKey name the two keys of a class, for example
// ratelimit.rl.read.per_minute.
func perMinuteKey(c ratelimit.Class) string { return "ratelimit." + string(c) + ".per_minute" }
func burstKey(c ratelimit.Class) string     { return "ratelimit." + string(c) + ".burst" }

// rateLimitConfigKeys returns the keys of the rate limiter (M1-19): per_minute and burst of every
// class of 05 "Rate-limit classes" (05 "Configuration values of this chapter"), including
// rl.terminal and rl.host, which no operation uses before M2 (P-24), and max_tracked_keys and
// sweep_interval of 02 "Configuration values of this chapter". The upper bound of a class value is
// ratelimit.MaxLimit, which keeps the fixed-point bucket arithmetic exact. A class without a
// default panics: the tests catch it before a release.
func rateLimitConfigKeys() []config.Key {
	limit := strconv.Itoa(ratelimit.MaxLimit)
	var keys []config.Key
	for _, c := range ratelimit.Classes() {
		def, ok := rateLimitDefaults[c]
		if !ok {
			panic("wire: no default rate limits for class " + string(c))
		}
		keys = append(keys,
			config.Key{
				Name: perMinuteKey(c), Kind: config.KindInt, Default: strconv.Itoa(def.PerMinute), Min: "1", Max: limit,
				Doc:     "Tokens per minute that the bucket of " + string(c) + " refills, per key of the class.",
				Chapter: chapter05,
			},
			config.Key{
				Name: burstKey(c), Kind: config.KindInt, Default: strconv.Itoa(def.Burst), Min: "1", Max: limit,
				Doc:     "Capacity of the bucket of " + string(c) + ": requests a key may send at once.",
				Chapter: chapter05,
			})
	}
	return append(keys,
		config.Key{
			// At most 100 million buckets: far above any single node's memory budget (see the
			// README of platform/ratelimit for bytes per key), but a bound against typos.
			Name: keyMaxTrackedKeys, Kind: config.KindInt, Default: "100000", Min: "1", Max: "100000000",
			Doc:     "Most rate-limit buckets one process holds; beyond it full, then least recently used buckets are removed.",
			Chapter: chapter02,
		},
		config.Key{
			Name: keySweepInterval, Kind: config.KindDuration, Default: "1m", Min: "1s", Max: "1h",
			Doc:     "Interval in which full rate-limit buckets are removed; a full bucket decides like a missing one.",
			Chapter: chapter02,
		})
}
