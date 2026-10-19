// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// IdentityPrincipals implements Principals over the ports of identity: Directory for reads and
// the handle lookup, PrincipalWrites for the app principal (M2-24 design note 12).
type IdentityPrincipals struct {
	Directory identity.Directory
	Writes    identity.PrincipalWrites
}

// Compile-time check: IdentityPrincipals is the consumer port.
var _ Principals = IdentityPrincipals{}

// Principal implements Principals.
func (p IdentityPrincipals) Principal(ctx context.Context, id ids.ID) (PrincipalFacts, error) {
	v, err := p.Directory.Principal(ctx, id)
	if err != nil {
		return PrincipalFacts{}, err
	}
	return PrincipalFacts{Kind: v.Kind, DisplayName: v.DisplayName}, nil
}

// DisplayNames implements Principals.
func (p IdentityPrincipals) DisplayNames(ctx context.Context, list []ids.ID) (map[ids.ID]string, error) {
	views, err := p.Directory.Principals(ctx, list)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]string, len(views))
	for id, v := range views {
		out[id] = v.DisplayName
	}
	return out, nil
}

// HandleTaken implements Principals.
func (p IdentityPrincipals) HandleTaken(ctx context.Context, handle string) (bool, error) {
	_, err := p.Directory.PrincipalByHandle(ctx, handle)
	if e, ok := errs.As(err); ok && e.Code == errs.CodeNotFound {
		return false, nil
	}
	return err == nil, err
}

// CreateApp implements Principals.
func (p IdentityPrincipals) CreateApp(ctx context.Context, id ids.ID, handle, displayName string) error {
	_, err := p.Writes.CreatePrincipal(ctx, identity.NewPrincipalCmd{ID: id, Kind: actor.KindApp, Handle: handle, DisplayName: displayName})
	return err
}

// SetActive implements Principals.
func (p IdentityPrincipals) SetActive(ctx context.Context, id ids.ID, active bool) error {
	status := identity.StatusDisabled
	if active {
		status = identity.StatusActive
	}
	return p.Writes.SetPrincipalStatus(ctx, id, status)
}

// Rename implements Principals.
func (p IdentityPrincipals) Rename(ctx context.Context, id ids.ID, displayName string) error {
	return p.Writes.UpdateDisplayName(ctx, id, displayName)
}
