// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// Window is what one round of status.probe checks: the time of the round and the minute buckets
// [From, To) of stats_minutes the measured checks read (domain.Window, D-M7-09-1).
type Window struct {
	Now, From, To time.Time
}

// Source is one automatic check of docs/spec/16-operations.md 12 (api.http, sync.fanout,
// push.apns, push.fcm, files.storage, files.scan, search.query). Check returns the outcome of one
// round; a check that cannot get an answer returns domain.NoAnswer, never an error. It must
// return within the context.
type Source interface {
	Check(ctx context.Context, w Window) domain.Outcome
}

// SourceFunc is a Source as a function.
type SourceFunc func(ctx context.Context, w Window) domain.Outcome

// Check implements Source.
func (f SourceFunc) Check(ctx context.Context, w Window) domain.Outcome { return f(ctx, w) }

// The presence states an AgentSource reports (agents.State* of the agents module).
const (
	AgentOnline   = "online"
	AgentDegraded = "degraded"
	AgentOffline  = "offline"
	AgentUnknown  = "unknown"
)

// ActiveAgent is one active agent with its presence, as the check agent.presence reads it.
type ActiveAgent struct {
	ID          ids.ID
	Handle      string
	DisplayName string
	// State is AgentOnline, AgentDegraded, AgentOffline or AgentUnknown.
	State string
}

// AgentSource lists the active agents with their presence (agents.reported_runtime_state, 16 12
// "Agents").
type AgentSource interface {
	ActiveAgents(ctx context.Context) ([]ActiveAgent, error)
}

// Sources are the checks of a process: Checks by the key of 16 12 and the agents. A key without
// a source gives no evidence.
type Sources struct {
	Checks map[string]Source
	Agents AgentSource
}

// ProbeDeps are the collaborators of the job status.probe.
type ProbeDeps struct {
	Store PageStore
	UoW   txn.UnitOfWork
	// Events appends status.updated.
	Events eventlog.Appender
	// Clock nil means clock.System(); Log nil means slog.Default().
	Clock clock.Clock
	Log   *slog.Logger
}

// Probe is the job status.probe (brief M7-09, 16 12): every status.check_interval it keeps the
// standard components and one component per active agent, runs the checks, sets the automatic
// states and opens and resolves the automatic incidents. Its memory of failing runs lives in the
// process (domain.Track, D-M7-09-3). It is safe for concurrent use; rounds run one at a time.
type Probe struct {
	cfg domain.ProbeConfig
	src Sources
	d   ProbeDeps

	mu     sync.Mutex
	tracks map[ids.ID]domain.Track
}

