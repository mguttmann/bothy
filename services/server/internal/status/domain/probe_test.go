// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// cfg holds the defaults of 16 14 as the tests name them: status.check_interval,
// status.degrade_after, status.incident_after and status.resolve_after.
var cfg = domain.ProbeConfig{
	Interval: 30 * time.Second, DegradeAfter: 2 * time.Minute, IncidentAfter: 5 * time.Minute, ResolveAfter: 10 * time.Minute,
}

var t0 = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// rounds runs Step once per check interval with the outcomes, keeping the stored state and its
// updated_at as the store of the job does, and returns the state after each.
func rounds(start domain.ComponentState, outcomes ...domain.Outcome) []domain.ComponentState {
	st, upd, tr := start, t0, domain.Track{}
	out := make([]domain.ComponentState, 0, len(outcomes))
	for i, o := range outcomes {
		now := t0.Add(time.Duration(i) * cfg.Interval)
		var next domain.ComponentState
		next, tr = domain.Step(domain.Observed{State: st, UpdatedAt: upd}, tr, o, now, cfg)
		if next != st {
			st, upd = next, now
		}
		out = append(out, st)
	}
	return out
}

func repeat(o domain.Outcome, n int) []domain.Outcome {
	out := make([]domain.Outcome, n)
	for i := range out {
		out[i] = o
	}
	return out
}

