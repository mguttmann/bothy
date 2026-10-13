// SPDX-License-Identifier: Apache-2.0

// Package loginoptions is the identity side of the source port instance.LoginOptionsSource
// (M1-46): discovery of instance learns the enabled identity providers of M1-36 and the
// configuration value auth.local_password_enabled through it, because instance may not import
// identity (docs/spec/02-architektur.md 4.4) and identity implements the ports it declares (02
// 4.1 item 4). internal/wire builds it over the identity repository.
//
// Example:
//
//	src := loginoptions.New(identityRepo, cfg.LocalPasswordEnabled) // passed to instance/app.NewAdmin
package loginoptions

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance"
)

// ProviderLister reads the identity providers; app.ProviderStore of identity (the postgres
// repository) implements it.
type ProviderLister interface {
	// Providers returns the providers (only the enabled ones when enabledOnly) ordered by
	// lower(name), id.
	Providers(ctx context.Context, enabledOnly bool) ([]identity.Provider, error)
}

// Source implements instance.LoginOptionsSource. It is safe for concurrent use.
type Source struct {
	providers     ProviderLister
	localPassword bool
}

// Compile-time check: Source is the source port of instance.
var _ instance.LoginOptionsSource = (*Source)(nil)

// New returns the source over providers with auth.local_password_enabled.
func New(providers ProviderLister, localPasswordEnabled bool) (*Source, error) {
	if providers == nil {
		return nil, errors.New("loginoptions: New needs the provider store")
	}
	return &Source{providers: providers, localPassword: localPasswordEnabled}, nil
}

// LoginOptions implements instance.LoginOptionsSource: id, kind and name of the enabled
// providers in the order of the store, and the configuration switch of the password. Errors: the
// database error.
func (s *Source) LoginOptions(ctx context.Context) (instance.LoginOptions, error) {
	list, err := s.providers.Providers(ctx, true)
	if err != nil {
		return instance.LoginOptions{}, err
	}
	out := instance.LoginOptions{Providers: make([]instance.ProviderInfo, 0, len(list)), LocalPasswordEnabled: s.localPassword}
	for _, p := range list {
		out.Providers = append(out.Providers, instance.ProviderInfo{ID: p.ID, Kind: string(p.Kind), Name: p.Name})
	}
	return out, nil
}
