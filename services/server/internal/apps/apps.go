// SPDX-License-Identifier: Apache-2.0

package apps

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The states of an installation (app_installations.state, schema AppInstallation).
const (
	StateActive      = "active"
	StateSuspended   = "suspended"
	StateUninstalled = "uninstalled"
)

// App is an app as the API shows it (schema App): the app principal, its manifest and its latest
// installation.
type App struct {
	// ID is the principal id of the app.
	ID ids.ID
	// Slug is apps.slug, the handle of the principal.
	Slug string
	// DisplayName is the display name of the principal.
	DisplayName string
	// Manifest is the stored manifest (apps.manifest) as JSON.
	Manifest []byte
	// ManifestVersion is the SemVer of the manifest.
	ManifestVersion string
	// Installation is the latest installation of the app.
	Installation Installation
}

// Installation is an installation as the API shows it (schema AppInstallation).
type Installation struct {
	ID ids.ID
	// GrantedScopes and EventSubscriptions are sorted and never nil.
	GrantedScopes, EventSubscriptions []string
	// State is StateActive, StateSuspended or StateUninstalled.
	State string
	// InstalledBy is the owner who installed the app.
	InstalledBy ids.ID
	InstalledAt time.Time
}

// InstallCmd is the body of installApp and upgradeAppManifest (schema AppInstall).
type InstallCmd struct {
	// Manifest is the member manifest exactly as received: its SHA-256 is apps.manifest_sha256.
	Manifest []byte
	// GrantedScopes is the consent of the owner.
	GrantedScopes []string
	// EventSubscriptions are the subscribed events; EventSubscriptionsSet false means the member
	// is absent (install: none; upgrade: the current ones that stay allowed).
	EventSubscriptions    []string
	EventSubscriptionsSet bool
	// Reason is the mandatory reason of the audit entry.
	Reason string
}

// UpgradeCmd is the body of upgradeAppManifest; it has the members of InstallCmd.
type UpgradeCmd = InstallCmd

// SetScopesCmd is the body of setAppScopes (schema AppScopesPut).
type SetScopesCmd struct {
	GrantedScopes         []string
	EventSubscriptions    []string
	EventSubscriptionsSet bool
	Reason                string
}

// FieldError is one rule error of a manifest (schema FieldError): a JSON Pointer relative to the
// manifest root, a code and a short English detail that never repeats the submitted value.
type FieldError struct {
	Pointer, Code, Detail string
}

// ManifestValidation is the answer of validateManifest (schema ManifestValidation).
type ManifestValidation struct {
	Valid bool
	// Errors is empty, never nil, when Valid is true.
	Errors []FieldError
}

// Page is one page of apps, ordered by lower(slug), then id; NextCursor is "" on the last page.
type Page struct {
	Items      []App
	NextCursor string
}

// Apps is the incoming port of the module apps (05 section 4.11, M2-24): the nine app operations.
// Every method takes the caller and authorizes it first; the writing ones are owner only with
// step-up, run in one unit of work declared as a rights change, write their allow audit entry in
// it and raise the rights version, unless the call is without effect (06 section 2), which is
// decided before and audited with no_change. Errors are kernel/errs errors: 400 validation_failed
// or webhook_host_not_allowed, 401 step_up_required or unauthenticated, 403 forbidden owner_only,
// 404 not_found, 409 conflict or invalid_state_transition.
type Apps interface {
	// List returns the apps whose latest installation is active or suspended (app.read on the
	// collection of apps: a non-owner gets 403 owner_only). Errors: cursor_invalid.
	List(ctx context.Context, a actor.Actor, cursor string, limit int) (Page, error)
	// Get returns any app with its latest installation, uninstalled included (app.read on the
	// app: a non-owner gets the 404 of an unknown id).
	Get(ctx context.Context, a actor.Actor, appID ids.ID) (App, error)
	// Validate checks a manifest (the request body as received) without storing anything or
	// writing an audit entry (app.read on the collection of apps).
	Validate(ctx context.Context, a actor.Actor, manifest []byte) (ManifestValidation, error)
	// Install installs an app from its manifest (app.install): principal, app and installation
	// in one transaction; no token and no grant.
	Install(ctx context.Context, a actor.Actor, cmd InstallCmd) (App, error)
	// Upgrade installs a new manifest version of the app (app.install): new scopes and events
	// stay inactive until SetScopes; what the new manifest drops acts at once.
	Upgrade(ctx context.Context, a actor.Actor, appID ids.ID, cmd UpgradeCmd) (App, error)
	// SetScopes changes the granted scopes and the event subscriptions (app.scopes.update).
	SetScopes(ctx context.Context, a actor.Actor, appID ids.ID, cmd SetScopesCmd) (App, error)
	// Suspend stops the app without revoking anything (app.suspend).
	Suspend(ctx context.Context, a actor.Actor, appID ids.ID) error
	// Resume restarts a suspended app (app.resume).
	Resume(ctx context.Context, a actor.Actor, appID ids.ID) error
	// Uninstall revokes the tokens and grants of the app and disables its principal
	// (app.uninstall); its messages keep their author.
	Uninstall(ctx context.Context, a actor.Actor, appID ids.ID, reason string) error
}
