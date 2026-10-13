// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The answers of the host commands that cmd/bothy maps to exit codes (02 section 3.3, brief
// M1-37); errors.Is matches them through any wrapping. They never reach an HTTP response.
var (
	// ErrSetupCompleted: bootstrap-owner found setup_completed_at set and changed nothing (exit 3).
	ErrSetupCompleted = errors.New("setup is already complete")
	// ErrSetupPending: issue-enrollment-code before the setup is complete (exit 4).
	ErrSetupPending = errors.New(`setup is not complete; run "bothy setup bootstrap-owner" first`)
	// ErrNotActiveOwner: issue-enrollment-code for a handle that is no active owner (exit 2).
	ErrNotActiveOwner = errors.New("not an active owner")
)

// BootstrapOwnerCmd is the input of bothy setup bootstrap-owner.
type BootstrapOwnerCmd struct {
	// Handle and DisplayName are required and validated like createUser (M1-26).
	Handle, DisplayName string
	// Locale is a BCP 47 tag, "" for de; TimeZone an IANA name, "" for UTC.
	Locale, TimeZone string
}

// BootstrapResult is the outcome of the bootstrap: the first owner, its code and the local
// provider.
type BootstrapResult struct {
	OwnerID    ids.ID
	ProviderID ids.ID
	Code       IssuedEnrollmentCode
}

// The values of the bootstrap (08 section 4, brief M1-37).
const (
	bootstrapReason    = "bootstrap"
	localProviderName  = "Local accounts"
	defaultSetupLocale = "de"
	defaultSetupZone   = "UTC"
	resourceInstance   = "instance"
	attrOwnerID        = "owner_principal_id"
	attrLocalProvider  = "local_provider_id"
)

// BootstrapOwner creates the first owner (08 "Erstinstallation", brief M1-37): after validating
// the input and creating the missing signing keys (instance.Keys.EnsureActive, its own
// transactions), one unit of work declared as the rights change instance.bootstrap for the new
// person locks the instance row (setup complete: ErrSetupCompleted, nothing changed), reuses or
// creates the local identity provider, creates principal, person, owner row (granted_by NULL,
// reason bootstrap) and local identity (subject = handle), issues a first_login code, records the
// allow entry of instance.bootstrap without actor and sets setup_completed_at. The caller has no
// actor: it is the host. Errors: validation_failed or conflict for the input, ErrSetupCompleted,
// the errors of the stores and the signing keys.
func (s *Service) BootstrapOwner(ctx context.Context, cmd BootstrapOwnerCmd) (BootstrapResult, error) {
	u, err := s.bootstrapUser(cmd)
	if err != nil {
		return BootstrapResult{}, err
	}
	if err := s.d.Keys.EnsureActive(ctx); err != nil {
		return BootstrapResult{}, err
	}
	id := u.Principal.ID
	var out BootstrapResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.bootstrap(ctx, u)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionInstanceBootstrap), Resource: txn.Ref{Kind: resourceInstance}, Principals: []ids.ID{id},
	}))
	return out, err
}

// bootstrapUser validates the input into the new active person.
func (s *Service) bootstrapUser(cmd BootstrapOwnerCmd) (domain.User, error) {
	h, err := domain.NewHandle(cmd.Handle, s.cfg.HandleMaxChars)
	if err != nil {
		return domain.User{}, err
	}
	d, err := domain.NewDisplayName(cmd.DisplayName, s.cfg.DisplayNameMaxChars)
	if err != nil {
		return domain.User{}, err
	}
	locale, zone := cmd.Locale, cmd.TimeZone
	if locale == "" {
		locale = defaultSetupLocale
	}
	if zone == "" {
		zone = defaultSetupZone
	}
	if err := domain.CheckLocaleLength(locale); err != nil {
		return domain.User{}, err
	}
	canonical, ok := s.d.Locales.Canonical(locale)
	if !ok {
		return domain.User{}, domain.InvalidLocale()
	}
	if err := domain.CheckTimeZone(zone); err != nil {
		return domain.User{}, err
	}
	p, err := domain.NewPrincipal(ids.New(), domain.KindUser, h, d)
	if err != nil {
		return domain.User{}, err
	}
	return domain.NewUser(p, "", canonical, zone)
}

