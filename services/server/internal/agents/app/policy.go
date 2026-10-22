// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"slices"
	"strconv"

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

// pointerHolder is the JSON pointer of the credential holder in the policy bodies.
const pointerHolder = "/credential_holder_id"

// holderInvalid is the refusal of a credential holder that is not an active person: the holder's
// own subscription runs the agent, so only a person who can act may hold it (10 section 1).
func holderInvalid() error {
	const detail = "the credential holder must be an active person"
	return errs.New(errs.CodeValidationFailed, detail).
		WithFields(errs.FieldError{Pointer: pointerHolder, Code: "not_active_person", Detail: detail})
}

// checkReason checks the reason of a policy body: CheckReason, and with limit also
// audit.reason_max_chars (M2-21 design note 2).
func (s *Service) checkReason(reason string, limit bool) error {
	if limit {
		return domain.CheckReasonMax(reason, s.cfg.ReasonMaxChars)
	}
	return domain.CheckReason(reason)
}

// checkHolder refuses a holder that is unknown, not a person or not active. It reads inside the
// ambient transaction, so the second unit of a change sees a holder disabled meanwhile.
func (s *Service) checkHolder(ctx context.Context, holder ids.ID) error {
	if holder.IsZero() {
		return nil
	}
	u, err := s.d.Directory.User(ctx, holder)
	if e, ok := errs.As(err); ok && e.Kind() == errs.KindNotFound {
		return holderInvalid()
	}
	if err != nil {
		return err
	}
	if u.Status != identity.StatusActive {
		return holderInvalid()
	}
	return nil
}

// policyAttrs are the audit details of a switch of one agent (10 section 14): old and new mode,
// old and new holder where there is one, the reason.
func policyAttrs(from, to domain.Policy, reason string) auditing.Attrs {
	attrs := auditing.Attrs{
		attrFromMode: auditing.Code(string(from.Mode)), attrToMode: auditing.Code(string(to.Mode)), attrReason: auditing.ReasonText(reason),
	}
	if !from.HolderID.IsZero() {
		attrs[attrFromHolder] = auditing.ID(from.HolderID)
	}
	if !to.HolderID.IsZero() {
		attrs[attrToHolder] = auditing.ID(to.HolderID)
	}
	return attrs
}

// change is the shared frame of the four single-agent changes with If-Match: a first unit
// authorizes act, checks the body with decide against the current record and records a call
// without effect; a second unit declared as the rights change act for the agent locks the record,
// checks If-Match and decide again under the lock and lets apply write.
type change struct {
	act    authorize.Action
	reason string
	match  agents.Precondition
	// decide validates the request against rec and reports whether it changes anything.
	decide func(ctx context.Context, rec domain.Agent) (bool, error)
	// apply changes rec and returns the audit details.
	apply func(rec *domain.Agent) (auditing.Attrs, error)
	// write, when set, replaces apply and the shared store, audit and event step: it writes the
	// locked rec itself (the switch to organization through applySwitchToOrganization, M2-21).
	write func(ctx context.Context, rec domain.Agent) (agents.Agent, error)
	// limitReason checks the reason against audit.reason_max_chars as well (M2-21 design note 2).
	limitReason bool
}

// run executes c for the agent id.
func (s *Service) run(ctx context.Context, a actor.Actor, id ids.ID, c change) (agents.Agent, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Agent{}, err
	}
	var (
		out     agents.Agent
		changes bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, changes, err = s.runCheck(ctx, a, id, c)
		return err
	})
	if err != nil || !changes {
		return out, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.runWrite(ctx, id, c)
		return err
	}, rightsChange(c.act, agentRef(id), id))
	if err == nil {
		s.presenceChanged(ctx, &out)
	}
	return out, err
}

// runCheck is the first unit of run: load, authorize, reason, If-Match and decide; a call
// without effect is recorded and answered with the current agent (changes false).
func (s *Service) runCheck(ctx context.Context, a actor.Actor, id ids.ID, c change) (agents.Agent, bool, error) {
	rec, err := s.load(ctx, a, c.act, id)
	if err != nil {
		return agents.Agent{}, false, err
	}
	if err := s.checkReason(c.reason, c.limitReason); err != nil {
		return agents.Agent{}, false, err
	}
	if err := s.precondition(ctx, c.match, rec); err != nil {
		return agents.Agent{}, false, err
	}
	if changes, err := c.decide(ctx, rec); err != nil || changes {
		return agents.Agent{}, changes, err
	}
	return s.unchanged(ctx, c, rec)
}

