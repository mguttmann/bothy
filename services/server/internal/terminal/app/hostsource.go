// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
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
	return hostFacts(h), true, nil
}

// hostFacts maps a host row to the facts of the source port, the name included (M2-40, only for
// the preview text).
func hostFacts(h domain.Host) authz.HostFacts {
	f := authz.HostFacts{
		ID: h.ID, State: string(h.State), AssignedUserID: h.AssignedUserID, LLMCredentials: h.LLMCredentials,
		CredentialHolderID: h.CredentialHolderID, DeclaredServerHost: h.DeclaredServerHost, Name: h.Name,
	}
	if h.ReportedIsServerHost != nil {
		f.ReportedIsServerHost = *h.ReportedIsServerHost
	}
	return f
}

// PersonalHosts implements authz.HostSource (M2-40): the hosts with a personal LLM login.
func (s HostSource) PersonalHosts(ctx context.Context) ([]authz.HostFacts, error) {
	list, err := s.store.PersonalHosts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]authz.HostFacts, 0, len(list))
	for _, h := range list {
		out = append(out, hostFacts(h))
	}
	return out, nil
}

// SessionUsersOf implements authz.HostSource (M2-40, 07 SR12): the valid assignments of userID,
// Reported by the exact comparison of the name with the names its host reports (24 section 5.3).
func (s HostSource) SessionUsersOf(ctx context.Context, userID ids.ID) ([]authz.SessionUserFact, error) {
	list, err := s.store.OwnSessionUsers(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]authz.SessionUserFact, 0, len(list))
	for _, u := range list {
		out = append(out, authz.SessionUserFact{
			HostID: u.HostID, HostSessionUserID: u.ID, OSUser: u.OSUser, Reported: domain.ReportsAssignment(u.HostReport, u.OSUser, u.OwnerAccount),
			OwnerAccount: u.OwnerAccount,
		})
	}
	return out, nil
}

// HostIDs implements authz.HostSource.
func (s HostSource) HostIDs(ctx context.Context) ([]ids.ID, error) { return s.store.HostIDs(ctx) }

// AssignedTo implements authz.HostSource.
func (s HostSource) AssignedTo(ctx context.Context, userID ids.ID) ([]ids.ID, error) {
	return s.store.AssignedTo(ctx, userID)
}
