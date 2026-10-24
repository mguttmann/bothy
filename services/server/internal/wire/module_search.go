// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
)

// searchState is the part of the server state the search module fills.
type searchState struct {
	// search is the search coordinator (M3-14) with its REST handlers, built in the files step
	// after files, because it reads through messaging, chat and files.
	searchModule *Search
}

// moduleSearch is the name of the module in architecture/modules.yaml.
const moduleSearch = "search"

// searchWiring wires the search coordinator into the start step files, after the files module.
func searchWiring() moduleWiring {
	return moduleWiring{
		module:   moduleSearch,
		parts:    []part{{order: 1410, step: moduleFiles, start: (*server).startSearch}},
		handlers: func(s *server, h *Handlers) { s.searchModule.fillHandlers(h) },
	}
}

// startSearch builds the search coordinator (M3-14) over the pool, the unit of work of the audit
// step, the Authorizer, the clock and the read operations of messaging, chat and files.
func (s *server) startSearch(context.Context) error {
	cfg, err := SearchConfig(s.values)
	if err != nil {
		return err
	}
	langs, err := SearchLanguages(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, m, c, f := s.pool, s.audit, s.authorizer, s.messaging, s.chat, s.files
	s.mu.Unlock()
	if a == nil || az == nil || m == nil || c == nil || f == nil {
		return errors.New("wire: search needs audit, the Authorizer, messaging, chat and files")
	}
	sr, err := NewSearch(cfg, langs.Languages(), pool, a.UoW(), az, SearchReads{
		Messaging: m.Service, Chat: c.Service, Files: f.Service,
	}, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.searchModule = sr
	s.mu.Unlock()
	return nil
}