// TestStateMachine (16 12 "Automatic state"): a healthy component stays operational; failing
// checks change nothing before status.degrade_after; after it a failing check gives degraded, an
// error ratio above status.error_ratio_outage or no answer major_outage; a passing check gives
// operational at once; no evidence changes nothing; maintenance is the owner's and stays.
func TestStateMachine(t *testing.T) {
	t.Parallel()
	// status.degrade_after is four check intervals: the fifth failing round (t0 + 2m) degrades.
	failing := 4
	t.Run("healthy", func(t *testing.T) {
		for i, st := range rounds(domain.StateOperational, repeat(domain.Pass, 10)...) {
			if st != domain.StateOperational {
				t.Fatalf("round %d: %s", i, st)
			}
		}
	})
	for _, c := range []struct {
		name string
		o    domain.Outcome
		want domain.ComponentState
	}{
		{"failing check gives degraded", domain.Fail, domain.StateDegraded},
		{"outage ratio gives major_outage", domain.Outage, domain.StateMajorOutage},
		{"no answer gives major_outage", domain.NoAnswer, domain.StateMajorOutage},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := rounds(domain.StateOperational, repeat(c.o, failing+2)...)
			for i := 0; i < failing; i++ {
				if got[i] != domain.StateOperational {
					t.Fatalf("round %d below status.degrade_after: %s", i, got[i])
				}
			}
			if got[failing] != c.want || got[failing+1] != c.want {
				t.Fatalf("after status.degrade_after: %v, want %s", got, c.want)
			}
		})
	}
	t.Run("a pass in between restarts the wait", func(t *testing.T) {
		seq := append(repeat(domain.Fail, failing), domain.Pass)
		seq = append(seq, repeat(domain.Fail, failing)...)
		for i, st := range rounds(domain.StateOperational, seq...) {
			if st != domain.StateOperational {
				t.Fatalf("round %d: %s", i, st)
			}
		}
	})
	t.Run("recovery is at once", func(t *testing.T) {
		got := rounds(domain.StateOperational, append(repeat(domain.NoAnswer, failing+1), domain.Pass)...)
		if got[failing] != domain.StateMajorOutage || got[failing+1] != domain.StateOperational {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("lower ratio while in outage gives degraded", func(t *testing.T) {
		got := rounds(domain.StateOperational, append(repeat(domain.Outage, failing+1), domain.Fail)...)
		if got[failing+1] != domain.StateDegraded {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("no evidence changes nothing", func(t *testing.T) {
		got := rounds(domain.StateDegraded, repeat(domain.NoEvidence, 5)...)
		for i, st := range got {
			if st != domain.StateDegraded {
				t.Fatalf("round %d: %s", i, st)
			}
		}
		// Rounds without evidence do not interrupt a run of failing rounds.
		seq := append(repeat(domain.Fail, 2), domain.NoEvidence, domain.Fail, domain.Fail)
		if got := rounds(domain.StateOperational, seq...); got[4] != domain.StateDegraded {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("maintenance stays", func(t *testing.T) {
		for i, st := range rounds(domain.StateMaintenance, repeat(domain.NoAnswer, 10)...) {
			if st != domain.StateMaintenance {
				t.Fatalf("round %d: %s", i, st)
			}
		}
	})
	t.Run("a gap in the rounds restarts the wait", func(t *testing.T) {
		// The memory of this process saw no round for more than three check intervals (another
		// process ran them, or none ran): the failing run counts from the next round.
		obs := domain.Observed{State: domain.StateOperational, UpdatedAt: t0}
		_, tr := domain.Step(obs, domain.Track{}, domain.Fail, t0, cfg)
		var st domain.ComponentState
		resumed := t0.Add(10 * cfg.Interval)
		for i := 0; i < failing; i++ {
			if st, tr = domain.Step(obs, tr, domain.Fail, resumed.Add(time.Duration(i)*cfg.Interval), cfg); st != domain.StateOperational {
				t.Fatalf("round %d after the gap: %s", i, st)
			}
		}
		if st, _ = domain.Step(obs, tr, domain.Fail, resumed.Add(cfg.DegradeAfter), cfg); st != domain.StateDegraded {
			t.Fatalf("status.degrade_after after the gap: %s", st)
		}
	})
	t.Run("the memory keeps its times against other writes", func(t *testing.T) {
		// An owner change of the name moves updated_at, but not the time the outage began.
		obs := domain.Observed{State: domain.StateOperational, UpdatedAt: t0}
		_, tr := domain.Step(obs, domain.Track{}, domain.NoAnswer, t0, cfg)
		var st domain.ComponentState
		for i := 1; i <= failing; i++ {
			st, tr = domain.Step(obs, tr, domain.NoAnswer, t0.Add(time.Duration(i)*cfg.Interval), cfg)
		}
		began := t0.Add(cfg.DegradeAfter)
		if st != domain.StateMajorOutage || !tr.EnteredAt.Equal(began) || !tr.OutSince.Equal(began) || !tr.OutSinceSeen {
			t.Fatalf("outage: %s %+v", st, tr)
		}
		edited := domain.Observed{State: domain.StateMajorOutage, UpdatedAt: began.Add(time.Minute)}
		if _, tr = domain.Step(edited, tr, domain.Fail, began.Add(cfg.Interval), cfg); !tr.OutSince.Equal(began) || tr.State != domain.StateDegraded {
			t.Fatalf("after an edit and a lower ratio: %+v", tr)
		}
		if _, tr = domain.Step(domain.Observed{State: domain.StateDegraded, UpdatedAt: began.Add(cfg.Interval)}, tr, domain.NoAnswer,
			began.Add(2*cfg.Interval), cfg); !tr.OutSince.Equal(began) || tr.State != domain.StateMajorOutage {
			t.Fatalf("back to major_outage: the outage still began at %s: %+v", began, tr)
		}
		// A state the owner set is a new start: the memory seeds from updated_at.
		owner := domain.Observed{State: domain.StateOperational, UpdatedAt: began.Add(5 * time.Minute)}
		if _, tr = domain.Step(owner, tr, domain.Pass, began.Add(6*time.Minute), cfg); !tr.EnteredAt.Equal(owner.UpdatedAt) || !tr.OutSince.IsZero() {
			t.Fatalf("after the owner's state: %+v", tr)
		}
		// A memory seeded out of operational does not claim to have seen the start of the outage.
		down := domain.Observed{State: domain.StateMajorOutage, UpdatedAt: began.Add(7 * time.Minute)}
		if _, tr = domain.Step(down, domain.Track{}, domain.NoAnswer, began.Add(8*time.Minute), cfg); tr.OutSinceSeen || !tr.OutSince.Equal(down.UpdatedAt) {
			t.Fatalf("seeded out of operational: %+v", tr)
		}
	})
}

// TestOutcomes: the error ratio of a window against status.error_ratio_degraded and
// status.error_ratio_outage (strictly above), no traffic as no evidence, and the combination of
// the checks of a group (the worst counts, no evidence is ignored).
func TestOutcomes(t *testing.T) {
	t.Parallel()
	th := domain.Thresholds{Degraded: 0.05, Outage: 0.5}
	for _, c := range []struct {
		bad, total int64
		want       domain.Outcome
	}{
		{0, 0, domain.NoEvidence}, {0, 100, domain.Pass}, {5, 100, domain.Pass}, {6, 100, domain.Fail},
		{50, 100, domain.Fail}, {51, 100, domain.Outage}, {100, 100, domain.Outage},
	} {
		if got := domain.RatioOutcome(c.bad, c.total, th); got != c.want {
			t.Errorf("RatioOutcome(%d, %d) = %v, want %v", c.bad, c.total, got, c.want)
		}
	}
	for _, c := range []struct {
		in   []domain.Outcome
		want domain.Outcome
	}{
		{nil, domain.NoEvidence},
		{[]domain.Outcome{domain.NoEvidence, domain.Pass}, domain.Pass},
		{[]domain.Outcome{domain.Pass, domain.Fail}, domain.Fail},
		{[]domain.Outcome{domain.Outage, domain.Fail}, domain.Outage},
		{[]domain.Outcome{domain.NoAnswer, domain.Outage, domain.NoEvidence}, domain.NoAnswer},
	} {
		if got := domain.Combine(c.in...); got != c.want {
			t.Errorf("Combine(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestWindow (design note 1): the minutes of stats_minutes that end within the last
// status.check_interval, at least the last complete minute.
func TestWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 25, 12, 7, 20, 0, time.UTC)
	for _, c := range []struct {
		interval time.Duration
		from, to time.Time
	}{
		{30 * time.Second, now.Truncate(time.Minute).Add(-time.Minute), now.Truncate(time.Minute)},
		{time.Minute, now.Truncate(time.Minute).Add(-time.Minute), now.Truncate(time.Minute)},
		{5 * time.Minute, time.Date(2026, 10, 25, 12, 2, 0, 0, time.UTC), now.Truncate(time.Minute)},
	} {
		from, to := domain.Window(now, c.interval)
		if !from.Equal(c.from) || !to.Equal(c.to) {
			t.Errorf("Window(%s) = [%s, %s), want [%s, %s)", c.interval, from, to, c.from, c.to)
		}
	}
}

// TestIncidentAutomation (16 12 "Incidents", design note 3, D-M7-09-7): a major_outage longer
// than status.incident_after opens one automatic incident per outage; status.resolve_after of
// health resolves the open automatic ones; an incident the owner resolved during the outage is not
// opened again, also not after the memory was seeded (restart, gap), and a new outage after a
// recovery gets its own; an incident of the owner is never touched (the decision names only the
// automatic ones the caller passes).
func TestIncidentAutomation(t *testing.T) {
	t.Parallel()
	since := t0
	seen := domain.Track{OutSince: since, OutSinceSeen: true}
	seeded := domain.Track{OutSince: since}
	byOwner := &domain.LatestIncident{StartedAt: since, ResolvedByOwner: true}
	earlierByOwner := &domain.LatestIncident{StartedAt: since.Add(-time.Hour), ResolvedByOwner: true}
	byJob := &domain.LatestIncident{StartedAt: since.Add(-time.Hour)}
	open := &domain.LatestIncident{StartedAt: since, Open: true}
	for _, c := range []struct {
		name   string
		state  domain.ComponentState
		now    time.Time
		track  domain.Track
		latest *domain.LatestIncident
		open   bool
		want   domain.IncidentAction
	}{
		{"outage below incident_after", domain.StateMajorOutage, since.Add(cfg.IncidentAfter - time.Second), seen, nil, false, domain.IncidentNone},
		{"outage at incident_after", domain.StateMajorOutage, since.Add(cfg.IncidentAfter), seen, nil, false, domain.IncidentOpen},
		{"seeded memory without any incident", domain.StateMajorOutage, since.Add(cfg.IncidentAfter), seeded, nil, false, domain.IncidentOpen},
		{"never a second while one is open", domain.StateMajorOutage, since.Add(cfg.IncidentAfter * 3), seen, open, true, domain.IncidentNone},
		{"the owner resolved it during the outage", domain.StateMajorOutage, since.Add(cfg.IncidentAfter * 3), seen, byOwner, false, domain.IncidentNone},
		{"the owner resolved it, then a restart", domain.StateMajorOutage, since.Add(cfg.IncidentAfter * 3), seeded, earlierByOwner, false, domain.IncidentNone},
		{"the owner resolved an earlier outage", domain.StateMajorOutage, since.Add(cfg.IncidentAfter * 3), seen, earlierByOwner, false, domain.IncidentOpen},
		{"the job resolved an earlier outage", domain.StateMajorOutage, since.Add(cfg.IncidentAfter * 3), seeded, byJob, false, domain.IncidentOpen},
		{"degraded opens none", domain.StateDegraded, since.Add(cfg.IncidentAfter * 3), seen, nil, false, domain.IncidentNone},
		{"healthy below resolve_after", domain.StateOperational, since.Add(cfg.ResolveAfter - time.Second), domain.Track{}, nil, true, domain.IncidentNone},
		{"healthy at resolve_after", domain.StateOperational, since.Add(cfg.ResolveAfter), domain.Track{}, nil, true, domain.IncidentResolve},
		{"healthy without an open incident", domain.StateOperational, since.Add(cfg.ResolveAfter), domain.Track{}, nil, false, domain.IncidentNone},
		{"degraded keeps it open", domain.StateDegraded, since.Add(cfg.ResolveAfter * 3), seen, nil, true, domain.IncidentNone},
	} {
		got := domain.DecideIncident(domain.IncidentFacts{State: c.state, Since: since, Track: c.track, Latest: c.latest, OpenAutomatic: c.open}, c.now, cfg)
		if got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// The texts of an automatic incident name the component and the change, nothing else.
	title, first := domain.AutomaticIncidentTexts("Search")
	if title != "Search: major outage" || first == "" || domain.AutomaticResolvedText == "" {
		t.Errorf("texts %q %q %q", title, first, domain.AutomaticResolvedText)
	}
}
