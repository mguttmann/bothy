// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
)

// importerState is the part of the server state the importer module fills.
type importerState struct {
	// importer is the importer module (M8-03), built in the chat step over chat and identity.
	importer *Importer
}

// moduleImporter is the name of the module in architecture/modules.yaml.
const moduleImporter = "importer"

// importerWiring wires the importer module into the start step chat, after stats. The read-only
// guard of chat is built in the chat part itself (NewChat), over the importer store alone.
func importerWiring() moduleWiring {
	return moduleWiring{
		module: moduleImporter,
		parts:  []part{{order: 1380, step: moduleChat, start: (*server).startImporter}},
		handlers: func(s *server, h *Handlers) {
			if s.importer != nil {
				s.importer.fillHandlers(h)
			}
		},
	}
}

// startImporter builds the importer module (M8-03) over the pool, the unit of work and recorder of
// audit, the appender of sync, the Authorizer, the import structure of chat and the people of
// identity.
func (s *server) startImporter(context.Context) error {
	cfg, err := ImporterConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, c, id := s.pool, s.audit, s.authorizer, s.sync, s.chat, s.identity
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || c == nil || id == nil {
		return errors.New("wire: the importer needs audit, sync, the Authorizer, chat and identity")
	}
	im, err := NewImporter(cfg, pool, a.UoW(), az, ImporterDeps{Audit: a.Recorder(), Events: sy.Appender(), Structure: c.Service,
		People: id.ImportPeople}, s.deps.Clock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.importer = im
	s.mu.Unlock()
	return nil
}
