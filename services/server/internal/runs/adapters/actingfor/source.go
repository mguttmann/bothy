// SPDX-License-Identifier: Apache-2.0

// Package actingfor exposes the acting-for resolution of runs (07 guard rule SR9, brief M2-14
// Design note 3h) to the authenticator of identity: Source has only kernel types in its
// signature, so it implements identity.ActingForSource structurally without a module edge from
// runs to identity; internal/wire holds the compile-time assertion.
//
// Example:
//
//	uc, err := app.NewActingFor(invocationTokens, postgres.NewRepository(pool))
//	src := actingfor.NewSource(uc) // bound as identity.ActingForSource in internal/wire
package actingfor

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Resolver is the use case behind Source; app.ActingFor implements it.
type Resolver interface {
	ResolveActingFor(ctx context.Context, agentID ids.ID, token string) (domain.ActingForResult, error)
}

// Compile-time check: the use case of runs is the Resolver.
var _ Resolver = (*app.ActingFor)(nil)

// Source maps the result of the runs use case to kernel/actor.ActingFor. Safe for concurrent use
// when its Resolver is.
type Source struct {
	uc Resolver
}

// NewSource returns the source over uc (app.ActingFor).
func NewSource(uc Resolver) *Source { return &Source{uc: uc} }

// ResolveActingFor resolves token, the one value of the Invocation-Token header, for the agent
// agentID and returns the binding for the actor of the request: Status, UserID, RunID, ThreadID
// and JTI of the result. A status this mapping does not know gives ActingForInvalid with zero
// IDs, never the zero value, which would mean "no header" (fail closed). Errors: those of the
// use case (key store, run read), with the zero ActingFor; the authenticator answers 503.
func (s *Source) ResolveActingFor(ctx context.Context, agentID ids.ID, token string) (actor.ActingFor, error) {
	res, err := s.uc.ResolveActingFor(ctx, agentID, token)
	if err != nil {
		return actor.ActingFor{}, err
	}
	var status actor.ActingForStatus
	//exhaustive:ignore // invalid and every unknown status map to invalid (default, fail closed).
	switch res.Status {
	case domain.ActingForBound:
		status = actor.ActingForBound
	case domain.ActingForRunInactive:
		status = actor.ActingForRunInactive
	default:
		return actor.ActingFor{Status: actor.ActingForInvalid}, nil
	}
	return actor.ActingFor{Status: status, UserID: res.UserID, RunID: res.RunID, ThreadID: res.ThreadID, JTI: res.JTI}, nil
}
