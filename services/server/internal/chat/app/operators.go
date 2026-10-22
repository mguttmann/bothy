// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The audit details of the operator actions (D4 of M1-54): codes and IDs only.
const (
	attrTargetKind = "target_kind"
	attrTargetID   = "target_id"
	attrOldMode    = "old_mode"
	attrOldAgentID = "old_agent_id"
	attrNewMode    = "new_mode"
	attrNewAgentID = "new_agent_id"
)

// assignment converts the port form into the domain form.
func assignment(a chat.OperatorAssignment) domain.OperatorAssignment {
	return domain.OperatorAssignment{Mode: domain.OperatorMode(a.Mode), AgentID: a.AgentID}
}

// operatorAttrs returns the audit details of a change of the operator of the target from old to
// want; absent agents are left out.
func operatorAttrs(kind string, target ids.ID, old, want domain.OperatorAssignment, reason string) auditing.Attrs {
	attrs := auditing.Attrs{
		attrTargetKind: auditing.Code(kind), attrTargetID: auditing.ID(target),
		attrOldMode: auditing.Code(string(old.Mode)), attrNewMode: auditing.Code(string(want.Mode)), attrReason: auditing.ReasonText(reason),
	}
	if !old.AgentID.IsZero() {
		attrs[attrOldAgentID] = auditing.ID(old.AgentID)
	}
	if !want.AgentID.IsZero() {
		attrs[attrNewAgentID] = auditing.ID(want.AgentID)
	}
	return attrs
}

// withNoChange marks attrs as a call without effect (06 section 2 "No effect").
func withNoChange(attrs auditing.Attrs) auditing.Attrs {
	attrs[attrNoChange] = auditing.Bool(true)
	return attrs
}

// resolveRequest converts the guided flow of the port into the request of authz; nil stays nil.
func resolveRequest(r *chat.ResolveI1) *authz.ResolveI1 {
	if r == nil {
		return nil
	}
	return &authz.ResolveI1{AgentIDs: slices.Clone(r.AgentIDs), Reason: r.Reason}
}

// errNoResolver is the refusal of a guided flow without a bound resolver (fail closed).
func errNoResolver() error {
	return errs.New(errs.CodeInternalError, "chat: the guided flow needs the resolver of authz")
}

// checkOperatorBody validates the operator body (checkOperator) and the guided flow req together, so their
// field errors come in one answer (06 section 5 point 3; M2-21 design note 2).
func (s *Service) checkOperatorBody(ctx context.Context, want domain.OperatorAssignment, space bool, kind domain.ChannelKind, reason string, req *authz.ResolveI1) error {
	var resolveErr error
	if req != nil {
		if s.d.Resolver == nil {
			return errNoResolver()
		}
		resolveErr = s.d.Resolver.CheckResolveI1(*req)
	}
	err := s.checkOperator(ctx, want, space, kind, reason)
	if resolveErr == nil {
		return err
	}
	if err == nil {
		return resolveErr
	}
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		return err
	}
	re, _ := errs.As(resolveErr)
	return errs.New(errs.CodeValidationFailed, e.Detail).WithFields(append(slices.Clone(e.Fields), re.Fields...)...)
}

// resolve runs the switch part of the guided flow inside the second unit of an operator change,
// after the checks under the lock and before the first write (M2-21 design note 3 e).
func (s *Service) resolve(ctx context.Context, a actor.Actor, req *authz.ResolveI1) error {
	if req == nil {
		return nil
	}
	if s.d.Resolver == nil {
		return errNoResolver()
	}
	return s.d.Resolver.ResolveI1(ctx, a, *req)
}

// checkOperator validates a request of D1: mode and agent_id (inherit only for a channel), the
// reason, no dm channel, and for mode agent an active agent. Errors: validation_failed.
func (s *Service) checkOperator(ctx context.Context, want domain.OperatorAssignment, space bool, kind domain.ChannelKind, reason string) error {
	check := want.CheckForChannel
	if space {
		check = want.CheckForSpace
	}
	if err := check(); err != nil {
		return err
	}
	if err := domain.CheckReason(reason); err != nil {
		return err
	}
	if kind == domain.ChannelDM {
		// dm channels cannot reach this code in M1 (owners never read them, SR6); refused anyway.
		return codeError("", domain.FieldNotAllowedForKind, "a dm channel has no operator")
	}
	if want.Mode != domain.OperatorAgent {
		return nil
	}
	ok, err := s.activeAgent(ctx, want.AgentID)
	if err != nil {
		return err
	}
	if !ok {
		return codeError("/agent_id", domain.FieldAgentNotActive, "agent_id names no active agent")
	}
	return nil
}

