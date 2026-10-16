// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ProvidersDeps are the collaborators of the provider use cases (M1-36).
type ProvidersDeps struct {
	// Store reads and writes identity_providers.
	Store ProviderStore
	// Rules validates the configuration per kind; DefaultProviderRules gives the schemas of 08
	// section 2.6. A kind without rules cannot be created.
	Rules map[identity.ProviderKind]ProviderKindRules
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Authorizer decides identity_provider.read and identity_provider.manage.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries of identity_provider.manage.
	Audit auditing.Recorder
	// Auth revokes the sessions of a disabled provider's identities.
	Auth identity.Auth
	// Clock is the time of created_at, updated_at and the session check; nil means clock.System().
	Clock clock.Clock
}

// Providers implements identity.Providers. It is safe for concurrent use.
type Providers struct {
	nameMax int
	d       ProvidersDeps
}

// Compile-time check: the incoming port.
var _ identity.Providers = (*Providers)(nil)

// NewProviders returns the provider use cases. A provider name is at most
// principal.display_name_max_chars characters (cfg.DisplayNameMaxChars), the limit of display
// names (docs/DECISIONS.md M1-36). Errors: a missing collaborator or a limit below 1.
func NewProviders(cfg Config, d ProvidersDeps) (*Providers, error) {
	if d.Store == nil || d.Rules == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Auth == nil {
		return nil, errors.New("identity: NewProviders needs every collaborator")
	}
	if cfg.DisplayNameMaxChars < 1 {
		return nil, errors.New("identity: principal.display_name_max_chars must be at least 1")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Providers{nameMax: cfg.DisplayNameMaxChars, d: d}, nil
}

// resourceIdentityProvider is the audit resource kind of the provider actions.
const resourceIdentityProvider = "identity_provider"

// instanceResource is the resource of the provider actions (05 section 4.2).
var instanceResource = authorize.Resource{Kind: authorize.ResourceInstance}

// check authorizes act on the instance and returns the decision as an error (nil for allow).
func (s *Providers) check(ctx context.Context, a actor.Actor, act authorize.Action) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, instanceResource)
	if err != nil {
		return err
	}
	return d.Err()
}

// record writes the allow entry of identity_provider.manage for p: the provider id as resource,
// its kind and attrs, never configuration values (brief M1-36 "Design notes").
func (s *Providers) record(ctx context.Context, p identity.Provider, attrs auditing.Attrs) error {
	if attrs == nil {
		attrs = auditing.Attrs{}
	}
	attrs[attrProviderKind] = auditing.Code(string(p.Kind))
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionIdentityProviderManage), Decision: auditing.DecisionAllow,
		ResourceKind: resourceIdentityProvider, ResourceID: p.ID, Attrs: attrs,
	})
}

// List implements identity.Providers (owner only).
func (s *Providers) List(ctx context.Context, a actor.Actor) ([]identity.Provider, error) {
	if err := s.check(ctx, a, authorize.ActionIdentityProviderRead); err != nil {
		return nil, err
	}
	return s.d.Store.Providers(ctx, false)
}

// Get implements identity.Providers (owner only); an unknown id is not_found.
func (s *Providers) Get(ctx context.Context, a actor.Actor, id ids.ID) (identity.Provider, error) {
	if err := s.check(ctx, a, authorize.ActionIdentityProviderRead); err != nil {
		return identity.Provider{}, err
	}
	p, found, err := s.d.Store.Provider(ctx, id)
	if err == nil && !found {
		err = invisible()
	}
	return p, err
}

// Enabled implements identity.Providers: the login drivers call it, so it authorizes nothing.
func (s *Providers) Enabled(ctx context.Context, id ids.ID) (identity.Provider, error) {
	p, found, err := s.d.Store.EnabledProvider(ctx, id)
	if err == nil && !found {
		err = invisible()
	}
	return p, err
}

// ListForDiscovery implements identity.Providers: id, kind and name of the enabled providers.
func (s *Providers) ListForDiscovery(ctx context.Context) ([]identity.ProviderInfo, error) {
	list, err := s.d.Store.Providers(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]identity.ProviderInfo, 0, len(list))
	for _, p := range list {
		out = append(out, identity.ProviderInfo{ID: p.ID, Kind: p.Kind, Name: p.Name})
	}
	return out, nil
}

// validate returns the normalized configuration of raw for kind; a missing configuration is
// required at /config.
func (s *Providers) validate(kind identity.ProviderKind, raw json.RawMessage) (json.RawMessage, error) {
	rules, ok := s.d.Rules[kind]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed, "").
			WithFields(errs.FieldError{Pointer: "/kind", Code: provider.CodeInvalidEnum, Detail: "The kind is not one of oidc, trusted_header, local."})
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errs.New(errs.CodeValidationFailed, "").
			WithFields(errs.FieldError{Pointer: "/config", Code: provider.CodeRequired, Detail: "The configuration is required."})
	}
	return rules.Validate(raw)
}

