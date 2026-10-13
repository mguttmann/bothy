// SPDX-License-Identifier: Apache-2.0

package loginoptions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/loginoptions"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// lister returns fixed providers and records whether only enabled ones were asked for.
type lister struct {
	list        []identity.Provider
	err         error
	enabledOnly bool
}

func (l *lister) Providers(_ context.Context, enabledOnly bool) ([]identity.Provider, error) {
	l.enabledOnly = enabledOnly
	return l.list, l.err
}

func TestLoginOptions_EnabledProvidersAndSwitch(t *testing.T) {
	t.Parallel()
	if _, err := loginoptions.New(nil, true); err == nil {
		t.Fatal("New without a store did not fail")
	}
	id := ids.New()
	l := &lister{list: []identity.Provider{{ID: id, Kind: identity.ProviderLocal, Name: "Local", Enabled: true}}}
	src, err := loginoptions.New(l, true)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := src.LoginOptions(context.Background())
	if err != nil || !l.enabledOnly || !opts.LocalPasswordEnabled || len(opts.Providers) != 1 ||
		opts.Providers[0].ID != id || opts.Providers[0].Kind != "local" || opts.Providers[0].Name != "Local" {
		t.Fatalf("options %+v %v", opts, err)
	}
	l.err = errors.New("down")
	if _, err := src.LoginOptions(context.Background()); !errors.Is(err, l.err) {
		t.Fatalf("error %v", err)
	}
}
