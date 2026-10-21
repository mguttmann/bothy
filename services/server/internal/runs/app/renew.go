// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// stageRenewal is the audit stage of a renewal refused by the checks before a delivery.
const stageRenewal = "renewal"

// renewResult is what one attempt of the unit of work of a renewal returns: the grant, or a
// refusal that commits with the failed run (design note 4 d).
type renewResult struct {
	grant     runs.TokenGrant
	committed error
	handle    string
	reason    string
}

// RenewToken implements runs.Runs (renewInvocationToken, run.token.renew; 09 4.5.1, M2-09 design
// note 4) in one unit of work with the run row locked: step 1 verifies the token without its
// expiry, RenewVerdict runs steps 2 and 3, authorize decides run.token.renew on the verdict, step 4
// repeats the checks before a delivery for the starter with current permissions (a refusal fails
// the run, is audited and answers 403 agent_invocation_denied after the commit), step 5 issues a
// fresh token and replaces invocation_jti through the repository function of the delivery. No
// event and no audit entry on success (Audit denials).
func (s *Service) RenewToken(ctx context.Context, cmd runs.RenewToken) (runs.TokenGrant, error) {
	v, err := s.verifier()
	if err != nil {
		return runs.TokenGrant{}, err
	}
	box := s.tokens.Load()
	if s.d.Versions == nil || s.d.Audit == nil {
		return runs.TokenGrant{}, errs.New(errs.CodeInternalError, "runs: the renewal has no versions or audit")
	}
	a := caller(ctx)
	var res renewResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = s.renewTx(ctx, a, cmd, v, box.issuer)
		return err
	})
	if err != nil {
		return runs.TokenGrant{}, err
	}
	if res.committed != nil {
		s.telemetry().Denied(context.WithoutCancel(ctx), res.handle, res.reason)
		return runs.TokenGrant{}, res.committed
	}
	return res.grant, nil
}

// renewTx is RenewToken inside its unit of work.
func (s *Service) renewTx(ctx context.Context, a actor.Actor, cmd runs.RenewToken, v TokenVerifier, issuer TokenIssuer) (renewResult, error) {
	row, found, err := s.d.Repo.LockRun(ctx, cmd.RunID)
	if err != nil {
		return renewResult{}, err
	}
	check := domain.RenewCheck{Verdict: domain.VerdictInvalid, Next: domain.NextDeny}
	var c tokenClaims
	if found {
		var ok bool
		if c, ok, err = s.verifyToken(ctx, a, cmd.Token, row.ID, v); err != nil {
			return renewResult{}, err
		}
		if ok {
			check = domain.RenewVerdict(domain.TokenClaims{JTI: c.jti, ExpiresAt: c.expiresAt}, tokenState(row))
		}
	}
	d, err := s.authorizeAgent(ctx, a, authorize.ActionRunTokenRenew, row, found, cmd.RunID, &authorize.TokenFacts{Verdict: authzVerdict(check.Verdict)})
	if err != nil {
		return renewResult{}, err
	}
	if !d.Allowed() {
		return renewResult{}, d.Err()
	}
	switch check.Next {
	case domain.NextAlreadyStarted:
		return renewResult{}, errs.New(errs.CodeAlreadyStarted, "the run has already started or ended")
	case domain.NextTransition:
	default:
		return renewResult{}, errs.New(errs.CodeInternalError, "runs: run.token.renew allowed a token that is not valid")
	}
	return s.renewChecked(ctx, row, c, check.WasCurrent, issuer)
}

// renewChecked runs steps 4 and 5 for a current token of row.
func (s *Service) renewChecked(ctx context.Context, row RunRow, c tokenClaims, wasCurrent bool, issuer TokenIssuer) (renewResult, error) {
	in, err := renewalInput(row, c.claims)
	if err != nil {
		return renewResult{}, err
	}
	res := resolverFor(in.Type, row)
	if res == nil {
		return renewResult{}, errs.New(errs.CodeInternalError, "runs: no renewal for the trigger kind of this token yet")
	}
	tr, err := res(s, ctx, row, in)
	if err != nil {
		return renewResult{}, err
	}
	if tr.gone {
		// The interaction of the token no longer exists (its card was deleted, M2-15): the token
		// names a trigger that cannot be checked again, so it is not renewed.
		return renewResult{}, errs.New(errs.CodeInvocationTokenInvalid, "").WithReason("invalid")
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{row.AgentID})
	if err != nil {
		return renewResult{}, err
	}
	agent := facts[row.AgentID]
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return renewResult{}, err
	}
	version, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return renewResult{}, err
	}
	r := authorize.Resource{
		Kind: authorize.ResourceThread, ID: row.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: row.AgentID, TriggerOrigin: tr.origin, MentionsAgent: tr.mentions,
			ActiveRunAgentID: row.AgentID, NewThread: tr.newThread},
		Interaction: tr.facts,
	}
	d, err := s.d.Authorizer.Authorize(authorize.WithPermissionVersion(authorize.Filter(ctx), version), actor.Recheck(tr.person),
		authorize.ActionAgentInvoke, r)
	if err != nil {
		return renewResult{}, err
	}
	if !d.Allowed() {
		return s.refuseRenewal(ctx, row, in.Type, tr, string(d.Reason), agent.Status == agentActive, agent.Handle)
	}
	g, err := domain.DeliveryGrant(domain.DeliveryFacts{
		EventType: in.Type, RunTriggerKind: row.TriggerKind, RunID: row.ID, AgentID: row.AgentID, TriggeredBy: tr.person,
		ObjectID: tr.object, ThreadID: row.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, BillingMode: agent.BillingMode,
		ToolClass: agent.ToolClass, PermissionVersion: version,
	})
	if err != nil {
		return renewResult{}, errs.Wrap(errs.CodeInternalError, err, "runs: the grant of a renewal")
	}
	tok, err := issuer.Issue(ctx, g)
	if err != nil {
		return renewResult{}, err
	}
	// A retry with the replaced token keeps renewed_from_jti; a renewal of the current one records
	// it (09 4.5.1 step 5, AG-32).
	from := row.RenewedFromJTI
	if wasCurrent {
		from = c.jti
	}
	if _, err := s.d.Repo.RecordDelivery(ctx, DeliveryWrite{ID: row.ID, JTI: tok.JTI, RenewedFromJTI: from, BillingModeExpected: agent.BillingMode}); err != nil {
		return renewResult{}, err
	}
	return renewResult{grant: runs.TokenGrant{Token: tok.Token.Reveal(), ExpiresAt: tok.ExpiresAt}}, nil
}