// bootstrap is the unit of work of BootstrapOwner, steps 1 to 6 of 08 section 4.
func (s *Service) bootstrap(ctx context.Context, u domain.User) (BootstrapResult, error) {
	rec, err := s.d.Instance.LockForSetup(ctx)
	if err != nil {
		return BootstrapResult{}, err
	}
	if rec.SetupCompleted() {
		return BootstrapResult{}, ErrSetupCompleted
	}
	prov, err := s.localProvider(ctx)
	if err != nil {
		return BootstrapResult{}, err
	}
	if err := s.insertOwner(ctx, u, prov.ID); err != nil {
		return BootstrapResult{}, err
	}
	code, err := s.d.Enrollment.Issue(ctx, u.Principal.ID, PurposeFirstLogin, nil)
	if err != nil {
		return BootstrapResult{}, err
	}
	err = s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionInstanceBootstrap), Decision: auditing.DecisionAllow, ResourceKind: resourceInstance,
		Attrs: auditing.Attrs{
			attrOwnerID: auditing.ID(u.Principal.ID), attrEnrollmentCodeID: auditing.ID(code.ID), attrLocalProvider: auditing.ID(prov.ID),
		},
	})
	if err != nil {
		return BootstrapResult{}, err
	}
	if err := s.d.Instance.CompleteSetup(ctx, s.d.Clock.Now()); err != nil {
		return BootstrapResult{}, err
	}
	return BootstrapResult{OwnerID: u.Principal.ID, ProviderID: prov.ID, Code: code}, nil
}

// localProvider returns the local identity provider, created enabled without provisioning and
// with the empty configuration of identity_provider.local.v1 when there is none (system path, no
// authorize).
func (s *Service) localProvider(ctx context.Context) (identity.Provider, error) {
	list, err := s.d.Providers.Providers(ctx, false)
	if err != nil {
		return identity.Provider{}, err
	}
	for _, p := range list {
		if p.Kind == identity.ProviderLocal {
			return p, nil
		}
	}
	now := s.d.Clock.Now()
	return s.d.Providers.InsertProvider(ctx, identity.Provider{
		ID: ids.New(), Kind: identity.ProviderLocal, Name: localProviderName, Config: json.RawMessage(`{}`), Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	})
}

// insertOwner inserts principal, person, the first owner row and the local identity of u at the
// provider providerID (subject = handle).
func (s *Service) insertOwner(ctx context.Context, u domain.User, providerID ids.ID) error {
	if err := s.d.People.InsertPrincipal(ctx, &u.Principal); err != nil {
		return err
	}
	if err := s.d.People.InsertUser(ctx, u); err != nil {
		return err
	}
	o, err := domain.NewOwner(ids.New(), u.Principal.ID, ids.ID{}, bootstrapReason)
	if err != nil {
		return err
	}
	if err := s.d.People.InsertOwner(ctx, &o); err != nil {
		return err
	}
	_, err = s.d.Identities.InsertIdentity(ctx, identity.Identity{
		ID: ids.New(), UserID: u.Principal.ID, ProviderID: providerID, ProviderKind: identity.ProviderLocal,
		Subject: u.Principal.Handle.String(), LinkedAt: s.d.Clock.Now(),
	})
	return err
}

// IssueRecoveryCode issues a recovery code for the active owner with handle (bothy setup
// issue-enrollment-code, 08 section 4, R-ID-09): setup must be complete (ErrSetupPending), the
// handle must name a person with status active and an active owners row (ErrNotActiveOwner). The
// code has created_by NULL, and the allow entry is the system action owner.recover without actor
// on the person, details {purpose, enrollment_code_id}. Errors: those above and the store errors.
func (s *Service) IssueRecoveryCode(ctx context.Context, handle string) (IssuedEnrollmentCode, error) {
	rec, err := s.d.Instance.Load(ctx)
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	if !rec.SetupCompleted() {
		return IssuedEnrollmentCode{}, ErrSetupPending
	}
	var out IssuedEnrollmentCode
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		p, found, err := s.d.People.PrincipalByHandle(ctx, handle)
		if err != nil {
			return err
		}
		if !found || p.Kind != domain.KindUser || p.Status != domain.StatusActive {
			return ErrNotActiveOwner
		}
		if _, found, err := s.d.People.ActiveOwner(ctx, p.ID); err != nil || !found {
			if err == nil {
				err = ErrNotActiveOwner
			}
			return err
		}
		out, err = s.d.Enrollment.Issue(ctx, p.ID, PurposeRecovery, nil)
		if err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, codeEntry(authorize.ActionOwnerRecover, p.ID, out))
	})
	return out, err
}
