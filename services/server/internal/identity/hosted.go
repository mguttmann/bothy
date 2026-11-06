// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The reasons of the 409 conflicts around the tokens of built-in agents (A1-03,
// docs/spec/26-builtin-agents.md 3.3).
const (
	// ReasonHostedAgentToken refuses createToken for a built-in agent: only its hosting runner
	// receives such a token, through issueHostedAgentToken.
	ReasonHostedAgentToken = "hosted_agent_token"
	// ReasonDPoPRequired refuses a hosted token for a runner whose calling token is not bound to
	// a DPoP key, because the hosted token takes that key.
	ReasonDPoPRequired = "dpop_required"
)

// HostedAgents is the consumer port through which identity learns whether an agent principal is
// a built-in agent (adapter_kind claude_agent, 26 3.1); the agents module implements it and
// internal/wire binds it with Tokens.BindHostedAgents, so identity never imports agents (02
// section 4.4). createToken refuses a token for such an agent (26 3.3 item 3).
type HostedAgents interface {
	// IsHostedAgent reports whether principalID is a built-in agent. It joins the ambient unit of
	// work of ctx. Errors: the database error of the agents module.
	IsHostedAgent(ctx context.Context, principalID ids.ID) (bool, error)
}

// HostedTokenCmd is the input of HostedTokens.IssueHosted: a token for the built-in agent
// HolderID, issued to its hosting runner IssuerID, which called with its own API token
// IssuerTokenID.
type HostedTokenCmd struct {
	// HolderID is the built-in agent the token authenticates.
	HolderID ids.ID
	// IssuerID is the hosting runner; it becomes created_by of the new token.
	IssuerID ids.ID
	// IssuerTokenID is the API token of the request; its dpop_jkt binds the new token.
	IssuerTokenID ids.ID
	// Name is the label of the new token (hosted by <runner handle>).
	Name string
	// ExpiresAt is now plus agents.hosted_token_ttl.
	ExpiresAt time.Time
}

// HostedIssue is the result of IssueHosted: the new token with its secret and the number of older
// tokens of the holder from the same runner it revoked.
type HostedIssue struct {
	Issued  IssuedToken
	Revoked int
}

// HostedTokens is the incoming port of the tokens of built-in agents (26 3.3, A1-03). Only the
// agents module calls it, after it authorized the caller; it authorizes nothing itself. Every
// method runs in the caller's unit of work, which must be a declared rights change, because the
// Authorizer caches the state of a token by rights version; outside one it fails with
// internal_error. A hosted token is one whose created_by is its holder's hosting runner
// (docs/decisions.d/A1-01.md D-A1-01-4), so no column marks it.
//
// Example (inside the rights change of agent.hosted_token.issue):
//
//	out, err := hosted.IssueHosted(ctx, identity.HostedTokenCmd{HolderID: agent, IssuerID: runner,
//		IssuerTokenID: a.TokenID, Name: "hosted by build-runner", ExpiresAt: now.Add(ttl)})
type HostedTokens interface {
	// IssueHosted locks the issuer's calling token, revokes every active token of the holder that
	// the issuer created and inserts a new agent token for the holder bound to the dpop_jkt of the
	// calling token. Errors: not_found when the calling token is not an active token of the
	// issuer; conflict with dpop_required when it has no dpop_jkt; the validation errors of a
	// token (an expiry beyond auth.api_token_max_ttl).
	IssueHosted(ctx context.Context, cmd HostedTokenCmd) (HostedIssue, error)
	// RevokeHosted revokes every active token of holderID that issuerID created (the host of a
	// built-in agent changes) and returns their number.
	RevokeHosted(ctx context.Context, holderID, issuerID, by ids.ID) (int, error)
	// RevokeIssued revokes every active token that issuerID created for another principal (the
	// hosting runner is disabled) and returns their number.
	RevokeIssued(ctx context.Context, issuerID, by ids.ID) (int, error)
}
