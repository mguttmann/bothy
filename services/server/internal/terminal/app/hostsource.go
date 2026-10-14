// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HostSource implements authz.HostSource over the store of the terminal module (02 section 4.3:
// terminal implements the source port authz declares). It reads fresh on every call, inside the
// ambient transaction of ctx when there is one; internal/wire binds it for the Authorizer, the
// rights hook and the grant use cases.
type HostSource struct {
	store Store
}

// Compile-time check: the source port of authz.
var _ authz.HostSource = HostSource{}

// NewHostSource returns the source over store.
func NewHostSource(store Store) HostSource { return HostSource{store: store} }

// Host implements authz.HostSource: state, holder, LLM declaration and the server host facts.
func (s HostSource) Host(ctx context.Context, id ids.ID) (authz.HostFacts, bool, error) {
	h, found, err := s.store.Host(ctx, id)
	if err != nil || !found {
		return authz.HostFacts{}, found, err
	}
	f := authz.HostFacts{
		ID: h.ID, State: string(h.State), AssignedUserID: h.AssignedUserID, LLMCredentials: h.LLMCredentials,
		CredentialHolderID: h.CredentialHolderID, DeclaredServerHost: h.DeclaredServerHost,
	}
	if h.ReportedIsServerHost != nil {
		f.ReportedIsServerHost = *h.ReportedIsServerHost
	}
	return f, true, nil
}

// HostIDs implements authz.HostSource.
func (s HostSource) HostIDs(ctx context.Context) ([]ids.ID, error) { return s.store.HostIDs(ctx) }

// AssignedTo implements authz.HostSource.
func (s HostSource) AssignedTo(ctx context.Context, userID ids.ID) ([]ids.ID, error) {
	return s.store.AssignedTo(ctx, userID)
}
