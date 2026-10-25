// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// keySLO is the cache key of the traffic light: one report for every owner, because it needs
// stats.read on the instance and shows only values of the whole instance.
var keySLO = cacheKey("slo", nil, "instance")

// SLO implements stats.Views (getStatsSlo, VZ-T09, docs/spec/25-visualization.md 6.4, M6-14
// design note 2, D-M6-14-2): stats.read on the instance (a space manager gets 403), then per SLO of
// slibounds.gen.go the sums of count and good of its metric over [now - period, now) across all
// rows (sli.run_start and sli.turn_duration carry channels; the owner sees the whole instance) and
// the state of domain.EvaluateSLO. The report is cached for stats.cache_ttl because it scans the
// whole period; the cache of M6-05 holds it.
func (v *Views) SLO(ctx context.Context) (stats.SLOReport, error) {
	if _, err := v.d.Statistics.authorize(ctx, authorize.ActionStatsRead, ids.ID{}); err != nil {
		return stats.SLOReport{}, err
	}
	cache := v.d.Statistics.cache
	if c, ok := cache.Get(keySLO); ok {
		return c.(stats.SLOReport), nil
	}
	now := v.d.Clock.Now().UTC()
	metrics := make([]string, 0, len(domain.SLIBounds))
	for m := range domain.SLIBounds {
		metrics = append(metrics, m)
	}
	slices.Sort(metrics)
	out := stats.SLOReport{At: now, Items: make([]stats.SLOEntry, 0, len(metrics))}
	for _, m := range metrics {
		b := domain.SLIBounds[m]
		totals, err := v.d.SLI.SLITotals(ctx, m, now.Add(-b.Period), now)
		if err != nil {
			return stats.SLOReport{}, err
		}
		var total, good int64
		for _, s := range totals {
			total, good = total+s.Count, good+s.Good
		}
		e := domain.EvaluateSLO(b, total, good, v.cfg.SLOWarnBudgetRatio, v.cfg.SLOMinEvents)
		item := stats.SLOEntry{SLO: b.SLO, Metric: m, Objective: b.Objective, Threshold: b.Threshold,
			PeriodDays: int(b.Period / (24 * time.Hour)), Total: e.Total, Good: e.Good, State: e.State}
		if e.HasRatio {
			ratio, budget := e.Ratio, e.BudgetRemaining
			item.Ratio, item.BudgetRemaining = &ratio, &budget
		}
		out.Items = append(out.Items, item)
	}
	slices.SortFunc(out.Items, func(a, b stats.SLOEntry) int { return strings.Compare(a.SLO, b.SLO) })
	cache.Put(keySLO, out)
	return out, nil
}
