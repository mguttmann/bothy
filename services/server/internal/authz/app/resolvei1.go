// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ResolverDeps are the collaborators of Resolver.
type ResolverDeps struct {
	// Authorizer decides agent.billing_mode.set (and access.preview for the preview) of every
	// listed agent.
	Authorizer authorize.Authorizer
	// Policies tells an agent from an unknown id after authorize allowed (an owner may act on any
	// id): unknown ids answer the same not_found as invisible agents.
	Policies authz.AgentPolicySource
	// Rights names the listed agents as affected before the switch (the rights hook of authz).
	Rights authz.RightsChanges
	// ReasonMaxChars is audit.reason_max_chars (SEC-AUD-5).
	ReasonMaxChars int `config:"audit.reason_max_chars"`
}

// Resolver implements authz.I1Resolver, the switch part of the guided flow (05 rule 1.6, R-AZ-17,
// 10 section 11; M2-21). It starts with authz.EmptyAgentPolicySwitch, which fails closed;
// internal/wire binds the switch of the agents module with BindSwitch once that module exists,
// before any route serves. It is safe for concurrent use.
type Resolver struct {
	d  ResolverDeps
	sw atomic.Value // holds a switchHolder
}

// switchHolder lets atomic.Value hold implementations of different types.
type switchHolder struct{ sw authz.AgentPolicySwitch }

// Compile-time check: Resolver is the port of the guided flow.
var _ authz.I1Resolver = (*Resolver)(nil)

// NewResolver returns the resolver over d with the empty switch. Errors: a missing collaborator or
// a ReasonMaxChars below 1.
func NewResolver(d ResolverDeps) (*Resolver, error) {
	if d.Authorizer == nil || d.Policies == nil || d.Rights == nil {
		return nil, errors.New("authz: NewResolver needs the Authorizer, the agent policy source and the rights hook")
	}
	if d.ReasonMaxChars < 1 {
		return nil, errors.New("authz: audit.reason_max_chars must be at least 1")
	}
	r := &Resolver{d: d}
	r.sw.Store(switchHolder{sw: authz.EmptyAgentPolicySwitch{}})
	return r, nil
}

// BindSwitch binds the agent policy switch of the agents module (internal/wire); nil keeps the
// current one.
func (r *Resolver) BindSwitch(sw authz.AgentPolicySwitch) {
	if sw != nil {
		r.sw.Store(switchHolder{sw: sw})
	}
}

// UsesEmptySwitch reports whether no switch of the agents module is bound yet; the wiring test of
// internal/wire asserts the production graph is not in this state.
func (r *Resolver) UsesEmptySwitch() bool {
	_, empty := r.switcher().(authz.EmptyAgentPolicySwitch)
	return empty
}

// switcher returns the bound switch.
func (r *Resolver) switcher() authz.AgentPolicySwitch {
	return r.sw.Load().(switchHolder).sw
}

// CheckResolveI1 implements authz.I1Resolver (domain.CheckResolveI1 with audit.reason_max_chars).
func (r *Resolver) CheckResolveI1(req authz.ResolveI1) error {
	return domain.CheckResolveI1(req.AgentIDs, req.Reason, r.d.ReasonMaxChars)
}

// ResolveI1 implements authz.I1Resolver (M2-21 design note 3 e): validate, authorize
// agent.billing_mode.set per agent in ascending id with the existence check after each allow, then
// AddAffected and the switch with the trimmed reason. The denials are audited by the Authorizer
// (R-AZ-04).
func (r *Resolver) ResolveI1(ctx context.Context, by actor.Actor, req authz.ResolveI1) error {
	if err := r.CheckResolveI1(req); err != nil {
		return err
	}
	list := ascending(req.AgentIDs)
	for _, id := range list {
		if err := r.decide(ctx, by, authorize.ActionAgentBillingModeSet, authorize.Resource{Kind: authorize.ResourceAgent, ID: id, OwnerID: id}); err != nil {
			return err
		}
	}
	return r.apply(ctx, by, list, req.Reason)
}

// AuthorizePreview authorizes access.preview with the target action agent.billing_mode.set for
// every agent of list in ascending id (M2-21 design note 9; 05 section 4.2 row access.preview):
// the right and owner rule of the target action, never step-up, never audit. Unknown and invisible
// agents are the same not_found.
func (r *Resolver) AuthorizePreview(ctx context.Context, by actor.Actor, list []ids.ID) error {
	for _, id := range ascending(list) {
		res := authorize.Resource{Kind: authorize.ResourceAgent, ID: id, OwnerID: id, PreviewOf: authorize.ActionAgentBillingModeSet}
		if err := r.decide(ctx, by, authorize.ActionAccessPreview, res); err != nil {
			return err
		}
	}
	return nil
}

// ApplyPreview is the switch part of a dry run (M2-21 design note 9): AddAffected and the switch
// of list in ascending id, without authorize, inside the preview's unit of work that rolls back.
func (r *Resolver) ApplyPreview(ctx context.Context, by actor.Actor, list []ids.ID, reason string) error {
	return r.apply(ctx, by, ascending(list), reason)
}

// decide authorizes act on res and then checks that res names an agent: an id without an agents
// row (unknown, or a principal of another kind) is the same not_found as an invisible agent (05
// rule 1.2, R-AZ-01).
func (r *Resolver) decide(ctx context.Context, by actor.Actor, act authorize.Action, res authorize.Resource) error {
	d, err := r.d.Authorizer.Authorize(ctx, by, act, res)
	if err != nil {
		return err
	}
	if err := d.Err(); err != nil {
		return err
	}
	_, found, err := r.d.Policies.AgentPolicy(ctx, res.ID)
	if err != nil {
		return sourceError("agent policy source", err)
	}
	if !found {
		return errNotFound()
	}
	return nil
}

// apply names list as affected, so the rights hook records their visible targets before the
// switch, and switches them.
func (r *Resolver) apply(ctx context.Context, by actor.Actor, list []ids.ID, reason string) error {
	if err := r.d.Rights.AddAffected(ctx, list...); err != nil {
		return err
	}
	return r.switcher().SwitchToOrganization(ctx, by, list, strings.TrimSpace(reason))
}

// ascending returns list sorted by id without duplicates.
func ascending(list []ids.ID) []ids.ID {
	out := slices.Clone(list)
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return slices.Compact(out)
}

// joinFields returns the validation errors of list as one validation_failed with all their
// field errors (06 section 5 point 3), or the first error that is not a validation error; nil
// entries are skipped.
func joinFields(list ...error) error {
	var out *errs.Error
	for _, err := range list {
		if err == nil {
			continue
		}
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed {
			return err
		}
		if out == nil {
			out = errs.New(errs.CodeValidationFailed, e.Detail)
		}
		out.WithFields(e.Fields...)
	}
	if out == nil {
		return nil
	}
	return out
}