// activeAgent reports whether id is a principal of kind agent with status active (identity) and
// has an agents row (the agent policy of the agents module). Owners read every agent, so the
// answer leaks nothing (D1 of M1-54).
func (s *Service) activeAgent(ctx context.Context, id ids.ID) (bool, error) {
	p, err := s.d.Directory.Principal(ctx, id)
	if errs.KindOf(err) == errs.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if p.Kind != actor.KindAgent || p.Status != identity.StatusActive {
		return false, nil
	}
	_, found, err := s.d.Policies.AgentPolicy(ctx, id)
	return found, err
}

// SetChannelOperator implements chat.Structure (D2 to D4 of M1-54). A first unit loads and
// authorizes channel.operator.set, checks If-Match, the body (D1) and the archive state, and
// records the same assignment as a call without effect. Otherwise a second unit, declared as the
// rights change channel.operator.set with the channel as resource, so permission_version is
// locked first (03 section 6.3), locks the channel, checks If-Match and the archive state again,
// names the old and the new effective operator as affected, writes the operator columns with a
// new version, resets the readiness of a bound forum whose effective operator changed (M2-03),
// records the audit entry and appends channel.updated; the rights hook verifies I1 and writes
// access.changed.
func (s *Service) SetChannelOperator(ctx context.Context, a actor.Actor, cmd chat.SetChannelOperatorCmd) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	want := assignment(cmd.Operator)
	req := resolveRequest(cmd.ResolveI1)
	var (
		out    ChannelRow
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := s.loadChannel(ctx, a, authorize.ActionChannelOperatorSet, cmd.ChannelID)
		if err != nil {
			return err
		}
		if err := channelPrecondition(cmd.IfMatch, c); err != nil {
			return err
		}
		if err := s.checkOperatorBody(ctx, want, false, c.Channel.Kind, cmd.Reason, req); err != nil {
			return err
		}
		if archived(c) {
			return channelArchivedConflict()
		}
		out = c
		if c.Channel.Assignment() == want {
			return s.recordChannel(ctx, authorize.ActionChannelOperatorSet, c.Channel.ID,
				withNoChange(operatorAttrs(resourceChannel, c.Channel.ID, want, want, cmd.Reason)))
		}
		change = true
		return nil
	})
	if err != nil || !change {
		return channelDTO(out), err
	}
	ref := txn.Ref{Kind: resourceChannel, ID: cmd.ChannelID}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.setChannelOperatorWrite(ctx, a, cmd, want, req)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionChannelOperatorSet), Resource: ref}))
	return channelDTO(out), err
}

// setChannelOperatorWrite is the second unit of SetChannelOperator.
func (s *Service) setChannelOperatorWrite(ctx context.Context, a actor.Actor, cmd chat.SetChannelOperatorCmd, want domain.OperatorAssignment,
	req *authz.ResolveI1) (ChannelRow, error) {
	c, err := s.lockChannel(ctx, cmd.ChannelID)
	if err != nil {
		return ChannelRow{}, err
	}
	if archived(c) {
		return ChannelRow{}, channelArchivedConflict()
	}
	if err := channelPrecondition(cmd.IfMatch, c); err != nil {
		return ChannelRow{}, err
	}
	old := c.Channel.Assignment()
	if old == want {
		// A concurrent call set the same assignment first: answered as without effect.
		return c, s.recordChannel(ctx, authorize.ActionChannelOperatorSet, c.Channel.ID,
			withNoChange(operatorAttrs(resourceChannel, c.Channel.ID, want, want, cmd.Reason)))
	}
	if err := s.resolve(ctx, a, req); err != nil {
		return ChannelRow{}, err
	}
	before := c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID
	after := domain.EffectiveOperator(want.Mode == domain.OperatorInherit, want.AgentID, c.SpaceDefaultOperatorID)
	if err := s.d.RightsChanges.AddAffected(ctx, before, after); err != nil {
		return ChannelRow{}, err
	}
	c.Channel.SetOperator(want)
	if err := s.d.Store.SaveChannelOperator(ctx, c.Channel); err != nil {
		return ChannelRow{}, err
	}
	if before != after && c.ProjectBound {
		// The runner that reported readiness no longer serves the forum (M2-03 design note 7).
		if err := s.resetReadiness(ctx, []ids.ID{c.Channel.ID}); err != nil {
			return ChannelRow{}, err
		}
	}
	if err := s.recordChannel(ctx, authorize.ActionChannelOperatorSet, c.Channel.ID,
		operatorAttrs(resourceChannel, c.Channel.ID, old, want, cmd.Reason)); err != nil {
		return ChannelRow{}, err
	}
	return c, s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// SetSpaceOperator implements chat.Structure (D2 to D4 of M1-54) like SetChannelOperator. The
// second unit locks the space and then the inheriting channels that are not archived in
// ascending id, names the old and the new default operator as affected, writes the default with
// a new space version and bumps the version of every inheriting channel, because its effective
// operator changed (04 "Zustellgarantie" point 3), and resets the readiness of the inheriting
// bound forums (M2-03); it appends space.updated and then
// channel.updated per channel in ascending id.
func (s *Service) SetSpaceOperator(ctx context.Context, a actor.Actor, cmd chat.SetSpaceOperatorCmd) (chat.Space, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Space{}, err
	}
	want := assignment(cmd.Operator)
	req := resolveRequest(cmd.ResolveI1)
	var (
		out    domain.Space
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		sp, err := s.load(ctx, a, authorize.ActionSpaceOperatorSet, cmd.SpaceID)
		if err != nil {
			return err
		}
		if err := precondition(cmd.IfMatch, sp); err != nil {
			return err
		}
		if err := s.checkOperatorBody(ctx, want, true, "", cmd.Reason, req); err != nil {
			return err
		}
		if sp.Archived() {
			return archivedConflict()
		}
		out = sp
		if sp.Assignment() == want {
			return s.record(ctx, authorize.ActionSpaceOperatorSet, sp.ID, withNoChange(operatorAttrs(resourceSpace, sp.ID, want, want, cmd.Reason)))
		}
		change = true
		return nil
	})
	if err != nil || !change {
		return spaceView(out), err
	}
	ref := txn.Ref{Kind: resourceSpace, ID: cmd.SpaceID}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.setSpaceOperatorWrite(ctx, a, cmd, want, req)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionSpaceOperatorSet), Resource: ref}))
	return spaceView(out), err
}

