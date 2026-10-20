// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
)

// Sessions implements terminal.Sessions over the use cases of the module (M2-44); Service keeps
// the methods of terminal.Hosts, whose names List and Get the sessions share.
type Sessions struct {
	s *Service
}

// Compile-time check: the incoming port.
var _ terminal.Sessions = Sessions{}

// Sessions returns the terminal sessions of the service.
func (s *Service) Sessions() Sessions { return Sessions{s: s} }

// Open implements terminal.Sessions.
func (x Sessions) Open(ctx context.Context, p actor.Actor, cmd terminal.OpenSession) (terminal.Grant, error) {
	return x.s.openSession(ctx, p, cmd)
}

// Attach implements terminal.Sessions.
func (x Sessions) Attach(ctx context.Context, p actor.Actor, cmd terminal.AttachSession) (terminal.Grant, error) {
	return x.s.attachSession(ctx, p, cmd)
}

// List implements terminal.Sessions.
func (x Sessions) List(ctx context.Context, p actor.Actor, q terminal.ListSessions) (terminal.SessionPage, error) {
	return x.s.listSessions(ctx, p, q)
}

// Get implements terminal.Sessions.
func (x Sessions) Get(ctx context.Context, p actor.Actor, id ids.ID) (terminal.SessionView, error) {
	return x.s.getSession(ctx, p, id)
}

// Close implements terminal.Sessions.
func (x Sessions) Close(ctx context.Context, p actor.Actor, cmd terminal.CloseSession) (terminal.SessionView, error) {
	return x.s.closeSession(ctx, p, cmd)
}
