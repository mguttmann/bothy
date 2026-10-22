// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// organization is the target policy of every switch to organization: no credential holder.
var organization = domain.Policy{Mode: domain.Organization}

// applySwitchToOrganization is the one apply step of a switch to organization (M2-21 design note
// 4, docs/DECISIONS.md D-M2-21-5) that SetBillingMode, SetBillingModeAll and SwitchToOrganization
// share, so all three write the same row and the same agent.updated: billing_mode organization,
// credential_holder_id NULL, billing_mode_changed_at now, billing_mode_changed_by by, version + 1,
// then agent.updated with the agent after the change (receive action agent.read). The caller
// locked rec, checked that it is not on organization and writes its own audit entry.
func (s *Service) applySwitchToOrganization(ctx context.Context, by ids.ID, rec *domain.Agent) (agents.Agent, error) {
	if _, err := rec.SetPolicy(organization, by, s.d.Clock.Now()); err != nil {
		return agents.Agent{}, err
	}
	return s.storeAndEmit(ctx, *rec)
}

// storeAndEmit writes rec and appends agent.updated for it.
func (s *Service) storeAndEmit(ctx context.Context, rec domain.Agent) (agents.Agent, error) {
	if err := s.d.Store.UpdateAgent(ctx, rec); err != nil {
		return agents.Agent{}, err
	}
	v, err := s.view(ctx, rec)
	if err != nil {
		return agents.Agent{}, err
	}
	return v, s.emitUpdated(ctx, v)
}

// SwitchToOrganization is the switch part of the guided flow (05 rule 1.6, R-AZ-17; M2-21 design
// note 4), behind authz.AgentPolicySwitch (adapters/authzsource). Only authz calls it, after it
// authorized agent.billing_mode.set for every agent of list and named them as affected; so it
// authorizes nothing itself. It runs in the ambient unit of work of the outer action, locks the
// rows in ascending id (the lock order of set_all, 03 section 6.3) and switches every personal
// agent with the shared apply step and one allow entry agent.billing_mode.set per agent with the
// old and new mode, the old holder and reason; an agent already on organization keeps its row and
// gets an allow entry with no_change (another owner may have switched it since the preview). A
// disabled agent is switched like an active one; no If-Match is read (docs/DECISIONS.md
// D-M2-21-4). The presence tracker learns the new mode with the agent's next heartbeat (D-M2-21-7).
// Errors: internal outside a unit of work or for a caller other than the request principal;
// not_found for an id without an agents row; store errors.
func (s *Service) SwitchToOrganization(ctx context.Context, by actor.Actor, list []ids.ID, reason string) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "agents: SwitchToOrganization runs only inside the unit of work of the outer action")
	}
	ctx, err := withCaller(ctx, by)
	if err != nil {
		return err
	}
	want := uniqueSorted(list)
	rows, err := s.d.Store.LockAgents(ctx, want)
	if err != nil {
		return err
	}
	if len(rows) != len(want) {
		return invisible()
	}
	for i := range rows {
		rec := rows[i]
		if rec.Policy == organization {
			if err := s.noChange(ctx, authorize.ActionAgentBillingModeSet, rec.ID, reason); err != nil {
				return err
			}
			continue
		}
		from := rec.Policy
		if _, err := s.applySwitchToOrganization(ctx, by.ID, &rec); err != nil {
			return err
		}
		if err := s.record(ctx, authorize.ActionAgentBillingModeSet, rec.ID, policyAttrs(from, organization, reason)); err != nil {
			return err
		}
	}
	return nil
}