// runWrite is the second unit of run, declared as the rights change: the checks again under the
// row lock, then apply, store, audit and agent.updated.
func (s *Service) runWrite(ctx context.Context, id ids.ID, c change) (agents.Agent, error) {
	rec, err := s.lock(ctx, id)
	if err != nil {
		return agents.Agent{}, err
	}
	if err := s.precondition(ctx, c.match, rec); err != nil {
		return agents.Agent{}, err
	}
	changes, err := c.decide(ctx, rec)
	if err != nil {
		return agents.Agent{}, err
	}
	if !changes {
		// A concurrent change reached the same state first (06 section 2 "No effect").
		out, _, err := s.unchanged(ctx, c, rec)
		return out, err
	}
	if c.write != nil {
		return c.write(ctx, rec)
	}
	attrs, err := c.apply(&rec)
	if err != nil {
		return agents.Agent{}, err
	}
	return s.changedAndEmit(ctx, c.act, rec, attrs)
}

// unchanged records the call without effect of c and returns the current agent.
func (s *Service) unchanged(ctx context.Context, c change, rec domain.Agent) (agents.Agent, bool, error) {
	out, err := s.view(ctx, rec)
	if err != nil {
		return agents.Agent{}, false, err
	}
	return out, false, s.noChange(ctx, c.act, rec.ID, c.reason)
}

// SetBillingMode implements agents.Agents (10 section 9): to organization always, the holder
// becomes NULL and a given holder is refused; to personal only with an active person as holder,
// and the rights hook rolls the switch back with 409 i1_violation unless I1 holds afterwards. The
// same mode and holder answer the current agent without a write.
func (s *Service) SetBillingMode(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.BillingModeCmd) (agents.Agent, error) {
	target := domain.Policy{Mode: domain.BillingMode(cmd.BillingMode), HolderID: cmd.CredentialHolderID}
	c := change{
		act: authorize.ActionAgentBillingModeSet, reason: cmd.Reason, match: cmd.IfMatch, limitReason: true,
		decide: func(ctx context.Context, rec domain.Agent) (bool, error) {
			changed, err := rec.Policy.To(target)
			if err != nil || !changed {
				return false, err
			}
			return true, s.checkHolder(ctx, target.HolderID)
		},
		apply: func(rec *domain.Agent) (auditing.Attrs, error) {
			from := rec.Policy
			if _, err := rec.SetPolicy(target, a.ID, s.d.Clock.Now()); err != nil {
				return nil, err
			}
			return policyAttrs(from, target, cmd.Reason), nil
		},
	}
	if target == organization {
		// The shared apply step of every switch to organization (D-M2-21-5).
		c.write = func(ctx context.Context, rec domain.Agent) (agents.Agent, error) {
			from := rec.Policy
			v, err := s.applySwitchToOrganization(ctx, a.ID, &rec)
			if err != nil {
				return agents.Agent{}, err
			}
			return v, s.record(ctx, authorize.ActionAgentBillingModeSet, rec.ID, policyAttrs(from, organization, cmd.Reason))
		}
	}
	return s.run(ctx, a, id, c)
}

// SetCredentialHolder implements agents.Agents: only for a personal agent (else 409 conflict),
// the holder an active person; I1 is checked for the agent.
func (s *Service) SetCredentialHolder(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.CredentialHolderCmd) (agents.Agent, error) {
	return s.run(ctx, a, id, change{
		act: authorize.ActionAgentCredentialHolderSet, reason: cmd.Reason, match: cmd.IfMatch, limitReason: true,
		decide: func(ctx context.Context, rec domain.Agent) (bool, error) {
			probe := rec
			changed, err := probe.SetHolder(cmd.CredentialHolderID)
			if err != nil || !changed {
				return false, err
			}
			return true, s.checkHolder(ctx, cmd.CredentialHolderID)
		},
		apply: func(rec *domain.Agent) (auditing.Attrs, error) {
			from := rec.Policy.HolderID
			if _, err := rec.SetHolder(cmd.CredentialHolderID); err != nil {
				return nil, err
			}
			return auditing.Attrs{
				attrFromHolder: auditing.ID(from), attrToHolder: auditing.ID(cmd.CredentialHolderID), attrReason: auditing.ReasonText(cmd.Reason),
			}, nil
		},
	})
}

