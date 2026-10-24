// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	statshttp "github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
)

// statsState is the part of the server state the stats module fills.
type statsState struct {
	// stats is the client metrics part of the stats module (M4-31), built in the chat step.
	stats *Stats
}

// moduleStats is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleStats = "stats"

// stepStatistics is the start step of the statistics queries (M6-05): after apps, whose
// statistics source it reads, and only in the roles api and all, which serve the REST operations.
const stepStatistics = "statistics"

// statsWiring wires the client metrics of the stats module into the start step chat, last, and
// the statistics queries into their own step after apps (roles api and all).
func statsWiring() moduleWiring {
	return moduleWiring{
		module: moduleStats,
		parts: []part{
			{order: 1370, step: moduleChat, start: (*server).startStats},
			{order: 1650, step: stepStatistics, when: (*server).api, start: (*server).startStatistics},
		},
		handlers: func(s *server, h *Handlers) { s.stats.fillHandlers(h) },
	}
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
