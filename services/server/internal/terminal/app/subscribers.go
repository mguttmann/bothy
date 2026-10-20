// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Subscribers are the in-transaction subscribers of the terminal module (M2-46, 02 section 4.3):
// authorize.AccessChanged, identity.DeviceRevoked and identity.SessionEnded. internal/wire builds
// them with the txn registry, before the module exists, and binds the use cases in the terminal
// step (Bind); every role builds the module, so the server always runs them bound. A process that
// never binds them (migrate, setup) has no terminal sessions to end: an unbound subscriber that
// finds an open session it would have to end fails the unit (fail closed), else it does nothing.
type Subscribers struct {
	store Store
	svc   atomic.Pointer[Service]
}

// NewSubscribers returns unbound subscribers that read the open sessions through store (nil in a
// registry that runs no unit of work, the golden test).
func NewSubscribers(store Store) *Subscribers { return &Subscribers{store: store} }

// Bind binds the use cases of the module; internal/wire calls it once in the terminal step.
func (s *Subscribers) Bind(svc *Service) { s.svc.Store(svc) }

// OnAccessChanged implements the subscription kernel.AccessChanged -> terminal (Service.OnAccessChanged).
func (s *Subscribers) OnAccessChanged(ctx context.Context, ev authorize.AccessChanged) error {
	if svc := s.svc.Load(); svc != nil {
		return svc.OnAccessChanged(ctx, ev)
	}
	users := make([]ids.ID, 0, len(ev.Changes))
	for _, c := range ev.Changes {
		users = append(users, c.PrincipalID)
	}
	return s.unbound(ctx, func(ctx context.Context) ([]domain.Session, error) { return s.store.LockOpenSessionsOfUsers(ctx, users) })
}

// OnDeviceRevoked implements the subscription identity.DeviceRevoked -> terminal (Service.OnDeviceRevoked).
func (s *Subscribers) OnDeviceRevoked(ctx context.Context, ev identity.DeviceRevoked) error {
	if svc := s.svc.Load(); svc != nil {
		return svc.OnDeviceRevoked(ctx, ev)
	}
	return s.unbound(ctx, func(ctx context.Context) ([]domain.Session, error) { return s.store.LockOpenSessionsOfDevice(ctx, ev.DeviceID) })
}

// OnSessionEnded implements the subscription identity.SessionEnded -> terminal (Service.OnSessionEnded).
func (s *Subscribers) OnSessionEnded(ctx context.Context, ev identity.SessionEnded) error {
	if svc := s.svc.Load(); svc != nil {
		return svc.OnSessionEnded(ctx, ev)
	}
	return s.unbound(ctx, func(ctx context.Context) ([]domain.Session, error) {
		return s.store.LockOpenSessionsOfAuthSessions(ctx, ev.SessionIDs)
	})
}

// unbound is a subscriber before Bind: nothing to do without open sessions, an internal error
// with one (the revocation must not commit while a session it ends stays open).
func (s *Subscribers) unbound(ctx context.Context, open func(context.Context) ([]domain.Session, error)) error {
	if s.store == nil {
		return errs.New(errs.CodeInternalError, "terminal: the subscribers are not bound")
	}
	list, err := open(ctx)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return errs.New(errs.CodeInternalError, "terminal: the subscribers are not bound")
	}
	return nil
}
