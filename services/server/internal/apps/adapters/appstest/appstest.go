// SPDX-License-Identifier: Apache-2.0

// Package appstest installs apps through the real use cases for the tests of M2-24 to M2-28 (02
// section 6: only _test.go files import it). It knows the ports only: the caller builds the module
// and passes an owner with a fresh step-up.
//
// Example:
//
//	got := appstest.Install(t, svc, tokens, owner, appstest.Manifest("tracker", "1.0.0", "messages:write"), time.Now().Add(time.Hour))
//	// got.Token is a Bearer app token with the granted scopes of the installation
package appstest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Installed is an app installed through the real use case with one API token.
type Installed struct {
	// AppID is the principal id of the app; InstallationID its installation.
	AppID, InstallationID ids.ID
	// TokenID and Token are the API token (shown once; a test secret, never real).
	TokenID ids.ID
	Token   string
	// Scopes are the granted scopes of the installation.
	Scopes []string
}

// Manifest returns a minimal valid app_manifest.v1 named slug at version with the required
// scopes; it names no event, command or address, so it needs no webhook.allowed_hosts.
func Manifest(slug, version string, required ...string) []byte {
	m := map[string]any{
		"schema_version": 1, "slug": slug, "name": "App " + slug, "description": "Test app " + slug, "publisher": "Example Org",
		"version": version, "min_api_version": "0.1",
		"scopes": map[string]any{"required": append([]string{}, required...), "optional": []string{}},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

// Install installs manifest with every required scope granted and no subscription, then creates
// one unbound app token valid until expires (createToken, M2-24 design note 10). owner must be an
// owner with a fresh step-up. It fails t on any error.
func Install(t testing.TB, svc apps.Apps, tokens identity.TokenAdmin, owner actor.Actor, manifest []byte, expires time.Time) Installed {
	t.Helper()
	var m struct {
		Scopes struct {
			Required []string `json:"required"`
		} `json:"scopes"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatalf("appstest: manifest: %v", err)
	}
	ctx := actor.WithActor(context.Background(), owner)
	a, err := svc.Install(ctx, owner, apps.InstallCmd{Manifest: manifest, GrantedScopes: m.Scopes.Required, Reason: "Test installation"})
	if err != nil {
		t.Fatalf("appstest: install: %v", err)
	}
	issued, err := tokens.CreateToken(ctx, owner, a.ID, identity.CreateTokenCmd{Name: "Test token", ExpiresAt: expires})
	if err != nil {
		t.Fatalf("appstest: token: %v", err)
	}
	return Installed{
		AppID: a.ID, InstallationID: a.Installation.ID, TokenID: issued.Token.ID, Token: issued.Secret.Reveal(),
		Scopes: a.Installation.GrantedScopes,
	}
}
