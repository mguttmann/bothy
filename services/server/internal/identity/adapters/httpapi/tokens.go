// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Tokens implements the API token operations of rest.StrictServerInterface (M1-44): listTokens,
// createToken and revokeToken over identity.TokenAdmin. The use cases authorize; the handlers
// convert. createToken marks its answer as a one-time secret, so the Idempotency-Key stage stores
// no part of it and a replay answers 409 one_time_secret (06 section 7, R-API-26). Errors go to
// the problem writer of the chain unchanged.
type Tokens struct {
	tokens identity.TokenAdmin
}

// NewTokens returns the handlers over the token use cases.
func NewTokens(tokens identity.TokenAdmin) *Tokens {
	return &Tokens{tokens: tokens}
}

// ListTokens implements rest.StrictServerInterface (GET /principals/{principal_id}/tokens).
func (h *Tokens) ListTokens(ctx context.Context, req rest.ListTokensRequestObject) (rest.ListTokensResponseObject, error) {
	p, err := h.tokens.ListTokens(ctx, principal(ctx), ids.ID(req.PrincipalId), pageOf(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := rest.ApiTokenPage{Items: make([]rest.ApiToken, 0, len(p.Items))}
	for _, t := range p.Items {
		out.Items = append(out.Items, tokenREST(t))
	}
	if p.NextCursor != "" {
		out.NextCursor = &p.NextCursor
	}
	return rest.ListTokens200JSONResponse{Body: out}, nil
}

// CreateToken implements rest.StrictServerInterface (POST /principals/{principal_id}/tokens). The
// secret leaves the server only here, once (Token.Reveal); the chain answers with Cache-Control:
// no-store.
func (h *Tokens) CreateToken(ctx context.Context, req rest.CreateTokenRequestObject) (rest.CreateTokenResponseObject, error) {
	idempotency.MarkOneTimeSecret(ctx)
	var cmd identity.CreateTokenCmd
	if b := req.Body; b != nil {
		cmd.Name, cmd.ExpiresAt = b.Name, b.ExpiresAt
		if b.DpopJkt != nil {
			cmd.DPoPJKT = *b.DpopJkt
		}
		if b.Scopes != nil {
			cmd.Scopes = make([]string, 0, len(*b.Scopes))
			for _, s := range *b.Scopes {
				cmd.Scopes = append(cmd.Scopes, string(s))
			}
		}
	}
	issued, err := h.tokens.CreateToken(ctx, principal(ctx), ids.ID(req.PrincipalId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateToken201JSONResponse{Body: rest.ApiTokenCreated{Token: tokenREST(issued.Token), Secret: issued.Secret.Reveal()}}, nil
}

// RevokeToken implements rest.StrictServerInterface (DELETE /tokens/{token_id}).
func (h *Tokens) RevokeToken(ctx context.Context, req rest.RevokeTokenRequestObject) (rest.RevokeTokenResponseObject, error) {
	if err := h.tokens.RevokeToken(ctx, principal(ctx), ids.ID(req.TokenId)); err != nil {
		return nil, err
	}
	return rest.RevokeToken204Response{}, nil
}

// tokenREST converts a token (schema ApiToken); empty values become absent members, never null
// (ADR 0053). Neither the secret nor the hash is part of it.
func tokenREST(t identity.APIToken) rest.ApiToken {
	scopes := make([]rest.AppScope, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, rest.AppScope(s))
	}
	return rest.ApiToken{
		Id: rest.Uuid(t.ID), PrincipalId: rest.Uuid(t.PrincipalID), Name: t.Name, TokenPrefix: t.TokenPrefix, Scopes: scopes,
		DpopJkt: optional(t.DPoPJKT), ExpiresAt: optionalTime(t.ExpiresAt), CreatedAt: ms(t.CreatedAt), LastUsedAt: optionalTime(t.LastUsedAt),
		RevokedAt: optionalTime(t.RevokedAt),
	}
}
