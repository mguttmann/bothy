// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
)

// statsState is the part of the server state the stats module fills.
type statsState struct {
	// stats is the client metrics part of the stats module (M4-31), built in the chat step.
	stats *Stats
}

// moduleStats is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleStats = "stats"

// statsWiring wires the client metrics of the stats module into the start step chat, last.
func statsWiring() moduleWiring {
	return moduleWiring{
		module:   moduleStats,
		parts:    []part{{order: 1370, step: moduleChat, start: (*server).startStats}},
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
