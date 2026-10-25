// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// The states of the traffic light (schema StatsSloState, docs/spec/25-visualization.md 6.4).
const (
	SLOGreen            = "green"
	SLOAmber            = "amber"
	SLORed              = "red"
	SLOInsufficientData = "insufficient_data"
)

// SLOEvaluation is the traffic light of one SLO over its period: the sums of count (Total) and
// good of its sli.* metric, the good share (Ratio) and the remaining error budget, both only when
// HasRatio (Total positive), and the State.
type SLOEvaluation struct {
	Total, Good     int64
	HasRatio        bool
	Ratio           float64
	BudgetRemaining float64
	State           string
}

// EvaluateSLO computes the traffic light of b from the sums total and good of its period (25 6.4,
// M6-14 design note 2): ratio = good / total, remaining budget
// 1 - (1 - ratio) / (1 - objective/100); the state is insufficient_data below minEvents
// (stats.slo_min_events) events, else red at a remaining budget of 0 or less, amber below
// warnRatio (stats.slo_warn_budget_ratio), else green. An objective of 100 percent has no budget:
// any bad event is red and none is green.
//
// Example:
//
//	e := domain.EvaluateSLO(domain.SLIBounds["sli.run_start"], 10000, 9950, 0.25, 100)
//	// e.Ratio == 0.995, e.BudgetRemaining == 0.5, e.State == "green"
func EvaluateSLO(b SLIBound, total, good int64, warnRatio float64, minEvents int64) SLOEvaluation {
	e := SLOEvaluation{Total: total, Good: good, State: SLOInsufficientData}
	if total <= 0 {
		return e
	}
	e.HasRatio = true
	e.Ratio = float64(good) / float64(total)
	if allowed := 1 - b.Objective/100; allowed > 0 {
		e.BudgetRemaining = 1 - (1-e.Ratio)/allowed
	} else if good < total {
		e.BudgetRemaining = -1
	} else {
		e.BudgetRemaining = 1
	}
	switch {
	case total < minEvents:
		e.State = SLOInsufficientData
	case e.BudgetRemaining <= 0:
		e.State = SLORed
	case e.BudgetRemaining < warnRatio:
		e.State = SLOAmber
	default:
		e.State = SLOGreen
	}
	return e
}

// TimelineLimits are the bounds of a timeline range: MaxRange is stats.timeline_max_range,
// Retention retention.stats_minutes.
type TimelineLimits struct {
	MaxRange  time.Duration
	Retention time.Duration
}

// ValidateTimeline checks the range [from, to) of getStatsTimeline at now (25 sections 6.5 and
// 11, M6-14 design note 1). Errors (validation_failed with one field error): from not before to
// (/from, invalid_range), from or to not on a full minute (/from or /to, not_full_minute), a range
// longer than lim.MaxRange (/to, range_too_long), from before now - lim.Retention (/from,
// outside_retention).
//
// Example:
//
//	err := domain.ValidateTimeline(from, from.Add(time.Hour), now, domain.TimelineLimits{MaxRange: 24 * time.Hour, Retention: 90 * 24 * time.Hour})
func ValidateTimeline(from, to, now time.Time, lim TimelineLimits) error {
	if !from.Before(to) {
		return fieldError("/from", "invalid_range", "from must be before to.")
	}
	if !Minute(from).Equal(from) {
		return fieldError("/from", "not_full_minute", "from must be a full minute.")
	}
	if !Minute(to).Equal(to) {
		return fieldError("/to", "not_full_minute", "to must be a full minute.")
	}
	if to.Sub(from) > lim.MaxRange {
		return fieldError("/to", "range_too_long", "The range is longer than stats.timeline_max_range.")
	}
	return checkRetention(from, now, lim.Retention)
}

// ValidateBackendRange checks the range [from, to) of getStatsBackends at now (M6-14, D-M6-14-3):
// from before to (/from, invalid_range) and not before now - retention (/from,
// outside_retention), because the view sums stats_minutes, which holds nothing older. The range
// needs no full minutes: a row counts when its minute starts in it.
func ValidateBackendRange(from, to, now time.Time, retention time.Duration) error {
	if !from.Before(to) {
		return fieldError("/from", "invalid_range", "from must be before to.")
	}
	return checkRetention(from, now, retention)
}

// checkRetention refuses a start before now - retention.stats_minutes.
func checkRetention(from, now time.Time, retention time.Duration) error {
	if from.Before(RetentionCutoff(now, retention)) {
		return fieldError("/from", "outside_retention", "from is older than retention.stats_minutes.")
	}
	return nil
}
