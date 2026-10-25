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
// with the stop step stats_flush (Measure.Start); Deps.StatsMeasure, when set, receives it.
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
	me.Start(s.runner)
	s.mu.Lock()
	s.measure = me
	s.mu.Unlock()
	if s.deps.StatsMeasure != nil {
		s.deps.StatsMeasure(me)
	}
	return nil
}

// measureRecorder returns the recorder of the own measurement for the modules; before the step
// stats_measure, or in a process built without it, it records nothing.
func (s *server) measureRecorder() measure.Recorder {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.measure == nil {
		return measure.Nop{}
	}
	return s.measure.Recorder
}

// startStats builds the client metrics of the stats module (M4-31) over the Authorizer and the
// settings of the personal data of sync.
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
	s.mu.Lock()
	s.stats = st
	s.mu.Unlock()
	return nil
}

// startStatistics builds the statistics queries (M6-05) over the pool and the Authorizer, with the
// clock of the process for the cache, and adds their handlers to the stats module of the chat step.
func (s *server) startStatistics(context.Context) error {
	cfg, err := StatsConfig(s.values)
	if err != nil {
		return err
	}
	bcfg, err := BillingConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, az, st := s.pool, s.authorizer, s.stats
	s.mu.Unlock()
	if st == nil {
		return errors.New("wire: the statistics need the stats module of the chat step")
	}
	q, err := NewStatistics(cfg, bcfg, pool, az, s.deps.Clock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	st.Statistics, st.StatisticsHandlers = q, statshttp.NewStatistics(q)
	s.mu.Unlock()
	return nil
}
