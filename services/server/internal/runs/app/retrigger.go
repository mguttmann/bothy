// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// retriggerable are the trigger_state_reason values of a failed message that retriggerMessage
// decides again (09 3.8, F-PROJEKT-BEREIT): causes an owner can fix without the author writing
// the message again.
var retriggerable = map[string]bool{
	string(domain.ReasonProjectNotReady):            true,
	string(domain.ReasonOperatorCannotServeProject): true,
	string(domain.ReasonOperatorNotInChannel):       true,
}

// Compile-time check: runs decides retriggerMessage for the handlers of messaging.
var _ messaging.Retriggers = (*Service)(nil)

// Retrigger implements messaging.Retriggers (retriggerMessage, 09 3.8, F-PROJEKT-BEREIT) in one
// unit of work: only the author of the message, then the thread lock (the lock order of the
// acceptance), the message read again under it, which must be failed with a reason of
// retriggerable and not deleted (409 invalid_state_transition otherwise), then the thread takes
// the channel's effective operator when its fixed operator never completed a run in it (the
// exception to R-DM-12), and the acceptance runs again as for a new message. The caller has
// already read the message with message.read (getMessage), so an invisible message never gets here.
func (s *Service) Retrigger(ctx context.Context, messageID ids.ID) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.retriggerTx(ctx, a, messageID) })
}

// retriggerTx is Retrigger inside its unit of work.
func (s *Service) retriggerTx(ctx context.Context, a actor.Actor, messageID ids.ID) error {
	m, err := s.d.Messages.TriggerMessageOf(ctx, messageID)
	if err != nil {
		return err
	}
	if a.Kind != actor.KindUser || m.Author.ID != a.ID {
		return errs.New(errs.CodeForbidden, "only the author may trigger the message again").WithReason("not_self")
	}
	if err := s.d.Threads.LockForMessageChange(ctx, m.ThreadID); err != nil {
		return err
	}
	if m, err = s.d.Messages.TriggerMessageOf(ctx, messageID); err != nil {
		return err
	}
	if m.Deleted || m.TriggerState != string(domain.TriggerStateFailed) || !retriggerable[m.TriggerStateReason] {
		return errs.New(errs.CodeInvalidStateTransition,
			"only a message that failed because the project or the thread operator was not ready can be triggered again")
	}
	if err := s.repinOperator(ctx, m.ThreadID); err != nil {
		return err
	}
	if err := s.retriggerRight(ctx, a, m); err != nil {
		return err
	}
	return s.acceptTrigger(ctx, messaging.MessagePosted{
		MessageID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, ThreadSeq: m.ThreadSeq, AuthorID: m.Author.ID,
		AuthorKind: m.Author.Kind, Origin: m.Origin, Kind: m.Kind, MentionedIDs: m.MentionedIDs, HasAttachments: m.HasAttachments,
	})
}

// retriggerRight decides agent.invoke in filter mode for the caller on the message's thread
// with the operator that would take it (the thread's, else the channel's): the rights of
// triggering. A caller without trigger there (level_insufficient, no_grant) is refused with 403
// agent_invocation_denied; every other refusal is left to the acceptance, which writes it on the
// message. The check is also the route's proof of agent.invoke when the acceptance decides
// without one (no trigger, or a rejection before agent.invoke).
func (s *Service) retriggerRight(ctx context.Context, a actor.Actor, m messaging.TriggerMessage) error {
	info, err := s.d.Threads.Info(ctx, m.ThreadID)
	if err != nil {
		return err
	}
	op := info.OperatorAgentID
	if op.IsZero() {
		ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
		if err != nil {
			return err
		}
		op = ch.EffectiveOperatorID
	}
	res := authorize.Resource{
		Kind: authorize.ResourceThread, ID: info.ID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: op, TriggerOrigin: originHuman, MentionsAgent: slices.Contains(m.MentionedIDs, op)},
	}
	v, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionAgentInvoke, res)
	if err != nil {
		return err
	}
	if !v.Allowed() && (v.Reason == denyLevelInsufficient || v.Reason == denyNoGrant) {
		return invocationDenied(string(v.Reason))
	}
	return nil
}

// repinOperator gives the thread the channel's effective operator when its fixed operator is
// another agent that never completed a run in it (09 3.8): that operator has no session to keep
// (R-DM-12), and step 4a would refuse every message for it.
func (s *Service) repinOperator(ctx context.Context, threadID ids.ID) error {
	info, err := s.d.Threads.Info(ctx, threadID)
	if err != nil || info.Kind != string(domain.ThreadThread) || info.OperatorAgentID.IsZero() {
		return err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil || ch.EffectiveOperatorID.IsZero() || ch.EffectiveOperatorID == info.OperatorAgentID {
		return err
	}
	worked, err := s.d.Repo.CompletedRunOfAgent(ctx, info.ID, info.OperatorAgentID)
	if err != nil || worked {
		return err
	}
	if s.d.ThreadOperators == nil {
		return errs.New(errs.CodeInternalError, "runs: no thread operator port bound")
	}
	_, err = s.d.ThreadOperators.RepinThreadOperator(ctx, info.ID)
	return err
}
