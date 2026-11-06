// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: the port of the tokens of built-in agents (A1-03).
var _ identity.HostedTokens = (*Tokens)(nil)

// BindHostedAgents sets the source that tells a built-in agent from another agent (the agents
// module, bound by internal/wire after both modules exist). Until it is bound no agent counts as
// built in: without the agents module no such agent can exist.
func (s *Tokens) BindHostedAgents(h identity.HostedAgents) {
	if h == nil {
		s.hosted.Store(nil)
		return
	}
	s.hosted.Store(&h)
}

// isHosted reports whether principalID is a built-in agent through the bound source.
func (s *Tokens) isHosted(ctx context.Context, principalID ids.ID) (bool, error) {
	h := s.hosted.Load()
	if h == nil {
		return false, nil
	}
	return (*h).IsHostedAgent(ctx, principalID)
}

// IssueHosted implements identity.HostedTokens (26 3.3 item 1). It locks the calling token, so a
// concurrent revocation of the runner's token either comes first (not_found here) or waits and
// then takes the new hosted token too, which carries the same key. The older tokens of the holder
// from the same runner end before the new one is inserted, so at most one is active per agent and
// runner (design note 3 of the brief A1-03).
func (s *Tokens) IssueHosted(ctx context.Context, cmd identity.HostedTokenCmd) (identity.HostedIssue, error) {
	if !txn.InRightsChange(ctx) {
		return identity.HostedIssue{}, errNoRightsChange()
	}
	caller, found, err := s.d.Store.LockAPIToken(ctx, cmd.IssuerTokenID)
	if err != nil {
		return identity.HostedIssue{}, err
	}
	if !found || caller.PrincipalID != cmd.IssuerID || !caller.Active(s.now()) {
		return identity.HostedIssue{}, invisible()
	}
	if caller.DPoPJKT == "" {
		return identity.HostedIssue{}, errs.New(errs.CodeConflict, "A hosted token needs a calling token bound to a DPoP key.").
			WithReason(identity.ReasonDPoPRequired)
	}
	revoked, err := s.d.Store.RevokeIssuedAPITokens(ctx, cmd.IssuerID, cmd.HolderID, "", cmd.IssuerID, s.now())
	if err != nil {
		return identity.HostedIssue{}, err
	}
	agent, _ := kindOf(domain.KindAgent)
	issued, err := s.issue(ctx, agent, identity.IssueTokenCmd{
		PrincipalID: cmd.HolderID, Name: cmd.Name, DPoPJKT: caller.DPoPJKT, ExpiresAt: cmd.ExpiresAt, CreatedBy: cmd.IssuerID,
	})
	if err != nil {
		return identity.HostedIssue{}, err
	}
	return identity.HostedIssue{Issued: issued, Revoked: revoked}, nil
}

// RevokeHosted implements identity.HostedTokens: the tokens of holderID that issuerID created.
func (s *Tokens) RevokeHosted(ctx context.Context, holderID, issuerID, by ids.ID) (int, error) {
	if !txn.InRightsChange(ctx) {
		return 0, errNoRightsChange()
	}
	return s.d.Store.RevokeIssuedAPITokens(ctx, issuerID, holderID, "", by, s.now())
}

// RevokeIssued implements identity.HostedTokens: every token issuerID created for another
// principal.
func (s *Tokens) RevokeIssued(ctx context.Context, issuerID, by ids.ID) (int, error) {
	if !txn.InRightsChange(ctx) {
		return 0, errNoRightsChange()
	}
	return s.d.Store.RevokeIssuedAPITokens(ctx, issuerID, ids.ID{}, "", by, s.now())
}
