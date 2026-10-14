// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/api/routes"
	appshttp "github.com/OWNER/bothy/services/server/internal/apps/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/apps/adapters/identitysource"
	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// AppsConfig binds the keys of the module apps (app.manifest_max_bytes, webhook.allowed_hosts,
// the command limits, the principal limits, audit.reason_max_chars and the page sizes). The error
// names the key whose binding failed, never a value.
func AppsConfig(v *config.Values) (appsapp.Config, error) {
	var c appsapp.Config
	if err := config.Bind(v, &c); err != nil {
		return appsapp.Config{}, err
	}
	return c, nil
}

// NewAppInstallations returns identity.AppInstallations over the apps tables of pool
// (apps/adapters/identitysource, M2-24 design note 12): identity reads the installation of an app
// principal through it, so identity never imports apps.
func NewAppInstallations(pool *pg.Pool) identity.AppInstallations {
	return identitysource.New(appspg.NewRepository(pool))
}

// principalSource is the authz.PrincipalSource of identity over pool with the installations of
// apps, so an app without an active installation has an empty snapshot (M2-24 design note 11 b).
func principalSource(pool *pg.Pool) *identityapp.PrincipalSource {
	return identityapp.NewPrincipalSource(identitypg.NewRepository(pool)).WithApps(NewAppInstallations(pool))
}

// Apps is the module apps of the process (M2-24): the use cases and the REST handlers of the nine
// app operations.
type Apps struct {
	// Service implements apps.Apps.
	Service *appsapp.Service
	// Handlers are the REST handlers.
	Handlers *appshttp.Apps
}

// AppsDeps are the collaborators of the module apps from the other start steps.
type AppsDeps struct {
	// UoW is the unit of work of the audit step; Rights its rights hook (AddAffected).
	UoW    txn.UnitOfWork
	Rights authz.RightsChanges
	// Authorizer decides every call; Audit writes the allow entries.
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	// Identity provides the directory, the principal writes and the API tokens.
	Identity *Identity
	// Clock is the time of the writes; nil means clock.System().
	Clock clock.Clock
}

// NewApps builds the module apps over pool: the repository, the use cases with the ports of
// identity (directory, principal writes, tokens), the grant revocation of authz (the Admin
// service of M1-29 over the same pool), the scope table of authz and the spec version of the
// contract (routes.SpecVersion), and the REST handlers. It opens no connection. Errors: a missing
// collaborator or a value the use cases refuse.
func NewApps(cfg appsapp.Config, pool *pg.Pool, d AppsDeps) (*Apps, error) {
	if d.UoW == nil || d.Rights == nil || d.Authorizer == nil || d.Audit == nil || d.Identity == nil {
		return nil, errors.New("wire: NewApps needs the unit of work, the rights hook, the Authorizer, the recorder and identity")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	admin, err := authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: d.UoW, Authorizer: d.Authorizer, Store: authzpg.NewRepository(pool), Principals: principalSource(pool),
		Structure: NewStructureSource(pool), Hosts: NewHostSource(pool), Rights: d.Rights, Audit: d.Audit, Clock: d.Clock,
	})
	if err != nil {
		return nil, err
	}
	svc, err := appsapp.New(cfg, appsapp.Deps{
		Repo: appspg.NewRepository(pool), UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit,
		Principals: appsapp.IdentityPrincipals{Directory: d.Identity.Directory, Writes: d.Identity.PrincipalWrites},
		Tokens:     d.Identity.Tokens, Grants: admin, Rights: d.Rights, Scopes: authzapp.ScopeTable{}, Cursors: appshttp.Cursors{},
		SpecVersion: routes.SpecVersion, Clock: d.Clock,
	})
	if err != nil {
		return nil, err
	}
	return &Apps{Service: svc, Handlers: appshttp.NewApps(svc)}, nil
}
