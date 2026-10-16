// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SecretRefs checks the syntax of a secret reference without resolving it (M1-36). internal/wire
// implements it with the parser of platform/secrets, which the domain and app may not import.
type SecretRefs = provider.SecretRefs

// ProviderKindRules are the rules of one provider kind. Validate returns the normalized
// configuration of raw or validation_failed with pointers into /config/... The login tickets
// extend the rules of their kind (M1-43 adds the loopback check of trusted_header, M1-41 uses the
// OIDC rules) by wrapping the default rules in NewProviders' map.
type ProviderKindRules interface {
	Validate(raw json.RawMessage) (json.RawMessage, error)
}

// rulesFunc adapts a validation function to ProviderKindRules.
type rulesFunc func(raw json.RawMessage) (json.RawMessage, error)

// Validate implements ProviderKindRules.
func (f rulesFunc) Validate(raw json.RawMessage) (json.RawMessage, error) { return f(raw) }

// DefaultProviderRules returns the schema rules of 08 section 2.6 for the three kinds, with refs
// checking the syntax of the OIDC secret references.
func DefaultProviderRules(refs SecretRefs) map[identity.ProviderKind]ProviderKindRules {
	return map[identity.ProviderKind]ProviderKindRules{
		identity.ProviderOIDC: rulesFunc(func(raw json.RawMessage) (json.RawMessage, error) {
			return provider.ValidateOIDC(raw, refs)
		}),
		identity.ProviderTrustedHeader: rulesFunc(provider.ValidateTrustedHeader),
		identity.ProviderLocal:         rulesFunc(provider.ValidateLocal),
	}
}

// ProviderStore is the outgoing port over identity_providers (implemented by adapters/postgres).
// Reads join the ambient transaction of ctx when there is one; writes and locks run only inside a
// unit of work. Errors are kernel/errs errors without SQL or values: a taken name is conflict at
// /name, a second trusted_header or local provider conflict at /kind.
type ProviderStore interface {
	// InsertProvider inserts p (created_at and updated_at = p.UpdatedAt) and returns the row.
	InsertProvider(ctx context.Context, p identity.Provider) (identity.Provider, error)
	// Provider returns the provider id; found is false for an unknown id.
	Provider(ctx context.Context, id ids.ID) (p identity.Provider, found bool, err error)
	// LockProvider returns the provider id and locks its row until the end of the unit of work.
	LockProvider(ctx context.Context, id ids.ID) (p identity.Provider, found bool, err error)
	// EnabledProvider returns the provider id when it is enabled. Inside a unit of work it holds a
	// share lock on the row, so a concurrent disable waits for the caller's commit.
	EnabledProvider(ctx context.Context, id ids.ID) (p identity.Provider, found bool, err error)
	// Providers returns the providers (only the enabled ones when enabledOnly) ordered by
	// lower(name), id.
	Providers(ctx context.Context, enabledOnly bool) ([]identity.Provider, error)
	// UpdateProvider writes name, config, enabled, allow_provisioning and updated_at of p.
	UpdateProvider(ctx context.Context, p identity.Provider) (identity.Provider, error)
	// DeleteProvider deletes the provider id; deleted is false when there was none. A provider an
	// identity or a claim group references is conflict with a hint to disable it instead.
	DeleteProvider(ctx context.Context, id ids.ID) (deleted bool, err error)
	// LiveSessionsOfProvider returns the sessions, ascending, that are neither revoked nor past
	// their end at now and were started with an identity of the provider id.
	LiveSessionsOfProvider(ctx context.Context, id ids.ID, now time.Time) ([]ids.ID, error)
}

// IdentityRecord is a row of user_identities with the kind and state of its provider.
type IdentityRecord struct {
	identity.Identity
	// ProviderEnabled is identity_providers.enabled of the identity's provider.
	ProviderEnabled bool
	// UnlinkedAt is unlinked_at; zero while the identity is active.
	UnlinkedAt time.Time
}

// IdentityStore is the outgoing port over user_identities and the counts of the remaining sign-in
// methods (implemented by adapters/postgres). Same conventions as ProviderStore; an active
// subject linked twice is conflict at /subject.
type IdentityStore interface {
	// InsertIdentity inserts rec and returns the row.
	InsertIdentity(ctx context.Context, rec identity.Identity) (identity.Identity, error)
	// ActiveIdentityBySubject returns the active identity of (providerID, subject). Inside a unit of
	// work it holds a share lock on the row, so a concurrent unlink waits for the caller's commit.
	ActiveIdentityBySubject(ctx context.Context, providerID ids.ID, subject string) (rec identity.Identity, found bool, err error)
	// IdentityOfUser returns the identity id of userID, active or unlinked.
	IdentityOfUser(ctx context.Context, userID, id ids.ID) (rec IdentityRecord, found bool, err error)
	// ActiveIdentitiesOfUser returns the active identities of userID ordered by linked_at, id.
	ActiveIdentitiesOfUser(ctx context.Context, userID ids.ID) ([]IdentityRecord, error)
	// LockActiveIdentitiesOfUser locks the active identities of userID in ascending id.
	LockActiveIdentitiesOfUser(ctx context.Context, userID ids.ID) ([]ids.ID, error)
	// CountOtherIdentitiesAtEnabledProviders counts the active identities of userID at enabled
	// providers, except.
	CountOtherIdentitiesAtEnabledProviders(ctx context.Context, userID, except ids.ID) (int, error)
	// CountPairingDevices counts the active devices of userID that can approve a pairing.
	CountPairingDevices(ctx context.Context, userID ids.ID) (int, error)
	// UnlinkIdentity sets unlinked_at = at on the active identity id; unlinked is false when it was
	// not active.
	UnlinkIdentity(ctx context.Context, id ids.ID, at time.Time) (unlinked bool, err error)
	// MarkIdentityLogin sets last_login_at = at; found is false for an unknown id.
	MarkIdentityLogin(ctx context.Context, id ids.ID, at time.Time) (found bool, err error)
	// LiveSessionsOfIdentity returns the sessions, ascending, that are neither revoked nor past
	// their end at now and were started with the identity id.
	LiveSessionsOfIdentity(ctx context.Context, id ids.ID, now time.Time) ([]ids.ID, error)
}
