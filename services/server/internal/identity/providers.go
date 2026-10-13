// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ProviderKind is identity_providers.kind, the sign-in driver (08 section 2.1, ADR 0019).
type ProviderKind string

// The three drivers. At most one trusted_header and one local provider exist per instance, any
// number of oidc providers.
const (
	// ProviderOIDC is an OpenID Connect provider (08 section 2.2).
	ProviderOIDC ProviderKind = "oidc"
	// ProviderTrustedHeader is a reverse proxy that sets the login in a header (08 section 2.4).
	ProviderTrustedHeader ProviderKind = "trusted_header"
	// ProviderLocal is the local account driver (08 section 2.5).
	ProviderLocal ProviderKind = "local"
)

// Valid reports whether k is one of the three drivers.
func (k ProviderKind) Valid() bool {
	return k == ProviderOIDC || k == ProviderTrustedHeader || k == ProviderLocal
}

// ExternalIdentity is what every driver delivers after a successful check (08 section 2.1).
// Everything after it (account, device, session, tokens) is the same for all drivers; the mapping
// to a person uses only ProviderID, Issuer and Subject (R-ID-02).
type ExternalIdentity struct {
	// ProviderID is the identity provider that checked the identity.
	ProviderID ids.ID
	// Issuer is the OIDC iss; empty for trusted_header and local.
	Issuer string
	// Subject is the OIDC sub, the header login or the local login name.
	Subject string
	// DisplayName is for display only.
	DisplayName string
	// EmailHint is for display only, never for mapping or rights (08 section 2.3).
	EmailHint string
	// Groups are the values of the group claim; only oidc with groups_claim delivers them.
	Groups []string
	// AuthTime is the time of the sign-in at the provider.
	AuthTime time.Time
	// AMR are the authentication methods the provider names, when it delivers them.
	AMR []string
}

// Provider is a configured identity provider (schema IdentityProvider). Config is the normalized
// configuration of schema identity_provider.<kind>.v1; secrets appear in it only as references.
type Provider struct {
	ID                ids.ID
	Kind              ProviderKind
	Name              string
	Config            json.RawMessage
	Enabled           bool
	AllowProvisioning bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CreateProvider is the input of Providers.Create (schema IdentityProviderCreate). Enabled nil
// means true, AllowProvisioning nil means false (03 defaults).
type CreateProvider struct {
	Kind              ProviderKind
	Name              string
	Config            json.RawMessage
	Enabled           *bool
	AllowProvisioning *bool
}

// UpdateProvider is the input of Providers.Update (schema IdentityProviderUpdate) with PATCH
// semantics: a nil field stays unchanged. The kind never changes; Config, if given, is validated
// against the stored kind.
type UpdateProvider struct {
	Name              *string
	Config            *json.RawMessage
	Enabled           *bool
	AllowProvisioning *bool
}

// ProviderInfo is an enabled provider as the discovery document names it (schema AuthProviderInfo).
type ProviderInfo struct {
	ID   ids.ID
	Kind ProviderKind
	Name string
}

// Providers manages the identity providers (05 section 4.2). List and Get authorize
// identity_provider.read, Create, Update and Delete identity_provider.manage (owner, step-up,
// audit); Enabled and ListForDiscovery authorize nothing: the login drivers and the discovery
// call them.
//
// Errors are kernel/errs errors: validation_failed with pointers into /name and /config/...,
// conflict for a taken name, a second trusted_header or local provider and a delete while
// identities or claim groups reference the provider, not_found for an unknown id, and the
// decisions of authorize.
type Providers interface {
	// List returns every provider ordered by name without regard to case.
	List(ctx context.Context, a actor.Actor) ([]Provider, error)
	// Create validates and inserts a provider.
	Create(ctx context.Context, a actor.Actor, in CreateProvider) (Provider, error)
	// Get returns the provider id.
	Get(ctx context.Context, a actor.Actor, id ids.ID) (Provider, error)
	// Update changes the given members of the provider id. Disabling ends the live sessions of
	// its identities (08 section 3.3, R-ID-12) in the same unit of work.
	Update(ctx context.Context, a actor.Actor, id ids.ID, in UpdateProvider) (Provider, error)
	// Delete removes a provider no identity references.
	Delete(ctx context.Context, a actor.Actor, id ids.ID) error
	// Enabled returns the provider id when it is enabled and not_found otherwise.
	Enabled(ctx context.Context, id ids.ID) (Provider, error)
	// ListForDiscovery returns the enabled providers ordered by name without regard to case.
	ListForDiscovery(ctx context.Context) ([]ProviderInfo, error)
}

// ResolutionOutcome is the result of mapping an external identity (08 section 3.1).
type ResolutionOutcome string

// The two outcomes. The login tickets apply steps 2 to 4 of 08 section 3.1 to unmatched.
const (
	// ResolutionMatched found an active identity at an enabled provider.
	ResolutionMatched ResolutionOutcome = "matched"
	// ResolutionUnmatched found none.
	ResolutionUnmatched ResolutionOutcome = "unmatched"
)

// Resolution is the result of Identities.Resolve; UserID and IdentityID are set for matched only.
type Resolution struct {
	Outcome    ResolutionOutcome
	UserID     ids.ID
	IdentityID ids.ID
}

// Identity is a linked identity of a person (schema Identity). Zero values stand for absent
// members.
type Identity struct {
	ID           ids.ID
	UserID       ids.ID
	ProviderID   ids.ID
	ProviderKind ProviderKind
	Subject      string
	Issuer       string
	EmailAtLink  string
	LinkedAt     time.Time
	LastLoginAt  time.Time
}

// Passkey is a passkey of a person (schema Passkey); M1-40 fills it.
type Passkey struct {
	ID             ids.ID
	Name           string
	AAGUID         ids.ID
	BackupEligible bool
	CreatedAt      time.Time
	LastUsedAt     time.Time
}

// CredentialOverview lists the sign-in methods of a person without secrets (schema
// CredentialOverview). Identities and Passkeys are never nil.
type CredentialOverview struct {
	Identities       []Identity
	Passkeys         []Passkey
	LocalPasswordSet bool
	TOTPEnrolled     bool
}

// Identities maps external identities to people and manages the links (08 sections 3.1 and 3.3).
// Resolve, Link and MarkLogin authorize nothing and join the ambient unit of work of the calling
// login flow; Credentials authorizes identity.read and Unlink identity.unlink.
type Identities interface {
	// Resolve maps ext to a person only through (provider_id, subject) of an active identity at an
	// enabled provider, for oidc also the issuer; never through e-mail or display name (R-ID-02).
	Resolve(ctx context.Context, ext ExternalIdentity) (Resolution, error)
	// Link links ext to the person userID. Errors: not_found for a provider that is not enabled,
	// conflict for a subject that is already linked. The caller writes the identity.link audit.
	Link(ctx context.Context, userID ids.ID, ext ExternalIdentity) (Identity, error)
	// MarkLogin sets last_login_at of the identity to at.
	MarkLogin(ctx context.Context, identityID ids.ID, at time.Time) error
	// Credentials returns the sign-in methods of userID (self or owner, others not_found).
	Credentials(ctx context.Context, a actor.Actor, userID ids.ID) (CredentialOverview, error)
	// Unlink unlinks identityID of userID unless no sign-in method would remain (conflict with
	// reason no_login_method_left) and ends the sessions started with it.
	Unlink(ctx context.Context, a actor.Actor, userID, identityID ids.ID) error
}
