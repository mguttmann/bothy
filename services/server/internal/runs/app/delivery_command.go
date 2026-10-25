// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The audit attribute of the command of a delivery denial (brief M2-16 design note 13 d).
const attrCommandID = "command_id"

// resolveInvocationCommand is agent.invocation with trigger_kind command (design note 13 b): the
// run's trigger, the command of the payload as the triggering object (the payload is immutable,
// while runs.triggering_command_id may become NULL), the command addresses its owner like a
// mention, no auto thread and no triggering message, so no trigger_state changes.
func resolveInvocationCommand(_ *Service, _ context.Context, run RunRow, in runs.DeliveryInput) (trigger, error) {
	id, err := domain.InvocationCommandID(in.Payload)
	if err != nil {
		return trigger{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation of a command")
	}
	return trigger{person: run.TriggeredBy, object: id, origin: originHuman, mentions: true}, nil
}

// prepareInvoked is Prepare for command.invoked (design note 13 a, c, d) inside its unit of work:
// no run row is locked and no jti is recorded (the M2-08 matrix); agent.invoke is decided in filter
// mode for the re-check principal of the invoker at the pinned permission version, on the thread
// of the payload resolved now, with the command as an explicit address and no active run. A denial
// sends nothing, changes no run and writes one audit entry; an allow carries a fresh token with
// trigger_kind command, command_id and run_id exactly when the payload names a run.
func (s *Service) prepareInvoked(ctx context.Context, in runs.DeliveryInput, issuer TokenIssuer) (outcome, error) {
	f, err := domain.InvokedFactsOf(in.Payload)
	if err != nil || in.ResourceKind != resourceCommand || f.CommandID != in.ResourceID {
		return outcome{}, errs.New(errs.CodeInternalError, "runs: command.invoked without its command")
	}
	info, err := s.d.Threads.Info(ctx, f.ThreadID)
	if err != nil {
		return outcome{}, err
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{in.AgentID})
	if err != nil {
		return outcome{}, err
	}
	agent := facts[in.AgentID]
	v, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return outcome{}, err
	}
	sctx, end := s.telemetry().StartDispatch(ctx, f.RunID, agent.Handle)
	r := authorize.Resource{
		Kind: authorize.ResourceThread, ID: info.ID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: in.AgentID, TriggerOrigin: originHuman, MentionsAgent: true},
	}
	d, err := s.d.Authorizer.Authorize(authorize.WithPermissionVersion(authorize.Filter(sctx), v), actor.Recheck(f.InvokedBy),
		authorize.ActionAgentInvoke, r)
	if err != nil {
		end(decisionDeny)
		return outcome{}, err
	}
	if !d.Allowed() {
		end(decisionDeny)
		attrs := auditing.Attrs{
			attrCommandID: auditing.ID(f.CommandID), attrAgentID: auditing.ID(in.AgentID), attrStage: auditing.Code(stageDelivery),
			attrEventType: auditing.Code(in.Type), attrTriggerKind: auditing.Code(string(domain.TriggerCommand)),
		}
		if !f.RunID.IsZero() {
			attrs[attrRunID] = auditing.ID(f.RunID)
		}
		if err := s.auditDenial(ctx, f.InvokedBy, string(d.Reason), info.ID, attrs); err != nil {
			return outcome{}, err
		}
		return outcome{checked: true, reason: domain.DeliveryReason(string(d.Reason), agent.Status == agentActive), handle: agent.Handle,
			channel: info.ChannelID}, nil
	}
	g, err := domain.DeliveryGrant(domain.DeliveryFacts{
		EventType: in.Type, RunID: f.RunID, AgentID: in.AgentID, TriggeredBy: f.InvokedBy, ObjectID: f.CommandID, ThreadID: info.ID,
		ChannelID: info.ChannelID, SpaceID: info.SpaceID, BillingMode: agent.BillingMode, ToolClass: agent.ToolClass,
		PermissionVersion: v, Control: true,
	})
	if err != nil {
		end(decisionDeny)
		return outcome{}, errs.Wrap(errs.CodeInternalError, err, "runs: the grant of a delivery")
	}
	tok, err := issuer.Issue(sctx, g)
	if err != nil {
		end(decisionDeny)
		return outcome{}, err
	}
	end(decisionAllow)
	return outcome{out: runs.DeliveryOutcome{Send: true, InvocationToken: tok.Token.Reveal()}, checked: true, allowed: true,
		handle: agent.Handle}, nil
}
