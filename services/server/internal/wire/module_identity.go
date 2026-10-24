// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/secretbox"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// stepSessions is the step of the session use cases (identity) and of the parts that need them:
// the acting-for binding of runs and the push registrations of notify.
const stepSessions = "sessions"

// identityState is the part of the server state the identity module fills.
type identityState struct {
	// identity is the identity module (M1-33): devices, the DPoP verifier and the REST handlers.
	identity *Identity
	// totpBox seals the TOTP secrets of local accounts with auth.totp_key_ref (M1-39), resolved in
	// the identity step and bound with the sessions.
	totpBox *secretbox.Box
}

// moduleIdentity is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleIdentity = "identity"

// identityWiring wires the identity module: its start step (6e), the sessions after the signing
// keys (M1-34) and the job identity.auth_cleanup.
func identityWiring() moduleWiring {
	return moduleWiring{
		module: moduleIdentity,
		parts: []part{
			{order: 1100, step: moduleIdentity, start: (*server).startIdentity},
			{order: 1900, step: stepSessions, start: (*server).startSessions},
		},
		handlers: func(s *server, h *Handlers) {
			if s.identity != nil {
				s.identity.fillHandlers(h)
			}
		},
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			id := s.identity
			s.mu.Unlock()
			if id == nil || id.AuthCleanup == nil {
				return errors.New("wire: the jobs step needs the sessions step")
			}
			m.AuthCleanup, m.AuthCleanupInterval = id.AuthCleanup, id.cfg.CleanupInterval
			return nil
		},
	}
}

// startIdentity is start step 6e (M1-33): the identity module over the pool, the unit of work and
// the recorder of the audit step, the appender of the sync step (M1-31) and the Authorizer of
// step 6c, with the token pepper resolved once.
func (s *server) startIdentity(ctx context.Context) error {
	cfg, err := IdentityConfig(s.values)
	if err != nil {
		return err
	}
	pepper, err := TokenPepper(ctx, s.sp, cfg.TokenPepperRef)
	if err != nil {
		return err
	}
	// The TOTP key is resolved once here: a missing key or one that is not 32 bytes stops the
	// start (M1-39), not the first password login.
	box, err := TOTPBox(ctx, s.sp, cfg.TOTPKeyRef)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, st := s.pool, s.audit, s.authorizer, s.sync, s.settings
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || st == nil {
		return errors.New("wire: the identity step needs the unit of work and recorder of audit, the appender of sync, the Authorizer and the instance settings")
	}
	fresh, err := FreshStepUpCheck(s.values)
	if err != nil {
		return err
	}
	id, err := NewIdentity(cfg, pool, a.UoW(), az, IdentityWrites{Audit: a.Recorder(), Events: sy.Appender(), Settings: st.Admin, Flags: st.Admin,
		Fresh: fresh, Apps: NewAppInstallations(pool), Search: s.search, Resolver: s.resolver()}, pepper, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	id.Start(s.runner)
	s.mu.Lock()
	s.identity, s.totpBox = id, box
	s.mu.Unlock()
	return nil
}

// startSessions is the second extension point of start step 7 (M1-34): the session part of
// identity over the signing keys of the previous step and the instance id, the issuer of access
// tokens. From here on the HTTP chain authenticates people with identity's Authenticator.
func (s *server) startSessions(context.Context) error {
	s.mu.Lock()
	id, keys, rec, admin, box := s.identity, s.signingKeys, s.instance, s.authzAdmin, s.totpBox
	s.mu.Unlock()
	if id == nil || keys == nil || admin == nil {
		return errors.New("wire: the sessions need the identity step, the signing keys and the authz admin")
	}
	if err := id.BindSessions(keys.Keys, rec.InstanceID); err != nil {
		return err
	}
	eg, err := EgressConfig(s.values)
	if err != nil {
		return err
	}
	sec, err := NewProviderSecrets(s.sp, s.values)
	if err != nil {
		return err
	}
	if err := id.BindOIDC(OIDCDeps{Egress: eg, Secrets: sec, ClaimGroups: admin.Admin, EgressDenyAll: s.deps.EgressDenyAll,
		RootCAs: s.deps.OIDCRootCAs}); err != nil {
		return err
	}
	return id.BindLocal(box, nil)
}

// authenticator returns the Authenticator of the HTTP chain: identity's after startSessions,
// actor.AnonymousOnly before (fail closed).
func (s *server) authenticator() actor.Authenticator {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.identity != nil && s.identity.Authenticator != nil {
		return s.identity.Authenticator
	}
	return actor.AnonymousOnly{}
}

// fillHandlers sets every nil identity handler of h to the one id built (M1-44/18).
func (id *Identity) fillHandlers(h *Handlers) {
	fill(&h.Devices, id.Handlers)
	fill(&h.People, id.PeopleHandlers)
	fill(&h.Avatars, id.AvatarHandlers)
	fill(&h.Sessions, id.SessionHandlers)
	fill(&h.StepUps, id.StepUpHandlers)
	fill(&h.Providers, id.ProviderHandlers)
	fill(&h.Credentials, id.CredentialHandlers)
	fill(&h.Tokens, id.TokenHandlers)
	fill(&h.Enrollment, id.EnrollmentHandlers)
	fill(&h.Approval, id.ApprovalHandlers)
	fill(&h.Passkeys, id.PasskeyHandlers)
	fill(&h.OIDC, id.OIDCHandlers)
	fill(&h.TrustedHeader, id.TrustedHeaderHandlers)
	fill(&h.Lifecycle, id.LifecycleHandlers)
	fill(&h.Local, id.LocalHandlers)
	fill(&h.Console, id.ConsoleHandlers)
}

// fill sets *dst to built when *dst is nil, so an injected handler stays.
func fill[T any](dst **T, built *T) {
	if *dst == nil {
		*dst = built
	}
}