// refuseRenewal applies a denial of step 4 (design note 4 d): the run fails with the reason the
// delivery check maps (AZ-M51), the denial is audited as at delivery, and 403
// agent_invocation_denied with that reason follows after the commit (ruling D-M2-09-8).
func (s *Service) refuseRenewal(ctx context.Context, row RunRow, eventType string, tr trigger, authzReason string, agentActive bool,
	handle string) (renewResult, error) {
	reason := domain.DeliveryReason(authzReason, agentActive)
	if _, err := s.endRun(ctx, row, domain.StateFailed, reason, domain.SetterServer, TransitionStore{}); err != nil {
		return renewResult{}, err
	}
	if err := s.auditDenial(ctx, tr.person, authzReason, row.ThreadID, auditing.Attrs{
		attrRunID: auditing.ID(row.ID), attrAgentID: auditing.ID(row.AgentID), attrStage: auditing.Code(stageRenewal),
		attrEventType: auditing.Code(eventType), attrTriggerKind: auditing.Code(triggerKindOf(eventType, row)),
	}); err != nil {
		return renewResult{}, err
	}
	return renewResult{committed: errs.New(errs.CodeAgentInvocationDenied, "").WithReason(reason), handle: handle, reason: reason}, nil
}

// renewalInput is the delivery the presented token came with, rebuilt from its claims for the
// resolver of the delivery check: trigger_kind message is agent.invocation, followup is
// agent.followup with its message; interaction (M2-15) is agent.invocation for the interaction
// that opened the run and interaction.created for any other interaction of the run. command comes
// with M2-16; until then its tokens are not renewed (internal error, nothing issued).
func renewalInput(row RunRow, c domain.InvocationClaims) (runs.DeliveryInput, error) {
	in := runs.DeliveryInput{AgentID: row.AgentID, ResourceKind: resourceRun, ResourceID: row.ID}
	switch domain.TriggerKind(c.TriggerKind) {
	case domain.TriggerMessage:
		in.Type = domain.EventAgentInvocation
	case domain.TriggerFollowup:
		msg, err := ids.Parse(c.MessageID)
		if err != nil {
			return runs.DeliveryInput{}, errs.New(errs.CodeInternalError, "runs: a follow-up token without its message")
		}
		p, err := domain.FollowupPayload(row.ID, row.ThreadID, msg)
		if err != nil {
			return runs.DeliveryInput{}, errs.Wrap(errs.CodeInternalError, err, "runs: the follow-up of a renewal")
		}
		in.Type, in.Payload = domain.EventAgentFollowup, p
	case domain.TriggerInteraction:
		ix, err := ids.Parse(c.InteractionID)
		if err != nil || ix.IsZero() {
			return runs.DeliveryInput{}, errs.New(errs.CodeInternalError, "runs: an interaction token without its interaction")
		}
		if row.TriggerKind == triggerKindInteraction && (row.TriggeringInteractionID == ix || row.TriggeringInteractionID.IsZero()) {
			p, err := json.Marshal(struct {
				InteractionID string `json:"interaction_id"`
			}{ix.String()})
			if err != nil {
				return runs.DeliveryInput{}, errs.Wrap(errs.CodeInternalError, err, "runs: the interaction of a renewal")
			}
			in.Type, in.Payload = domain.EventAgentInvocation, p
			return in, nil
		}
		p, err := json.Marshal(map[string]map[string]string{"interaction": {"id": ix.String(), "run_id": row.ID.String()}})
		if err != nil {
			return runs.DeliveryInput{}, errs.Wrap(errs.CodeInternalError, err, "runs: the interaction of a renewal")
		}
		in.Type, in.ResourceKind, in.ResourceID, in.Payload = domain.EventInteractionCreated, resourceInteraction, ix, p
	default:
		return runs.DeliveryInput{}, errs.New(errs.CodeInternalError, "runs: no renewal for the trigger kind of this token yet")
	}
	return in, nil
}