// NewProbe returns the job over d with the periods of cfg and the sources src. Errors: a missing
// collaborator, an agent source missing, or a period that is not positive.
func NewProbe(cfg domain.ProbeConfig, src Sources, d ProbeDeps) (*Probe, error) {
	if d.Store == nil || d.UoW == nil || d.Events == nil || src.Agents == nil {
		return nil, errors.New("status: status.probe needs the store, the unit of work, the appender and the agent source")
	}
	if cfg.Interval <= 0 || cfg.DegradeAfter <= 0 || cfg.IncidentAfter <= 0 || cfg.ResolveAfter <= 0 {
		return nil, errors.New("status: status.check_interval and the status.*_after periods must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Probe{cfg: cfg, src: src, d: d, tracks: map[ids.ID]domain.Track{}}, nil
}

// Interval is status.check_interval, the period of the job.
func (p *Probe) Interval() time.Duration { return p.cfg.Interval }

// round is what one round changed, for status.updated.
type round struct {
	components []ids.ID
	incidents  []ids.ID
	changed    bool
}

// Run runs one round: the checks outside a transaction, each bounded by status.check_interval,
// then in one unit of work the components, the states, the incidents and, when anything changed,
// status.updated. A round without a change appends nothing (brief requirement 15).
func (p *Probe) Run(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.d.Clock.Now()
	from, to := domain.Window(now, p.cfg.Interval)
	outcomes := p.check(ctx, Window{Now: now, From: from, To: to})
	agents, err := p.src.Agents.ActiveAgents(ctx)
	if err != nil {
		return err
	}
	tracks := map[ids.ID]domain.Track{}
	err = p.d.UoW.Do(ctx, func(ctx context.Context) error {
		comps, err := p.d.Store.LockComponents(ctx)
		if err != nil {
			return err
		}
		var r round
		if comps, err = p.reconcile(ctx, comps, agents, now, &r); err != nil {
			return err
		}
		if err := p.apply(ctx, comps, outcomes, agents, now, tracks, &r); err != nil {
			return err
		}
		if !r.changed {
			return nil
		}
		latest, err := p.d.Store.Components(ctx)
		if err != nil {
			return err
		}
		return appendUpdated(ctx, p.d.Events, overall(latest), r.components, r.incidents)
	})
	if err != nil {
		return err
	}
	p.tracks = tracks
	return nil
}

// check runs every source once, all at the same time and each bounded by status.check_interval,
// so a round takes at most one interval before its transaction; it returns the outcome per key.
func (p *Probe) check(ctx context.Context, w Window) map[string]domain.Outcome {
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make(map[string]domain.Outcome, len(p.src.Checks))
	for key, s := range p.src.Checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, p.cfg.Interval)
			defer cancel()
			o := s.Check(cctx, w)
			mu.Lock()
			out[key] = o
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// reconcile creates the missing standard components and the components of active agents without
// one, and removes the components of agents that are no longer active (disabled or deleted). It
// returns the components after these changes in sort order.
func (p *Probe) reconcile(ctx context.Context, comps []domain.Component, agents []ActiveAgent, now time.Time, r *round) ([]domain.Component, error) {
	active := map[ids.ID]bool{}
	for _, a := range agents {
		active[a.ID] = true
	}
	kept := comps[:0:0]
	for _, c := range comps {
		gone := (!c.AgentID.IsZero() && !active[c.AgentID]) || (c.HealthSource == domain.SourceAgentPresence && c.AgentID.IsZero())
		if !gone {
			kept = append(kept, c)
			continue
		}
		if _, err := p.d.Store.DeleteComponent(ctx, c.ID); err != nil {
			return nil, err
		}
		r.changed = true
	}
	var want []domain.Component
	for _, s := range domain.StandardComponents() {
		if !slices.ContainsFunc(kept, func(c domain.Component) bool { return c.HealthSource == s.HealthSource }) {
			want = append(want, domain.Component{Name: s.Name, Description: s.Description, HealthSource: s.HealthSource,
				AgentID: ids.ID{}, State: domain.StateOperational})
		}
	}
	for _, a := range agents {
		if !slices.ContainsFunc(kept, func(c domain.Component) bool { return c.AgentID == a.ID }) {
			want = append(want, domain.Component{Name: a.DisplayName, HealthSource: domain.SourceAgentPresence, AgentID: a.ID,
				State: domain.StateOperational, Description: "@" + a.Handle})
		}
	}
	for _, c := range want {
		c.ID, c.UpdatedAt, c.SortKey = ids.New(), now, domain.SortKey(len(kept))
		ok, err := p.insertNamed(ctx, &c)
		if err != nil {
			return nil, err
		}
		if !ok {
			p.d.Log.WarnContext(ctx, msgComponentNameTaken, slog.String(logKeyHealthSource, c.HealthSource))
			continue
		}
		kept = append(kept, c)
		r.changed = true
		if !isAgentComponent(c) {
			r.components = append(r.components, c.ID)
		}
	}
	if len(want) > 0 || len(kept) != len(comps) {
		order := componentIDs(kept)
		if err := renumber(ctx, p.d.Store, kept, order); err != nil {
			return nil, err
		}
	}
	return kept, nil
}

// The log line of a component the job cannot create because both of its names are taken; the
// attribute names the check, never a name.
const (
	msgComponentNameTaken = "status: the name of an automatic component is taken by another component"
	logKeyHealthSource    = "health_source"
)

// insertNamed inserts c under its name and, when that is taken, under the name with its
// health_source (a standard component) or its handle (an agent) appended.
func (p *Probe) insertNamed(ctx context.Context, c *domain.Component) (bool, error) {
	ok, err := p.d.Store.InsertComponent(ctx, *c)
	if err != nil || ok {
		return ok, err
	}
	suffix := c.HealthSource
	if !c.AgentID.IsZero() {
		suffix = c.Description
	}
	c.Name = c.Name + " (" + suffix + ")"
	return p.d.Store.InsertComponent(ctx, *c)
}

// apply runs the state machine and the incident automation over the automatic components.
func (p *Probe) apply(ctx context.Context, comps []domain.Component, outcomes map[string]domain.Outcome, agents []ActiveAgent,
	now time.Time, tracks map[ids.ID]domain.Track, r *round) error {
	presence := map[ids.ID]string{}
	for _, a := range agents {
		presence[a.ID] = a.State
	}
	open, err := p.d.Store.OpenAutomaticIncidents(ctx)
	if err != nil {
		return err
	}
	for _, c := range comps {
		if !c.Automatic() {
			continue
		}
		next, tr := domain.Step(c.State, p.tracks[c.ID], p.outcome(c, outcomes, presence), now, p.cfg)
		tracks[c.ID] = tr
		since := c.UpdatedAt
		if next != c.State {
			if err := p.d.Store.SetComponentState(ctx, c.ID, next, now); err != nil {
				return err
			}
			since = now
			r.changed = true
			if !isAgentComponent(c) {
				r.components = append(r.components, c.ID)
			}
		}
		c.State = next
		if err := p.incidents(ctx, c, since, open[c.ID], now, r); err != nil {
			return err
		}
	}
	return nil
}

// outcome is the outcome of component c in this round.
func (p *Probe) outcome(c domain.Component, outcomes map[string]domain.Outcome, presence map[ids.ID]string) domain.Outcome {
	if c.HealthSource == domain.SourceAgentPresence {
		switch presence[c.AgentID] {
		case AgentOnline:
			return domain.Pass
		case AgentDegraded:
			return domain.Fail
		case AgentOffline:
			return domain.NoAnswer
		}
		// unknown: the agent never reported; no evidence (D-M7-09-4).
		return domain.NoEvidence
	}
	keys, _ := domain.Checks(c.HealthSource)
	list := make([]domain.Outcome, 0, len(keys))
	for _, k := range keys {
		list = append(list, outcomes[k])
	}
	return domain.Combine(list...)
}

// incidents opens or resolves the automatic incidents of c after the round.
func (p *Probe) incidents(ctx context.Context, c domain.Component, since time.Time, open []ids.ID, now time.Time, r *round) error {
	facts := domain.IncidentFacts{State: c.State, Since: since, OpenAutomatic: len(open) > 0}
	if c.State == domain.StateMajorOutage {
		ep, err := p.d.Store.AutomaticIncidentSince(ctx, c.ID, since)
		if err != nil {
			return err
		}
		facts.EpisodeIncident = ep
	}
	switch domain.DecideIncident(facts, now, p.cfg) {
	case domain.IncidentOpen:
		title, body := domain.AutomaticIncidentTexts(c.Name)
		inc := domain.Incident{ID: ids.New(), Title: title, Kind: domain.KindIncident, Impact: domain.ImpactMajor,
			State: domain.IncidentInvestigating, StartedAt: since, ComponentIDs: []ids.ID{c.ID}}
		first := domain.IncidentUpdate{ID: ids.New(), IncidentID: inc.ID, State: inc.State, BodyMD: body, CreatedAt: now}
		if err := p.d.Store.InsertIncident(ctx, inc, first); err != nil {
			return err
		}
		r.changed = true
		r.incidents = appendVisible(r.incidents, c, inc.ID)
	case domain.IncidentResolve:
		for _, id := range open {
			u := domain.IncidentUpdate{ID: ids.New(), IncidentID: id, State: domain.IncidentResolved, BodyMD: domain.AutomaticResolvedText, CreatedAt: now}
			if err := p.d.Store.AddIncidentUpdate(ctx, u, now); err != nil {
				return err
			}
			r.changed = true
			r.incidents = appendVisible(r.incidents, c, id)
		}
	case domain.IncidentNone:
	}
	return nil
}

// appendVisible appends the incident id of component c to list unless c belongs to an agent: the
// payload of status.updated reaches every reader with status.read (D-M7-09-5).
func appendVisible(list []ids.ID, c domain.Component, id ids.ID) []ids.ID {
	if isAgentComponent(c) {
		return list
	}
	return append(list, id)
}
