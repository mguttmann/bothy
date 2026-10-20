// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Heartbeat implements agents.Presence, the use case of brief M2-20 note 2, in this order (the
// rate limit rl.agent_runtime ran before, in the chain or the gateway):
//
//  1. authorize(actor, agent.heartbeat, the actor's own agent): people and apps get forbidden
//     subject_kind_not_allowed from AgentSelf, audited by authz; the body carries no agent id, so
//     an agent reports only for itself;
//  2. decode and validate (domain.DecodeHeartbeat): one validation_failed with every field error;
//  3. read the configured and stored values, every heartbeat, so a value changed in another
//     process counts at once;
//  4. project readiness, only when the list changed since this process applied it, in its own
//     unit of work through chat (D-M2-20-10);
//  5. a change of the reported mode or class: lock the row, compare again, store at once and
//     audit agent.heartbeat in the same transaction (D-M2-20-3);
//  6. update the entry (dirty unless step 5 wrote it), reassess, re-arm the deadline;
//  7. publish agent.presence_changed when state or reported mode changed (D-M2-20-4).
//
// It writes no agents.version, no agent.updated, no permission_version and no event_log row.
//
// Locks (docs/DECISIONS.md D-M2-21-11): the heartbeats of one agent run one after the other under
// the entry's beat lock, which only Heartbeat takes and never inside a unit of work (the route is
// exempt from Idempotency-Key). Steps 3 to 5 wait for the database, so they run under the beat
// lock only; the entry lock guards the fields and is never held across a database call, because
// its other holders (reads for agent.updated, Changed, the gauges, the flush, the timer) may run
// in a unit that holds the agents or a channel row a heartbeat waits for.
func (t *Tracker) Heartbeat(ctx context.Context, body json.RawMessage) (agents.PresenceStatus, error) {
	a, ok := actor.FromContext(ctx)
	if !ok {
		a = actor.Anonymous()
	}
	d, err := t.d.Authorizer.Authorize(ctx, a, authorize.ActionAgentHeartbeat, agentResource(a.ID))
	if err != nil {
		return agents.PresenceStatus{}, err
	}
	if err := d.Err(); err != nil {
		return agents.PresenceStatus{}, err
	}
	report, fes := domain.DecodeHeartbeat(body)
	if len(fes) > 0 {
		return agents.PresenceStatus{}, errs.New(errs.CodeValidationFailed, "The heartbeat is not valid.").WithFields(fes...)
	}
	e, err := t.lookup(ctx, a.ID)
	if err != nil {
		return agents.PresenceStatus{}, internal(err, "agents: the principal of a heartbeat")
	}
	e.beat.Lock()
	defer e.beat.Unlock()
	e.mu.Lock()
	changes := e.changes
	e.mu.Unlock()
	row, found, err := t.d.Store.GetPresenceRow(ctx, a.ID)
	if err != nil || !found {
		return agents.PresenceStatus{}, internal(err, "agents: the record of a heartbeat")
	}
	now := t.d.Clock.Now()
	e.mu.Lock()
	if !e.loaded {
		t.fill(e, row, now)
	}
	e.mu.Unlock()
	if err := t.applyReadiness(ctx, e, report); err != nil {
		return agents.PresenceStatus{}, err
	}
	wrote, err := t.storeChange(ctx, a.ID, row, report, now)
	if err != nil {
		return agents.PresenceStatus{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	// A Changed that ran since the read above took newer configured values.
	if e.changes == changes {
		e.target = row.Target
	}
	e.report, e.last, e.state = report, now, report.State
	e.dirty = !wrote
	e.deadline = now.Add(t.cfg.PresenceTimeout)
	t.reassess(e, now)
	if !t.stopped.Load() {
		t.arm(e, now)
	}
	by := a.ID
	t.publish(ctx, e, &by)
	return t.status(e, now), nil
}

// internal wraps a failure of the heartbeat as internal_error; a kernel/errs error of kind
// internal passes.
func internal(err error, what string) error {
	if err == nil {
		return errs.New(errs.CodeInternalError, what)
	}
	return errs.Wrap(errs.CodeInternalError, err, what)
}

// applyReadiness applies project_readiness of r through chat when the member is present and the
// list differs from the last one this process applied for the agent (the caller holds e.beat,
// which guards the applied list). Any
// error of the port fails the heartbeat with internal_error, except the validation error of a
// duplicate channel, which domain.DecodeHeartbeat already refuses.
func (t *Tracker) applyReadiness(ctx context.Context, e *entry, r domain.Report) error {
	if !r.ReadinessGiven || (e.readinessSet && slices.Equal(e.readiness, r.Readiness)) {
		return nil
	}
	_, port := t.bound()
	if port == nil {
		return errs.New(errs.CodeInternalError, "agents: no readiness port bound")
	}
	reports := make([]chat.ProjectReadiness, 0, len(r.Readiness))
	for _, rd := range r.Readiness {
		reports = append(reports, chat.ProjectReadiness{ChannelID: rd.ChannelID, Ready: rd.Ready})
	}
	if err := port.ApplyProjectReadiness(ctx, e.id, reports); err != nil {
		if errs.KindOf(err) == errs.KindBadRequest {
			return err
		}
		return internal(err, "agents: applying project readiness")
	}
	e.readiness, e.readinessSet = slices.Clone(r.Readiness), true
	return nil
}

// storeChange stores and audits a change of the reported mode or class at once (D-M2-20-3): one
// unit of work locks the row, compares again and, when still different, writes both columns with
// the whole reported state and records agent.heartbeat allow with the old and new values. wrote
// is false when nothing changed or another process stored the same values first.
func (t *Tracker) storeChange(ctx context.Context, id ids.ID, row domain.PresenceRow, r domain.Report, now time.Time) (bool, error) {
	if _, changed := domain.ReportedChange(row.Stored.Reported(), r.Reported()); !changed {
		return false, nil
	}
	wrote := false
	err := t.d.UoW.Do(ctx, func(ctx context.Context) error {
		wrote = false
		locked, found, err := t.d.Store.LockPresenceRow(ctx, id)
		if err != nil || !found {
			return internal(err, "agents: locking the record of a heartbeat")
		}
		details, changed := domain.ReportedChange(locked.Stored.Reported(), r.Reported())
		if !changed {
			return nil
		}
		if err := t.d.Store.WriteReportedChange(ctx, PresenceWrite{ID: id, State: r.State, Report: r, LastHeartbeatAt: now}); err != nil {
			return err
		}
		wrote = true
		return t.d.Audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionAgentHeartbeat), Decision: auditing.DecisionAllow,
			ResourceKind: resourceAgent, ResourceID: id, Attrs: auditAttrs(details)})
	})
	if err != nil {
		return false, internal(err, "agents: storing a change of the reported mode")
	}
	return wrote, nil
}

// auditAttrs converts the details of a reported change; every value is a code (SEC-AUD-5).
func auditAttrs(d domain.AuditDetails) auditing.Attrs {
	attrs := auditing.Attrs{}
	for k, v := range d.Map() {
		attrs[k] = auditing.Code(v)
	}
	return attrs
}
