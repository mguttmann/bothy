// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ProbeConfig holds the periods of the automatic checks (docs/spec/16-operations.md 14); code
// and docs name the keys, never the values.
type ProbeConfig struct {
	// Interval is status.check_interval, the period of the job status.probe.
	Interval time.Duration
	// DegradeAfter is status.degrade_after: how long checks fail until degraded or major_outage.
	DegradeAfter time.Duration
	// IncidentAfter is status.incident_after: how long major_outage lasts until the automatic
	// incident.
	IncidentAfter time.Duration
	// ResolveAfter is status.resolve_after: how long the component stays operational until the
	// automatic incident is resolved.
	ResolveAfter time.Duration
}

// Outcome is the result of one check round of a component.
type Outcome int

// The outcomes, from no evidence to the most severe.
const (
	// NoEvidence: the window had no traffic or the source is off; it changes nothing (design
	// note 1: no traffic is no evidence, not a failure).
	NoEvidence Outcome = iota
	// Pass: the check succeeded.
	Pass
	// Fail: an error ratio above status.error_ratio_degraded, or a fan-out lag above
	// status.fanout_lag_degraded.
	Fail
	// Outage: an error ratio above status.error_ratio_outage.
	Outage
	// NoAnswer: the source gave no answer (a failed probe, an unreachable clamd, an agent that is
	// offline).
	NoAnswer
)

// String names the outcome for logs and test failures.
func (o Outcome) String() string {
	switch o {
	case NoEvidence:
		return "no_evidence"
	case Pass:
		return "pass"
	case Fail:
		return "fail"
	case Outage:
		return "outage"
	case NoAnswer:
		return "no_answer"
	}
	return fmt.Sprintf("outcome(%d)", int(o))
}

// failing reports whether o fails the check.
func (o Outcome) failing() bool { return o >= Fail }

// Combine returns the outcome of a component with several checks (a group key): the most severe
// one; no evidence counts only when no check has evidence.
func Combine(outcomes ...Outcome) Outcome {
	out := NoEvidence
	for _, o := range outcomes {
		if o > out {
			out = o
		}
	}
	return out
}

// Thresholds are status.error_ratio_degraded and status.error_ratio_outage.
type Thresholds struct {
	Degraded, Outage float64
}

// RatioOutcome returns the outcome of bad out of total in one window: no evidence without
// traffic, Outage above Thresholds.Outage, Fail above Thresholds.Degraded, else Pass.
//
// Example:
//
//	domain.RatioOutcome(6, 100, domain.Thresholds{Degraded: 0.05, Outage: 0.5}) // Fail
func RatioOutcome(bad, total int64, th Thresholds) Outcome {
	if total <= 0 {
		return NoEvidence
	}
	ratio := float64(max(bad, 0)) / float64(total)
	switch {
	case ratio > th.Outage:
		return Outage
	case ratio > th.Degraded:
		return Fail
	}
	return Pass
}

// Window returns the minute buckets of stats_minutes one round reads (design note 1, D-M7-09-1):
// the minutes that end within the last interval (status.check_interval), at least the last
// complete minute, as the half-open range [from, to) of bucket_start.
func Window(now time.Time, interval time.Duration) (from, to time.Time) {
	to = now.Truncate(time.Minute)
	from = now.Add(-interval - time.Minute).Truncate(time.Minute).Add(time.Minute)
	if !from.Before(to) {
		from = to.Add(-time.Minute)
	}
	return from, to
}

// gapRounds is how many check intervals without a round this process may miss before its memory
// of a component counts as broken: another process ran the rounds in between, or none ran.
const gapRounds = 3

// Observed is a component as the round reads it from status_components.
type Observed struct {
	// State is status_components.state.
	State ComponentState
	// UpdatedAt is status_components.updated_at: the time of the last change of the row, which a
	// change of the owner moves as well. It seeds a memory that is missing or broken, so a seed
	// can only delay the timers, never hasten them.
	UpdatedAt time.Time
}

// Track is the memory of the job status.probe about one component in one process (D-M7-09-3):
// the state it saw after its last round, since when, since when the component is out of
// operational, when the current run of failing rounds began, and when it saw the last round. The
// timers of the state machine and of the incidents read only this memory, never a column that
// other writes move (D-M7-09-7).
type Track struct {
	// State is the state after the last round; "" before the first.
	State ComponentState
	// EnteredAt is the time State began.
	EnteredAt time.Time
	// OutSince is the time the component last left operational (the start of the current
	// outage, in which it may move between degraded and major_outage); zero while operational.
	OutSince time.Time
	// FailingSince is the time of the first failing round of the current run; zero when the last
	// round with evidence passed.
	FailingSince time.Time
	// LastRound is the time of the last round; zero before the first.
	LastRound time.Time
}

// seed returns the memory of a component that this process has no valid memory of.
func seed(c Observed) Track {
	tr := Track{State: c.State, EnteredAt: c.UpdatedAt}
	if c.State != StateOperational {
		tr.OutSince = c.UpdatedAt
	}
	return tr
}

