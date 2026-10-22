// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/authz"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// authzState is the part of the server state the authz module fills.
type authzState struct {
	// authorizer is the kernel/authorize.Authorizer of the process (M1-28); the use cases of later
	// tickets receive it.
	authorizer authorize.Authorizer
	// authzAdmin is the group and grant management of authz with its REST handlers (M1-48).
	authzAdmin *AuthzAdmin
}

// moduleAuthz is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleAuthz = "authz"

// authzWiring wires the authz module: its start step (6c) and the jobs authz.expiry_sweep and
// authz.expiry_reminder (M1-30).
func authzWiring() moduleWiring {
	return moduleWiring{
		module: moduleAuthz,
		parts:  []part{{order: 900, step: moduleAuthz, start: (*server).startAuthz}},
		handlers: func(s *server, h *Handlers) {
			if h.Access == nil && s.authzAdmin != nil {
				h.Access = s.authzAdmin.Handlers
			}
		},
		jobs: func(s *server, m *JobModules) error {
			expiryCfg, err := AuthzExpiryConfig(s.values)
			if err != nil {
				return err
			}
			s.mu.Lock()
			pool, a, sy := s.pool, s.audit, s.sync
			s.mu.Unlock()
			expiry, err := NewExpiryService(pool, a, sy, expiryCfg, s.logger())
			if err != nil {
				return err
			}
			m.Expiry, m.ExpiryInterval = expiry, expiryCfg.Interval
			return nil
		},
	}
}

// startAuthz is start step 6c (M1-28): it builds the Authorizer of authz over the pool with the
// recorder of the audit module (NewAuthorizer) and keeps it for the use cases of later tickets,
// binds the rights hooks to the appender of sync and the recorder (M1-29), and builds the group
// and grant management with its REST handlers (M1-48).
func (s *server) startAuthz(context.Context) error {
	cfg, err := AuthzConfig(s.values)
	if err != nil {
		return err
	}
	log := s.logger()
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, sy := s.pool, s.audit, s.sync
	s.mu.Unlock()
	az, err := NewAuthorizer(pool, cfg, cat, a.Recorder(), s.deps.Clock, log)
	if err != nil {
		return err
	}
	if sy == nil {
		return errors.New("wire: the authz step needs the event log of the sync step")
	}
	adminCfg, err := AuthzAdminConfig(s.values)
	if err != nil {
		return err
	}
	admin, err := NewAuthzAdmin(pool, adminCfg, a, sy, az, s.deps.Clock)
	if err != nil {
		return err
	}
	// The rights hooks of the unit of work need the appender and the recorder, which need the unit
	// of work themselves; they are bound here, before any route or job runs (M1-29).
	if err := a.RightsHook().Bind(authzapp.RightsBindings{Events: sy.Appender(), Audit: a.Recorder()}); err != nil {
		return err
	}
	if err := a.SyncSubscribers().Bind(sy.Appender()); err != nil {
		return err
	}
	s.mu.Lock()
	s.authorizer, s.authzAdmin = az, admin
	s.mu.Unlock()
	return nil
}

// resolver returns the guided flow of authz (M2-21) as the port identity and chat call; nil
// before startAuthz, which identity and chat then refuse with kind internal.
func (s *server) resolver() authz.I1Resolver {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.authzAdmin == nil || s.authzAdmin.Resolver == nil {
		return nil
	}
	return s.authzAdmin.Resolver
}

// authorizerOf returns the Authorizer of the process, nil before the step authz ran.
func (s *server) authorizerOf() authorize.Authorizer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authorizer
}