// SetToolClass implements agents.Agents (07 section 2.11): the target class with reason; SR2
// reads the effective class, so the change is a rights change for the agent.
func (s *Service) SetToolClass(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.ToolClassCmd) (agents.Agent, error) {
	return s.run(ctx, a, id, change{
		act: authorize.ActionAgentToolClassSet, reason: cmd.Reason, match: cmd.IfMatch,
		decide: func(_ context.Context, rec domain.Agent) (bool, error) {
			c, err := domain.ParseToolClass(cmd.ToolClass)
			if err != nil {
				return false, err
			}
			return c != rec.ToolClass, nil
		},
		apply: func(rec *domain.Agent) (auditing.Attrs, error) {
			from := rec.ToolClass
			rec.SetToolClass(domain.ToolClass(cmd.ToolClass))
			return auditing.Attrs{
				attrFromToolClass: auditing.Code(string(from)), attrToToolClass: auditing.Code(cmd.ToolClass), attrReason: auditing.ReasonText(cmd.Reason),
			}, nil
		},
	})
}

// SetBillingModeAll implements agents.Agents (10 section 9.1, BR-08). A first unit authorizes
// agent.billing_mode.set_all on the instance and checks the body: personal needs agent_ids and an
// active person as holder, organization takes no holder, an unknown id is 400 on its index. A
// second unit declared as the rights change locks the rows in ascending id (docs/DECISIONS.md
// M1-51), switches every agent whose policy differs from the target and writes one audit entry
// with the list; the rights hook checks I1 for all of them at once, so one violator rolls back
// every switch. Without agent_ids organization affects every agent not yet on organization,
// disabled agents included (the emergency switch).
func (s *Service) SetBillingModeAll(ctx context.Context, a actor.Actor, cmd agents.BillingModeAllCmd) ([]ids.ID, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	target := domain.Policy{Mode: domain.BillingMode(cmd.BillingMode), HolderID: cmd.CredentialHolderID}
	list := uniqueSorted(cmd.AgentIDs)
	pending := false
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		pending, err = s.checkAll(ctx, a, cmd, target, list)
		return err
	})
	if err != nil || !pending {
		return []ids.ID{}, err
	}
	var changed []ids.ID
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		changed, err = s.switchAll(ctx, a, cmd, target, list)
		return err
	}, rightsChange(authorize.ActionAgentBillingModeSetAll, txn.Ref{Kind: resourceInstance}, list...))
	if err == nil {
		s.notifyPresence(ctx, changed)
	}
	return changed, err
}

// checkAll is the first unit of SetBillingModeAll: authorize, the body rules and whether any
// agent would change; without one it records the call without effect (pending false).
func (s *Service) checkAll(ctx context.Context, a actor.Actor, cmd agents.BillingModeAllCmd, target domain.Policy, list []ids.ID) (bool, error) {
	if err := s.check(ctx, a, authorize.ActionAgentBillingModeSetAll, instanceResource); err != nil {
		return false, err
	}
	if err := domain.CheckReasonMax(cmd.Reason, s.cfg.ReasonMaxChars); err != nil {
		return false, err
	}
	if _, err := (domain.Policy{}).To(target); err != nil {
		return false, err
	}
	if target.Mode == domain.Personal && len(list) == 0 {
		const detail = "personal needs an explicit list of agents"
		return false, errs.New(errs.CodeValidationFailed, detail).
			WithFields(errs.FieldError{Pointer: "/agent_ids", Code: domain.FieldRequired, Detail: detail})
	}
	if err := s.checkHolder(ctx, target.HolderID); err != nil {
		return false, err
	}
	if err := s.knownAgents(ctx, cmd.AgentIDs); err != nil {
		return false, err
	}
	if pending, err := s.wouldSwitch(ctx, cmd, target, list); err != nil || pending {
		return pending, err
	}
	return false, s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAgentBillingModeSetAll), Decision: auditing.DecisionAllow, ResourceKind: resourceInstance,
		Attrs: auditing.Attrs{attrToMode: auditing.Code(string(target.Mode)), attrReason: auditing.ReasonText(cmd.Reason), attrNoChange: auditing.Bool(true)},
	})
}

// wouldSwitch reports whether any agent of the request has a policy other than target: the listed
// agents, or without a list every personal agent.
func (s *Service) wouldSwitch(ctx context.Context, cmd agents.BillingModeAllCmd, target domain.Policy, list []ids.ID) (bool, error) {
	var rows []domain.Agent
	if cmd.AgentIDsGiven {
		for _, id := range list {
			rec, _, err := s.d.Store.Agent(ctx, id)
			if err != nil {
				return false, err
			}
			rows = append(rows, rec)
		}
	} else {
		var err error
		if rows, err = s.d.Store.PersonalAgents(ctx); err != nil {
			return false, err
		}
	}
	for _, rec := range rows {
		if rec.Policy != target {
			return true, nil
		}
	}
	return false, nil
}