// Step applies the outcome o of the round at now to the component c (16 12 "Automatic state"): a
// pass gives operational at once; failing rounds give degraded (Fail) or major_outage (Outage,
// NoAnswer) once they have lasted status.degrade_after; no evidence changes nothing; maintenance
// is the owner's and stays. The memory tr is valid only while this process saw every round of the
// component (at least every gapRounds intervals) and the stored state is the one it left;
// otherwise it starts again from c, so a broken memory delays, never hastens, a change.
func Step(c Observed, tr Track, o Outcome, now time.Time, cfg ProbeConfig) (ComponentState, Track) {
	if tr.LastRound.IsZero() || now.Sub(tr.LastRound) > gapRounds*cfg.Interval || tr.State != c.State {
		tr = seed(c)
	}
	tr.LastRound = now
	next := nextState(c.State, &tr, o, now, cfg)
	if next != tr.State {
		switch {
		case next == StateOperational:
			tr.OutSince = time.Time{}
		case tr.State == StateOperational:
			tr.OutSince = now
		}
		tr.State, tr.EnteredAt = next, now
	}
	return next, tr
}

// nextState is the state after the round; it keeps FailingSince of tr.
func nextState(cur ComponentState, tr *Track, o Outcome, now time.Time, cfg ProbeConfig) ComponentState {
	if cur == StateMaintenance {
		tr.FailingSince = time.Time{}
		return cur
	}
	switch {
	case o == NoEvidence:
		return cur
	case !o.failing():
		tr.FailingSince = time.Time{}
		return StateOperational
	}
	if tr.FailingSince.IsZero() {
		tr.FailingSince = now
	}
	if now.Sub(tr.FailingSince) < cfg.DegradeAfter {
		return cur
	}
	if o == Fail {
		return StateDegraded
	}
	return StateMajorOutage
}

// IncidentAction is what the job does with the automatic incidents of one component.
type IncidentAction int

// The incident actions.
const (
	// IncidentNone changes nothing.
	IncidentNone IncidentAction = iota
	// IncidentOpen opens one automatic incident for the component.
	IncidentOpen
	// IncidentResolve resolves the open automatic incidents of the component.
	IncidentResolve
)

// LatestIncident is the newest automatic incident of a component (by started_at).
type LatestIncident struct {
	// ID is the incident.
	ID ids.ID
	// StartedAt is status_incidents.started_at: the start of the outage it was opened for.
	StartedAt time.Time
	// ResolvedAt is status_incidents.resolved_at; zero while open.
	ResolvedAt time.Time
	// Open reports a state other than resolved.
	Open bool
	// Ended reports that the job recorded the end of its outage: an update of the job (created_by
	// NULL) with state resolved, written after status.resolve_after of operational. An incident
	// a person resolved has none until then, so its outage counts as going on (D-M7-09-7).
	Ended bool
}

// IncidentFacts are what DecideIncident needs about one component.
type IncidentFacts struct {
	// State is the component state after the round.
	State ComponentState
	// Since is the time the component entered State (Track.EnteredAt).
	Since time.Time
	// Latest is the newest automatic incident of the component; nil when it never had one.
	Latest *LatestIncident
	// OpenAutomatic reports an automatic incident of the component that is not resolved.
	OpenAutomatic bool
}

// newOutage reports whether the current outage may get its own automatic incident (D-M7-09-7):
// none is open, and the newest one belongs to an outage whose end the job recorded. An outage
// ends after status.resolve_after of operational; the job records it on the newest automatic
// incident also when a person resolved that incident before. The decision reads only stored
// rows, so a restart, a gap of rounds or another worker cannot change it.
func newOutage(f IncidentFacts) bool {
	switch {
	case f.OpenAutomatic:
		return false
	case f.Latest == nil:
		return true
	}
	return !f.Latest.Open && f.Latest.Ended
}

// endPending reports an outage whose end the job has not recorded yet: an open automatic
// incident, or a newest one a person resolved without the job's end.
func endPending(f IncidentFacts) bool {
	return f.OpenAutomatic || (f.Latest != nil && !f.Latest.Ended)
}

// DecideIncident returns the incident action of the round at now (16 12 "Incidents", design
// note 3): open one incident when major_outage has lasted status.incident_after and the outage
// has none yet (newOutage); after status.resolve_after of operational resolve the open automatic
// ones and record the end of the outage (endPending). Incidents of the owner are never an input,
// so they are never touched.
func DecideIncident(f IncidentFacts, now time.Time, cfg ProbeConfig) IncidentAction {
	switch {
	case f.State == StateMajorOutage && now.Sub(f.Since) >= cfg.IncidentAfter && newOutage(f):
		return IncidentOpen
	case f.State == StateOperational && endPending(f) && now.Sub(f.Since) >= cfg.ResolveAfter:
		return IncidentResolve
	}
	return IncidentNone
}

// The texts of the automatic incidents: they name the component and the change, never a
// measurement or content (design note 3).
const (
	// automaticTitleFormat is the title: the component name and the state.
	automaticTitleFormat = "%s: major outage"
	// AutomaticOpenText is the first update of an automatic incident.
	AutomaticOpenText = "The automatic checks report a major outage of this component."
	// AutomaticResolvedText is the update that resolves an automatic incident.
	AutomaticResolvedText = "The automatic checks report this component operational again."
)

// AutomaticIncidentTexts returns the title and the first update of the automatic incident of the
// component named name.
func AutomaticIncidentTexts(name string) (title, body string) {
	return fmt.Sprintf(automaticTitleFormat, name), AutomaticOpenText
}
