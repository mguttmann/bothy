// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StoredApp is a row of apps.
type StoredApp struct {
	PrincipalID ids.ID
	Slug        string
	// Manifest is the stored manifest as JSON (jsonb).
	Manifest        []byte
	ManifestVersion string
	// ManifestSHA256 is the SHA-256 of the manifest bytes as received.
	ManifestSHA256       []byte
	Publisher            string
	HomepageURL          string
	CreatedAt, UpdatedAt time.Time
}

// InstallationRecord is a row of app_installations.
type InstallationRecord struct {
	ID, AppID ids.ID
	// GrantedScopes and EventSubscriptions are sorted.
	GrantedScopes, EventSubscriptions []string
	State                             domain.State
	InstalledBy                       ids.ID
	InstalledAt                       time.Time
	// UninstalledAt is zero and UninstalledBy the zero id until the uninstall.
	UninstalledAt time.Time
	UninstalledBy ids.ID
}

// ListedApp is one row of the app list: the app with its live installation.
type ListedApp struct {
	App          StoredApp
	Installation InstallationRecord
}

// Repository is the outgoing port of the use cases over apps and app_installations
// (adapters/postgres). Reads run in the ambient transaction of ctx or on the pool; writes and
// locks only inside a unit of work (else internal). Errors are kernel/errs errors without SQL: a
// unique violation is conflict, anything else internal or unavailable.
type Repository interface {
	// App returns the app id; found is false for an unknown id.
	App(ctx context.Context, id ids.ID) (StoredApp, bool, error)
	// AppBySlug returns the app whose slug equals slug without regard to case.
	AppBySlug(ctx context.Context, slug string) (StoredApp, bool, error)
	// LockAppBySlug is AppBySlug with FOR UPDATE.
	LockAppBySlug(ctx context.Context, slug string) (StoredApp, bool, error)
	// LockApp is App with FOR UPDATE.
	LockApp(ctx context.Context, id ids.ID) (StoredApp, bool, error)
	// InsertApp inserts r and returns it with the times of the database.
	InsertApp(ctx context.Context, r StoredApp) (StoredApp, error)
	// UpdateApp replaces the manifest columns of r and sets updated_at to at.
	UpdateApp(ctx context.Context, r StoredApp, at time.Time) (StoredApp, error)
	// LatestInstallation returns the newest installation of appID (installed_at, then id).
	LatestInstallation(ctx context.Context, appID ids.ID) (InstallationRecord, bool, error)
	// LockLiveInstallation returns the installation of appID that is not uninstalled, FOR UPDATE.
	LockLiveInstallation(ctx context.Context, appID ids.ID) (InstallationRecord, bool, error)
	// ActiveInstallation returns the installation of appID with state active.
	ActiveInstallation(ctx context.Context, appID ids.ID) (InstallationRecord, bool, error)
	// ShareActiveInstallation is ActiveInstallation with FOR SHARE.
	ShareActiveInstallation(ctx context.Context, appID ids.ID) (InstallationRecord, bool, error)
	// InsertInstallation inserts r and returns it with installed_at of the database.
	InsertInstallation(ctx context.Context, r InstallationRecord) (InstallationRecord, error)
	// UpdateInstallationGrants replaces granted_scopes and event_subscriptions of id.
	UpdateInstallationGrants(ctx context.Context, id ids.ID, g domain.Grants) error
	// SetInstallationState sets the state of id; for uninstalled also uninstalled_at and
	// uninstalled_by.
	SetInstallationState(ctx context.Context, id ids.ID, state domain.State, at time.Time, by ids.ID) error
	// ListLive returns up to limit apps with a live installation after the keyset position
	// (lower(slug), principal_id) of afterSlug and afterID (afterSlug "" for the first page).
	ListLive(ctx context.Context, afterSlug string, afterID ids.ID, limit int) ([]ListedApp, error)
}

// Principals is the consumer port of identity for the app principal (identity.Directory and
// identity.PrincipalWrites).
type Principals interface {
	// Principal returns the principal id. Errors: not_found.
	Principal(ctx context.Context, id ids.ID) (PrincipalFacts, error)
	// PrincipalsByID returns the display names of the known principals among list.
	DisplayNames(ctx context.Context, list []ids.ID) (map[ids.ID]string, error)
	// HandleTaken reports a principal whose handle equals handle without regard to case.
	HandleTaken(ctx context.Context, handle string) (bool, error)
	// CreateApp creates the principal of kind app with status active.
	CreateApp(ctx context.Context, id ids.ID, handle, displayName string) error
	// SetActive sets the principal active (true) or disabled (false).
	SetActive(ctx context.Context, id ids.ID, active bool) error
	// Rename sets the display name.
	Rename(ctx context.Context, id ids.ID, displayName string) error
}

// PrincipalFacts are the facts of a principal the use cases read.
type PrincipalFacts struct {
	Kind        actor.Kind
	DisplayName string
}

// Tokens is the consumer port of identity for the API tokens of an app.
type Tokens interface {
	// RevokeAllForPrincipal revokes every active token of principalID inside the caller's rights
	// change and returns their number.
	RevokeAllForPrincipal(ctx context.Context, principalID, by ids.ID) (int, error)
}

// Grants is the consumer port of authz for the grants of an app (authz.Admin).
type Grants interface {
	// RevokeSubjectGrants revokes every active grant of subjectID inside the caller's rights
	// change and returns their number.
	RevokeSubjectGrants(ctx context.Context, caller actor.Actor, subjectID ids.ID, reason string) (int, error)
}

// Rights is the consumer port of authz for a rights change (authz.RightsChanges).
type Rights interface {
	// AddAffected names principals the change learned after its declaration.
	AddAffected(ctx context.Context, principals ...ids.ID) error
}

// Scopes is the consumer port of authz over the scope table (authz.AppScopes).
type Scopes interface {
	EventScope(t authorize.EventType) (scope string, subscribable bool)
	Scopes() []string
}

// Cursors encodes and checks the cursors of listApps (httpserver.EncodeCursor and DecodeCursor).
type Cursors interface {
	Encode(list string, binding []byte, key any) (string, error)
	Decode(list string, binding []byte, s string, key any) error
}

// Commands is the consumer port of agents for the commands of a manifest (agents.OwnedCommands,
// M2-25 design note 11): the owner action decided and audited here, so the methods authorize
// nothing and write no audit entry of their own.
type Commands interface {
	UpsertOwned(ctx context.Context, ownerID ids.ID, def agents.RegisterCommand) (c agents.Command, changed bool, err error)
	RemoveOwned(ctx context.Context, ownerID ids.ID, names []string) ([]ids.ID, error)
	RemoveAllOwned(ctx context.Context, ownerID ids.ID) ([]ids.ID, error)
}

// Components is the consumer port of runs for the components of an app's cards
// (runs.Interactions.DisableAuthoredBy, M2-25 design note 11): app.suspend and app.uninstall
// disable them; the messages stay.
type Components interface {
	DisableAuthoredBy(ctx context.Context, authorID ids.ID) (int, error)
}
