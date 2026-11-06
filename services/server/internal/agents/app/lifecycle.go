// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Disable implements agents.Agents (09 section 2.1). A first unit loads, authorizes
// agent.disable and checks the reason; an agent that is already disabled is a call without
// effect. Otherwise a second unit, declared as the rights change agent.disable for the agent,
// locks the record, disables the principal (disabled_at), revokes every API token of the agent
// and every token it issued as a hosting runner to its built-in agents (26 3.3 item 3), publishes
// AgentDisabled, bumps the version and appends agent.updated, and for a built-in agent
// agent.hosting_changed to its runner. Open gateway connections end through the revocation path
// of the gateway (M1-65).
func (s *Service) Disable(ctx context.Context, a actor.Actor, id ids.ID, reason string) (agents.Agent, error) {
	return s.setStatus(ctx, a, id, reason, authorize.ActionAgentDisable, identity.StatusDisabled)
}

// Enable implements agents.Agents like Disable, the other way: the rights hook checks I1 for the
// agent, which counts again once it is active (10 section 5.1). Tokens are not restored. A
// built-in agent's runner gets agent.hosting_changed, so it lists the agent again at once
// (docs/decisions.d/A1-03.md D-A1-03-8).
func (s *Service) Enable(ctx context.Context, a actor.Actor, id ids.ID, reason string) (agents.Agent, error) {
	return s.setStatus(ctx, a, id, reason, authorize.ActionAgentEnable, identity.StatusActive)
}

// setStatus is Disable and Enable with the action act and the target status.
func (s *Service) setStatus(ctx context.Context, a actor.Actor, id ids.ID, reason string, act authorize.Action, target identity.Status) (agents.Agent, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Agent{}, err
	}
	var (
		out    agents.Agent
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rec, err := s.load(ctx, a, act, id)
		if err != nil {
			return err
		}
		if err := domain.CheckReason(reason); err != nil {
			return err
		}
		out, err = s.view(ctx, rec)
		if err != nil {
			return err
		}
		switch identity.Status(out.Status) {
		case target:
			return s.noChange(ctx, act, id, reason)
		case identity.StatusActive, identity.StatusDisabled:
			change = true
			return nil
		case identity.StatusPending, identity.StatusDeleted:
		}
		return errs.New(errs.CodeInvalidStateTransition, "the agent cannot change its status")
	})
	if err != nil || !change {
		return out, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.statusWrite(ctx, a, id, reason, act, target)
		return err
	}, rightsChange(act, agentRef(id), id))
	if err == nil {
		s.presenceChanged(ctx, &out)
	}
	return out, err
}

// revokeOnDisable revokes every API token of the disabled agent id and every token it issued as
// a hosting runner to its built-in agents (26 3.3 item 3), counts both in attrs and publishes
// AgentDisabled. Tokens of another agent are never created by an agent, so the second count is
// zero for every agent that hosts nothing.
func (s *Service) revokeOnDisable(ctx context.Context, a actor.Actor, id ids.ID, attrs auditing.Attrs) error {
	n, err := s.d.Tokens.RevokeAllForPrincipal(ctx, id, a.ID)
	if err != nil {
		return err
	}
	attrs[attrRevokedTokens] = auditing.Int(int64(n))
	hosted, err := s.d.Hosted.RevokeIssued(ctx, id, a.ID)
	if err != nil {
		return err
	}
	if hosted > 0 {
		attrs[attrRevokedHosted] = auditing.Int(int64(hosted))
	}
	return txn.Publish(ctx, agents.AgentDisabled{AgentID: id})
}

// statusWrite is the second unit of setStatus, declared as the rights change act.
func (s *Service) statusWrite(ctx context.Context, a actor.Actor, id ids.ID, reason string, act authorize.Action, target identity.Status) (agents.Agent, error) {
	rec, err := s.lock(ctx, id)
	if err != nil {
		return agents.Agent{}, err
	}
	_, changed, err := s.d.Principals.SetPrincipalStatus(ctx, id, target)
	if err != nil {
		return agents.Agent{}, err
	}
	if !changed {
		// A concurrent call won the race: answered as without effect (06 section 2).
		v, err := s.view(ctx, rec)
		if err != nil {
			return agents.Agent{}, err
		}
		return v, s.noChange(ctx, act, id, reason)
	}
	from := identity.StatusActive
	if target == identity.StatusActive {
		from = identity.StatusDisabled
	}
	attrs := auditing.Attrs{attrFromStatus: auditing.Code(string(from)), attrReason: auditing.ReasonText(reason)}
	if target == identity.StatusDisabled {
		if err := s.revokeOnDisable(ctx, a, id, attrs); err != nil {
			return agents.Agent{}, err
		}
	}
	rec.Bump()
	out, err := s.changedAndEmit(ctx, act, rec, attrs)
	if err != nil {
		return agents.Agent{}, err
	}
	return out, s.emitHostingChanged(ctx, rec.ID, rec.HostedBy)
}
