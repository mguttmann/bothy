// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CacheLen returns the number of cached snapshots (tests of the LRU bound).
func (a *Authorizer) CacheLen() int { return a.cache.len() }

// ExplainAt computes the explanation of a space or channel at t without authorizing a caller
// (the differential test of M1-49).
func (a *Authorizer) ExplainAt(ctx context.Context, kind authorize.ResourceKind, id ids.ID, t time.Time) ([]authz.AccessEntry, error) {
	x := &explainer{a: a, t: t, states: map[ids.ID]*principalState{}}
	return x.explain(ctx, kind, id)
}

// RightsOn returns the final rights of p on the space or channel the way authorize derives them
// (principalOf, resolve, the rights of the acting principal); ok is false when the liveness checks
// refuse p.
func (a *Authorizer) RightsOn(ctx context.Context, p authorize.Principal, kind authorize.ResourceKind, id ids.ID) (domain.Rights, bool, error) {
	v, err := a.version(ctx)
	if err != nil {
		return domain.Rights{}, false, err
	}
	s, d, err := a.principalOf(ctx, p, string(authorize.ActionMessageRead), v, a.clock.Now())
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return domain.Rights{}, false, err
	}
	res, found, err := a.resolve(ctx, authorize.ActionChannelRead, authorize.Resource{Kind: kind, ID: id})
	if err != nil || !found {
		return domain.Rights{}, false, err
	}
	r, _ := s.rights(res.path)
	return r, true, nil
}
