// SPDX-License-Identifier: Apache-2.0

// Package authzsource is the adapter through which the agents module answers authz (M1-51): Source
// implements authz.AgentPolicySource over the agents records and the principal directory of
// identity, so the rights core decides with the real backend policy, credential holder and tool
// class (docs/spec/02-architecture.md section 4.3, "Source port"). authz never reads the table
// agents.
//
// Example:
//
//	src := authzsource.New(agentspg.NewRepository(pool), identityapp.NewDirectory(identitypg.NewRepository(pool)))
//	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Policies: src}, nil, log)
package authzsource

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Source implements authz.AgentPolicySource. It reads inside the ambient transaction of ctx when
// there is one, so the rights hook sees the policy a unit of work just wrote. It is safe for
// concurrent use when its collaborators are.
type Source struct {
	store app.Store
	dir   identity.Directory
}

// Compile-time check: Source is the source port of authz.
var _ authz.AgentPolicySource = (*Source)(nil)

// New returns the source over the records of store and the principals of dir.
func New(store app.Store, dir identity.Directory) *Source {
	return &Source{store: store, dir: dir}
}

// PersonalAgents implements authz.AgentPolicySource: the agents with billing_mode personal whose
// principal status is active, ascending by id. A disabled agent cannot be triggered, so I1 does
// not count it until agent.enable checks it again (10 section 5.1).
func (s *Source) PersonalAgents(ctx context.Context) ([]authz.PersonalAgent, error) {
	recs, err := s.store.PersonalAgents(ctx)
	if err != nil || len(recs) == 0 {
		return nil, err
	}
	list := make([]ids.ID, 0, len(recs))
	for _, r := range recs {
		list = append(list, r.ID)
	}
	principals, err := s.dir.Principals(ctx, list)
	if err != nil {
		return nil, err
	}
	var out []authz.PersonalAgent
	for _, r := range recs {
		if p, ok := principals[r.ID]; ok && p.Status == identity.StatusActive {
			out = append(out, authz.PersonalAgent{AgentID: r.ID, CredentialHolderID: r.Policy.HolderID})
		}
	}
	return out, nil
}

// AgentPolicy implements authz.AgentPolicySource: the target mode and holder, the target tool
// class and the reported one (authz reckons with the higher of the two classes, 07 section 2.11)
// and the billing modes the adapter reported (M1-49). ok is false for an unknown agent.
func (s *Source) AgentPolicy(ctx context.Context, agentID ids.ID) (authz.AgentPolicy, bool, error) {
	rec, found, err := s.store.Agent(ctx, agentID)
	if err != nil || !found {
		return authz.AgentPolicy{}, false, err
	}
	var modes []string
	for _, m := range rec.Reported.BillingModesSupported {
		modes = append(modes, string(m))
	}
	return authz.AgentPolicy{
		BillingMode: string(rec.Policy.Mode), CredentialHolderID: rec.Policy.HolderID,
		ToolClass: string(rec.ToolClass), ReportedToolClass: string(rec.Reported.ToolClass), BillingModesSupported: modes,
	}, true, nil
}

// DevEnvironmentReady implements authz.AgentPolicySource: no dev environment is ready until V2
// builds them (SR3).
func (s *Source) DevEnvironmentReady(context.Context, ids.ID, ids.ID) (bool, error) {
	return false, nil
}
