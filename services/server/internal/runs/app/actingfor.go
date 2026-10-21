// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// ActingFor resolves the Invocation-Token header of a request to the acting-for binding of guard
// rule SR9 (07, brief M2-14 Design note 3): it verifies the token for the agent of the request
// and reads the run it names. adapters/actingfor exposes it to the authenticator of identity. It
// holds no state and caches nothing, so a revocation or the end of a run counts on the next
// request in every process (Design note 2e). Safe for concurrent use.
type ActingFor struct {
	tokens TokenVerifier
	runs   RunBindings
}

// NewActingFor builds the use case over the invocation token verifier (InvocationTokens) and the
// binding read of the runs (adapters/postgres.Repository). Errors: a missing collaborator.
//
// Example:
//
//	uc, err := app.NewActingFor(invocationTokens, postgres.NewRepository(pool))
//	res, err := uc.ResolveActingFor(ctx, agentID, headerValue)
func NewActingFor(tokens TokenVerifier, runs RunBindings) (*ActingFor, error) {
	if tokens == nil || runs == nil {
		return nil, errors.New("runs: the acting-for resolution needs the invocation tokens and the run bindings")
	}
	return &ActingFor{tokens: tokens, runs: runs}, nil
}

// ResolveActingFor resolves token, the one value of the Invocation-Token header, for the agent
// agentID. It verifies the token with the agent as audience and without its expiry and without a
// run (Design note 3a and 3e: signature, typ, kid of purpose invocation, iss, aud, iat); every
// verdict of the verifier gives ActingForInvalid. Claims without a run_id that parses give
// ActingForInvalid without a read (3b). Otherwise it reads the run on the pool (3c, 3g) and
// returns domain.ActingForVerdict: bound for an active run, run_inactive for an ended one,
// invalid for a run that does not match the token. Errors: a failure of the key store or the
// instance row (no verdict about the token) and of the run read, with their kinds; the caller
// answers 503 and never builds an actor without the binding (Design note 2d). No error contains
// the token.
func (a *ActingFor) ResolveActingFor(ctx context.Context, agentID ids.ID, token string) (domain.ActingForResult, error) {
	invalid := domain.ActingForResult{Status: domain.ActingForInvalid}
	c, err := a.tokens.Verify(ctx, token, VerifyInvocation{Audience: agentID, IgnoreExpiry: true})
	if fe, ok := errs.As(err); ok && fe.Code == errs.CodeInvocationTokenInvalid {
		return invalid, nil
	}
	if err != nil {
		return domain.ActingForResult{}, err
	}
	runID, err := ids.Parse(c.RunID)
	if err != nil {
		return invalid, nil
	}
	run, found, err := a.runs.RunBinding(ctx, runID)
	if err != nil {
		return domain.ActingForResult{}, err
	}
	return domain.ActingForVerdict(c, run, found, agentID), nil
}