// Create implements identity.Providers: identity_provider.manage (owner, step-up), then name and
// configuration are validated and the row inserted; a taken name or a second trusted_header or
// local provider is conflict. Audit allow with the kind.
func (s *Providers) Create(ctx context.Context, a actor.Actor, in identity.CreateProvider) (identity.Provider, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.Provider{}, err
	}
	var out identity.Provider
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionIdentityProviderManage); err != nil {
			return err
		}
		name, err := provider.NewName(in.Name, s.nameMax)
		if err != nil {
			return err
		}
		cfg, err := s.validate(in.Kind, in.Config)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		p := identity.Provider{
			ID: ids.New(), Kind: in.Kind, Name: name, Config: cfg, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
		if in.Enabled != nil {
			p.Enabled = *in.Enabled
		}
		if in.AllowProvisioning != nil {
			p.AllowProvisioning = *in.AllowProvisioning
		}
		if out, err = s.d.Store.InsertProvider(ctx, p); err != nil {
			return err
		}
		return s.record(ctx, out, nil)
	})
	return out, err
}

// Update implements identity.Providers: identity_provider.manage, the row locked, absent members
// unchanged, config validated against the stored kind, updated_at = now. A change without effect
// writes the audit entry with no_change and keeps updated_at. Disabling revokes, in the same unit
// of work, every live session started with an identity of the provider (reason admin, families
// revoked, one session.revoked each; 08 section 3.3, R-ID-12).
func (s *Providers) Update(ctx context.Context, a actor.Actor, id ids.ID, in identity.UpdateProvider) (identity.Provider, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.Provider{}, err
	}
	var out identity.Provider
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.update(ctx, a, id, in)
		return err
	})
	return out, err
}

// update is Update inside its unit of work.
func (s *Providers) update(ctx context.Context, a actor.Actor, id ids.ID, in identity.UpdateProvider) (identity.Provider, error) {
	if err := s.check(ctx, a, authorize.ActionIdentityProviderManage); err != nil {
		return identity.Provider{}, err
	}
	cur, err := s.locked(ctx, id)
	if err != nil {
		return identity.Provider{}, err
	}
	next, changed, err := s.apply(cur, in)
	if err != nil {
		return identity.Provider{}, err
	}
	if len(changed) == 0 {
		return cur, s.record(ctx, cur, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	next.UpdatedAt = s.d.Clock.Now()
	out, err := s.d.Store.UpdateProvider(ctx, next)
	if err != nil {
		return identity.Provider{}, err
	}
	attrs := auditing.Attrs{attrChangedFields: codes(changed)}
	if cur.Enabled && !next.Enabled {
		n, err := s.endSessions(ctx, a, id)
		if err != nil {
			return identity.Provider{}, err
		}
		attrs[attrSessionsRevoked] = auditing.Int(int64(n))
	}
	return out, s.record(ctx, out, attrs)
}

// endSessions revokes every live session started with an identity of the provider id (reason
// admin, by the caller) and returns their number.
func (s *Providers) endSessions(ctx context.Context, a actor.Actor, id ids.ID) (int, error) {
	sessions, err := s.d.Store.LiveSessionsOfProvider(ctx, id, s.d.Clock.Now())
	if err != nil {
		return 0, err
	}
	return len(sessions), revokeAll(ctx, s.d.Auth, sessions, identity.RevokeAdmin, a.ID)
}

// locked returns the provider id under its row lock; an unknown id is not_found.
func (s *Providers) locked(ctx context.Context, id ids.ID) (identity.Provider, error) {
	p, found, err := s.d.Store.LockProvider(ctx, id)
	if err == nil && !found {
		err = invisible()
	}
	return p, err
}

// apply returns cur with the members of in and the JSON names of the members whose value changed.
func (s *Providers) apply(cur identity.Provider, in identity.UpdateProvider) (identity.Provider, []string, error) {
	next := cur
	var changed []string
	if in.Name != nil {
		name, err := provider.NewName(*in.Name, s.nameMax)
		if err != nil {
			return identity.Provider{}, nil, err
		}
		if name != cur.Name {
			next.Name, changed = name, append(changed, "name")
		}
	}
	if in.Config != nil {
		cfg, err := s.validate(cur.Kind, *in.Config)
		if err != nil {
			return identity.Provider{}, nil, err
		}
		if !jsonEqual(cfg, cur.Config) {
			next.Config, changed = cfg, append(changed, "config")
		}
	}
	if in.Enabled != nil && *in.Enabled != cur.Enabled {
		next.Enabled, changed = *in.Enabled, append(changed, "enabled")
	}
	if in.AllowProvisioning != nil && *in.AllowProvisioning != cur.AllowProvisioning {
		next.AllowProvisioning, changed = *in.AllowProvisioning, append(changed, "allow_provisioning")
	}
	return next, changed, nil
}

// jsonEqual compares two JSON documents by value; the stored jsonb may order members differently.
func jsonEqual(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return bytes.Equal(ja, jb)
}

// Delete implements identity.Providers: identity_provider.manage, then the row is deleted (its
// login flows cascade). A provider an identity (also an unlinked one) or a claim group references
// is conflict with the hint to disable it instead; an unknown id is not_found, also on a repeated
// delete, because providers have no tombstone (06 section 7).
func (s *Providers) Delete(ctx context.Context, a actor.Actor, id ids.ID) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionIdentityProviderManage); err != nil {
			return err
		}
		p, err := s.locked(ctx, id)
		if err != nil {
			return err
		}
		deleted, err := s.d.Store.DeleteProvider(ctx, id)
		if err != nil {
			return err
		}
		if !deleted {
			return invisible()
		}
		return s.record(ctx, p, nil)
	})
}
