// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/measure"

	statshttp "github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
)

// statsState is the part of the server state the stats module fills.
type statsState struct {
	// stats is the client metrics part of the stats module (M4-31), built in the chat step.
	stats *Stats
	// measure is the own measurement (M6-13), built in the step stats_measure right after the
	// schema step, before every module that records.
	measure *Measure
}

// moduleStats is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleStats = "stats"

// stepStatsMeasure is the start step of the own measurement.
const stepStatsMeasure = "stats_measure"

// stepStatistics is the start step of the statistics queries (M6-05): after apps, whose
// statistics source it reads, and only in the roles api and all, which serve the REST operations.
const stepStatistics = "statistics"

// statsWiring wires the stats module: the own measurement in its step after the schema (every
// role, M6-13), whose recorder the modules built later receive; the client metrics in the start
// step chat, last; the statistics queries in their own step after apps (roles api and all,
// M6-05); and the job stats.retention.
func statsWiring() moduleWiring {
	return moduleWiring{
		module: moduleStats,
		parts: []part{
			{order: 550, step: stepStatsMeasure, start: (*server).startMeasure},
			{order: 1370, step: moduleChat, start: (*server).startStats},
			{order: 1650, step: stepStatistics, when: (*server).api, start: (*server).startStatistics},
		},
		handlers: func(s *server, h *Handlers) { s.stats.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			me := s.measure
			s.mu.Unlock()
			if me == nil {
				return errors.New("wire: the jobs step needs the own measurement")
			}
			m.StatsRetention = me.Retention
			return nil
		},
	}
}

// startMeasure builds the own measurement over the pool (M6-13) and starts its periodic flush
// with the stop step stats_flush (Measure.Start), except in maintenance mode, which writes no
// measurement (M7-08); Deps.StatsMeasure, when set, receives it.
func (s *server) startMeasure(context.Context) error {
	cfg, err := MeasureConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	me, err := NewMeasure(cfg, pool, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	if !s.maint.ReadOnly && s.role != roleRestore {
		// Maintenance mode measures nothing (M7-08): the rows would be missing on the target of a
		// relocation, and every 503 maintenance would count against sli.api_availability.
		// bothy restore finalize measures nothing either: it serves no request (M7-07).
		me.Start(s.runner)
	}
	s.mu.Lock()
	s.measure = me
	s.mu.Unlock()
	if s.deps.StatsMeasure != nil {
		s.deps.StatsMeasure(me)
	}
	return nil
}

// measureRecorder returns the recorder of the own measurement for the modules; before the step
// stats_measure, in a process built without it and in maintenance mode, it records nothing.
func (s *server) measureRecorder() measure.Recorder {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.measure == nil || s.maint.ReadOnly {
		return measure.Nop{}
	}
	return s.measure.Recorder
}

// startStats builds the client metrics of the stats module (M4-31) over the Authorizer and the
// settings of the personal data of sync, and in the roles api and all the live stream (M6-15,
// startLive).
func (s *server) startStats(context.Context) error {
	s.mu.Lock()
	az, p := s.authorizer, s.personal
	s.mu.Unlock()
	if p == nil {
		return errors.New("wire: stats needs the personal data of sync")
	}
	st, err := NewStats(az, p.Service.Settings())
	if err != nil {
		return err
	}
	if s.api() {
		if err := s.startLive(st); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.stats = st
	s.mu.Unlock()
	return nil
}

// startStatistics builds the statistics queries (M6-05) over the pool and the Authorizer, with the
// clock of the process for the cache, and the views over them (M6-14) with the backend source of
// agents and the channel places of chat, and adds their handlers to the stats module of the chat
// step.
func (s *server) startStatistics(context.Context) error {
	cfg, err := StatsConfig(s.values)
	if err != nil {
		return err
	}
	bcfg, err := BillingConfig(s.values)
	if err != nil {
		return err
	}
	vcfg, err := StatsViewsConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, az, st, ag, ch := s.pool, s.authorizer, s.stats, s.agents, s.chat
	s.mu.Unlock()
	if st == nil || ag == nil || ch == nil {
		return errors.New("wire: the statistics need the stats module of the chat step, agents and chat")
	}
	q, err := NewStatistics(cfg, bcfg, pool, az, s.deps.Clock)
	if err != nil {
		return err
	}
	v, err := NewStatsViews(vcfg, StatsViewsDeps{Statistics: q, Agents: ag.Service, Places: ch.Service, Pool: pool, Clock: s.deps.Clock})
	if err != nil {
		return err
	}
	s.mu.Lock()
	st.Statistics, st.StatisticsHandlers = q, statshttp.NewStatistics(q)
	st.Views, st.ViewsHandlers = v, statshttp.NewViews(v)
	s.mu.Unlock()
	return nil
}

// startLive builds the live stream of the node over runs, chat, agents and the pool (M6-15),
// its handler of getStatsLive, and runs its builder loop with the stop step stats_live;
// Deps.StatsLive, when set, receives it. The gateway mount binds the hub to it as its sink and
// its connection source.
func (s *server) startLive(st *Stats) error {
	cfg, err := StatsLiveConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	az, rn, ch, ag, pool := s.authorizer, s.runs, s.chat, s.agents, s.pool
	s.mu.Unlock()
	if rn == nil || ch == nil || ag == nil || ag.Presence == nil {
		return errors.New("wire: the live stream needs runs, chat, agents and the presence of the agents")
	}
	live, err := NewStatsLive(cfg, StatsLiveDeps{Authorizer: az, Runs: rn.Service, Places: ch.Service, Facts: ag.Service, Presence: ag.Presence,
		Pool: pool, Clock: s.deps.Clock, Log: s.logger()})
	if err != nil {
		return err
	}
	st.Live, st.LiveHandler = live, statshttp.NewLive(live)
	s.runner.OnStop(startLiveLoop(live))
	if s.deps.StatsLive != nil {
		s.deps.StatsLive(live)
	}
	return nil
}
