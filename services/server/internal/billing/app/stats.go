// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// StatsReader is the outgoing port of Stats: the statistics reads of adapters/postgres
// (queries/stats.sql). showPersonal lets the entries of mode personal count.
type StatsReader interface {
	CostStats(ctx context.Context, q billing.CostStatsQuery, showPersonal bool) ([]billing.CostStatsValue, error)
	CostCurrencies(ctx context.Context, q billing.CostStatsQuery, showPersonal bool) ([]string, error)
}

// Stats implements billing.StatsSource (M6-05) over a StatsReader with the display rule of 10
// section 12.2 (billing.show_personal_estimates): it checks the input and reads nothing for an
// empty channel set (fail closed, 03 principle 12). It never authorizes; the stats use case did.
type Stats struct {
	r            StatsReader
	showPersonal bool
}

// Compile-time check: Stats is the statistics port of billing.
var _ billing.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r with the display rule of cfg. Errors: a nil
// reader.
func NewStats(cfg Config, r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("billing: the statistics source needs its reader")
	}
	return &Stats{r: r, showPersonal: cfg.ShowPersonalEstimates}, nil
}

// statsGroupings are the groupings of CostStatsQuery.GroupBy.
var statsGroupings = map[string]bool{billing.GroupBySpace: true, billing.GroupByAgent: true, billing.GroupByNone: true}

// CostStats implements billing.StatsSource.
func (s *Stats) CostStats(ctx context.Context, q billing.CostStatsQuery) ([]billing.CostStatsValue, error) {
	if err := checkCostQuery(q); err != nil {
		return nil, err
	}
	if len(q.ChannelIDs) == 0 {
		return nil, nil
	}
	return s.r.CostStats(ctx, q, s.showPersonal)
}

// CostCurrencies implements billing.StatsSource.
func (s *Stats) CostCurrencies(ctx context.Context, q billing.CostStatsQuery) ([]string, error) {
	if err := checkCostQuery(q); err != nil {
		return nil, err
	}
	if len(q.ChannelIDs) == 0 {
		return nil, nil
	}
	return s.r.CostCurrencies(ctx, q, s.showPersonal)
}

// checkCostQuery checks granularity, time zone and grouping.
func checkCostQuery(q billing.CostStatsQuery) error {
	if !statsGroupings[q.GroupBy] {
		return errs.New(errs.CodeValidationFailed, "billing: unknown statistics grouping")
	}
	return checkStatsQuery(q.Granularity, q.TimeZone, 0, 0)
}

// statsGranularities are the bucket sizes date_trunc of the statistics reads accepts.
var statsGranularities = map[string]bool{"hour": true, "day": true, "week": true, "month": true}

// checkStatsQuery refuses what the statistics reads cannot run: an unknown granularity or time zone
// and parallel lists of different lengths (validation_failed, a programming error of the caller).
func checkStatsQuery(granularity, tz string, keys, groups int) error {
	if !statsGranularities[granularity] {
		return errs.New(errs.CodeValidationFailed, ": unknown statistics granularity")
	}
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return errs.New(errs.CodeValidationFailed, ": unknown statistics time zone")
	}
	if keys != groups {
		return errs.New(errs.CodeValidationFailed, ": the statistics keys and groups differ in length")
	}
	return nil
}