// knownAgents refuses the first id of the request that is no agent, with its index.
func (s *Service) knownAgents(ctx context.Context, list []ids.ID) error {
	for i, id := range list {
		_, found, err := s.d.Store.Agent(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			const detail = "no agent has this id"
			return errs.New(errs.CodeValidationFailed, detail).
				WithFields(errs.FieldError{Pointer: "/agent_ids/" + strconv.Itoa(i), Code: "unknown_agent", Detail: detail})
		}
	}
	return nil
}

// switchAll is the second unit of SetBillingModeAll.
func (s *Service) switchAll(ctx context.Context, a actor.Actor, cmd agents.BillingModeAllCmd, target domain.Policy, list []ids.ID) ([]ids.ID, error) {
	if err := s.checkHolder(ctx, target.HolderID); err != nil {
		return nil, err
	}
	rows, err := s.lockForAll(ctx, cmd, list)
	if err != nil {
		return nil, err
	}
	var pending []domain.Agent
	var pendingIDs []ids.ID
	for _, rec := range rows {
		if rec.Policy != target {
			pending, pendingIDs = append(pending, rec), append(pendingIDs, rec.ID)
		}
	}
	if len(pending) > 0 && !cmd.AgentIDsGiven {
		// Before the rows change, so their visible targets before the switch are recorded.
		if err := s.d.Rights.AddAffected(ctx, pendingIDs...); err != nil {
			return nil, err
		}
	}
	sw := switched{}
	for i := range pending {
		rec := pending[i]
		from := rec.Policy
		if err := s.switchOne(ctx, a.ID, &rec, target); err != nil {
			return nil, err
		}
		sw.add(rec, from)
	}
	return sw.ids, s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAgentBillingModeSetAll), Decision: auditing.DecisionAllow, ResourceKind: resourceInstance,
		Attrs: sw.attrs(target, cmd.Reason),
	})
}

// switchOne switches the locked rec of SetBillingModeAll to target: organization through the
// shared apply step (D-M2-21-5), personal with the same row write and event.
func (s *Service) switchOne(ctx context.Context, by ids.ID, rec *domain.Agent, target domain.Policy) error {
	if target == organization {
		_, err := s.applySwitchToOrganization(ctx, by, rec)
		return err
	}
	if _, err := rec.SetPolicy(target, by, s.d.Clock.Now()); err != nil {
		return err
	}
	_, err := s.storeAndEmit(ctx, *rec)
	return err
}

// lockForAll locks the rows of SetBillingModeAll in ascending id: the listed agents, or without a
// list every personal agent. A listed agent that vanished is refused on its index.
func (s *Service) lockForAll(ctx context.Context, cmd agents.BillingModeAllCmd, list []ids.ID) ([]domain.Agent, error) {
	if !cmd.AgentIDsGiven {
		return s.d.Store.LockPersonalAgents(ctx)
	}
	rows, err := s.d.Store.LockAgents(ctx, list)
	if err == nil && len(rows) != len(list) {
		err = s.knownAgents(ctx, cmd.AgentIDs)
	}
	return rows, err
}

// switched collects the agents SetBillingModeAll changed with their old values.
type switched struct {
	ids                            []ids.ID
	changed, fromModes, held, from []auditing.Value
}

// add records rec, whose policy was from.
func (sw *switched) add(rec domain.Agent, from domain.Policy) {
	sw.ids = append(sw.ids, rec.ID)
	sw.changed = append(sw.changed, auditing.ID(rec.ID))
	sw.fromModes = append(sw.fromModes, auditing.Code(string(from.Mode)))
	if !from.HolderID.IsZero() {
		sw.held = append(sw.held, auditing.ID(rec.ID))
		sw.from = append(sw.from, auditing.ID(from.HolderID))
	}
}

// attrs are the details of the one audit entry (10 section 14): the switched agents with their
// old modes, the old holders of those that had one, the target, the new holder and the reason.
func (sw switched) attrs(target domain.Policy, reason string) auditing.Attrs {
	attrs := auditing.Attrs{attrToMode: auditing.Code(string(target.Mode)), attrReason: auditing.ReasonText(reason)}
	if len(sw.ids) == 0 {
		attrs[attrNoChange] = auditing.Bool(true)
	} else {
		attrs[attrAgentIDs], attrs[attrFromModes] = auditing.List(sw.changed...), auditing.List(sw.fromModes...)
	}
	if len(sw.held) > 0 {
		attrs[attrHeldAgentIDs], attrs[attrFromHolders] = auditing.List(sw.held...), auditing.List(sw.from...)
	}
	if !target.HolderID.IsZero() {
		attrs[attrToHolder] = auditing.ID(target.HolderID)
	}
	return attrs
}

// uniqueSorted returns list ascending without duplicates.
func uniqueSorted(list []ids.ID) []ids.ID {
	out := slices.Clone(list)
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return slices.Compact(out)
}