// setSpaceOperatorWrite is the second unit of SetSpaceOperator.
func (s *Service) setSpaceOperatorWrite(ctx context.Context, a actor.Actor, cmd chat.SetSpaceOperatorCmd, want domain.OperatorAssignment,
	req *authz.ResolveI1) (domain.Space, error) {
	sp, err := s.lock(ctx, cmd.SpaceID)
	if err != nil {
		return domain.Space{}, err
	}
	if sp.Archived() {
		return domain.Space{}, archivedConflict()
	}
	if err := precondition(cmd.IfMatch, sp); err != nil {
		return domain.Space{}, err
	}
	old := sp.Assignment()
	if old == want {
		// A concurrent call set the same default first: answered as without effect.
		return sp, s.record(ctx, authorize.ActionSpaceOperatorSet, sp.ID, withNoChange(operatorAttrs(resourceSpace, sp.ID, want, want, cmd.Reason)))
	}
	inheriting, err := s.d.Store.LockInheritingChannels(ctx, sp.ID)
	if err != nil {
		return domain.Space{}, err
	}
	if err := s.resolve(ctx, a, req); err != nil {
		return domain.Space{}, err
	}
	if err := s.d.RightsChanges.AddAffected(ctx, old.AgentID, want.AgentID); err != nil {
		return domain.Space{}, err
	}
	sp.SetOperator(want)
	if err := s.d.Store.SaveSpaceOperator(ctx, sp); err != nil {
		return domain.Space{}, err
	}
	if err := s.touchInheriting(ctx, inheriting, sp.DefaultOperatorAgentID); err != nil {
		return domain.Space{}, err
	}
	if err := s.record(ctx, authorize.ActionSpaceOperatorSet, sp.ID, operatorAttrs(resourceSpace, sp.ID, old, want, cmd.Reason)); err != nil {
		return domain.Space{}, err
	}
	if err := s.emitSpace(ctx, authorize.EventSpaceUpdated, sp); err != nil {
		return domain.Space{}, err
	}
	for _, c := range inheriting {
		if err := s.emitChannel(ctx, authorize.EventChannelUpdated, c); err != nil {
			return domain.Space{}, err
		}
	}
	return sp, nil
}

// touchInheriting bumps the version of every inheriting channel of a space whose default operator
// became operator and resets the readiness of the bound forums among them: their effective
// operator changed, so the readiness came from the old one (M2-03 design note 7).
func (s *Service) touchInheriting(ctx context.Context, inheriting []ChannelRow, operator ids.ID) error {
	var bound []ids.ID
	for i := range inheriting {
		inheriting[i].Channel.Touch()
		inheriting[i].SpaceDefaultOperatorID = operator
		if err := s.d.Store.TouchChannel(ctx, inheriting[i].Channel); err != nil {
			return err
		}
		if inheriting[i].ProjectBound {
			bound = append(bound, inheriting[i].Channel.ID)
		}
	}
	return s.resetReadiness(ctx, bound)
}
