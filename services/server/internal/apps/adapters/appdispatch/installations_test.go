// SPDX-License-Identifier: Apache-2.0

package appdispatch_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/appdispatch"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// repo is the fake apps repository.
type repo struct {
	inst    map[ids.ID]app.InstallationRecord
	apps    map[ids.ID]app.StoredApp
	readErr error
}

func (r repo) ActiveInstallation(_ context.Context, appID ids.ID) (app.InstallationRecord, bool, error) {
	if r.readErr != nil {
		return app.InstallationRecord{}, false, r.readErr
	}
	i, ok := r.inst[appID]
	return i, ok, nil
}

func (r repo) App(_ context.Context, id ids.ID) (app.StoredApp, bool, error) {
	a, ok := r.apps[id]
	return a, ok, nil
}

// TestNewInstallations: the port over the apps repository returns the active installation with
// its scopes and parsed manifest, found false without one, and internal for a broken invariant.
func TestNewInstallations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	appID, noRow, badManifest, instID := ids.New(), ids.New(), ids.New(), ids.New()
	manifest := []byte(`{"schema_version":1,"slug":"tracker","name":"Tracker","description":"Tickets","publisher":"Example",` +
		`"version":"1.0.0","min_api_version":"1","scopes":{"required":["commands:write"],"optional":[]},` +
		`"delivery":{"mode":"webhook","interactions_url":"https://tracker.example.com/hooks"}}`)
	r := repo{
		inst: map[ids.ID]app.InstallationRecord{
			appID:       {ID: instID, AppID: appID, GrantedScopes: []string{"commands:write"}, State: domain.StateActive},
			noRow:       {ID: ids.New(), AppID: noRow, State: domain.StateActive},
			badManifest: {ID: ids.New(), AppID: badManifest, State: domain.StateActive},
		},
		apps: map[ids.ID]app.StoredApp{
			appID:       {PrincipalID: appID, Manifest: manifest},
			badManifest: {PrincipalID: badManifest, Manifest: []byte(`{"unknown":true}`)},
		},
	}
	inst := appdispatch.NewInstallations(r)
	got, found, err := inst.ActiveInstallation(ctx, appID)
	if err != nil || !found || got.ID != instID || !slices.Equal(got.GrantedScopes, []string{"commands:write"}) ||
		got.Manifest.Delivery == nil || got.Manifest.Delivery.InteractionsURL != "https://tracker.example.com/hooks" {
		t.Fatalf("ActiveInstallation = %+v %v %v", got, found, err)
	}
	if _, found, err := inst.ActiveInstallation(ctx, ids.New()); err != nil || found {
		t.Fatalf("ActiveInstallation(none) = %v %v", found, err)
	}
	for _, id := range []ids.ID{noRow, badManifest} {
		if _, _, err := inst.ActiveInstallation(ctx, id); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("ActiveInstallation(broken) = %v, want internal", err)
		}
	}
	boom := errors.New("database down")
	if _, _, err := appdispatch.NewInstallations(repo{readErr: boom}).ActiveInstallation(ctx, appID); !errors.Is(err, boom) {
		t.Fatalf("ActiveInstallation(read error) = %v", err)
	}
}
