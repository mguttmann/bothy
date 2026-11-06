// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The operations of the hosted agents (A1-03, docs/spec/26-builtin-agents.md 9): setAgentHost,
// listHostedAgents and issueHostedAgentToken, over agents.Hosting of the use cases.

// hosting returns the hosting port of the use cases; a use case set without it answers
// internal_error.
func (h *Agents) hosting() (agents.Hosting, error) {
	x, ok := h.agents.(agents.Hosting)
	if !ok {
		return nil, errs.New(errs.CodeInternalError, "agents: no hosting bound")
	}
	return x, nil
}

// SetAgentHost implements rest.StrictServerInterface (PUT /agents/{agent_id}/host).
func (h *Agents) SetAgentHost(ctx context.Context, req rest.SetAgentHostRequestObject) (rest.SetAgentHostResponseObject, error) {
	x, err := h.hosting()
	if err != nil {
		return nil, err
	}
	b := req.Body
	a, err := x.SetHost(ctx, principal(ctx), ids.ID(req.AgentId), agents.HostCmd{
		HostedBy: ids.ID(b.HostedBy), Reason: b.Reason, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetAgentHost200JSONResponse{Body: toREST(a), Headers: rest.SetAgentHost200ResponseHeaders{ETag: etag(a)}}, nil
}

// ListHostedAgents implements rest.StrictServerInterface (GET /agents/me/hosted).
func (h *Agents) ListHostedAgents(ctx context.Context, req rest.ListHostedAgentsRequestObject) (rest.ListHostedAgentsResponseObject, error) {
	x, err := h.hosting()
	if err != nil {
		return nil, err
	}
	var p agents.Page
	if req.Params.Cursor != nil {
		p.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		p.Limit = *req.Params.Limit
	}
	page, err := x.ListHosted(ctx, principal(ctx), p)
	if err != nil {
		return nil, err
	}
	out := rest.AgentPage{Items: make([]rest.Agent, 0, len(page.Items))}
	for _, a := range page.Items {
		out.Items = append(out.Items, toREST(a))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListHostedAgents200JSONResponse{Body: out}, nil
}

// IssueHostedAgentToken implements rest.StrictServerInterface (POST
// /agents/me/hosted/{agent_id}/token). The answer carries the secret once; the Idempotency-Key
// stage stores no part of it and a replay answers 409 one_time_secret (06 section 7, R-API-26).
func (h *Agents) IssueHostedAgentToken(ctx context.Context, req rest.IssueHostedAgentTokenRequestObject) (rest.IssueHostedAgentTokenResponseObject, error) {
	idempotency.MarkOneTimeSecret(ctx)
	x, err := h.hosting()
	if err != nil {
		return nil, err
	}
	issued, err := x.IssueHostedToken(ctx, principal(ctx), ids.ID(req.AgentId))
	if err != nil {
		return nil, err
	}
	return rest.IssueHostedAgentToken201JSONResponse{Body: rest.ApiTokenCreated{Token: hostedTokenREST(issued.Token), Secret: issued.Secret.Reveal()}}, nil
}

// hostedTokenREST converts the new token (schema ApiToken); empty values become absent members,
// never null (ADR 0053). Neither the secret nor the hash is part of it.
func hostedTokenREST(t agents.HostedTokenInfo) rest.ApiToken {
	return rest.ApiToken{
		Id: rest.Uuid(t.ID), PrincipalId: rest.Uuid(t.PrincipalID), Name: t.Name, TokenPrefix: t.TokenPrefix, Scopes: []rest.AppScope{},
		DpopJkt: optional(t.DPoPJKT), ExpiresAt: optionalTime(t.ExpiresAt), CreatedAt: t.CreatedAt.UTC().Truncate(time.Millisecond),
	}
}
